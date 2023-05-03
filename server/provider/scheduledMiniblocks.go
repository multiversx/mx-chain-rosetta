package provider

import (
	"github.com/multiversx/mx-chain-core-go/data/api"
	dataBlock "github.com/multiversx/mx-chain-core-go/data/block"
	"github.com/multiversx/mx-chain-core-go/data/transaction"
)

func (provider *networkProvider) simplifyBlockWithScheduledTransactions(block *api.Block) error {
	previousBlock, err := provider.doGetBlockByNonce(block.Nonce - 1)
	if err != nil {
		return err
	}

	nextBlock, err := provider.doGetBlockByNonce(block.Nonce + 1)
	if err != nil {
		return err
	}

	doSimplifyBlockWithScheduledTransactions(previousBlock, block, nextBlock)
	deduplicatePreviouslyAppearingContractResults(previousBlock, block)

	return nil
}

func doSimplifyBlockWithScheduledTransactions(previousBlock *api.Block, block *api.Block, nextBlock *api.Block) {
	// Discard "processed" miniblocks in block N, since they already produced effects in N-1
	removeProcessedMiniblocksOfBlock(block)

	// Move "processed" miniblocks from N+1 to N
	processedMiniblocksInNextBlock := findProcessedMiniblocks(nextBlock)
	appendMiniblocksToBlock(block, processedMiniblocksInNextBlock)

	// Build an artificial miniblock holding the "invalid" transactions that produced their effects in block N,
	// and replace the existing (one or two "invalid" miniblocks).
	invalidTxs := gatherInvalidTransactions(previousBlock, block, nextBlock)
	invalidMiniblock := &api.MiniBlock{
		Type:         dataBlock.InvalidBlock.String(),
		Transactions: invalidTxs,
	}
	removeInvalidMiniblocks(block)

	if len(invalidMiniblock.Transactions) > 0 {
		appendMiniblocksToBlock(block, []*api.MiniBlock{invalidMiniblock})
	}

	// Discard "scheduled" miniblocks of N, since we've already brought the "processed" ones from N+1,
	// and also handled the "invalid" ones.
	removeScheduledMiniblocks(block)
}

func removeProcessedMiniblocksOfBlock(block *api.Block) {
	removeMiniblocksFromBlock(block, func(miniblock *api.MiniBlock) bool {
		return miniblock.ProcessingType == dataBlock.Processed.String()
	})
}

func removeScheduledMiniblocks(block *api.Block) {
	removeMiniblocksFromBlock(block, func(miniblock *api.MiniBlock) bool {
		hasProcessingTypeScheduled := miniblock.ProcessingType == dataBlock.Scheduled.String()
		hasConstructionStateNotFinal := miniblock.ConstructionState != dataBlock.Final.String()
		shouldRemove := hasProcessingTypeScheduled && hasConstructionStateNotFinal
		return shouldRemove
	})
}

func removeInvalidMiniblocks(block *api.Block) {
	removeMiniblocksFromBlock(block, func(miniblock *api.MiniBlock) bool {
		return miniblock.Type == dataBlock.InvalidBlock.String()
	})
}

func gatherInvalidTransactions(previousBlock *api.Block, block *api.Block, nextBlock *api.Block) []*transaction.ApiTransactionResult {
	// Find "invalid" transactions that are "final" in N
	invalidTxsInBlock := findInvalidTransactions(block)
	// If also present in N-1, discard them
	scheduledTxsHashesPreviousBlock := findScheduledTransactionsHashes(previousBlock)
	invalidTxsInBlock = discardTransactions(invalidTxsInBlock, scheduledTxsHashesPreviousBlock)

	// Find "invalid" transactions in N+1 that are "scheduled" in N
	invalidTxsInNextBlock := findInvalidTransactions(nextBlock)
	scheduledTxsHashesInBlock := findScheduledTransactionsHashes(block)
	invalidTxsScheduledInBlock := filterTransactions(invalidTxsInNextBlock, scheduledTxsHashesInBlock)

	// Duplication might occur, since a block can contain two "invalid" miniblocks,
	// one added to block body, one saved in the receipts unit (at times, they have different content, different hashes).
	invalidTxs := append(invalidTxsInBlock, invalidTxsScheduledInBlock...)
	invalidTxs = deduplicateTransactions(invalidTxs)

	return invalidTxs
}

func findScheduledTransactionsHashes(block *api.Block) map[string]struct{} {
	txs := make(map[string]struct{})

	for _, miniblock := range block.MiniBlocks {
		hasProcessingTypeScheduled := miniblock.ProcessingType == dataBlock.Scheduled.String()
		hasConstructionStateNotFinal := miniblock.ConstructionState != dataBlock.Final.String()
		shouldAccumulateTxs := hasProcessingTypeScheduled && hasConstructionStateNotFinal

		if shouldAccumulateTxs {
			for _, tx := range miniblock.Transactions {
				txs[tx.Hash] = struct{}{}
			}
		}
	}

	return txs
}

func findProcessedMiniblocks(block *api.Block) []*api.MiniBlock {
	foundMiniblocks := make([]*api.MiniBlock, 0, len(block.MiniBlocks))

	for _, miniblock := range block.MiniBlocks {
		if miniblock.ProcessingType == dataBlock.Processed.String() {
			foundMiniblocks = append(foundMiniblocks, miniblock)
		}
	}

	return foundMiniblocks
}

func findInvalidTransactions(block *api.Block) []*transaction.ApiTransactionResult {
	invalidTxs := make([]*transaction.ApiTransactionResult, 0)

	for _, miniblock := range block.MiniBlocks {
		if miniblock.Type == dataBlock.InvalidBlock.String() {
			for _, tx := range miniblock.Transactions {
				invalidTxs = append(invalidTxs, tx)
			}
		}
	}

	return invalidTxs
}

// Sometimes, an invalid transaction processed in a scheduled miniblock
// might have its smart contract result (if any) saved in the receipts unit of both blocks N and N+1.
// This function removes the duplicate entries in block N.
func deduplicatePreviouslyAppearingContractResults(previousBlock *api.Block, block *api.Block) {
	previouslyAppearingContractResultsHashes := make(map[string]struct{})

	// First, gather the hashes of SCRs in "Normal" miniblocks, in block N-1.
	for _, miniblock := range previousBlock.MiniBlocks {
		isResultsMiniblock := miniblock.Type == dataBlock.SmartContractResultBlock.String()
		isNormalMiniblock := miniblock.ProcessingType == dataBlock.Normal.String()
		shouldHandleMiniblock := isResultsMiniblock && isNormalMiniblock

		if !shouldHandleMiniblock {
			continue
		}

		for _, tx := range miniblock.Transactions {
			previouslyAppearingContractResultsHashes[tx.Hash] = struct{}{}
		}
	}

	// Now, remove the duplicate entries in block N.
	for _, miniblock := range block.MiniBlocks {
		isResultsMiniblock := miniblock.Type == dataBlock.SmartContractResultBlock.String()
		isNormalMiniblock := miniblock.ProcessingType == dataBlock.Normal.String()
		shouldHandleMiniblock := isResultsMiniblock && isNormalMiniblock

		if !shouldHandleMiniblock {
			continue
		}

		transactions := make([]*transaction.ApiTransactionResult, 0, len(miniblock.Transactions))

		for _, tx := range miniblock.Transactions {
			_, isDuplicated := previouslyAppearingContractResultsHashes[tx.Hash]
			if !isDuplicated {
				transactions = append(transactions, tx)
			}
		}

		miniblock.Transactions = transactions
	}
}
