package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const validConfig = `
[server]
host = "127.0.0.1"
port = 9090

[network]
name = "mainnet"
num_shards = 3
genesis_block_hash = "cd229e4ad2753708e4bab01d7f249affe29441829524c9529e84d51b6d12f2a7"
genesis_timestamp = 1596117600

[metachain]
observer_urls = ["http://observer-meta:8080"]

[[shard]]
id = 0
rosetta_urls = ["http://rosetta-shard0:8091"]

[[shard]]
id = 1
rosetta_urls = ["http://rosetta-shard1:8091", "http://rosetta-shard1-replica:8091"]

[[shard]]
id = 2
rosetta_urls = ["http://rosetta-shard2:8091"]

[index]
path = "./aggregator-index"
start_epoch = 1800
`

func TestLoad_Valid(t *testing.T) {
	config, err := Load([]byte(validConfig))
	require.NoError(t, err)

	require.Equal(t, "127.0.0.1", config.Server.Host)
	require.Equal(t, 9090, config.Server.Port)
	require.False(t, config.Server.Offline)
	require.False(t, config.Server.EnableCors)

	require.Equal(t, "mainnet", config.Network.Name)
	require.Equal(t, uint32(3), config.Network.NumShards)
	require.False(t, config.Network.HandleContracts)
	require.Equal(t, int64(1596117600), config.Network.GenesisTimestamp)

	require.Equal(t, []uint32{0, 1, 2}, config.ShardIDs())
	require.Len(t, config.Shards[1].RosettaUrls, 2)
	require.Equal(t, "./aggregator-index", config.Index.Path)
	require.Equal(t, uint32(1800), *config.Index.StartEpoch)

	// Defaults.
	require.Equal(t, "MultiversX", config.Network.Blockchain)
	require.Equal(t, "erd", config.Network.AddressPrefix)
	require.Equal(t, 60*time.Second, config.GetBackendTimeout())
	require.Equal(t, 500*time.Millisecond, config.GetSyncInterval())
	require.Equal(t, 32, config.Limits.MaxConcurrentBackendRequests)
	require.Equal(t, 256, config.Limits.BlockCacheSize)
	require.Equal(t, int64(1<<20), config.Limits.MaxRequestBodyBytes)
	require.True(t, config.IsForwardLinksEnabled())
}

func TestLoad_Overrides(t *testing.T) {
	content := validConfig + `
[limits]
backend_timeout_seconds = 5
sync_interval_milliseconds = 100
max_concurrent_backend_requests = 4
block_cache_size = 16
max_request_body_bytes = 2048

[cross_shard]
forward_links = false
`
	config, err := Load([]byte(content))
	require.NoError(t, err)

	require.Equal(t, 5*time.Second, config.GetBackendTimeout())
	require.Equal(t, 100*time.Millisecond, config.GetSyncInterval())
	require.Equal(t, 4, config.Limits.MaxConcurrentBackendRequests)
	require.Equal(t, 16, config.Limits.BlockCacheSize)
	require.Equal(t, int64(2048), config.Limits.MaxRequestBodyBytes)
	require.False(t, config.IsForwardLinksEnabled())
}

func TestLoad_Invalid(t *testing.T) {
	replace := func(old string, new string) string {
		require.Contains(t, validConfig, old)
		return strings.Replace(validConfig, old, new, 1)
	}

	testCases := []struct {
		name          string
		content       string
		expectedError string
	}{
		{"not TOML", "this is not TOML", "cannot parse configuration"},
		{"unknown key (typo)", replace(`num_shards = 3`, "num_shards = 3\nnum_shard = 3"), "unknown keys"},
		{"unknown section", validConfig + "\n[indexx]\npath = \"foo\"\n", "unknown keys"},
		{"port out of range", replace(`port = 9090`, `port = 70000`), "server.port"},
		{"missing network name", replace(`name = "mainnet"`, ``), "network.name"},
		{"no shards", replace(`num_shards = 3`, `num_shards = 0`), "network.num_shards"},
		{"bad genesis hash", replace(`cd229e4ad2753708e4bab01d7f249affe29441829524c9529e84d51b6d12f2a7`, `cd229e`), "network.genesis_block_hash"},
		{"genesis hash is not hexadecimal", replace(`cd229e4ad2753708e4bab01d7f249affe29441829524c9529e84d51b6d12f2a7`, strings.Repeat("z", 64)), "network.genesis_block_hash"},
		{"missing genesis timestamp", replace(`genesis_timestamp = 1596117600`, ``), "network.genesis_timestamp"},
		{"missing metachain", replace(`observer_urls = ["http://observer-meta:8080"]`, `observer_urls = [" "]`), "metachain.observer_urls"},
		{"shard out of range", replace("id = 2", "id = 3"), "shard 3 is out of range"},
		{"shard configured twice", replace("id = 2", "id = 1"), "shard 1 is configured more than once"},
		{"shard without URLs", replace(`rosetta_urls = ["http://rosetta-shard2:8091"]`, `rosetta_urls = []`), "shard 2 has no rosetta_urls"},
		{"a shard is not configured", replace("num_shards = 3", "num_shards = 4"), "shard 3 is not configured"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := Load([]byte(testCase.content))
			require.ErrorContains(t, err, testCase.expectedError)
		})
	}

	t.Run("no shard at all", func(t *testing.T) {
		config := &Config{}
		config.Network.Name = "mainnet"
		config.Network.NumShards = 3
		config.Network.GenesisBlockHash = strings.Repeat("ab", 32)
		config.Network.GenesisTimestamp = 1
		config.ApplyDefaults()
		require.ErrorContains(t, config.Validate(), "no [[shard]] is configured")
	})
}

func TestLoad_OfflineModeNeedsOneShardInstanceOnly(t *testing.T) {
	content := `
[server]
offline = true

[network]
name = "mainnet"
num_shards = 3
genesis_block_hash = "cd229e4ad2753708e4bab01d7f249affe29441829524c9529e84d51b6d12f2a7"
genesis_timestamp = 1596117600

[[shard]]
id = 0
rosetta_urls = ["http://rosetta-offline:8091"]
`
	config, err := Load([]byte(content))
	require.NoError(t, err)
	require.True(t, config.Server.Offline)
	require.Equal(t, 8090, config.Server.Port)

	// The same configuration is not valid in online mode.
	_, err = Load([]byte(strings.Replace(content, "offline = true", "offline = false", 1)))
	require.ErrorContains(t, err, "shard 1 is not configured")
}

func TestLoadFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "aggregator.toml")
	require.NoError(t, os.WriteFile(path, []byte(validConfig), 0o600))

	config, err := LoadFromFile(path)
	require.NoError(t, err)
	require.Equal(t, "mainnet", config.Network.Name)

	_, err = LoadFromFile(filepath.Join(t.TempDir(), "missing.toml"))
	require.ErrorContains(t, err, "cannot read configuration file")
}
