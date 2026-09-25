package services

import (
	"math/big"
	"testing"

	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/multiversx/mx-chain-core-go/data/api"
	"github.com/multiversx/mx-chain-core-go/data/transaction"
	"github.com/multiversx/mx-chain-rosetta/testscommon"
	"github.com/stretchr/testify/require"
)

func TestSupernovaProposalOnlyTransactionsHaveNoEffects(t *testing.T) {
	provider := testscommon.NewNetworkProviderMock()
	transformer := newTransactionsTransformer(provider)
	for _, txType := range []transaction.TxType{transaction.TxTypeNormal, transaction.TxTypeUnsigned, transaction.TxTypeInvalid} {
		tx := &transaction.ApiTransactionResult{
			Hash: "proposal", Type: string(txType), Status: transaction.TxStatusNotExecutable,
			Sender: testscommon.TestAddressBob, Receiver: testscommon.TestAddressBob,
			Value: "100", InitiallyPaidFee: "10",
			Logs: &transaction.ApiLogs{Events: []*transaction.Events{{Identifier: transactionEventSCDeploy}}},
		}
		block := &api.Block{Nonce: 10, LastExecutionResult: &api.ExecutionResult{},
			MiniBlocks: []*api.MiniBlock{{Transactions: []*transaction.ApiTransactionResult{tx}}}}
		result, err := transformer.transformBlockTxs(block)
		require.NoError(t, err)
		require.Empty(t, result)
		require.Zero(t, tx.BlockNonce)
	}
}

func TestFailedCrossShardMoveBalanceReconciles(t *testing.T) {
	provider := testscommon.NewNetworkProviderMock()
	transformer := newTransactionsTransformer(provider)
	tx := &transaction.ApiTransactionResult{
		Hash: "transfer", Type: string(transaction.TxTypeNormal), Status: transaction.TxStatusPending,
		Sender: testscommon.TestAddressAlice, Receiver: testscommon.TestAddressBob,
		SourceShard: 1, DestinationShard: 0, Value: "100", InitiallyPaidFee: "10",
		ProcessingTypeOnSource:      transactionProcessingTypeMoveBalance,
		ProcessingTypeOnDestination: transactionProcessingTypeMoveBalance,
		ReceiverUsername:            []byte("wrong"),
	}
	refund := &transaction.ApiTransactionResult{
		Hash: "refund", OriginalTransactionHash: tx.Hash, Type: string(transaction.TxTypeUnsigned),
		Status: transaction.TxStatusSuccess, Sender: tx.Receiver, Receiver: tx.Sender,
		SourceShard: 0, DestinationShard: 1, Value: "100",
	}
	transform := func(shard uint32, txs ...*transaction.ApiTransactionResult) []*types.Transaction {
		provider.MockObservedActualShard = shard
		result, err := transformer.transformBlockTxs(&api.Block{
			Nonce: 10, Shard: shard, LastExecutionResult: &api.ExecutionResult{},
			MiniBlocks: []*api.MiniBlock{{Transactions: txs}},
		})
		require.NoError(t, err)
		return result
	}
	before := transform(1, tx)
	require.Equal(t, "-110", successfulDelta(before, tx.Sender))
	tx.Status = transaction.TxStatusFail
	destination := transform(0, tx, refund)
	require.Equal(t, "0", successfulDelta(destination, tx.Receiver))
	for _, item := range destination {
		for _, op := range item.Operations {
			require.Equal(t, opStatusFailure, *op.Status)
			require.Contains(t, op.Metadata, "nodeTransactionStatus")
		}
	}
	after := transform(1, refund)
	require.Equal(t, "100", successfulDelta(after, tx.Sender))
	reversed := transform(0, refund, tx)
	require.Equal(t, "0", successfulDelta(reversed, tx.Receiver))
	require.Zero(t, tx.BlockNonce)
	require.Zero(t, refund.BlockNonce)
}

func TestFailedIntrashardMoveBalanceRetainsFee(t *testing.T) {
	provider := testscommon.NewNetworkProviderMock()
	transformer := newTransactionsTransformer(provider)
	tx := &transaction.ApiTransactionResult{
		Hash: "failed", Type: string(transaction.TxTypeNormal), Status: transaction.TxStatusFail,
		Sender: testscommon.TestUserAShard0.Address, Receiver: testscommon.TestUserBShard0.Address,
		Value: "100", InitiallyPaidFee: "10",
		ProcessingTypeOnSource:      transactionProcessingTypeMoveBalance,
		ProcessingTypeOnDestination: transactionProcessingTypeMoveBalance,
	}
	result, err := transformer.transformBlockTxs(&api.Block{
		MiniBlocks: []*api.MiniBlock{{Transactions: []*transaction.ApiTransactionResult{tx}}},
	})
	require.NoError(t, err)
	require.Equal(t, "-10", successfulDelta(result, tx.Sender))
	require.Equal(t, "0", successfulDelta(result, tx.Receiver))
}

func TestExecutedInvalidAndSuccessfulTransactionsRemain(t *testing.T) {
	provider := testscommon.NewNetworkProviderMock()
	transformer := newTransactionsTransformer(provider)
	for _, txType := range []transaction.TxType{transaction.TxTypeNormal, transaction.TxTypeInvalid} {
		tx := &transaction.ApiTransactionResult{
			Hash: "tx", Type: string(txType), Status: transaction.TxStatusSuccess,
			Sender: testscommon.TestAddressBob, Receiver: testscommon.TestAddressAlice,
			Value: "100", InitiallyPaidFee: "10",
		}
		result, err := transformer.transformBlockTxs(&api.Block{Nonce: 10,
			MiniBlocks: []*api.MiniBlock{{Transactions: []*transaction.ApiTransactionResult{tx}}}})
		require.NoError(t, err)
		expected := "-110"
		if txType == transaction.TxTypeInvalid {
			expected = "-10"
		}
		require.Equal(t, expected, successfulDelta(result, tx.Sender))
	}
}

func TestFailedMoveBalanceDoesNotChangeOtherEffects(t *testing.T) {
	tx := &transaction.ApiTransactionResult{
		Hash: "tx", Type: string(transaction.TxTypeNormal), Status: transaction.TxStatusFail,
		Sender: testscommon.TestAddressAlice, Receiver: testscommon.TestAddressBob,
		SourceShard: 1, DestinationShard: 0, Value: "100",
		ProcessingTypeOnSource:      transactionProcessingTypeMoveBalance,
		ProcessingTypeOnDestination: transactionProcessingTypeMoveBalance,
	}
	failed := findFailedMoveBalanceTransfers([]*transaction.ApiTransactionResult{tx}, 0)
	for _, test := range []struct {
		name   string
		change func(*transaction.ApiTransactionResult)
	}{
		{"other parent", func(scr *transaction.ApiTransactionResult) { scr.OriginalTransactionHash = "other" }},
		{"other value", func(scr *transaction.ApiTransactionResult) { scr.Value = "99" }},
		{"other receiver", func(scr *transaction.ApiTransactionResult) { scr.Receiver = testscommon.TestAddressCarol }},
		{"other sender", func(scr *transaction.ApiTransactionResult) { scr.Sender = testscommon.TestAddressCarol }},
		{"other type", func(scr *transaction.ApiTransactionResult) { scr.Type = string(transaction.TxTypeNormal) }},
		{"fee refund", func(scr *transaction.ApiTransactionResult) { scr.IsRefund = true }},
	} {
		t.Run(test.name, func(t *testing.T) {
			scr := &transaction.ApiTransactionResult{
				Hash: "scr", OriginalTransactionHash: tx.Hash, Type: string(transaction.TxTypeUnsigned),
				Sender: tx.Receiver, Receiver: tx.Sender, Value: "100",
			}
			test.change(scr)
			result := &types.Transaction{Operations: []*types.Operation{{
				Type: opScResult, Account: addressToAccountIdentifier(scr.Sender),
			}}}
			applyFailedMoveBalanceOperations(scr, result, failed)
			require.Nil(t, result.Operations[0].Status)
		})
	}
	require.Empty(t, findFailedMoveBalanceTransfers([]*transaction.ApiTransactionResult{tx}, 1))
	tx.Status = transaction.TxStatusSuccess
	require.Empty(t, findFailedMoveBalanceTransfers([]*transaction.ApiTransactionResult{tx}, 0))
	tx.Status = transaction.TxStatusFail
	tx.ProcessingTypeOnDestination = transactionProcessingTypeContractInvoking
	require.Empty(t, findFailedMoveBalanceTransfers([]*transaction.ApiTransactionResult{tx}, 0))
}

func TestSupernovaExecutionEffectsReportedOnlyInTheirOwnBlock(t *testing.T) {
	provider := testscommon.NewNetworkProviderMock()
	transformer := newTransactionsTransformer(provider)
	executed := &transaction.ApiTransactionResult{
		Hash: "executed", Type: string(transaction.TxTypeNormal), Status: transaction.TxStatusSuccess,
		Sender: testscommon.TestAddressBob, Receiver: testscommon.TestAddressAlice,
		Value: "100", InitiallyPaidFee: "10",
	}
	proposalOnly := *executed
	proposalOnly.Status = transaction.TxStatusNotExecutable
	refund := &transaction.ApiTransactionResult{
		Hash: "gas-refund", Type: string(transaction.TxTypeUnsigned), Status: transaction.TxStatusSuccess,
		Sender: testscommon.TestAddressAlice, Receiver: testscommon.TestAddressBob,
		Value: "2", IsRefund: true,
	}
	miniBlocks := []*api.MiniBlock{{Transactions: []*transaction.ApiTransactionResult{executed, refund}}}
	later := *executed
	later.Hash, later.Value, later.InitiallyPaidFee = "later", "30", "3"
	resultReference := &api.ExecutionResult{HeaderHash: "block-ten", HeaderNonce: 10, MiniBlocks: miniBlocks}
	blocks := []*api.Block{
		{Nonce: 10, Hash: "block-ten", LastExecutionResult: &api.ExecutionResult{},
			MiniBlocks: append([]*api.MiniBlock{{Transactions: []*transaction.ApiTransactionResult{&proposalOnly}}}, miniBlocks...)},
		{Nonce: 11, LastExecutionResult: &api.ExecutionResult{}},
		{Nonce: 12, LastExecutionResult: resultReference, ExecutionResults: []*api.ExecutionResult{resultReference},
			MiniBlocks: []*api.MiniBlock{{Transactions: []*transaction.ApiTransactionResult{&later}}}},
		{Nonce: 13, LastExecutionResult: resultReference},
	}
	counts := make(map[string]int)
	all := make([]*types.Transaction, 0)
	for _, block := range blocks {
		converted, err := transformer.transformBlockTxs(block)
		require.NoError(t, err)
		if block.Nonce == 11 || block.Nonce == 13 {
			require.Empty(t, converted, "referencing execution results must not repeat their effects")
		}
		if block.Nonce == 12 {
			require.Len(t, converted, 1)
			require.Equal(t, "later", converted[0].TransactionIdentifier.Hash)
		}
		again, err := transformer.transformBlockTxs(block)
		require.NoError(t, err)
		require.Equal(t, converted, again)
		for _, tx := range converted {
			counts[tx.TransactionIdentifier.Hash]++
			for _, operation := range tx.Operations {
				require.Equal(t, "success", operation.Metadata["nodeTransactionStatus"])
			}
		}
		all = append(all, converted...)
	}
	require.Equal(t, map[string]int{"executed": 1, "gas-refund": 1, "later": 1}, counts)
	require.Equal(t, "-141", successfulDelta(all, executed.Sender))
	require.Zero(t, executed.BlockNonce)
	require.Zero(t, refund.BlockNonce)
	require.Equal(t, transaction.TxStatusNotExecutable, proposalOnly.Status)
}

func successfulDelta(txs []*types.Transaction, address string) string {
	total := new(big.Int)
	for _, tx := range txs {
		for _, op := range tx.Operations {
			if op.Account.Address == address && op.Status != nil && *op.Status == opStatusSuccess {
				value, ok := new(big.Int).SetString(op.Amount.Value, 10)
				if !ok {
					panic("invalid test amount")
				}
				total.Add(total, value)
			}
		}
	}
	return total.String()
}
