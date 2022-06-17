package provider

import (
	"github.com/ElrondNetwork/elrond-go-core/data/transaction"
	"github.com/ElrondNetwork/elrond-proxy-go/data"
)

func (provider *networkProvider) simplifyBlockWrtScheduledTransactions(block *data.Block) error {
	// Simply ignore miniblocks with processing type "processed"
	filterOutProcessedMiniblocksOnceScheduled(block)

	// Let's check if there's any scheduled miniblock "from me" in the block.
	// If so, special handling of invalid transactions is required.
	containsScheduledMiniblocksFromMe := provider.blockContainsScheduledMiniblockFromMe(block)
	if !containsScheduledMiniblocksFromMe {
		return nil
	}

	// If so, get the block N+1 and look for invalid miniblocks holding transactions that were scheduled in block N.
	// Strip them from block N (where scheduled).
	nextBlock, err := provider.GetBlockByNonce(block.Nonce + 1)
	if err != nil {
		return err
	}

	invalidTxsInNextBlock := findInvalidTransactionsHashes(nextBlock)
	filterOutTransactionsFromScheduledMiniblocks(block, invalidTxsInNextBlock)

	return nil
}

func filterOutProcessedMiniblocksOnceScheduled(block *data.Block) {
	filteredMiniblocks := make([]*data.MiniBlock, 0, len(block.MiniBlocks))

	for _, miniblock := range block.MiniBlocks {
		if miniblock.ProcessingType != string(Processed) {
			filteredMiniblocks = append(filteredMiniblocks, miniblock)
		}
	}

	block.MiniBlocks = filteredMiniblocks
}

func (provider *networkProvider) blockContainsScheduledMiniblockFromMe(block *data.Block) bool {
	for _, miniblock := range block.MiniBlocks {
		isScheduled := miniblock.ProcessingType == string(Scheduled)
		isFromMe := miniblock.SourceShard == provider.observedActualShard
		if isScheduled && isFromMe {
			return true
		}
	}

	return false
}

func findInvalidTransactionsHashes(block *data.Block) map[string]struct{} {
	invalidTxs := make(map[string]struct{})

	for _, miniblock := range block.MiniBlocks {
		if miniblock.Type == string(transaction.TxTypeInvalid) {
			for _, tx := range miniblock.Transactions {
				invalidTxs[tx.Hash] = struct{}{}
			}
		}
	}

	return invalidTxs
}

func filterOutTransactionsFromScheduledMiniblocks(block *data.Block, txsToFilterOut map[string]struct{}) {
	for _, miniblock := range block.MiniBlocks {
		if miniblock.ProcessingType != string(Scheduled) {
			continue
		}

		filteredTxs := make([]*data.FullTransaction, 0, len(miniblock.Transactions))

		for _, tx := range miniblock.Transactions {
			_, shouldFilterOut := txsToFilterOut[tx.Hash]
			if !shouldFilterOut {
				filteredTxs = append(filteredTxs, tx)
			}
		}

		miniblock.Transactions = filteredTxs
	}
}
