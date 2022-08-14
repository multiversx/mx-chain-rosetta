package services

import (
	"context"
	"testing"

	"github.com/ElrondNetwork/rosetta/server/resources"
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
	_, err := getAccountBalance(service, "")
	require.Equal(t, ErrInvalidAccountAddress, errCode(err.Code))

	// When account does not exist
	response, err := getAccountBalance(service, testscommon.TestAddressAlice)
	require.Equal(t, ErrUnableToGetAccount, errCode(err.Code))
	require.Nil(t, response)

	// When account exists
	networkProvider.MockAccountsNativeBalances[testscommon.TestAddressAlice] = &resources.AccountNativeBalance{
		Balance: "100",
	}
	networkProvider.MockNextAccountBlockCoordinates.Nonce = 42
	networkProvider.MockNextAccountBlockCoordinates.Hash = "abba"

	response, err = getAccountBalance(service, testscommon.TestAddressAlice)
	require.Nil(t, err)
	require.Equal(t, "100", response.Balances[0].Value)
	require.Equal(t, int64(42), response.BlockIdentifier.Index)
	require.Equal(t, "abba", response.BlockIdentifier.Hash)
}

func getAccountBalance(service server.AccountAPIServicer, address string) (*types.AccountBalanceResponse, *types.Error) {
	return service.AccountBalance(context.Background(), &types.AccountBalanceRequest{
		AccountIdentifier: &types.AccountIdentifier{Address: address},
	})
}
