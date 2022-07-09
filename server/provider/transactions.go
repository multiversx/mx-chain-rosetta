package provider

import "github.com/ElrondNetwork/elrond-proxy-go/data"

func discardTransactions(txs []*data.FullTransaction, txsHashesToDiscard map[string]struct{}) []*data.FullTransaction {
	txsToKeep := make([]*data.FullTransaction, 0, len(txs))

	for _, tx := range txs {
		_, shouldDiscard := txsHashesToDiscard[tx.Hash]
		if shouldDiscard {
			continue
		}

		txsToKeep = append(txsToKeep, tx)
	}

	return txsToKeep
}

func filterTransactions(txs []*data.FullTransaction, txsHashesToKeep map[string]struct{}) []*data.FullTransaction {
	txsToKeep := make([]*data.FullTransaction, 0, len(txs))

	for _, tx := range txs {
		_, shouldKeep := txsHashesToKeep[tx.Hash]
		if shouldKeep {
			txsToKeep = append(txsToKeep, tx)
		}
	}

	return txsToKeep
}

func deduplicateTransactions(txs []*data.FullTransaction) []*data.FullTransaction {
	deduplicatedTxs := make([]*data.FullTransaction, 0, len(txs))
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
