package services

import (
	"github.com/ElrondNetwork/elrond-go-core/core"
	"github.com/ElrondNetwork/elrond-go-core/core/pubkeyConverter"
	"github.com/ElrondNetwork/elrond-go-core/data/transaction"
	"github.com/ElrondNetwork/elrond-proxy-go/data"
)

var bech32PubkeyConverter, _ = pubkeyConverter.NewBech32PubkeyConverter(32, log)

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
func filterOutContractResultsWithDataHavingContractSenderSameAsReceiver(txs []*data.FullTransaction) ([]*data.FullTransaction, error) {
	filteredTxs := make([]*data.FullTransaction, 0, len(txs))

	for _, tx := range txs {
		isContractResult := tx.Type == string(transaction.TxTypeUnsigned)
		hasData := len(tx.Data) > 0
		isSenderContract := isSmartContractAddress(tx.Sender)
		isSenderSameAsReceiver := tx.Sender == tx.Receiver

		if isContractResult && hasData && isSenderContract && isSenderSameAsReceiver {
			continue
		}

		filteredTxs = append(filteredTxs, tx)
	}

	return filteredTxs, nil
}

func isSmartContractAddress(address string) bool {
	pubkey, err := bech32PubkeyConverter.Decode(address)
	if err != nil {
		// E.g., when address = "metachain"
		return false
	}

	return core.IsSmartContractAddress(pubkey)
}
