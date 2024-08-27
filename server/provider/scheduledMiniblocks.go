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

// reportScheduledTransactions logs the number of transactions in miniblocks of types: "Scheduled", "Processed", and "Invalid".
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

	// Downstream, when recovering balance-changing operations, we do not care about the original container (miniblock) of the effective transaction.
	// We group all transactions in an "artificial" miniblock, and all receipts in another "artificial" miniblock.
	block.MiniBlocks = []*api.MiniBlock{
		{
			Type:         miniblockTypeArtificial,
			Transactions: txs,
		},
		{
			Type:     miniblockTypeArtificial,
			Receipts: receipts,
		},
	}
}

// gatherEffectiveTransactions gathers transactions whose effects (mutation of accounts state) are visible in the current block.
// They are gathered from the previous, current and next block.
func gatherEffectiveTransactions(selfShard uint32, previousBlock *api.Block, currentBlock *api.Block, nextBlock *api.Block) []*transaction.ApiTransactionResult {
	txsInCurrentBlock := gatherAllTransactions(currentBlock)

	scheduledTxsInPreviousBlock := gatherScheduledTransactions(previousBlock)
	scheduledTxsInCurrentBlock := gatherScheduledTransactions(currentBlock)

	if len(scheduledTxsInPreviousBlock) == 0 && len(scheduledTxsInCurrentBlock) == 0 {
		// Trivial case, no special handling needed.
		return txsInCurrentBlock
	}

	var previouslyExecutedResults []*transaction.ApiTransactionResult
	var currentlyExecutedResults []*transaction.ApiTransactionResult

	if len(scheduledTxsInPreviousBlock) > 0 {
		// Look behind, for any contract results that, even if present in the current block, had their effects in the previous block.
		previouslyExecutedResults = findImmediatelyExecutingContractResults(selfShard, scheduledTxsInPreviousBlock, txsInCurrentBlock)
	}
	if len(scheduledTxsInCurrentBlock) > 0 {
		// Look ahead, for any contract results that, even if present in the next block, have their effects in the current block.
		txsInNextBlock := gatherAllTransactions(nextBlock)
		currentlyExecutedResults = findImmediatelyExecutingContractResults(selfShard, scheduledTxsInCurrentBlock, txsInNextBlock)
	}

	// effectiveTxs
	//	= txsInCurrentBlock																		term (a)
	//	- txsInPreviousBlock (excludes transactions in "processed" miniblocks, for example)		term (b)
	//	- previouslyExecutedResults																term (c)
	//	+ currentlyExecutedResults																term (d)

	effectiveTxsByHash := make(map[string]*transaction.ApiTransactionResult)

	// term (a)
	for _, tx := range txsInCurrentBlock {
		effectiveTxsByHash[tx.Hash] = tx
	}

	if len(scheduledTxsInPreviousBlock) > 0 {
		txsInPreviousBlock := gatherAllTransactions(previousBlock)

		// term (b)
		for _, tx := range txsInPreviousBlock {
			delete(effectiveTxsByHash, tx.Hash)
		}

		// term (c)
		for _, tx := range previouslyExecutedResults {
			delete(effectiveTxsByHash, tx.Hash)
		}
	}

	if len(scheduledTxsInCurrentBlock) > 0 {
		// term (d)
		for _, tx := range currentlyExecutedResults {
			effectiveTxsByHash[tx.Hash] = tx
		}
	}

	// Collect the effective transactions in a slice.
	effectiveTxs := make([]*transaction.ApiTransactionResult, 0, len(effectiveTxsByHash))

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
	immediatelyExecutingContractResults := make([]*transaction.ApiTransactionResult, 0)
	nextContractResultsByHash := make(map[string][]*transaction.ApiTransactionResult)

	for _, item := range maybeContractResults {
		nextContractResultsByHash[item.PreviousTransactionHash] = append(nextContractResultsByHash[item.PreviousTransactionHash], item)
	}

	for _, tx := range transactions {
		immediatelyExecutingContractResultsPart := findImmediatelyExecutingContractResultsOfTransaction(selfShard, tx, nextContractResultsByHash)
		immediatelyExecutingContractResults = append(immediatelyExecutingContractResults, immediatelyExecutingContractResultsPart...)
	}

	return immediatelyExecutingContractResults
}

func findImmediatelyExecutingContractResultsOfTransaction(
	selfShard uint32,
	tx *transaction.ApiTransactionResult,
	nextContractResultsByHash map[string][]*transaction.ApiTransactionResult,
) []*transaction.ApiTransactionResult {
	immediatelyExecutingContractResults := make([]*transaction.ApiTransactionResult, 0)

	for _, nextContractResult := range nextContractResultsByHash[tx.Hash] {
		if nextContractResult.SourceShard != selfShard {
			// Cross-shard, not immediately executing.
			continue
		}

		immediatelyExecutingContractResults = append(immediatelyExecutingContractResults, nextContractResult)
		// Recursive call:
		immediatelyExecutingContractResultsPart := findImmediatelyExecutingContractResultsOfTransaction(selfShard, nextContractResult, nextContractResultsByHash)
		immediatelyExecutingContractResults = append(immediatelyExecutingContractResults, immediatelyExecutingContractResultsPart...)
	}

	return immediatelyExecutingContractResults
}

// gatherScheduledTransactions gathers transactions in miniblocks of type "Scheduled"
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

// gatherAllTransactions gathers all transactions, from all miniblocks
func gatherAllTransactions(block *api.Block) []*transaction.ApiTransactionResult {
	txs := make([]*transaction.ApiTransactionResult, 0)

	for _, miniblock := range block.MiniBlocks {
		for _, tx := range miniblock.Transactions {
			txs = append(txs, tx)
		}
	}

	return txs
}

// gatherAllReceipts gathers all receipts, from all miniblocks
func gatherAllReceipts(block *api.Block) []*transaction.ApiReceipt {
	receipts := make([]*transaction.ApiReceipt, 0)

	for _, miniblock := range block.MiniBlocks {
		for _, receipt := range miniblock.Receipts {
			receipts = append(receipts, receipt)
		}
	}

	return receipts
}
