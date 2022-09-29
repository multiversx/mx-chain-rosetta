package services

import (
	"github.com/ElrondNetwork/elrond-go-core/data/transaction"
	"github.com/coinbase/rosetta-sdk-go/types"
)

func filterOutIntrashardContractResultsWhoseOriginalTransactionIsInInvalidMiniblock(txs []*transaction.ApiTransactionResult) []*transaction.ApiTransactionResult {
	filteredTxs := make([]*transaction.ApiTransactionResult, 0, len(txs))
	invalidTxs := make(map[string]struct{})

	for _, tx := range txs {
		if tx.Type == string(transaction.TxTypeInvalid) {
			invalidTxs[tx.Hash] = struct{}{}
		}
	}

	for _, tx := range txs {
		isContractResult := tx.Type == string(transaction.TxTypeUnsigned)
		_, isResultOfInvalid := invalidTxs[tx.OriginalTransactionHash]

		if isContractResult && isResultOfInvalid {
			continue
		}

		filteredTxs = append(filteredTxs, tx)
	}

	return filteredTxs
}

func filterOutIntrashardRelayedTransactionAlreadyHeldInInvalidMiniblock(txs []*transaction.ApiTransactionResult) []*transaction.ApiTransactionResult {
	filteredTxs := make([]*transaction.ApiTransactionResult, 0, len(txs))
	invalidTxs := make(map[string]struct{})

	for _, tx := range txs {
		if tx.Type == string(transaction.TxTypeInvalid) {
			invalidTxs[tx.Hash] = struct{}{}
		}
	}

	for _, tx := range txs {
		isRelayedTransaction := (tx.Type == string(transaction.TxTypeNormal)) &&
			(tx.ProcessingTypeOnSource == transactionProcessingTypeRelayed) &&
			(tx.ProcessingTypeOnDestination == transactionProcessingTypeRelayed)

		_, alreadyHeldInInvalidMiniblock := invalidTxs[tx.Hash]

		if isRelayedTransaction && alreadyHeldInInvalidMiniblock {
			continue
		}

		filteredTxs = append(filteredTxs, tx)
	}

	return filteredTxs
}

func filterOutRosettaTransactionsWithNoOperations(rosettaTxs []*types.Transaction) []*types.Transaction {
	filtered := make([]*types.Transaction, 0, len(rosettaTxs))

	for _, rosettaTx := range rosettaTxs {
		if len(rosettaTx.Operations) > 0 {
			filtered = append(filtered, rosettaTx)
		}
	}

	return filtered
}
