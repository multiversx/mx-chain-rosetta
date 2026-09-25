package services

import (
	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/multiversx/mx-chain-core-go/data/transaction"
)

func failedMoveBalanceTransfers(txs []*transaction.ApiTransactionResult, shard uint32) map[string]*transaction.ApiTransactionResult {
	var failed map[string]*transaction.ApiTransactionResult
	for _, tx := range txs {
		if tx.Type == string(transaction.TxTypeNormal) && tx.Status == transaction.TxStatusFail &&
			tx.DestinationShard == shard &&
			tx.ProcessingTypeOnSource == transactionProcessingTypeMoveBalance &&
			tx.ProcessingTypeOnDestination == transactionProcessingTypeMoveBalance {
			if failed == nil {
				failed = make(map[string]*transaction.ApiTransactionResult)
			}
			failed[tx.Hash] = tx
		}
	}
	return failed
}

func applyFailedMoveBalanceOperations(tx *transaction.ApiTransactionResult, result *types.Transaction, failed map[string]*transaction.ApiTransactionResult) {
	if _, found := failed[tx.Hash]; found {
		for _, operation := range result.Operations {
			if operation.Type == opTransfer {
				operation.Status = &opStatusFailure
			}
		}
		return
	}

	original, found := failed[tx.OriginalTransactionHash]
	if !found || tx.Type != string(transaction.TxTypeUnsigned) || tx.IsRefund ||
		tx.Sender != original.Receiver || tx.Receiver != original.Sender || tx.Value != original.Value {
		return
	}

	// The destination never received the value, so its mirrored refund is not a debit.
	// The refund credit still takes effect when processed on the source shard.
	for _, operation := range result.Operations {
		if operation.Type == opScResult && operation.Account.Address == tx.Sender {
			operation.Status = &opStatusFailure
		}
	}
}
