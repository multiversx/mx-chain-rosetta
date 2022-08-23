package services

import (
	"testing"

	"github.com/ElrondNetwork/rosetta/testscommon"
	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/stretchr/testify/require"
)

func TestNetworkProviderExtension_ValueToNativeAmount(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	networkProvider.MockNativeCurrencySymbol = "EGLD"
	extension := newNetworkProviderExtension(networkProvider)

	amount := extension.valueToNativeAmount("1")
	expecteAmount := &types.Amount{
		Value: "1",
		Currency: &types.Currency{
			Symbol:   "EGLD",
			Decimals: 18,
		},
	}

	require.Equal(t, expecteAmount, amount)
}

func TestNetworkProviderExtension_ValueToCustomAmount(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	extension := newNetworkProviderExtension(networkProvider)

	amount := extension.valueToCustomAmount("1", "ABC-abcdef")
	expecteAmount := &types.Amount{
		Value: "1",
		Currency: &types.Currency{
			Symbol: "ABC-abcdef",
		},
	}

	require.Equal(t, expecteAmount, amount)
}