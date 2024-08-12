package services

import (
	"testing"

	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/multiversx/mx-chain-rosetta/server/resources"
	"github.com/multiversx/mx-chain-rosetta/testscommon"
	"github.com/stretchr/testify/require"
)

func TestNetworkProviderExtension_ValueToAmount(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	networkProvider.MockNativeCurrencySymbol = "EGLD"
	networkProvider.MockCustomCurrencies = []resources.Currency{
		{
			Symbol:   "ABC-abcdef",
			Decimals: 4,
		},
	}

	extension := newNetworkProviderExtension(networkProvider)

	t.Run("with native currency", func(t *testing.T) {
		amount := extension.valueToAmount("1", "EGLD")
		expectedAmount := &types.Amount{
			Value: "1",
			Currency: &types.Currency{
				Symbol:   "EGLD",
				Decimals: 18,
			},
		}

		require.Equal(t, expectedAmount, amount)
	})

	t.Run("with native currency (explicitly)", func(t *testing.T) {
		amount := extension.valueToNativeAmount("1")
		expectedAmount := &types.Amount{
			Value: "1",
			Currency: &types.Currency{
				Symbol:   "EGLD",
				Decimals: 18,
			},
		}

		require.Equal(t, expectedAmount, amount)
	})

	t.Run("with custom currency", func(t *testing.T) {
		amount := extension.valueToAmount("1", "ABC-abcdef")
		expectedAmount := &types.Amount{
			Value: "1",
			Currency: &types.Currency{
				Symbol:   "ABC-abcdef",
				Decimals: 4,
			},
		}

		require.Equal(t, expectedAmount, amount)
	})

	t.Run("with custom currency (explicitly)", func(t *testing.T) {
		amount := extension.valueToCustomAmount("1", "ABC-abcdef")
		expectedAmount := &types.Amount{
			Value: "1",
			Currency: &types.Currency{
				Symbol:   "ABC-abcdef",
				Decimals: 4,
			},
		}

		require.Equal(t, expectedAmount, amount)
	})
}
