package services

import (
	"context"
	"testing"

	"github.com/ElrondNetwork/elrond-go-core/data/transaction"
	"github.com/ElrondNetwork/elrond-proxy-go/data"
	"github.com/ElrondNetwork/rosetta/testscommon"
	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/stretchr/testify/require"
)

func TestMempoolService_MempoolTransactionCannotFindTxInPool(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	service := NewMempoolService(networkProvider)

	txResponse, err := getMempoolTransactionByHash(service, "aaaa")
	require.Equal(t, ErrTransactionIsNotInPool.Code, err.Code)
	require.Nil(t, txResponse)
}

func TestMempoolService_MempoolTransaction(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	extension := newNetworkProviderExtension(networkProvider)
	service := NewMempoolService(networkProvider)

	networkProvider.MockMempoolTransactionsByHash["aaaa"] = &data.FullTransaction{
		Hash:     "aaaa",
		Type:     string(transaction.TxTypeNormal),
		Receiver: testscommon.TestAddressBob,
		Sender:   testscommon.TestAddressAlice,
		Value:    "1234",
		GasLimit: 50000,
		GasPrice: 1000000000,
	}

	expectedRosettaTx := &types.Transaction{
		TransactionIdentifier: hashToTransactionIdentifier("aaaa"),
		Operations: []*types.Operation{
			{
				OperationIdentifier: indexToOperationIdentifier(0),
				Type:                opTransfer,
				Account:             addressToAccountIdentifier(testscommon.TestAddressAlice),
				Amount:              extension.valueToNativeAmount("-1234"),
			},
			{
				OperationIdentifier: indexToOperationIdentifier(1),
				Type:                opTransfer,
				Account:             addressToAccountIdentifier(testscommon.TestAddressBob),
				Amount:              extension.valueToNativeAmount("1234"),
			},
		},
	}

	txResponse, err := getMempoolTransactionByHash(service, "aaaa")
	require.Nil(t, err)
	require.Equal(t, expectedRosettaTx, txResponse.Transaction)
}

func getMempoolTransactionByHash(service server.MempoolAPIServicer, hash string) (*types.MempoolTransactionResponse, *types.Error) {
	return service.MempoolTransaction(context.Background(), &types.MempoolTransactionRequest{
		NetworkIdentifier:     nil,
		TransactionIdentifier: &types.TransactionIdentifier{Hash: hash},
	})
}
