// Package config holds the configuration of the aggregator
package config

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

const (
	defaultPort                         = 8090
	defaultBlockchain                   = "MultiversX"
	defaultAddressPrefix                = "erd"
	defaultBackendTimeoutSeconds        = 60
	defaultMaxConcurrentBackendRequests = 32
	defaultBlockCacheSize               = 256
	defaultSyncIntervalMilliseconds     = 500
	defaultMaxRequestBodyBytes          = 1 << 20
	hashLength                          = 32
)

// Config is the configuration of the aggregator
type Config struct {
	Server     ServerConfig     `toml:"server"`
	Network    NetworkConfig    `toml:"network"`
	Metachain  MetachainConfig  `toml:"metachain"`
	Shards     []ShardConfig    `toml:"shard"`
	Index      IndexConfig      `toml:"index"`
	Limits     LimitsConfig     `toml:"limits"`
	CrossShard CrossShardConfig `toml:"cross_shard"`
}

// ServerConfig configures the HTTP server of the aggregator
type ServerConfig struct {
	// Host is the address to bind to. Empty means all interfaces.
	Host string `toml:"host"`
	Port int    `toml:"port"`
	// Offline starts the aggregator in offline mode: only the endpoints that do not need access to the chain are available.
	Offline bool `toml:"offline"`
	// EnableCors adds permissive CORS headers to the responses (disabled by default)
	EnableCors bool `toml:"enable_cors"`
}

// NetworkConfig describes the network. The values must be consistent with the ones of the shard instances.
type NetworkConfig struct {
	Blockchain string `toml:"blockchain"`
	// Name must be equal to the "--network-name" of the shard instances
	Name      string `toml:"name"`
	NumShards uint32 `toml:"num_shards"`
	// HandleContracts must be equal to the "--handle-contracts" of the shard instances
	HandleContracts bool   `toml:"handle_contracts"`
	AddressPrefix   string `toml:"address_prefix"`
	// GenesisBlockHash is the hash of the genesis block of the metachain
	GenesisBlockHash string `toml:"genesis_block_hash"`
	// GenesisTimestamp is the timestamp of the genesis block, in seconds
	GenesisTimestamp int64 `toml:"genesis_timestamp"`
}

// MetachainConfig configures the source of metachain data
type MetachainConfig struct {
	// ObserverUrls are the URLs of the REST API of one (or more, for fail-over) metachain observers
	ObserverUrls []string `toml:"observer_urls"`
}

// ShardConfig configures the backend (shard instance) of a shard
type ShardConfig struct {
	ID uint32 `toml:"id"`
	// RosettaUrls are the URLs of one (or more, for fail-over) Rosetta instances that observe this shard
	RosettaUrls []string `toml:"rosetta_urls"`
}

// IndexConfig configures the notarization index
type IndexConfig struct {
	// Path is the folder where the index is persisted. If empty, the index is kept in memory (and rebuilt at each start).
	Path string `toml:"path"`
	// StartEpoch, if set, makes a new index start at the first metablock of this epoch.
	// It should be the first epoch for which the shard instances hold historical balances.
	StartEpoch *uint32 `toml:"start_epoch"`
	// StartNonce makes a new index start at this metablock (ignored if "start_epoch" is set). By default, the index starts at genesis.
	StartNonce uint64 `toml:"start_nonce"`
}

// LimitsConfig holds timeouts and limits
type LimitsConfig struct {
	BackendTimeoutSeconds        int   `toml:"backend_timeout_seconds"`
	MaxConcurrentBackendRequests int   `toml:"max_concurrent_backend_requests"`
	BlockCacheSize               int   `toml:"block_cache_size"`
	SyncIntervalMilliseconds     int   `toml:"sync_interval_milliseconds"`
	MaxRequestBodyBytes          int64 `toml:"max_request_body_bytes"`
}

// CrossShardConfig configures the presentation of cross-shard transactions
type CrossShardConfig struct {
	// ForwardLinks controls whether the source leg of a cross-shard transfer links ("forward") to its destination leg,
	// when the destination leg is certain to appear. Enabled by default.
	ForwardLinks *bool `toml:"forward_links"`
}

// LoadFromFile loads the configuration from a TOML file, applies the defaults and validates it
func LoadFromFile(path string) (*Config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read configuration file: %w", err)
	}

	return Load(content)
}

// Load loads the configuration from TOML content, applies the defaults and validates it
func Load(content []byte) (*Config, error) {
	config := &Config{}

	// Unknown keys are rejected, so that typos do not go unnoticed.
	decoder := toml.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()

	err := decoder.Decode(config)
	if err != nil {
		var strictErr *toml.StrictMissingError
		if errors.As(err, &strictErr) {
			return nil, fmt.Errorf("cannot parse configuration: unknown keys:\n%s", strictErr.String())
		}

		return nil, fmt.Errorf("cannot parse configuration: %w", err)
	}

	config.ApplyDefaults()

	err = config.Validate()
	if err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// ApplyDefaults sets the default values for the fields that are not set
func (config *Config) ApplyDefaults() {
	if config.Server.Port == 0 {
		config.Server.Port = defaultPort
	}
	if len(config.Network.Blockchain) == 0 {
		config.Network.Blockchain = defaultBlockchain
	}
	if len(config.Network.AddressPrefix) == 0 {
		config.Network.AddressPrefix = defaultAddressPrefix
	}
	if config.Limits.BackendTimeoutSeconds <= 0 {
		config.Limits.BackendTimeoutSeconds = defaultBackendTimeoutSeconds
	}
	if config.Limits.MaxConcurrentBackendRequests <= 0 {
		config.Limits.MaxConcurrentBackendRequests = defaultMaxConcurrentBackendRequests
	}
	if config.Limits.BlockCacheSize <= 0 {
		config.Limits.BlockCacheSize = defaultBlockCacheSize
	}
	if config.Limits.SyncIntervalMilliseconds <= 0 {
		config.Limits.SyncIntervalMilliseconds = defaultSyncIntervalMilliseconds
	}
	if config.Limits.MaxRequestBodyBytes <= 0 {
		config.Limits.MaxRequestBodyBytes = defaultMaxRequestBodyBytes
	}
}

// Validate checks the configuration
func (config *Config) Validate() error {
	if config.Server.Port <= 0 || config.Server.Port > 65535 {
		return fmt.Errorf("server.port is out of range: %d", config.Server.Port)
	}
	if len(config.Network.Name) == 0 {
		return errors.New("network.name is missing")
	}
	if config.Network.NumShards == 0 {
		return errors.New("network.num_shards must be greater than zero")
	}

	err := checkHash(config.Network.GenesisBlockHash)
	if err != nil {
		return fmt.Errorf("network.genesis_block_hash: %w", err)
	}
	if config.Network.GenesisTimestamp <= 0 {
		return errors.New("network.genesis_timestamp is missing")
	}

	if len(config.Shards) == 0 {
		return errors.New("no [[shard]] is configured")
	}

	seenShards := make(map[uint32]struct{}, len(config.Shards))
	for _, shard := range config.Shards {
		if shard.ID >= config.Network.NumShards {
			return fmt.Errorf("shard %d is out of range (network.num_shards = %d)", shard.ID, config.Network.NumShards)
		}
		if _, seen := seenShards[shard.ID]; seen {
			return fmt.Errorf("shard %d is configured more than once", shard.ID)
		}
		if countNonEmpty(shard.RosettaUrls) == 0 {
			return fmt.Errorf("shard %d has no rosetta_urls", shard.ID)
		}

		seenShards[shard.ID] = struct{}{}
	}

	if config.Server.Offline {
		// In offline mode, a single (offline) shard instance is sufficient, and the metachain is not needed.
		return nil
	}

	// All regular shards must be served: otherwise, the blocks notarized by the metachain could not be assembled.
	for shard := uint32(0); shard < config.Network.NumShards; shard++ {
		if _, seen := seenShards[shard]; !seen {
			return fmt.Errorf("shard %d is not configured (all %d shards must be configured)", shard, config.Network.NumShards)
		}
	}

	if countNonEmpty(config.Metachain.ObserverUrls) == 0 {
		return errors.New("metachain.observer_urls is missing")
	}

	return nil
}

func checkHash(hash string) error {
	decoded, err := hex.DecodeString(hash)
	if err != nil || len(decoded) != hashLength {
		return fmt.Errorf("expected %d hexadecimal characters", 2*hashLength)
	}

	return nil
}

func countNonEmpty(items []string) int {
	count := 0
	for _, item := range items {
		if len(strings.TrimSpace(item)) > 0 {
			count++
		}
	}

	return count
}

// ShardIDs returns the configured shards
func (config *Config) ShardIDs() []uint32 {
	shards := make([]uint32, 0, len(config.Shards))
	for _, shard := range config.Shards {
		shards = append(shards, shard.ID)
	}

	return shards
}

// IsForwardLinksEnabled returns whether forward links (from source legs to destination legs) are enabled
func (config *Config) IsForwardLinksEnabled() bool {
	if config.CrossShard.ForwardLinks == nil {
		return true
	}

	return *config.CrossShard.ForwardLinks
}

// GetBackendTimeout returns the timeout of the requests towards shard instances and metachain observers
func (config *Config) GetBackendTimeout() time.Duration {
	return time.Duration(config.Limits.BackendTimeoutSeconds) * time.Second
}

// GetSyncInterval returns the interval between two synchronization steps
func (config *Config) GetSyncInterval() time.Duration {
	return time.Duration(config.Limits.SyncIntervalMilliseconds) * time.Millisecond
}
