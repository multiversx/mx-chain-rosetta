package services

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/ElrondNetwork/elrond-go-core/core"
	"github.com/ElrondNetwork/elrond-proxy-go/data"
	"github.com/ElrondNetwork/rosetta/testscommon"
	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/stretchr/testify/require"
)

func TestTransactionsTransformer_UnsignedTxToRosettaTx(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	extension := newNetworkProviderExtension(networkProvider)
	transformer := newTransactionsTransformer(networkProvider)

	refundTx := &data.FullTransaction{
		Hash:     "aaaa",
		Sender:   testscommon.TestAddressOfContract,
		Receiver: testscommon.TestAddressAlice,
		Value:    "1234",
		IsRefund: true,
	}

	expectedRefundTx := &types.Transaction{
		TransactionIdentifier: hashToTransactionIdentifier("aaaa"),
		Operations: []*types.Operation{
			{
				Type:    opScResult,
				Account: addressToAccountIdentifier(testscommon.TestAddressAlice),
				Amount:  extension.valueToNativeAmount("1234"),
			},
		},
	}

	moveBalanceTx := &data.FullTransaction{
		Hash:     "aaaa",
		Sender:   testscommon.TestAddressOfContract,
		Receiver: testscommon.TestAddressAlice,
		Value:    "1234",
	}

	expectedMoveBalanceTx := &types.Transaction{
		TransactionIdentifier: hashToTransactionIdentifier("aaaa"),
		Operations: []*types.Operation{
			{
				Type:    opScResult,
				Account: addressToAccountIdentifier(testscommon.TestAddressOfContract),
				Amount:  extension.valueToNativeAmount("-1234"),
			},
			{
				Type:    opScResult,
				Account: addressToAccountIdentifier(testscommon.TestAddressAlice),
				Amount:  extension.valueToNativeAmount("1234"),
			},
		},
	}

	txsInBlock := []*data.FullTransaction{refundTx, moveBalanceTx}

	rosettaFefundTx := transformer.unsignedTxToRosettaTx(refundTx, txsInBlock)
	rosettaMoveBalanceTx := transformer.unsignedTxToRosettaTx(moveBalanceTx, txsInBlock)
	require.Equal(t, expectedRefundTx, rosettaFefundTx)
	require.Equal(t, expectedMoveBalanceTx, rosettaMoveBalanceTx)
}

func TestTransactionsTransformer_TransformTxsOfBlockWithESDTIssue(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	extension := newNetworkProviderExtension(networkProvider)
	transformer := newTransactionsTransformer(networkProvider)

	blocks, err := readTestBlocks("testdata/blocks_with_esdt_issue.json")
	require.Nil(t, err)

	// Block 27497 (issue ESDT)
	txs, err := transformer.transformTxsOfBlock(blocks[0])
	require.Nil(t, err)
	require.Len(t, txs, 1)

	expectedTx := &types.Transaction{
		TransactionIdentifier: hashToTransactionIdentifier("851d90b7b0770c648de5945ca76d2ded62a540856467911db5e550ce6a959807"),
		Operations: []*types.Operation{
			{
				Type:                opTransfer,
				OperationIdentifier: indexToOperationIdentifier(0),
				Account:             addressToAccountIdentifier("erd1testnlersh4z0wsv8kjx39me4rmnvjkwu8dsaea7ukdvvc9z396qykv7z7"),
				Amount:              extension.valueToNativeAmount("-50000000000000000"),
				Status:              &opStatusSuccess,
			},
			{
				Type:                opFee,
				OperationIdentifier: indexToOperationIdentifier(1),
				Account:             addressToAccountIdentifier("erd1testnlersh4z0wsv8kjx39me4rmnvjkwu8dsaea7ukdvvc9z396qykv7z7"),
				Amount:              extension.valueToNativeAmount("-1220275000000000"),
				Status:              &opStatusSuccess,
			},
		},
	}

	require.Equal(t, expectedTx, txs[0])

	// Block 27501 (results of issue ESDT)
	txs, err = transformer.transformTxsOfBlock(blocks[1])
	require.Nil(t, err)
	require.Len(t, txs, 2)
}

func readTestBlocks(filePath string) ([]*data.Block, error) {
	var blocks []*data.Block

	err := readJson(filePath, &blocks)
	if err != nil {
		return nil, err
	}

	return blocks, nil
}

func readJson(filePath string, value interface{}) error {
	file, err := core.OpenFile(filePath)
	if err != nil {
		return err
	}

	content, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}

	err = json.Unmarshal(content, value)
	if err != nil {
		return err
	}

	return nil
}
