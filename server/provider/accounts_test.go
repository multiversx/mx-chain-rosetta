package provider

import (
	"errors"
	"testing"

	"github.com/multiversx/mx-chain-rosetta/server/resources"
	"github.com/multiversx/mx-chain-rosetta/testscommon"
	"github.com/stretchr/testify/require"
)

func TestNetworkProvider_GetAccount(t *testing.T) {
	observerFacade := testscommon.NewObserverFacadeMock()
	args := createDefaultArgsNewNetworkProvider()
	args.ObserverFacade = observerFacade

	provider, err := NewNetworkProvider(args)
	require.Nil(t, err)
	require.NotNil(t, provider)

	t.Run("with success", func(t *testing.T) {
		observerFacade.MockNextError = nil
		observerFacade.MockGetResponse = resources.AccountApiResponse{
			Data: resources.AccountOnBlock{
				Account: resources.Account{
					Address: testscommon.TestAddressAlice,
					Balance: "1",
				},
				BlockCoordinates: resources.BlockCoordinates{
					Nonce: 1000,
				},
			},
		}

		account, err := provider.GetAccount(testscommon.TestAddressAlice)
		require.Nil(t, err)
		require.Equal(t, testscommon.TestAddressAlice, account.Account.Address)
		require.Equal(t, "1", account.Account.Balance)
		require.Equal(t, uint64(1000), account.BlockCoordinates.Nonce)
		require.Equal(t, args.ObserverUrl, observerFacade.RecordedBaseUrl)
		require.Equal(t, "/address/erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th?onFinalBlock=true", observerFacade.RecordedPath)
	})

	t.Run("with error", func(t *testing.T) {
		observerFacade.MockNextError = errors.New("arbitrary error")
		observerFacade.MockGetResponse = nil

		account, err := provider.GetAccount(testscommon.TestAddressAlice)
		require.ErrorIs(t, err, errCannotGetAccount)
		require.Nil(t, account)
		require.Equal(t, args.ObserverUrl, observerFacade.RecordedBaseUrl)
		require.Equal(t, "/address/erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th?onFinalBlock=true", observerFacade.RecordedPath)
	})
}

func TestNetworkProvider_GetAccountNativeBalance(t *testing.T) {
	observerFacade := testscommon.NewObserverFacadeMock()
	args := createDefaultArgsNewNetworkProvider()
	args.ObserverFacade = observerFacade

	provider, err := NewNetworkProvider(args)
	require.Nil(t, err)
	require.NotNil(t, provider)

	optionsOnFinal := resources.NewAccountQueryOptionsOnFinalBlock()

	t.Run("with success", func(t *testing.T) {
		observerFacade.MockNextError = nil
		observerFacade.MockGetResponse = resources.AccountApiResponse{
			Data: resources.AccountOnBlock{
				Account: resources.Account{
					Balance: "1",
				},
				BlockCoordinates: resources.BlockCoordinates{
					Nonce: 1000,
				},
			},
		}

		accountBalance, err := provider.GetAccountNativeBalance(testscommon.TestAddressAlice, optionsOnFinal)
		require.Nil(t, err)
		require.Equal(t, "1", accountBalance.Account.Balance)
		require.Equal(t, uint64(1000), accountBalance.BlockCoordinates.Nonce)
		require.Equal(t, args.ObserverUrl, observerFacade.RecordedBaseUrl)
		require.Equal(t, "/address/erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th?onFinalBlock=true", observerFacade.RecordedPath)
	})

	t.Run("with error", func(t *testing.T) {
		observerFacade.MockNextError = errors.New("arbitrary error")
		observerFacade.MockGetResponse = nil

		accountBalance, err := provider.GetAccountNativeBalance(testscommon.TestAddressAlice, optionsOnFinal)
		require.ErrorIs(t, err, errCannotGetAccount)
		require.Nil(t, accountBalance)
		require.Equal(t, args.ObserverUrl, observerFacade.RecordedBaseUrl)
		require.Equal(t, "/address/erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th?onFinalBlock=true", observerFacade.RecordedPath)
	})
}

func TestNetworkProvider_GetAccountESDTBalance(t *testing.T) {
	observerFacade := testscommon.NewObserverFacadeMock()
	args := createDefaultArgsNewNetworkProvider()
	args.ObserverFacade = observerFacade

	provider, err := NewNetworkProvider(args)
	require.Nil(t, err)
	require.NotNil(t, provider)

	optionsOnFinal := resources.NewAccountQueryOptionsOnFinalBlock()

	t.Run("with success", func(t *testing.T) {
		observerFacade.MockNextError = nil
		observerFacade.MockGetResponse = resources.AccountESDTBalanceApiResponse{
			Data: resources.AccountESDTBalance{
				Balance: "1",
				BlockCoordinates: resources.BlockCoordinates{
					Nonce: 1000,
				},
			},
		}

		accountBalance, err := provider.GetAccountESDTBalance(testscommon.TestAddressAlice, "ABC-abcdef", optionsOnFinal)
		require.Nil(t, err)
		require.Equal(t, "1", accountBalance.Balance)
		require.Equal(t, uint64(1000), accountBalance.BlockCoordinates.Nonce)
		require.Equal(t, args.ObserverUrl, observerFacade.RecordedBaseUrl)
		require.Equal(t, "/address/erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th/esdt/ABC-abcdef?onFinalBlock=true", observerFacade.RecordedPath)
	})

	t.Run("with error", func(t *testing.T) {
		observerFacade.MockNextError = errors.New("arbitrary error")
		observerFacade.MockGetResponse = nil

		accountBalance, err := provider.GetAccountESDTBalance(testscommon.TestAddressAlice, "ABC-abcdef", optionsOnFinal)
		require.ErrorIs(t, err, errCannotGetAccount)
		require.Nil(t, accountBalance)
		require.Equal(t, args.ObserverUrl, observerFacade.RecordedBaseUrl)
		require.Equal(t, "/address/erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th/esdt/ABC-abcdef?onFinalBlock=true", observerFacade.RecordedPath)
	})
}
