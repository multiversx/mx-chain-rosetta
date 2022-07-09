package provider

import (
	dataBlock "github.com/ElrondNetwork/elrond-go-core/data/block"
	"github.com/ElrondNetwork/elrond-proxy-go/data"
)

func (provider *networkProvider) simplifyBlockWithScheduledTransactions(block *data.Block) error {
	if hasOnlyNormalMiniblocks(block) {
		return nil
	}

	previousBlock, err := provider.doGetBlockByNonce(block.Nonce - 1)
	if err != nil {
		return err
	}

	nextBlock, err := provider.doGetBlockByNonce(block.Nonce + 1)
	if err != nil {
		return err
	}

	// Discard "processed" miniblocks in block N, since they already produced effects in N-1
	discardProcessedMiniblocks(block)

	// Move "processed" miniblocks from N+1 to N
	processedMiniblocksInNextBlock := findProcessedMiniblocks(nextBlock)
	appendMiniblocksToBlock(block, processedMiniblocksInNextBlock)

	// Find "invalid" transactions that are "final" in N
	invalidTxsInBlock := findInvalidTransactions(block)
	// If present in N-1, discard them
	scheduledTxsHashesPreviousBlock := findScheduledTransactionsHashes(previousBlock)
	invalidTxsInBlock = discardTransactions(invalidTxsInBlock, scheduledTxsHashesPreviousBlock)

	// Find "invalid" transactions in N+1 that are "scheduled" in N
	invalidTxsInNextBlock := findInvalidTransactions(nextBlock)
	scheduledTxsHashesInBlock := findScheduledTransactionsHashes(block)
	invalidTxsScheduledInBlock := filterTransactions(invalidTxsInNextBlock, scheduledTxsHashesInBlock)

	// Duplication might occur, since a block can contain two "invalid" miniblocks,
	// one added to block, one saved in the receipts unit (and they have different hashes).
	invalidTxs := append(invalidTxsInBlock, invalidTxsScheduledInBlock...)
	invalidTxs = deduplicateTransactions(invalidTxs)

	// Build an artificial miniblock holding the "invalid" transactions that produced their effects in block N,
	// and replace the existing (one or two "invalid" miniblocks).
	discardInvalidMiniblocks(block)
	appendMiniblocksToBlock(block, []*data.MiniBlock{
		{
			Type:         dataBlock.InvalidBlock.String(),
			Transactions: invalidTxs,
		},
	})

	// Also discard "scheduled" miniblocks of N, since we've already brought the "processed" ones from N+1,
	// and also handled the "invalid" ones.
	discardScheduledMiniblocks(block)

	return nil
}

func hasOnlyNormalMiniblocks(block *data.Block) bool {
	for _, miniblock := range block.MiniBlocks {
		if miniblock.ProcessingType != string(Normal) {
			return false
		}
	}

	return true
}

func discardProcessedMiniblocks(block *data.Block) {
	block.MiniBlocks = discardMiniblocks(block.MiniBlocks, func(miniblock *data.MiniBlock) bool {
		return miniblock.ProcessingType == string(Processed)
	})
}

func findScheduledTransactionsHashes(block *data.Block) map[string]struct{} {
	invalidTxs := make(map[string]struct{})

	for _, miniblock := range block.MiniBlocks {
		if miniblock.ProcessingType == string(Scheduled) {
			for _, tx := range miniblock.Transactions {
				invalidTxs[tx.Hash] = struct{}{}
			}
		}
	}

	return invalidTxs
}

func findProcessedMiniblocks(block *data.Block) []*data.MiniBlock {
	foundMiniblocks := make([]*data.MiniBlock, 0, len(block.MiniBlocks))

	for _, miniblock := range block.MiniBlocks {
		if miniblock.ProcessingType == string(Processed) {
			foundMiniblocks = append(foundMiniblocks, miniblock)
		}
	}

	return foundMiniblocks
}

func appendMiniblocksToBlock(block *data.Block, miniblocks []*data.MiniBlock) {
	block.MiniBlocks = append(block.MiniBlocks, miniblocks...)
}

func findInvalidTransactions(block *data.Block) []*data.FullTransaction {
	invalidTxs := make([]*data.FullTransaction, 0)

	for _, miniblock := range block.MiniBlocks {
		if miniblock.Type == dataBlock.InvalidBlock.String() {
			for _, tx := range miniblock.Transactions {
				invalidTxs = append(invalidTxs, tx)
			}
		}
	}

	return invalidTxs
}

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

func discardScheduledMiniblocks(block *data.Block) {
	block.MiniBlocks = discardMiniblocks(block.MiniBlocks, func(miniblock *data.MiniBlock) bool {
		return miniblock.ProcessingType == string(Scheduled)
	})
}

func discardInvalidMiniblocks(block *data.Block) {
	block.MiniBlocks = discardMiniblocks(block.MiniBlocks, func(miniblock *data.MiniBlock) bool {
		return miniblock.Type == dataBlock.InvalidBlock.String()
	})
}

func discardMiniblocks(miniblocks []*data.MiniBlock, predicate func(miniblock *data.MiniBlock) bool) []*data.MiniBlock {
	miniblocksToKeep := make([]*data.MiniBlock, 0, len(miniblocks))

	for _, miniblock := range miniblocks {
		if predicate(miniblock) {
			continue
		}

		miniblocksToKeep = append(miniblocksToKeep, miniblock)
	}

	return miniblocksToKeep
}
