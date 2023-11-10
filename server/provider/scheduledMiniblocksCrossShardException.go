package provider

import (
	"github.com/multiversx/mx-chain-core-go/data/api"
	dataBlock "github.com/multiversx/mx-chain-core-go/data/block"
	"github.com/multiversx/mx-chain-core-go/data/transaction"
)

func handleContractResultsHavingOriginalTransactionInCrossShardScheduledMiniblockInPreviousBlock(previousBlock *api.Block, block *api.Block, nextBlock *api.Block) {
	// Gather transactions in cross-shard scheduled, final miniblocks.
	// Afterwards, we will handle their contract results in two steps.
	transactionsInCrossShardScheduledMiniblockInPreviousBlock := gatherTransactionsInCrossShardScheduledFinalMiniblock(previousBlock)
	transactionsInCrossShardScheduledMiniblockInCurrentBlock := gatherTransactionsInCrossShardScheduledFinalMiniblock(block)

	// First, remove corresponding contract results from the current block (since they already produced their effects in the previous block)
	for _, miniblock := range block.MiniBlocks {
		if miniblock.Type != dataBlock.SmartContractResultBlock.String() {
			continue
		}

		contractResultsToKeep := make([]*transaction.ApiTransactionResult, 0, len(miniblock.Transactions))

		for _, contractResult := range miniblock.Transactions {
			_, ok := transactionsInCrossShardScheduledMiniblockInPreviousBlock[contractResult.OriginalTransactionHash]
			if ok {
				// Discard this contract result
				continue
			}

			contractResultsToKeep = append(contractResultsToKeep, contractResult)
		}

		miniblock.Transactions = contractResultsToKeep
	}

	// Secondly, bring here (in the current block) the corresponding contract results of the next block (since they produced their effects in the current block)
	contractResultsToMove := make([]*transaction.ApiTransactionResult, 0, len(nextBlock.MiniBlocks))

	for _, miniblockInNextBlock := range nextBlock.MiniBlocks {
		if miniblockInNextBlock.Type != dataBlock.SmartContractResultBlock.String() {
			continue
		}

		for _, contractResultInNextBlock := range miniblockInNextBlock.Transactions {
			_, ok := transactionsInCrossShardScheduledMiniblockInCurrentBlock[contractResultInNextBlock.OriginalTransactionHash]
			if ok {
				contractResultsToMove = append(contractResultsToMove, contractResultInNextBlock)
			}
		}
	}

	if len(contractResultsToMove) > 0 {
		// Extremely rare case.
		log.Info("handleContractResultsHavingOriginalTransactionInCrossShardScheduledMiniblockInPreviousBlock", "currentBlock", block.Nonce, "numContractResultsToMove", len(contractResultsToMove))

		artificialMiniblock := &api.MiniBlock{
			Type:         dataBlock.SmartContractResultBlock.String(),
			Transactions: contractResultsToMove,
		}

		block.MiniBlocks = append(block.MiniBlocks, artificialMiniblock)
	}
}

func gatherTransactionsInCrossShardScheduledFinalMiniblock(block *api.Block) map[string]struct{} {
	gathered := make(map[string]struct{})

	for _, miniblockInPreviousBlock := range block.MiniBlocks {
		isScheduled := miniblockInPreviousBlock.ProcessingType == dataBlock.Scheduled.String()
		isFinal := miniblockInPreviousBlock.ConstructionState == dataBlock.Final.String()
		isCrossShard := miniblockInPreviousBlock.SourceShard != miniblockInPreviousBlock.DestinationShard
		if !isScheduled || !isFinal || !isCrossShard {
			continue
		}

		for _, txInPreviousBlock := range miniblockInPreviousBlock.Transactions {
			gathered[txInPreviousBlock.Hash] = struct{}{}
		}
	}

	return gathered
}
