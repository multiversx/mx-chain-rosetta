package services

import (
	"testing"

	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/multiversx/mx-chain-rosetta/testscommon"
	"github.com/stretchr/testify/require"
)

func TestNetworkProviderExtension_ValueToNativeAmount(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	networkProvider.MockNativeCurrencySymbol = "EGLD"
	extension := newNetworkProviderExtension(networkProvider)

	amount := extension.valueToNativeAmount("1")
	expectedAmount := &types.Amount{
		Value: "1",
		Currency: &types.Currency{
			Symbol:   "EGLD",
			Decimals: 18,
		},
	}

	require.Equal(t, expectedAmount, amount)
}

func TestNetworkProviderExtension_ValueToCustomAmount(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	extension := newNetworkProviderExtension(networkProvider)

	amount := extension.valueToCustomAmount("1", "ABC-abcdef")
	expectedAmount := &types.Amount{
		Value: "1",
		Currency: &types.Currency{
			Symbol:   "ABC-abcdef",
			Decimals: 0,
		},
	}

	require.Equal(t, expectedAmount, amount)
}

func TestNetworkProviderExtension_IsAddressObserved(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	extension := newNetworkProviderExtension(networkProvider)

	networkProvider.MockObservedActualShard = 0

	isObserved, err := extension.isAddressObserved("erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th")
	require.NoError(t, err)
	require.False(t, isObserved)

	isObserved, err = extension.isAddressObserved("erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx")
	require.NoError(t, err)
	require.True(t, isObserved)

	isObserved, err = extension.isAddressObserved("erd1k2s324ww2g0yj38qn2ch2jwctdy8mnfxep94q9arncc6xecg3xaq6mjse8")
	require.NoError(t, err)
	require.False(t, isObserved)
}
