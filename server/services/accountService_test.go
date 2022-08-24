package services

import (
	"context"
	"testing"

	"github.com/ElrondNetwork/rosetta/server/resources"
	"github.com/ElrondNetwork/rosetta/testscommon"
	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/stretchr/testify/require"
)

func TestAccountService_AccountBalance(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	networkProvider.MockNetworkConfig.ChainID = "T"
	service := NewAccountService(networkProvider)

	t.Run("with empty address", func(t *testing.T) {
		requestWithEmptyAddress := &types.AccountBalanceRequest{
			AccountIdentifier: &types.AccountIdentifier{Address: ""},
		}

		_, err := service.AccountBalance(context.Background(), requestWithEmptyAddress)
		require.Equal(t, ErrInvalidAccountAddress, errCode(err.Code))
	})

	t.Run("with no specified currency, when account does not exist", func(t *testing.T) {
		request := &types.AccountBalanceRequest{
			AccountIdentifier: &types.AccountIdentifier{Address: "alice"},
		}

		response, err := service.AccountBalance(context.Background(), request)
		require.Equal(t, ErrUnableToGetAccount, errCode(err.Code))
		require.Nil(t, response)
	})

	t.Run("with no specified currency, when account exists", func(t *testing.T) {
		request := &types.AccountBalanceRequest{
			AccountIdentifier: &types.AccountIdentifier{Address: "alice"},
		}

		networkProvider.MockAccountsNativeBalances["alice"] = &resources.AccountNativeBalance{
			Balance: "100",
		}
		networkProvider.MockNextAccountBlockCoordinates.Nonce = 42
		networkProvider.MockNextAccountBlockCoordinates.Hash = "abba"

		response, err := service.AccountBalance(context.Background(), request)
		require.Nil(t, err)
		require.Equal(t, "100", response.Balances[0].Value)
		require.Equal(t, int64(42), response.BlockIdentifier.Index)
		require.Equal(t, "abba", response.BlockIdentifier.Hash)
	})

	t.Run("with native currency (specified)", func(t *testing.T) {
		request := &types.AccountBalanceRequest{
			AccountIdentifier: &types.AccountIdentifier{Address: "alice"},
			Currencies: []*types.Currency{
				{
					Symbol:   "XeGLD",
					Decimals: 18,
				},
			},
		}

		networkProvider.MockAccountsNativeBalances["alice"] = &resources.AccountNativeBalance{
			Balance: "1000",
		}
		networkProvider.MockNextAccountBlockCoordinates.Nonce = 42
		networkProvider.MockNextAccountBlockCoordinates.Hash = "abba"

		response, err := service.AccountBalance(context.Background(), request)
		require.Nil(t, err)
		require.Equal(t, "1000", response.Balances[0].Value)
		require.Equal(t, "XeGLD", response.Balances[0].Currency.Symbol)
	})

	t.Run("with one custom currency (specified)", func(t *testing.T) {
		request := &types.AccountBalanceRequest{
			AccountIdentifier: &types.AccountIdentifier{Address: "alice"},
			Currencies: []*types.Currency{
				{
					Symbol:   "FOO-abcdef",
					Decimals: 18,
				},
			},
		}

		networkProvider.MockAccountsESDTBalances["alice_FOO-abcdef"] = &resources.AccountESDTBalance{
			Balance: "500",
		}
		networkProvider.MockNextAccountBlockCoordinates.Nonce = 42
		networkProvider.MockNextAccountBlockCoordinates.Hash = "abba"

		response, err := service.AccountBalance(context.Background(), request)
		require.Nil(t, err)
		require.Equal(t, "500", response.Balances[0].Value)
		require.Equal(t, "FOO-abcdef", response.Balances[0].Currency.Symbol)
	})

	t.Run("with more than 1 (custom or not) currencies", func(t *testing.T) {
		request := &types.AccountBalanceRequest{
			AccountIdentifier: &types.AccountIdentifier{Address: "alice"},
			Currencies: []*types.Currency{
				{
					Symbol:   "FOO-abcdef",
					Decimals: 18,
				},
				{
					Symbol:   "BAR-abcdef",
					Decimals: 18,
				},
			},
		}

		networkProvider.MockAccountsESDTBalances["alice_FOO-abcdef"] = &resources.AccountESDTBalance{
			Balance: "500",
		}
		networkProvider.MockAccountsESDTBalances["alice_BAR-abcdef"] = &resources.AccountESDTBalance{
			Balance: "700",
		}
		networkProvider.MockNextAccountBlockCoordinates.Nonce = 42
		networkProvider.MockNextAccountBlockCoordinates.Hash = "abba"

		response, err := service.AccountBalance(context.Background(), request)
		require.Nil(t, response)
		require.Equal(t, err.Code, int32(ErrNotImplemented))
	})
}
