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

func TestBlockService_BlockByIndex(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	networkProvider.MockNetworkConfig.ChainID = "T"
	networkProvider.MockNumShards = 1
	extension := newNetworkProviderExtension(networkProvider)

	networkProvider.MockBlocksByNonce[7] = &data.Block{
		Hash:          "0007",
		Nonce:         7,
		Timestamp:     1,
		PrevBlockHash: "0006",
		Epoch:         1,
		Round:         42,
		Status:        "on-chain",
		MiniBlocks: []*data.MiniBlock{
			{
				Transactions: []*data.FullTransaction{
					{
						Hash:             "aaaa",
						Type:             string(transaction.TxTypeNormal),
						Sender:           testscommon.TestAddressAlice,
						Receiver:         testscommon.TestAddressBob,
						Value:            "1",
						InitiallyPaidFee: "50000000000000",
					},
				},
			},
		},
	}

	networkProvider.MockBlocksByNonce[8] = &data.Block{
		Hash:          "0008",
		Nonce:         8,
		Timestamp:     2,
		PrevBlockHash: "0007",
		Epoch:         1,
		Round:         43,
		Status:        "on-chain",
		MiniBlocks:    []*data.MiniBlock{{Transactions: []*data.FullTransaction{}}},
	}

	service := NewBlockService(networkProvider)

	blockSeven := &types.Block{
		BlockIdentifier:       &types.BlockIdentifier{Index: 7, Hash: "0007"},
		ParentBlockIdentifier: &types.BlockIdentifier{Index: 6, Hash: "0006"},
		Timestamp:             1000,
		Transactions: []*types.Transaction{
			{
				TransactionIdentifier: hashToTransactionIdentifier("aaaa"),
				Operations: []*types.Operation{
					{
						OperationIdentifier: indexToOperationIdentifier(0),
						Type:                opTransfer,
						Account:             addressToAccountIdentifier(testscommon.TestAddressAlice),
						Amount:              extension.valueToNativeAmount("-1"),
						Status:              &opStatusSuccess,
					},
					{
						OperationIdentifier: indexToOperationIdentifier(1),
						Type:                opTransfer,
						Account:             addressToAccountIdentifier(testscommon.TestAddressBob),
						Amount:              extension.valueToNativeAmount("1"),
						Status:              &opStatusSuccess,
					},
					{
						OperationIdentifier: indexToOperationIdentifier(2),
						Type:                opFee,
						Account:             addressToAccountIdentifier(testscommon.TestAddressAlice),
						Amount:              extension.valueToNativeAmount("-50000000000000"),
						Status:              &opStatusSuccess,
					},
				},
			},
		},
		Metadata: objectsMap{
			"epoch":  uint32(1),
			"round":  uint64(42),
			"shard":  uint32(0),
			"status": "on-chain",
		},
	}

	blockEight := &types.Block{
		BlockIdentifier:       &types.BlockIdentifier{Index: 8, Hash: "0008"},
		ParentBlockIdentifier: &types.BlockIdentifier{Index: 7, Hash: "0007"},
		Timestamp:             2000,
		Transactions:          []*types.Transaction{},
		Metadata: objectsMap{
			"epoch":  uint32(1),
			"round":  uint64(43),
			"shard":  uint32(0),
			"status": "on-chain",
		},
	}

	_, err := getBlockByIndex(service, 6)
	require.Equal(t, ErrUnableToGetBlock, errCode(err.Code))

	blockResponse, err := getBlockByIndex(service, 7)
	require.Nil(t, err)
	require.Equal(t, blockSeven, blockResponse.Block)

	blockResponse, err = getBlockByIndex(service, 8)
	require.Nil(t, err)
	require.Equal(t, blockEight, blockResponse.Block)
}

func getBlockByIndex(service server.BlockAPIServicer, index int64) (*types.BlockResponse, *types.Error) {
	return service.Block(context.Background(), &types.BlockRequest{
		NetworkIdentifier: nil,
		BlockIdentifier: &types.PartialBlockIdentifier{
			Index: &index,
			Hash:  nil,
		},
	})
}
