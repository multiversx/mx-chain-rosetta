package services

import (
	"github.com/ElrondNetwork/elrond-go-core/data/transaction"
	"github.com/ElrondNetwork/elrond-proxy-go/data"
)

func filterOutIntrashardContractResultsWhoseOriginalTransactionIsInInvalidMiniblock(txs []*data.FullTransaction) []*data.FullTransaction {
	filteredTxs := make([]*data.FullTransaction, 0, len(txs))
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

func filterOutIntrashardRelayedTransactionAlreadyHeldInInvalidMiniblock(txs []*data.FullTransaction) []*data.FullTransaction {
	filteredTxs := make([]*data.FullTransaction, 0, len(txs))
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

func filterOutContractResultsWithNoValue(txs []*data.FullTransaction) []*data.FullTransaction {
	filteredTxs := make([]*data.FullTransaction, 0, len(txs))

	for _, tx := range txs {
		isContractResult := tx.Type == string(transaction.TxTypeUnsigned)
		hasValue := tx.Value != "0" && tx.Value != ""
		hasNegativeValue := hasValue && tx.Value[0] == '-'

		if isContractResult && (!hasValue || hasNegativeValue) {
			continue
		}

		filteredTxs = append(filteredTxs, tx)
	}

	return filteredTxs
}

// This will not filter out, for example, SCRs of ClaimDeveloperRewards (since they do not have a data field)
func filterOutContractResultsWithDataHavingSenderSameAsReceiver(txs []*data.FullTransaction) []*data.FullTransaction {
	filteredTxs := make([]*data.FullTransaction, 0, len(txs))

	for _, tx := range txs {
		isContractResult := tx.Type == string(transaction.TxTypeUnsigned)
		hasData := len(tx.Data) > 0
		isSenderSameAsReceiver := tx.Sender == tx.Receiver

		if isContractResult && hasData && isSenderSameAsReceiver {
			continue
		}

		filteredTxs = append(filteredTxs, tx)
	}

	return filteredTxs
}
