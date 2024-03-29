package provider

import (
	"testing"

	"github.com/multiversx/mx-chain-core-go/data/api"
	dataBlock "github.com/multiversx/mx-chain-core-go/data/block"
	"github.com/multiversx/mx-chain-core-go/data/transaction"
	"github.com/stretchr/testify/require"
)

func TestGatherInvalidTransactions(t *testing.T) {
	// Block N-1
	previousBlock := &api.Block{
		MiniBlocks: []*api.MiniBlock{
			{
				Type: dataBlock.InvalidBlock.String(),
				Transactions: []*transaction.ApiTransactionResult{
					{Hash: "aaaa"},
				},
			},
		},
	}

	// Block N
	block := &api.Block{
		MiniBlocks: []*api.MiniBlock{
			{
				ProcessingType: dataBlock.Scheduled.String(),
				Transactions: []*transaction.ApiTransactionResult{
					{Hash: "bbbb"},
					{Hash: "cccc"},
				},
			},
			{
				Type: dataBlock.InvalidBlock.String(),
				Transactions: []*transaction.ApiTransactionResult{
					{Hash: "bbbb"},
				},
			},
		},
	}

	// Block N+1
	nextBlock := &api.Block{
		MiniBlocks: []*api.MiniBlock{
			{
				ProcessingType: dataBlock.Processed.String(),
				Transactions: []*transaction.ApiTransactionResult{
					{Hash: "cccc"},
				},
			},
			{
				Type: dataBlock.InvalidBlock.String(),
				Transactions: []*transaction.ApiTransactionResult{
					{Hash: "bbbb"},
					{Hash: "eeee"},
				},
			},
		},
	}

	// "aaaa" is ignored for N, since it produces effects in N-1
	// "eeee" is ignored for N, since it produces effects in N+1
	invalidTxs := gatherInvalidTransactions(previousBlock, block, nextBlock)
	require.Len(t, invalidTxs, 1)
	require.Equal(t, "bbbb", invalidTxs[0].Hash)
}

func TestGatherInvalidTransactions_WhenIntraShardIsMissingInPreviousBlock(t *testing.T) {
	// Edge case on start of epoch.

	// Block N-1
	previousBlock := &api.Block{
		MiniBlocks: []*api.MiniBlock{
			{
				ProcessingType: dataBlock.Scheduled.String(),
				Transactions: []*transaction.ApiTransactionResult{
					{Hash: "aaaa"},
				},
			},
			// "aaaa" is an invalid transaction that produces effects in N-1,
			// but the intra-shard, "invalid" miniblock is missing (not provided by the API).
		},
	}

	// Block N
	block := &api.Block{
		MiniBlocks: []*api.MiniBlock{
			{
				ProcessingType: dataBlock.Scheduled.String(),
				Transactions: []*transaction.ApiTransactionResult{
					{Hash: "abab"},
					{Hash: "cccc"},
				},
			},
			{
				Type: dataBlock.InvalidBlock.String(),
				Transactions: []*transaction.ApiTransactionResult{
					{Hash: "aaaa"},
				},
			},
			{
				// Intra-shard miniblock, holds both "aaaa" (scheduled in N-1, with effects in N)
				// and "abab" (scheduled in N, with effects in N)
				Type: dataBlock.InvalidBlock.String(),
				Transactions: []*transaction.ApiTransactionResult{
					{Hash: "aaaa"},
					{Hash: "abab"},
				},
			},
		},
	}

	// Block N+1
	nextBlock := &api.Block{
		MiniBlocks: []*api.MiniBlock{
			{
				ProcessingType: dataBlock.Processed.String(),
				Transactions: []*transaction.ApiTransactionResult{
					{Hash: "cccc"},
				},
			},
			{
				Type: dataBlock.InvalidBlock.String(),
				Transactions: []*transaction.ApiTransactionResult{
					{Hash: "abab"},
				},
			},
		},
	}

	// "aaaa" is ignored for N, since it produces effects in N-1
	invalidTxs := gatherInvalidTransactions(previousBlock, block, nextBlock)
	require.Len(t, invalidTxs, 1)
	require.Equal(t, "abab", invalidTxs[0].Hash)
}

func TestDoSimplifyBlockWithScheduledTransactions_WithRespectToConstructionState(t *testing.T) {
	// Edge case on cross-shard miniblocks, both scheduled and final.

	// Empty, trivial blocks at N-1 and N+1
	previousBlock := &api.Block{MiniBlocks: []*api.MiniBlock{}}
	nextBlock := &api.Block{MiniBlocks: []*api.MiniBlock{}}

	// Scheduled & Final (won't be removed)
	block := &api.Block{
		MiniBlocks: []*api.MiniBlock{
			{
				ProcessingType:    dataBlock.Scheduled.String(),
				ConstructionState: dataBlock.Final.String(),
				Transactions: []*transaction.ApiTransactionResult{
					{Hash: "aaaa"},
					{Hash: "bbbb"},
				},
			},
		},
	}

	doSimplifyBlockWithScheduledTransactions(previousBlock, block, nextBlock)
	require.Len(t, block.MiniBlocks, 1)
	require.Len(t, block.MiniBlocks[0].Transactions, 2)
	require.Equal(t, "aaaa", block.MiniBlocks[0].Transactions[0].Hash)
	require.Equal(t, "bbbb", block.MiniBlocks[0].Transactions[1].Hash)

	// Scheduled & !Final (will be removed)
	block = &api.Block{
		MiniBlocks: []*api.MiniBlock{
			{
				ProcessingType: dataBlock.Scheduled.String(),
				Transactions: []*transaction.ApiTransactionResult{
					{Hash: "aaaa"},
					{Hash: "bbbb"},
				},
			},
		},
	}

	doSimplifyBlockWithScheduledTransactions(previousBlock, block, nextBlock)
	require.Len(t, block.MiniBlocks, 0)
}

func TestDeduplicatePreviouslyAppearingContractResultsInReceipts(t *testing.T) {
	// Block N-1
	previousBlock := &api.Block{
		MiniBlocks: []*api.MiniBlock{
			// Should not be subject to deduplication (not from receipts)
			{
				IsFromReceiptsStorage: false,
				Type:                  dataBlock.SmartContractResultBlock.String(),
				Transactions: []*transaction.ApiTransactionResult{
					{Hash: "aaaa"},
				},
			},
			// Should not be subject to deduplication (due to "type")
			{
				IsFromReceiptsStorage: true,
				Type:                  dataBlock.InvalidBlock.String(),
				Transactions: []*transaction.ApiTransactionResult{
					{Hash: "bbbb"},
				},
			},
			// Should be subject to deduplication
			{
				IsFromReceiptsStorage: true,
				Type:                  dataBlock.SmartContractResultBlock.String(),
				Transactions: []*transaction.ApiTransactionResult{
					{Hash: "cccc"},
					{Hash: "dddd"},
				},
			},
		},
	}

	// Block N
	block := &api.Block{
		MiniBlocks: []*api.MiniBlock{
			// Should not be subject to deduplication (not from receipts)
			{
				IsFromReceiptsStorage: false,
				Type:                  dataBlock.SmartContractResultBlock.String(),
				Transactions: []*transaction.ApiTransactionResult{
					{Hash: "aaaa"},
				},
			},
			// Should not be subject to deduplication (due to "type")
			{
				IsFromReceiptsStorage: true,
				Type:                  dataBlock.InvalidBlock.String(),
				Transactions: []*transaction.ApiTransactionResult{
					{Hash: "bbbb"},
				},
			},
			// Should be subject to deduplication
			{
				IsFromReceiptsStorage: true,
				Type:                  dataBlock.SmartContractResultBlock.String(),
				Transactions: []*transaction.ApiTransactionResult{
					{Hash: "cccc"},
					{Hash: "eeee"},
				},
			},
		},
	}

	deduplicatePreviouslyAppearingContractResultsInReceipts(previousBlock, block)

	require.Len(t, block.MiniBlocks, 3)
	require.Len(t, block.MiniBlocks[0].Transactions, 1)
	require.Len(t, block.MiniBlocks[1].Transactions, 1)
	require.Len(t, block.MiniBlocks[2].Transactions, 1)
	require.Equal(t, "aaaa", block.MiniBlocks[0].Transactions[0].Hash)
	require.Equal(t, "bbbb", block.MiniBlocks[1].Transactions[0].Hash)
	require.Equal(t, "eeee", block.MiniBlocks[2].Transactions[0].Hash)
}

func TestFindContractResultsInReceipts(t *testing.T) {
	block := &api.Block{
		MiniBlocks: []*api.MiniBlock{
			{
				IsFromReceiptsStorage: false,
				Type:                  dataBlock.SmartContractResultBlock.String(),
				Transactions: []*transaction.ApiTransactionResult{
					{Hash: "aaaa"},
					{Hash: "bbbb"},
				},
			},
			{
				IsFromReceiptsStorage: true,
				Type:                  dataBlock.InvalidBlock.String(),
				Transactions: []*transaction.ApiTransactionResult{
					{Hash: "cccc"},
					{Hash: "dddd"},
				},
			},
			{
				IsFromReceiptsStorage: true,
				Type:                  dataBlock.SmartContractResultBlock.String(),
				Transactions: []*transaction.ApiTransactionResult{
					{Hash: "eeee"},
					{Hash: "ffff"},
				},
			},
		},
	}

	found := findContractResultsInReceipts(block)
	require.Len(t, found, 2)
	require.Contains(t, found, "eeee")
	require.Contains(t, found, "ffff")
}

func TestRemoveContractResultsInReceipts(t *testing.T) {
	block := &api.Block{
		MiniBlocks: []*api.MiniBlock{
			{
				IsFromReceiptsStorage: false,
				Type:                  dataBlock.SmartContractResultBlock.String(),
				Transactions: []*transaction.ApiTransactionResult{
					{Hash: "aaaa"},
					{Hash: "bbbb"},
				},
			},
			{
				IsFromReceiptsStorage: true,
				Type:                  dataBlock.InvalidBlock.String(),
				Transactions: []*transaction.ApiTransactionResult{
					{Hash: "cccc"},
					{Hash: "dddd"},
				},
			},
			{
				IsFromReceiptsStorage: true,
				Type:                  dataBlock.SmartContractResultBlock.String(),
				Transactions: []*transaction.ApiTransactionResult{
					{Hash: "eeee"},
					{Hash: "ffff"},
					{Hash: "abba"},
					{Hash: "aabb"},
				},
			},
		},
	}

	removeContractResultsInReceipts(block, map[string]struct{}{
		"eeee": {},
		"ffff": {},
	})

	require.Len(t, block.MiniBlocks[0].Transactions, 2)
	require.Len(t, block.MiniBlocks[1].Transactions, 2)
	require.Len(t, block.MiniBlocks[2].Transactions, 2)
	require.Equal(t, "abba", block.MiniBlocks[2].Transactions[0].Hash)
	require.Equal(t, "aabb", block.MiniBlocks[2].Transactions[1].Hash)
}
