package provider

import (
	"testing"

	"github.com/ElrondNetwork/rosetta/testscommon"
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
		ObserverPubkey:              "abba",
		ChainID:                     "T",
		GasPerDataByte:              1501,
		MinGasPrice:                 1000000001,
		MinGasLimit:                 50001,
		NativeCurrencySymbol:        "XeGLD",
		GenesisBlockHash:            "aaaa",
		GenesisTimestamp:            123456789,
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
	assert.Equal(t, "abba", provider.GetObserverPubkey())
	assert.Equal(t, "T", provider.GetChainID())
	assert.Equal(t, "T", provider.GetNetworkConfig().ChainID)
	assert.Equal(t, uint64(1501), provider.GetNetworkConfig().GasPerDataByte)
	assert.Equal(t, uint64(1000000001), provider.GetNetworkConfig().MinGasPrice)
	assert.Equal(t, uint64(50001), provider.GetNetworkConfig().MinGasLimit)
	assert.Equal(t, "XeGLD", provider.GetNativeCurrency().Symbol)
	assert.Equal(t, "aaaa", provider.GetGenesisBlockSummary().Hash)
	assert.Equal(t, int64(123456789), provider.GetGenesisTimestamp())
}
