package provider

import (
	"errors"
	"strings"
	"testing"

	"github.com/multiversx/mx-chain-core-go/data/api"
	"github.com/multiversx/mx-chain-core-go/data/transaction"
	"github.com/multiversx/mx-chain-proxy-go/common"
	"github.com/multiversx/mx-chain-proxy-go/data"
	"github.com/multiversx/mx-chain-rosetta/testscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNetworkProvider(t *testing.T) {
	args := ArgsNewNetworkProvider{
		IsOffline:                   true,
		ObservedActualShard:         42,
		ObservedProjectedShard:      42,
		ObservedProjectedShardIsSet: true,
		ObserverUrl:                 "http://my-observer:8080",
		NetworkID:                   "T",
		NetworkName:                 "testnet",
		GasPerDataByte:              1501,
		MinGasPrice:                 1000000001,
		MinGasLimit:                 50001,
		ExtraGasLimitGuardedTx:      50001,
		NativeCurrencySymbol:        "XeGLD",
		GenesisBlockHash:            "aaaa",
		GenesisTimestamp:            123456789,
		FirstHistoricalEpoch:        1000,
		NumHistoricalEpochs:         1024,
		ObserverFacade:              testscommon.NewObserverFacadeMock(),
		Hasher:                      testscommon.RealWorldBlake2bHasher,
		MarshalizerForHashing:       testscommon.MarshalizerForHashing,
		PubKeyConverter:             testscommon.RealWorldBech32PubkeyConverter,
	}

	provider, err := NewNetworkProvider(args)
	require.Nil(t, err)
	require.NotNil(t, provider)

	assert.Equal(t, true, provider.IsOffline())
	assert.Equal(t, uint32(42), provider.observedActualShard)
	assert.Equal(t, uint32(42), provider.observedProjectedShard)
	assert.Equal(t, true, provider.observedProjectedShardIsSet)
	assert.Equal(t, "http://my-observer:8080", provider.observerUrl)
	assert.Equal(t, "T", provider.GetNetworkConfig().NetworkID)
	assert.Equal(t, "testnet", provider.GetNetworkConfig().NetworkName)
	assert.Equal(t, uint64(1501), provider.GetNetworkConfig().GasPerDataByte)
	assert.Equal(t, uint64(1000000001), provider.GetNetworkConfig().MinGasPrice)
	assert.Equal(t, uint64(50001), provider.GetNetworkConfig().MinGasLimit)
	assert.Equal(t, uint64(50001), provider.GetNetworkConfig().ExtraGasLimitGuardedTx)
	assert.Equal(t, "XeGLD", provider.GetNativeCurrency().Symbol)
	assert.Equal(t, "aaaa", provider.GetGenesisBlockSummary().Hash)
	assert.Equal(t, int64(123456789), provider.GetGenesisTimestamp())
	assert.Equal(t, uint32(1000), provider.firstHistoricalEpoch)
	assert.Equal(t, uint32(1024), provider.numHistoricalEpochs)
}

func TestNetworkProvider_DoGetBlockByNonce(t *testing.T) {
	observerFacade := testscommon.NewObserverFacadeMock()
	args := createDefaultArgsNewNetworkProvider()
	args.ObserverFacade = observerFacade

	provider, err := NewNetworkProvider(args)
	require.Nil(t, err)
	require.NotNil(t, provider)

	t.Run("with error (not cached)", func(t *testing.T) {
		observerFacade.GetBlockByNonceCalled = func(shardID uint32, nonce uint64, options common.BlockQueryOptions) (*data.BlockApiResponse, error) {
			return nil, errors.New("arbitrary error")
		}

		block, err := provider.doGetBlockByNonce(42)
		require.Nil(t, block)
		require.ErrorContains(t, err, "arbitrary error")
		require.Equal(t, 0, provider.blocksCache.Len())
	})

	t.Run("with success (cached)", func(t *testing.T) {
		observerFacade.GetBlockByNonceCalled = func(shardID uint32, nonce uint64, options common.BlockQueryOptions) (*data.BlockApiResponse, error) {
			if nonce == 42 {
				return &data.BlockApiResponse{
					Data: data.BlockApiResponsePayload{
						Block: api.Block{
							Nonce: 42,
						},
					},
				}, nil
			}

			return nil, errors.New("unexpected request")
		}

		block, err := provider.doGetBlockByNonce(42)
		require.Nil(t, err)
		require.Equal(t, uint64(42), block.Nonce)
		require.Equal(t, 1, provider.blocksCache.Len())

		observerFacade.GetBlockByNonceCalled = func(shardID uint32, nonce uint64, options common.BlockQueryOptions) (*data.BlockApiResponse, error) {
			return nil, errors.New("unexpected request")
		}

		cachedBlock, err := provider.doGetBlockByNonce(42)
		require.Nil(t, err)
		require.Equal(t, block, cachedBlock)
	})

	t.Run("many requests, filling the cache", func(t *testing.T) {
		observerFacade.GetBlockByNonceCalled = func(shardID uint32, nonce uint64, options common.BlockQueryOptions) (*data.BlockApiResponse, error) {
			return &data.BlockApiResponse{
				Data: data.BlockApiResponsePayload{
					Block: api.Block{
						Nonce: nonce,
					},
				},
			}, nil
		}

		for i := uint64(0); i < uint64(blocksCacheCapacity*2); i++ {
			block, err := provider.doGetBlockByNonce(i)
			require.Nil(t, err)
			require.Equal(t, i, block.Nonce)

		}

		require.Equal(t, blocksCacheCapacity, provider.blocksCache.Len())
	})

	t.Run("the cache holds block copies", func(t *testing.T) {
		provider.blocksCache.Clear()

		observerFacade.GetBlockByNonceCalled = func(shardID uint32, nonce uint64, options common.BlockQueryOptions) (*data.BlockApiResponse, error) {
			return &data.BlockApiResponse{
				Data: data.BlockApiResponsePayload{
					Block: api.Block{
						Nonce: nonce,
						MiniBlocks: []*api.MiniBlock{
							{Hash: "aaaa"},
							{Hash: "bbbb"},
						},
					},
				},
			}, nil
		}

		block, err := provider.doGetBlockByNonce(7)
		require.Nil(t, err)
		require.Equal(t, uint64(7), block.Nonce)
		require.Len(t, block.MiniBlocks, 2)
		require.Equal(t, 1, provider.blocksCache.Len())

		// Simulate mutations performed by downstream handling of blocks, i.e. "simplifyBlockWithScheduledTransactions":
		block.MiniBlocks = []*api.MiniBlock{}

		cachedBlock, err := provider.doGetBlockByNonce(7)
		require.Nil(t, err)
		require.Equal(t, uint64(7), cachedBlock.Nonce)
		// Miniblocks removal (above) does not reflect in the cached data
		require.Len(t, cachedBlock.MiniBlocks, 2)
		// ... because the cache holds block copies:
		require.False(t, &block == &cachedBlock)
		require.Equal(t, 1, provider.blocksCache.Len())
	})
}

func Test_ComputeShardIdOfPubKey(t *testing.T) {
	args := createDefaultArgsNewNetworkProvider()
	provider, err := NewNetworkProvider(args)
	require.Nil(t, err)
	require.NotNil(t, provider)

	require.Equal(t, uint32(0), provider.ComputeShardIdOfPubKey(testscommon.TestPubKeyBob))
	require.Equal(t, uint32(1), provider.ComputeShardIdOfPubKey(testscommon.TestPubKeyAlice))
	require.Equal(t, uint32(2), provider.ComputeShardIdOfPubKey(testscommon.TestPubKeyCarol))
}

func Test_ComputeTransactionFeeForMoveBalance(t *testing.T) {
	args := createDefaultArgsNewNetworkProvider()
	provider, err := NewNetworkProvider(args)
	require.Nil(t, err)
	require.NotNil(t, provider)

	t.Run("without data, not guarded", func(t *testing.T) {
		fee := provider.ComputeTransactionFeeForMoveBalance(&transaction.ApiTransactionResult{
			Data:     nil,
			GasPrice: 1000000000,
		})

		assert.Equal(t, "50000000000000", fee.String())
	})

	t.Run("with data, not guarded", func(t *testing.T) {
		fee := provider.ComputeTransactionFeeForMoveBalance(&transaction.ApiTransactionResult{
			Data:     []byte("hello"),
			GasPrice: 1000000000,
		})

		assert.Equal(t, "57500000000000", fee.String())
	})

	t.Run("without data, guarded", func(t *testing.T) {
		fee := provider.ComputeTransactionFeeForMoveBalance(&transaction.ApiTransactionResult{
			Data:         nil,
			GasPrice:     1000000000,
			GuardianAddr: "erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th",
		})

		assert.Equal(t, "100000000000000", fee.String())
	})

	t.Run("with data, guarded", func(t *testing.T) {
		fee := provider.ComputeTransactionFeeForMoveBalance(&transaction.ApiTransactionResult{
			Data:         []byte("world"),
			GasPrice:     1000000000,
			GuardianAddr: "erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th",
		})

		assert.Equal(t, "107500000000000", fee.String())
	})
}

func createDefaultArgsNewNetworkProvider() ArgsNewNetworkProvider {
	return ArgsNewNetworkProvider{
		IsOffline:                   false,
		ObservedActualShard:         0,
		ObservedProjectedShard:      0,
		ObservedProjectedShardIsSet: false,
		ObserverUrl:                 "http://my-observer:8080",
		NetworkID:                   "T",
		GasPerDataByte:              1500,
		MinGasPrice:                 1000000000,
		MinGasLimit:                 50000,
		ExtraGasLimitGuardedTx:      50000,
		NativeCurrencySymbol:        "XeGLD",
		GenesisBlockHash:            strings.Repeat("0", 64),
		GenesisTimestamp:            123456789,
		ObserverFacade:              testscommon.NewObserverFacadeMock(),
		Hasher:                      testscommon.RealWorldBlake2bHasher,
		MarshalizerForHashing:       testscommon.MarshalizerForHashing,
		PubKeyConverter:             testscommon.RealWorldBech32PubkeyConverter,
	}
}
