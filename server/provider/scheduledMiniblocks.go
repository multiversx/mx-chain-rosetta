package provider

import (
	dataBlock "github.com/ElrondNetwork/elrond-go-core/data/block"
	"github.com/ElrondNetwork/elrond-proxy-go/data"
)

// QUESTION FOR REVIEW: perhaps do not do any miniblock deduplication in the Node API, and simply do it here?
// (and rename function to "simplifyBlock()")

func (provider *networkProvider) simplifyBlockWithScheduledTransactions(block *data.Block) error {
	previousBlock, err := provider.GetBlockByNonce(block.Nonce - 1)
	if err != nil {
		return err
	}

	// Simply ignore miniblocks with processing type "processed"
	filterOutProcessedMiniblocksOnceScheduled(block)

	// De-duplicate invalid miniblocks (the one from receipts storage takes precedence)
	whenThereAreTwoInvalidMiniblocksKeepTheOneFromReceiptsStorage(block)

	// De-duplicate invalid transactions, with respect to the previous block
	deduplicateInvalidTransactions(block, previousBlock)

	return nil
}

func filterOutProcessedMiniblocksOnceScheduled(block *data.Block) {
	filteredMiniblocks := make([]*data.MiniBlock, 0, len(block.MiniBlocks))

	for _, miniblock := range block.MiniBlocks {
		if miniblock.ProcessingType == string(Processed) {
			log.Debug("filterOutProcessedMiniblocksOnceScheduled", "miniblock", miniblock.Hash)
		} else {
			filteredMiniblocks = append(filteredMiniblocks, miniblock)
		}
	}

	block.MiniBlocks = filteredMiniblocks
}

func whenThereAreTwoInvalidMiniblocksKeepTheOneFromReceiptsStorage(block *data.Block) {
	// Say a series of 5 invalid transactions (e.g. transfer ESDT and execute, with insufficient funds) are broadcasted.
	// Some might be normally processed (e.g. 3 of them), while the others might be captured in scheduled miniblocks (e.g. 2 of them).
	// However, the receipts storage will contain a miniblock of type "InvalidBlock" holding all 5 of them.
	// Thus, when there are two invalid miniblocks in a block, we discard the one that does not originate from the receipts storage.
	//
	// Extra note: when we only see one invalid miniblock in a block, we don't follow the value of "isFromReceiptsStorage",
	// since the Node API returns (when there are no invalid scheduled transactions) the invalid miniblock added in the block body, not the one from the receipts storage
	// (due to a logic that does a general deduplication of miniblocks in the API response, by hash).

	filteredMiniblocks := make([]*data.MiniBlock, 0, len(block.MiniBlocks))

	var invalidMiniblockFromReceiptsStorage *data.MiniBlock
	var invalidMiniblockFromHeader *data.MiniBlock

	for _, miniblock := range block.MiniBlocks {
		if miniblock.Type == dataBlock.InvalidBlock.String() {
			if miniblock.IsFromReceiptsStorage {
				invalidMiniblockFromReceiptsStorage = miniblock
			} else {
				invalidMiniblockFromHeader = miniblock
			}
		} else {
			filteredMiniblocks = append(filteredMiniblocks, miniblock)
		}
	}

	if invalidMiniblockFromReceiptsStorage != nil {
		filteredMiniblocks = append(filteredMiniblocks, invalidMiniblockFromReceiptsStorage)
	} else if invalidMiniblockFromHeader != nil {
		// Fallback to use the one from the block body (in the usual, non-scheduled context)
		filteredMiniblocks = append(filteredMiniblocks, invalidMiniblockFromHeader)
	}

	block.MiniBlocks = filteredMiniblocks
}

func deduplicateInvalidTransactions(block *data.Block, previousBlock *data.Block) map[string]struct{} {
	invalidTxsInThisBlock := findInvalidTransactionsHashes(block)
	invalidTxsInPreviousBlock := findInvalidTransactionsHashes(previousBlock)

	for _, miniblock := range block.MiniBlocks {
		if miniblock.ProcessingType == string(Scheduled) {
			// The invalid transactions are already in the "invalid" miniblock from receipts storage
			filterOutTransactionsFromMiniblock(
				"seen in invalid miniblock from receipts storage",
				miniblock,
				invalidTxsInThisBlock,
			)
		} else if miniblock.Type == dataBlock.InvalidBlock.String() {
			// In previous block, they were both "scheduled" and "invalid" (from receipts storage)
			filterOutTransactionsFromMiniblock(
				"seen in invalid miniblock from receipts storage, in previous block",
				miniblock,
				invalidTxsInPreviousBlock,
			)
		}
	}
}

func findInvalidTransactionsHashes(block *data.Block) map[string]struct{} {
	invalidTxs := make(map[string]struct{})

	for _, miniblock := range block.MiniBlocks {
		if miniblock.Type == dataBlock.InvalidBlock.String() {
			for _, tx := range miniblock.Transactions {
				invalidTxs[tx.Hash] = struct{}{}
			}
		}
	}

	return invalidTxs
}

func filterOutTransactionsFromMiniblock(reason string, miniblock *data.MiniBlock, txsToFilterOut map[string]struct{}) {
	filteredTxs := make([]*data.FullTransaction, 0, len(miniblock.Transactions))

	for _, tx := range miniblock.Transactions {
		_, shouldFilterOut := txsToFilterOut[tx.Hash]
		if shouldFilterOut {
			log.Debug("filterOutTransactionsFromMiniblock()", "reason", reason, "miniblock", miniblock.Hash, "tx", tx.Hash)
		} else {
			filteredTxs = append(filteredTxs, tx)
		}
	}

	miniblock.Transactions = filteredTxs
}
