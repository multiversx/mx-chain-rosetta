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
