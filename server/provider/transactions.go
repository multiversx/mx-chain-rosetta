package provider

import "github.com/multiversx/mx-chain-core-go/data/transaction"

func discardTransactions(txs []*transaction.ApiTransactionResult, txsHashesToDiscard map[string]struct{}) []*transaction.ApiTransactionResult {
	txsToKeep := make([]*transaction.ApiTransactionResult, 0, len(txs))

	for _, tx := range txs {
		_, shouldDiscard := txsHashesToDiscard[tx.Hash]
		if shouldDiscard {
			continue
		}

		txsToKeep = append(txsToKeep, tx)
	}

	return txsToKeep
}

func filterTransactions(txs []*transaction.ApiTransactionResult, txsHashesToKeep map[string]struct{}) []*transaction.ApiTransactionResult {
	txsToKeep := make([]*transaction.ApiTransactionResult, 0, len(txs))

	for _, tx := range txs {
		_, shouldKeep := txsHashesToKeep[tx.Hash]
		if shouldKeep {
			txsToKeep = append(txsToKeep, tx)
		}
	}

	return txsToKeep
}

func deduplicateTransactions(txs []*transaction.ApiTransactionResult) []*transaction.ApiTransactionResult {
	deduplicatedTxs := make([]*transaction.ApiTransactionResult, 0, len(txs))
	seenTxsHashes := make(map[string]struct{})

	for _, tx := range txs {
		_, alreadySeen := seenTxsHashes[tx.Hash]
		if alreadySeen {
			continue
		}

		deduplicatedTxs = append(deduplicatedTxs, tx)
		seenTxsHashes[tx.Hash] = struct{}{}
	}

	return deduplicatedTxs
}
