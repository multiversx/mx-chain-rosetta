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

func TestNetworkProvider_GetAccountBalance(t *testing.T) {
	observerFacade := testscommon.NewObserverFacadeMock()
	args := createDefaultArgsNewNetworkProvider()
	args.ObserverFacade = observerFacade

	provider, err := NewNetworkProvider(args)
	require.Nil(t, err)
	require.NotNil(t, provider)

	optionsOnFinal := resources.NewAccountQueryOptionsOnFinalBlock()

	t.Run("native balance, with success", func(t *testing.T) {
		observerFacade.MockNextError = nil
		observerFacade.MockGetResponse = resources.AccountApiResponse{
			Data: resources.AccountOnBlock{
				Account: resources.Account{
					Balance: "1",
					Nonce:   42,
				},
				BlockCoordinates: resources.BlockCoordinates{
					Nonce: 1000,
				},
			},
		}

		accountBalance, err := provider.GetAccountBalance(testscommon.TestAddressAlice, "XeGLD", optionsOnFinal)
		require.Nil(t, err)
		require.Equal(t, "1", accountBalance.Balance)
		require.Equal(t, uint64(42), accountBalance.Nonce.Value)
		require.Equal(t, uint64(1000), accountBalance.BlockCoordinates.Nonce)
		require.Equal(t, args.ObserverUrl, observerFacade.RecordedBaseUrl)
		require.Equal(t, "/address/erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th?onFinalBlock=true", observerFacade.RecordedPath)
	})

	t.Run("native balance, with error", func(t *testing.T) {
		observerFacade.MockNextError = errors.New("arbitrary error")
		observerFacade.MockGetResponse = nil

		accountBalance, err := provider.GetAccountBalance(testscommon.TestAddressAlice, "XeGLD", optionsOnFinal)
		require.ErrorIs(t, err, errCannotGetAccount)
		require.Nil(t, accountBalance)
		require.Equal(t, args.ObserverUrl, observerFacade.RecordedBaseUrl)
		require.Equal(t, "/address/erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th?onFinalBlock=true", observerFacade.RecordedPath)
	})

	t.Run("fungible token, with success", func(t *testing.T) {
		observerFacade.MockNextError = nil
		observerFacade.MockGetResponse = resources.AccountESDTBalanceApiResponse{
			Data: resources.AccountESDTBalanceApiResponsePayload{
				TokenData: resources.AccountESDTTokenData{
					Balance: "1",
				},
				BlockCoordinates: resources.BlockCoordinates{
					Nonce: 1000,
				},
			},
		}

		accountBalance, err := provider.GetAccountBalance(testscommon.TestAddressAlice, "ABC-abcdef", optionsOnFinal)
		require.Nil(t, err)
		require.Equal(t, "1", accountBalance.Balance)
		require.False(t, accountBalance.Nonce.HasValue)
		require.Equal(t, uint64(1000), accountBalance.BlockCoordinates.Nonce)
		require.Equal(t, args.ObserverUrl, observerFacade.RecordedBaseUrl)
		require.Equal(t, "/address/erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th/esdt/ABC-abcdef?onFinalBlock=true", observerFacade.RecordedPath)
	})

	t.Run("fungible token, with error", func(t *testing.T) {
		observerFacade.MockNextError = errors.New("arbitrary error")
		observerFacade.MockGetResponse = nil

		accountBalance, err := provider.GetAccountBalance(testscommon.TestAddressAlice, "ABC-abcdef", optionsOnFinal)
		require.ErrorIs(t, err, errCannotGetAccount)
		require.Nil(t, accountBalance)
		require.Equal(t, args.ObserverUrl, observerFacade.RecordedBaseUrl)
		require.Equal(t, "/address/erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th/esdt/ABC-abcdef?onFinalBlock=true", observerFacade.RecordedPath)
	})

	t.Run("non-fungible token, with success", func(t *testing.T) {
		observerFacade.MockNextError = nil
		observerFacade.MockGetResponse = resources.AccountESDTBalanceApiResponse{
			Data: resources.AccountESDTBalanceApiResponsePayload{
				TokenData: resources.AccountESDTTokenData{
					Balance: "1",
				},
				BlockCoordinates: resources.BlockCoordinates{
					Nonce: 1000,
				},
			},
		}

		accountBalance, err := provider.GetAccountBalance(testscommon.TestAddressAlice, "ABC-abcdef-0a", optionsOnFinal)
		require.Nil(t, err)
		require.Equal(t, "1", accountBalance.Balance)
		require.False(t, accountBalance.Nonce.HasValue)
		require.Equal(t, uint64(1000), accountBalance.BlockCoordinates.Nonce)
		require.Equal(t, args.ObserverUrl, observerFacade.RecordedBaseUrl)
		require.Equal(t, "/address/erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th/nft/ABC-abcdef/nonce/10?onFinalBlock=true", observerFacade.RecordedPath)
	})

	t.Run("non-fungible token, with error", func(t *testing.T) {
		observerFacade.MockNextError = errors.New("arbitrary error")
		observerFacade.MockGetResponse = nil

		accountBalance, err := provider.GetAccountBalance(testscommon.TestAddressAlice, "ABC-abcdef-0a", optionsOnFinal)
		require.ErrorIs(t, err, errCannotGetAccount)
		require.Nil(t, accountBalance)
		require.Equal(t, args.ObserverUrl, observerFacade.RecordedBaseUrl)
		require.Equal(t, "/address/erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th/nft/ABC-abcdef/nonce/10?onFinalBlock=true", observerFacade.RecordedPath)
	})
}

func TestDecideCustomTokenBalanceUrl(t *testing.T) {
	args := createDefaultArgsNewNetworkProvider()
	provider, err := NewNetworkProvider(args)
	require.Nil(t, err)
	require.NotNil(t, provider)

	t.Run("for fungible", func(t *testing.T) {
		url, err := decideCustomTokenBalanceUrl(testscommon.TestAddressCarol, "ABC-abcdef", resources.AccountQueryOptions{})
		require.Nil(t, err)
		require.Equal(t, "/address/erd1k2s324ww2g0yj38qn2ch2jwctdy8mnfxep94q9arncc6xecg3xaq6mjse8/esdt/ABC-abcdef", url)
	})

	t.Run("for non-fungible", func(t *testing.T) {
		url, err := decideCustomTokenBalanceUrl(testscommon.TestAddressCarol, "ABC-abcdef-0a", resources.AccountQueryOptions{})
		require.Nil(t, err)
		require.Equal(t, "/address/erd1k2s324ww2g0yj38qn2ch2jwctdy8mnfxep94q9arncc6xecg3xaq6mjse8/nft/ABC-abcdef/nonce/10", url)
	})

	t.Run("with error", func(t *testing.T) {
		url, err := decideCustomTokenBalanceUrl(testscommon.TestAddressCarol, "ABC", resources.AccountQueryOptions{})
		require.ErrorIs(t, err, errCannotParseTokenIdentifier)
		require.Empty(t, url)
	})
}
