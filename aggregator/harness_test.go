package aggregator

import (
	"context"
	"encoding/json"
	"io"
	"math/big"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coinbase/rosetta-sdk-go/asserter"
	"github.com/coinbase/rosetta-sdk-go/client"
	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/multiversx/mx-chain-rosetta/aggregator/backend"
	"github.com/multiversx/mx-chain-rosetta/aggregator/config"
	"github.com/multiversx/mx-chain-rosetta/aggregator/index"
	"github.com/multiversx/mx-chain-rosetta/aggregator/metachain"
	"github.com/multiversx/mx-chain-rosetta/aggregator/mock"
	"github.com/stretchr/testify/require"
)

// harness runs the aggregator (served over HTTP, on the loopback interface) on top of a simulated network.
// Requests are made with the Rosetta client of rosetta-sdk-go, and responses are validated with its (client-side) asserter,
// as a Rosetta consumer would do. Nothing talks to a real network.
type harness struct {
	t         *testing.T
	simulator *mock.Simulator
	config    *config.Config
	app       *App
	server    *httptest.Server
	client    *client.APIClient
	network   *types.NetworkIdentifier

	responsesAsserter *asserter.Asserter
}

type harnessSetup struct {
	config     *config.Config
	backends   *backend.Set
	metaSource metachain.Source
	store      index.Store
}

func newConfigForSimulator(simulator *mock.Simulator) *config.Config {
	cfg := &config.Config{}
	cfg.Network.Blockchain = simulator.Network().Blockchain
	cfg.Network.Name = simulator.Network().Network
	cfg.Network.NumShards = simulator.NumShards()
	cfg.Network.GenesisBlockHash = simulator.GenesisBlockHash()
	cfg.Network.GenesisTimestamp = mock.GenesisTimestamp
	cfg.Metachain.ObserverUrls = []string{"http://mock-metachain-observer"}

	for shard := uint32(0); shard < simulator.NumShards(); shard++ {
		cfg.Shards = append(cfg.Shards, config.ShardConfig{ID: shard, RosettaUrls: []string{"http://mock-shard-instance"}})
	}

	cfg.ApplyDefaults()
	return cfg
}

// newHarness creates and starts the aggregator on top of the simulated network. "adjust" can alter the setup before the application is created.
func newHarness(t *testing.T, simulator *mock.Simulator, adjust ...func(setup *harnessSetup)) *harness {
	h, err := tryNewHarness(t, simulator, adjust...)
	require.NoError(t, err)
	return h
}

func tryNewHarness(t *testing.T, simulator *mock.Simulator, adjust ...func(setup *harnessSetup)) (*harness, error) {
	setup := &harnessSetup{
		config:     newConfigForSimulator(simulator),
		backends:   simulator.Backends(),
		metaSource: simulator.MetaSource(),
		store:      index.NewMemoryStore(),
	}

	for _, fn := range adjust {
		fn(setup)
	}

	require.NoError(t, setup.config.Validate())

	app, err := NewApp(ArgsNewApp{
		Config:     setup.config,
		Version:    "v0.0.0-test",
		Backends:   setup.backends,
		MetaSource: setup.metaSource,
		Store:      setup.store,
	})
	require.NoError(t, err)

	err = app.Start(context.Background())
	if err != nil {
		_ = app.Close()
		return nil, err
	}

	server := httptest.NewServer(app.Handler())
	t.Cleanup(func() {
		server.Close()
		_ = app.Close()
	})

	return &harness{
		t:         t,
		simulator: simulator,
		config:    setup.config,
		app:       app,
		server:    server,
		client:    client.NewAPIClient(client.NewConfiguration(server.URL, "test", server.Client())),
		network: &types.NetworkIdentifier{
			Blockchain: setup.config.Network.Blockchain,
			Network:    setup.config.Network.Name,
		},
	}, nil
}

// step performs one synchronization step (as done periodically by the running application)
func (h *harness) step() error {
	return h.app.Step(context.Background())
}

// sync performs synchronization steps until the tip of the unified chain reaches the last metablock of the simulated network
func (h *harness) sync() {
	lastMetaNonce := uint64(len(h.simulator.MetaBlocks()) - 1)

	for i := 0; i < 20; i++ {
		require.NoError(h.t, h.step())

		if h.tip() == lastMetaNonce {
			return
		}
	}

	require.Fail(h.t, "the unified chain did not reach the last metablock", "tip = %d, last metablock = %d", h.tip(), lastMetaNonce)
}

func (h *harness) tip() uint64 {
	snapshot, err := h.app.chain.Snapshot()
	require.NoError(h.t, err)
	return snapshot.Tip
}

func (h *harness) status() (*types.NetworkStatusResponse, *types.Error) {
	response, rosettaErr, _ := h.client.NetworkAPI.NetworkStatus(context.Background(), &types.NetworkRequest{NetworkIdentifier: h.network})
	return response, rosettaErr
}

func (h *harness) options() (*types.NetworkOptionsResponse, *types.Error) {
	response, rosettaErr, _ := h.client.NetworkAPI.NetworkOptions(context.Background(), &types.NetworkRequest{NetworkIdentifier: h.network})
	return response, rosettaErr
}

// getAsserter returns an asserter that validates responses the way Rosetta consumers (e.g. the Mesh CLI) do
func (h *harness) getAsserter() *asserter.Asserter {
	if h.responsesAsserter != nil {
		return h.responsesAsserter
	}

	status, rosettaErr := h.status()
	require.Nil(h.t, rosettaErr)
	options, rosettaErr := h.options()
	require.Nil(h.t, rosettaErr)

	// This also validates the responses of /network/status and /network/options.
	responsesAsserter, err := asserter.NewClientWithResponses(h.network, status, options, "")
	require.NoError(h.t, err)

	h.responsesAsserter = responsesAsserter
	return responsesAsserter
}

func (h *harness) tryGetBlock(identifier *types.PartialBlockIdentifier) (*types.Block, *types.Error) {
	response, rosettaErr, err := h.client.BlockAPI.Block(context.Background(), &types.BlockRequest{
		NetworkIdentifier: h.network,
		BlockIdentifier:   identifier,
	})
	if rosettaErr != nil {
		return nil, rosettaErr
	}

	require.NoError(h.t, err)
	require.NotNil(h.t, response.Block)
	require.NoError(h.t, h.getAsserter().Block(response.Block), "block %s is not a valid Rosetta block", types.PrintStruct(identifier))
	return response.Block, nil
}

func (h *harness) tryGetBlockByIndex(blockIndex int64) (*types.Block, *types.Error) {
	return h.tryGetBlock(&types.PartialBlockIdentifier{Index: &blockIndex})
}

func (h *harness) getBlock(blockIndex int64) *types.Block {
	block, rosettaErr := h.tryGetBlockByIndex(blockIndex)
	require.Nil(h.t, rosettaErr, "block %d: %s", blockIndex, types.PrintStruct(rosettaErr))
	return block
}

func (h *harness) tryGetBalance(address string, identifier *types.PartialBlockIdentifier) (*types.AccountBalanceResponse, *types.Error) {
	response, rosettaErr, err := h.client.AccountAPI.AccountBalance(context.Background(), &types.AccountBalanceRequest{
		NetworkIdentifier: h.network,
		AccountIdentifier: &types.AccountIdentifier{Address: address},
		BlockIdentifier:   identifier,
	})
	if rosettaErr != nil {
		return nil, rosettaErr
	}

	require.NoError(h.t, err)
	return response, nil
}

// getBalance returns the (native) balance of an account at a unified block
func (h *harness) getBalance(address string, blockIndex int64) *big.Int {
	response, rosettaErr := h.tryGetBalance(address, &types.PartialBlockIdentifier{Index: &blockIndex})
	require.Nil(h.t, rosettaErr, "balance of %s at block %d: %s", address, blockIndex, types.PrintStruct(rosettaErr))
	require.Equal(h.t, blockIndex, response.BlockIdentifier.Index)
	require.Len(h.t, response.Balances, 1)

	balance, ok := big.NewInt(0).SetString(response.Balances[0].Value, 10)
	require.True(h.t, ok)
	return balance
}

func (h *harness) getHealth() (*healthResponse, int) {
	httpResponse, err := h.server.Client().Get(h.server.URL + "/health")
	require.NoError(h.t, err)

	defer func() {
		_ = httpResponse.Body.Close()
	}()

	health := &healthResponse{}
	require.NoError(h.t, json.NewDecoder(httpResponse.Body).Decode(health))
	return health, httpResponse.StatusCode
}

// post sends a raw request (used for requests that the typed client cannot produce, e.g. malformed ones)
func (h *harness) post(path string, body string) (int, string) {
	httpResponse, err := h.server.Client().Post(h.server.URL+path, "application/json", strings.NewReader(body))
	require.NoError(h.t, err)

	defer func() {
		_ = httpResponse.Body.Close()
	}()

	content, err := io.ReadAll(httpResponse.Body)
	require.NoError(h.t, err)
	return httpResponse.StatusCode, string(content)
}

// sumSuccessfulOperations returns, per account, the sum of the amounts of the successful operations of a block
func sumSuccessfulOperations(t *testing.T, block *types.Block) map[string]*big.Int {
	deltas := make(map[string]*big.Int)

	for _, tx := range block.Transactions {
		for _, operation := range tx.Operations {
			require.NotNil(t, operation.Status)
			if *operation.Status != mock.OpStatusSuccess {
				continue
			}

			amount, ok := big.NewInt(0).SetString(operation.Amount.Value, 10)
			require.True(t, ok)

			address := operation.Account.Address
			if _, exists := deltas[address]; !exists {
				deltas[address] = big.NewInt(0)
			}

			deltas[address].Add(deltas[address], amount)
		}
	}

	return deltas
}
