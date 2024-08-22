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

	reportScheduledTransactions(block)
	doSimplifyBlockWithScheduledTransactions(previousBlock, block, nextBlock)

	return nil
}

func reportScheduledTransactions(block *api.Block) {
	numScheduled := 0
	numProcessed := 0
	numInvalid := 0

	for _, miniblock := range block.MiniBlocks {
		if miniblock.ProcessingType == dataBlock.Scheduled.String() {
			numScheduled += len(miniblock.Transactions)
		} else if miniblock.ProcessingType == dataBlock.Processed.String() {
			numProcessed += len(miniblock.Transactions)
		} else if miniblock.Type == dataBlock.InvalidBlock.String() {
			numInvalid += len(miniblock.Transactions)
		}
	}

	if numScheduled > 0 || numProcessed > 0 {
		log.Info("reportScheduledTransactions()", "scheduled", numScheduled, "processed", numProcessed, "invalid", numInvalid, "block", block.Nonce)
	}
}

func doSimplifyBlockWithScheduledTransactions(previousBlock *api.Block, block *api.Block, nextBlock *api.Block) {
	txs := gatherEffectiveTransactions(block.Shard, previousBlock, block, nextBlock)
	receipts := gatherAllReceipts(block)

	block.MiniBlocks = []*api.MiniBlock{
		{
			Type:         "Artificial",
			Transactions: txs,
		},
		{
			Type:     "Artificial",
			Receipts: receipts,
		},
	}
}

func gatherEffectiveTransactions(selfShard uint32, previousBlock *api.Block, currentBlock *api.Block, nextBlock *api.Block) []*transaction.ApiTransactionResult {
	txsInCurrentBlock := gatherAllTransactions(currentBlock)

	scheduledTxsInPreviousBlock := gatherScheduledTransactions(previousBlock)
	scheduledTxsInCurrentBlock := gatherScheduledTransactions(currentBlock)

	if len(scheduledTxsInPreviousBlock) == 0 && len(scheduledTxsInCurrentBlock) == 0 {
		return txsInCurrentBlock
	}

	var previouslyExecutedResults []*transaction.ApiTransactionResult
	var currentlyExecutedResults []*transaction.ApiTransactionResult

	if len(scheduledTxsInPreviousBlock) > 0 {
		previouslyExecutedResults = findImmediatelyExecutingContractResults(selfShard, scheduledTxsInPreviousBlock, txsInCurrentBlock)
	}
	if len(scheduledTxsInCurrentBlock) > 0 {
		txsInNextBlock := gatherAllTransactions(nextBlock)
		currentlyExecutedResults = findImmediatelyExecutingContractResults(selfShard, scheduledTxsInCurrentBlock, txsInNextBlock)
	}

	// effectiveTxs
	//	= txsInCurrentBlock
	//	- txsInPreviousBlock (excludes transactions in "processed" miniblocks, for example)
	//	- previouslyExecutedResults
	//	+ currentlyExecutedResults

	effectiveTxs := make([]*transaction.ApiTransactionResult, 0)
	effectiveTxsByHash := make(map[string]*transaction.ApiTransactionResult)

	for _, tx := range txsInCurrentBlock {
		effectiveTxsByHash[tx.Hash] = tx
	}

	if len(scheduledTxsInPreviousBlock) > 0 {
		txsInPreviousBlock := gatherAllTransactions(previousBlock)

		for _, tx := range txsInPreviousBlock {
			delete(effectiveTxsByHash, tx.Hash)
		}

		for _, tx := range previouslyExecutedResults {
			delete(effectiveTxsByHash, tx.Hash)
		}
	}

	if len(scheduledTxsInCurrentBlock) > 0 {
		for _, tx := range currentlyExecutedResults {
			effectiveTxsByHash[tx.Hash] = tx
		}
	}

	for _, tx := range effectiveTxsByHash {
		effectiveTxs = append(effectiveTxs, tx)
	}

	return effectiveTxs
}

func findImmediatelyExecutingContractResults(
	selfShard uint32,
	transactions []*transaction.ApiTransactionResult,
	maybeContractResults []*transaction.ApiTransactionResult,
) []*transaction.ApiTransactionResult {
	immediateleyExecutingContractResults := make([]*transaction.ApiTransactionResult, 0)
	nextContractResultsByHash := make(map[string][]*transaction.ApiTransactionResult)

	for _, item := range maybeContractResults {
		nextContractResultsByHash[item.PreviousTransactionHash] = append(nextContractResultsByHash[item.PreviousTransactionHash], item)
	}

	for _, tx := range transactions {
		immediateleyExecutingContractResultsPart := findImmediatelyExecutingContractResultsOfTransaction(selfShard, tx, nextContractResultsByHash)
		immediateleyExecutingContractResults = append(immediateleyExecutingContractResults, immediateleyExecutingContractResultsPart...)
	}

	return immediateleyExecutingContractResults
}

func findImmediatelyExecutingContractResultsOfTransaction(
	selfShard uint32,
	tx *transaction.ApiTransactionResult,
	nextContractResultsByHash map[string][]*transaction.ApiTransactionResult,
) []*transaction.ApiTransactionResult {
	immediatelyExecutingContractResults := make([]*transaction.ApiTransactionResult, 0)

	for _, nextContractResult := range nextContractResultsByHash[tx.Hash] {
		// Not immediately executing.
		if nextContractResult.SourceShard != selfShard {
			continue
		}

		immediatelyExecutingContractResults = append(immediatelyExecutingContractResults, nextContractResult)
		// Recursive call:
		immediatelyExecutingContractResultsPart := findImmediatelyExecutingContractResultsOfTransaction(selfShard, nextContractResult, nextContractResultsByHash)
		immediatelyExecutingContractResults = append(immediatelyExecutingContractResults, immediatelyExecutingContractResultsPart...)
	}

	return immediatelyExecutingContractResults
}

func gatherScheduledTransactions(block *api.Block) []*transaction.ApiTransactionResult {
	scheduledTxs := make([]*transaction.ApiTransactionResult, 0)

	for _, miniblock := range block.MiniBlocks {
		isScheduled := miniblock.ProcessingType == dataBlock.Scheduled.String()
		if !isScheduled {
			continue
		}

		for _, tx := range miniblock.Transactions {
			scheduledTxs = append(scheduledTxs, tx)
		}
	}

	return scheduledTxs
}

func gatherAllTransactions(block *api.Block) []*transaction.ApiTransactionResult {
	txs := make([]*transaction.ApiTransactionResult, 0)

	for _, miniblock := range block.MiniBlocks {
		for _, tx := range miniblock.Transactions {
			txs = append(txs, tx)
		}
	}

	return txs
}

func gatherAllReceipts(block *api.Block) []*transaction.ApiReceipt {
	receipts := make([]*transaction.ApiReceipt, 0)

	for _, miniblock := range block.MiniBlocks {
		for _, receipt := range miniblock.Receipts {
			receipts = append(receipts, receipt)
		}
	}

	return receipts
}
