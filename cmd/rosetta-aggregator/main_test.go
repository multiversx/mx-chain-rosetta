package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/coinbase/rosetta-sdk-go/client"
	"github.com/coinbase/rosetta-sdk-go/types"
	logger "github.com/multiversx/mx-chain-logger-go"
	"github.com/multiversx/mx-chain-rosetta/aggregator/config"
	"github.com/multiversx/mx-chain-rosetta/aggregator/mock"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	_ = logger.SetLogLevel("*:NONE")
	os.Exit(m.Run())
}

func TestExampleConfigIsValid(t *testing.T) {
	cfg, err := config.LoadFromFile("config.example.toml")
	require.NoError(t, err)

	require.Equal(t, "mainnet", cfg.Network.Name)
	require.Equal(t, []uint32{0, 1, 2}, cfg.ShardIDs())
	require.False(t, cfg.Server.Offline)
	require.True(t, cfg.IsForwardLinksEnabled())
}

// TestCreateApp_FromConfigurationFile starts the application as the binary does (configuration file, HTTP clients, index on disk),
// against a simulated network served over HTTP on the loopback interface: fake shard instances and a fake metachain observer.
func TestCreateApp_FromConfigurationFile(t *testing.T) {
	ctx := context.Background()

	simulator := mock.NewSimulator(mock.ArgsNewSimulator{NumShards: 3, NumAccountsPerShard: 2, Seed: 1})
	simulator.RunRounds(30)
	simulator.Settle()

	metaObserver := httptest.NewServer(mock.NewObserverHandler(simulator.MetaSource()))
	defer metaObserver.Close()

	shardSections := make([]string, 0)
	for shard := uint32(0); shard < 3; shard++ {
		handler, err := mock.NewShardInstanceHandler(simulator.Backend(shard))
		require.NoError(t, err)

		shardInstance := httptest.NewServer(handler)
		defer shardInstance.Close()

		shardSections = append(shardSections, fmt.Sprintf("[[shard]]\nid = %d\nrosetta_urls = [\"%s\"]\n", shard, shardInstance.URL))
	}

	directory := t.TempDir()
	configPath := filepath.Join(directory, "aggregator.toml")
	configContent := fmt.Sprintf(`
[network]
name = "%s"
num_shards = 3
genesis_block_hash = "%s"
genesis_timestamp = %d

[metachain]
observer_urls = ["%s"]

%s

[index]
path = "%s"

[limits]
backend_timeout_seconds = 10
`, simulator.Network().Network, simulator.GenesisBlockHash(), mock.GenesisTimestamp, metaObserver.URL, strings.Join(shardSections, "\n"), filepath.Join(directory, "index"))

	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0o600))

	cfg, err := config.LoadFromFile(configPath)
	require.NoError(t, err)

	app, err := createApp(cfg)
	require.NoError(t, err)

	defer func() {
		_ = app.Close()
	}()

	require.NoError(t, app.Start(ctx))

	lastMetaNonce := int64(len(simulator.MetaBlocks()) - 1)

	aggregatorServer := httptest.NewServer(app.Handler())
	defer aggregatorServer.Close()

	rosettaClient := client.NewAPIClient(client.NewConfiguration(aggregatorServer.URL, "test", aggregatorServer.Client()))
	network := simulator.Network()

	// The first synchronization step (performed at start) is sufficient to reach the tip.
	status, rosettaErr, err := rosettaClient.NetworkAPI.NetworkStatus(ctx, &types.NetworkRequest{NetworkIdentifier: network})
	require.NoError(t, err)
	require.Nil(t, rosettaErr)
	require.Equal(t, lastMetaNonce, status.CurrentBlockIdentifier.Index)
	require.True(t, *status.SyncStatus.Synced)

	// Every block of the unified chain can be fetched, and the last one holds what the metachain has notarized last.
	numTransactions := 0
	for blockIndex := int64(0); blockIndex <= lastMetaNonce; blockIndex++ {
		index := blockIndex
		response, rosettaErr, err := rosettaClient.BlockAPI.Block(ctx, &types.BlockRequest{
			NetworkIdentifier: network,
			BlockIdentifier:   &types.PartialBlockIdentifier{Index: &index},
		})
		require.NoError(t, err)
		require.Nil(t, rosettaErr)
		require.Equal(t, simulator.MetaBlocks()[blockIndex].Hash, response.Block.BlockIdentifier.Hash)

		numTransactions += len(response.Block.Transactions)
	}
	require.Greater(t, numTransactions, 0)

	account := simulator.AccountsOfShard(2)[0]
	balance, rosettaErr, err := rosettaClient.AccountAPI.AccountBalance(ctx, &types.AccountBalanceRequest{
		NetworkIdentifier: network,
		AccountIdentifier: &types.AccountIdentifier{Address: account.Address},
	})
	require.NoError(t, err)
	require.Nil(t, rosettaErr)

	expectedBalance, _ := simulator.Chain(2).GetBalance(account.Address, simulator.Chain(2).LatestIndex())
	require.Equal(t, expectedBalance.String(), balance.Balances[0].Value)
	require.Equal(t, lastMetaNonce, balance.BlockIdentifier.Index)

	httpResponse, err := aggregatorServer.Client().Get(aggregatorServer.URL + "/health")
	require.NoError(t, err)
	defer func() {
		_ = httpResponse.Body.Close()
	}()

	require.Equal(t, http.StatusOK, httpResponse.StatusCode)

	health := make(map[string]interface{})
	require.NoError(t, json.NewDecoder(httpResponse.Body).Decode(&health))
	require.Equal(t, true, health["ready"])
	require.Equal(t, true, health["synced"])
	require.Equal(t, float64(lastMetaNonce), health["tip"])

	// The index was persisted at the configured path.
	entries, err := os.ReadDir(filepath.Join(directory, "index"))
	require.NoError(t, err)
	require.NotEmpty(t, entries)
}

func TestCreateApp_Errors(t *testing.T) {
	cfg, err := config.LoadFromFile("config.example.toml")
	require.NoError(t, err)

	// The index cannot be created (the path is a file).
	filePath := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(filePath, []byte("not a folder"), 0o600))

	cfg.Index.Path = filePath
	_, err = createApp(cfg)
	require.ErrorContains(t, err, "cannot open the notarization index")

	// A shard without URLs.
	cfg.Index.Path = ""
	cfg.Shards[1].RosettaUrls = []string{" "}
	_, err = createApp(cfg)
	require.ErrorContains(t, err, "shard 1")
}

func TestCreateApp_InMemoryIndex(t *testing.T) {
	cfg, err := config.LoadFromFile("config.example.toml")
	require.NoError(t, err)

	// Nothing is contacted until the application is started.
	cfg.Index.Path = ""
	app, err := createApp(cfg)
	require.NoError(t, err)
	require.NoError(t, app.Close())
}

// TestBinary_StartServeAndShutdown builds the binary and runs it as a separate process, against a simulated network
// served over HTTP on the loopback interface. It checks what cannot be checked in-process: the start-up, the handling of
// the termination signal, and what happens when the port is taken.
func TestBinary_StartServeAndShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the binary")
	}

	directory := t.TempDir()
	binaryPath := filepath.Join(directory, "rosetta-aggregator")

	build := exec.Command("go", "build", "-o", binaryPath, ".")
	output, err := build.CombinedOutput()
	require.NoError(t, err, string(output))

	simulator := mock.NewSimulator(mock.ArgsNewSimulator{NumShards: 3, NumAccountsPerShard: 2, Seed: 2})
	simulator.RunRounds(20)
	simulator.Settle()

	metaObserver := httptest.NewServer(mock.NewObserverHandler(simulator.MetaSource()))
	defer metaObserver.Close()

	shardSections := make([]string, 0)
	for shard := uint32(0); shard < 3; shard++ {
		handler, err := mock.NewShardInstanceHandler(simulator.Backend(shard))
		require.NoError(t, err)

		shardInstance := httptest.NewServer(handler)
		defer shardInstance.Close()

		shardSections = append(shardSections, fmt.Sprintf("[[shard]]\nid = %d\nrosetta_urls = [\"%s\"]\n", shard, shardInstance.URL))
	}

	// Find a free port.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())

	writeConfig := func(name string, indexFolder string) string {
		content := fmt.Sprintf(`
[server]
host = "127.0.0.1"
port = %d

[network]
name = "%s"
num_shards = 3
genesis_block_hash = "%s"
genesis_timestamp = %d

[metachain]
observer_urls = ["%s"]

%s

[index]
path = "%s"

[limits]
sync_interval_milliseconds = 20
`, port, simulator.Network().Network, simulator.GenesisBlockHash(), mock.GenesisTimestamp, metaObserver.URL, strings.Join(shardSections, "\n"), filepath.Join(directory, indexFolder))

		path := filepath.Join(directory, name)
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
		return path
	}

	var processOutput bytes.Buffer
	process := exec.Command(binaryPath, "--config", writeConfig("first.toml", "index-first"), "--logs-folder", directory)
	process.Stdout = &processOutput
	process.Stderr = &processOutput
	require.NoError(t, process.Start())

	exited := make(chan error, 1)
	go func() {
		exited <- process.Wait()
	}()

	defer func() {
		_ = process.Process.Kill()
	}()

	baseUrl := fmt.Sprintf("http://127.0.0.1:%d", port)
	httpClient := &http.Client{Timeout: 5 * time.Second}

	// Wait for the process to serve.
	require.Eventually(t, func() bool {
		httpResponse, errGet := httpClient.Get(baseUrl + "/health")
		if errGet != nil {
			return false
		}

		_ = httpResponse.Body.Close()
		return httpResponse.StatusCode == http.StatusOK
	}, 30*time.Second, 50*time.Millisecond, "the process did not start serving")

	ctx := context.Background()
	rosettaClient := client.NewAPIClient(client.NewConfiguration(baseUrl, "test", httpClient))
	lastMetaNonce := int64(len(simulator.MetaBlocks()) - 1)

	status, rosettaErr, err := rosettaClient.NetworkAPI.NetworkStatus(ctx, &types.NetworkRequest{NetworkIdentifier: simulator.Network()})
	require.NoError(t, err)
	require.Nil(t, rosettaErr)
	require.Equal(t, lastMetaNonce, status.CurrentBlockIdentifier.Index)

	// The network advances: the process follows it (in the background).
	simulator.RunRounds(10)
	simulator.Settle()
	lastMetaNonce = int64(len(simulator.MetaBlocks()) - 1)

	require.Eventually(t, func() bool {
		status, rosettaErr, err = rosettaClient.NetworkAPI.NetworkStatus(ctx, &types.NetworkRequest{NetworkIdentifier: simulator.Network()})
		return err == nil && rosettaErr == nil && status.CurrentBlockIdentifier.Index == lastMetaNonce
	}, 30*time.Second, 20*time.Millisecond, "the process did not follow the network")

	response, rosettaErr, err := rosettaClient.BlockAPI.Block(ctx, &types.BlockRequest{
		NetworkIdentifier: simulator.Network(),
		BlockIdentifier:   &types.PartialBlockIdentifier{Index: &lastMetaNonce},
	})
	require.NoError(t, err)
	require.Nil(t, rosettaErr)
	require.Equal(t, simulator.MetaBlocks()[lastMetaNonce].Hash, response.Block.BlockIdentifier.Hash)

	// A second process, on the same port, stops with an error (instead of staying alive, serving nothing).
	second := exec.Command(binaryPath, "--config", writeConfig("second.toml", "index-second"), "--logs-folder", directory)
	secondOutput, err := second.CombinedOutput()
	require.Error(t, err)
	require.Contains(t, string(secondOutput), "cannot listen on")

	exitError := &exec.ExitError{}
	require.ErrorAs(t, err, &exitError)
	require.Equal(t, 1, exitError.ExitCode())

	// The termination signal (sent by Docker, systemd, Kubernetes) stops the process gracefully.
	require.NoError(t, process.Process.Signal(syscall.SIGTERM))

	select {
	case err = <-exited:
		require.NoError(t, err, "the process did not exit cleanly:\n%s", processOutput.String())
	case <-time.After(30 * time.Second):
		require.Fail(t, "the process did not stop upon SIGTERM")
	}

	require.Contains(t, processOutput.String(), "HTTP server stopped")

	// The port is released.
	_, err = httpClient.Get(baseUrl + "/health")
	require.Error(t, err)

	// A missing configuration file is reported.
	missing := exec.Command(binaryPath, "--config", filepath.Join(directory, "missing.toml"), "--logs-folder", directory)
	missingOutput, err := missing.CombinedOutput()
	require.Error(t, err)
	require.Contains(t, string(missingOutput), "cannot read configuration file")
}
