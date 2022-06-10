package services

import (
	"context"
	"testing"

	"github.com/ElrondNetwork/elrond-proxy-go/data"
	"github.com/ElrondNetwork/rosetta/testscommon"
	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/stretchr/testify/require"
)

func TestAccountService_AccountBalance(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	networkProvider.MockNetworkConfig.ChainID = "T"

	service := NewAccountService(networkProvider)

	// With bad input address
	_, err := getAccount(service, "")
	require.Equal(t, ErrInvalidAccountAddress, err)

	// When account does not exist
	response, err := getAccount(service, testscommon.TestAddressAlice)
	require.Equal(t, err.Code, ErrUnableToGetAccount.Code)
	require.Nil(t, response)

	// When account exists
	networkProvider.MockAccountsByAddress[testscommon.TestAddressAlice] = &data.Account{
		Address: testscommon.TestAddressAlice,
		Balance: "100",
	}
	networkProvider.MockLatestBlockSummary.Nonce = 42
	networkProvider.MockLatestBlockSummary.Hash = "abba"

	response, err = getAccount(service, testscommon.TestAddressAlice)
	require.Nil(t, err)
	require.Equal(t, "100", response.Balances[0].Value)
	require.Equal(t, int64(42), response.BlockIdentifier.Index)
	require.Equal(t, "abba", response.BlockIdentifier.Hash)
}

func getAccount(service server.AccountAPIServicer, address string) (*types.AccountBalanceResponse, *types.Error) {
	return service.AccountBalance(context.Background(), &types.AccountBalanceRequest{
		AccountIdentifier: &types.AccountIdentifier{Address: address},
	})
}
