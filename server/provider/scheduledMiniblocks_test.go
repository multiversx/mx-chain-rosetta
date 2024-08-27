package provider

import (
	"testing"

	"github.com/multiversx/mx-chain-core-go/data/api"
	dataBlock "github.com/multiversx/mx-chain-core-go/data/block"
	"github.com/multiversx/mx-chain-core-go/data/transaction"
	"github.com/stretchr/testify/require"
)

func TestGatherEffectiveTransactions(t *testing.T) {
	selfShard := uint32(1)

	block_5 := &api.Block{
		MiniBlocks: []*api.MiniBlock{
			{
				Type: dataBlock.InvalidBlock.String(),
				Transactions: []*transaction.ApiTransactionResult{
					{Hash: "aaaa"},
				},
			},
		},
	}

	block_6 := &api.Block{
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

	block_7 := &api.Block{
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

	block_8 := &api.Block{
		MiniBlocks: []*api.MiniBlock{},
	}

	txs := gatherEffectiveTransactions(selfShard, block_5, block_6, block_7)
	require.Len(t, txs, 2)
	require.Contains(t, txs, &transaction.ApiTransactionResult{Hash: "bbbb"})
	require.Contains(t, txs, &transaction.ApiTransactionResult{Hash: "cccc"})

	txs = gatherEffectiveTransactions(selfShard, block_6, block_7, block_8)
	require.Len(t, txs, 1)
	require.Contains(t, txs, &transaction.ApiTransactionResult{Hash: "eeee"})
}

func TestFindImmediatelyExecutingContractResults(t *testing.T) {
	selfShard := uint32(1)
	otherShard := uint32(0)

	t.Run("trivial case (no transactions, no smart contract results)", func(t *testing.T) {
		transactions := []*transaction.ApiTransactionResult{}
		maybeContractResults := []*transaction.ApiTransactionResult{}

		results := findImmediatelyExecutingContractResults(selfShard, transactions, maybeContractResults)
		require.Len(t, results, 0)
	})

	t.Run("trivial case (no smart contract results)", func(t *testing.T) {
		transactions := []*transaction.ApiTransactionResult{
			{Hash: "aaaa"},
			{Hash: "bbbb"},
		}

		maybeContractResults := []*transaction.ApiTransactionResult{}

		results := findImmediatelyExecutingContractResults(selfShard, transactions, maybeContractResults)
		require.Len(t, results, 0)
	})

	t.Run("with contract results (only direct descendants)", func(t *testing.T) {
		transactions := []*transaction.ApiTransactionResult{
			{Hash: "aaaa"},
			{Hash: "bbbb"},
		}

		maybeContractResults := []*transaction.ApiTransactionResult{
			{Hash: "aa00", PreviousTransactionHash: "aaaa", SourceShard: selfShard},
			{Hash: "aa11", PreviousTransactionHash: "aaaa", SourceShard: selfShard},
			{Hash: "bb00", PreviousTransactionHash: "bbbb", SourceShard: otherShard},
			{Hash: "bb11", PreviousTransactionHash: "bbbb", SourceShard: selfShard},
		}

		results := findImmediatelyExecutingContractResults(selfShard, transactions, maybeContractResults)
		require.Len(t, results, 3)
		require.Contains(t, results, &transaction.ApiTransactionResult{Hash: "aa00", PreviousTransactionHash: "aaaa", SourceShard: selfShard})
		require.Contains(t, results, &transaction.ApiTransactionResult{Hash: "aa11", PreviousTransactionHash: "aaaa", SourceShard: selfShard})
		require.Contains(t, results, &transaction.ApiTransactionResult{Hash: "bb11", PreviousTransactionHash: "bbbb", SourceShard: selfShard})
	})

	t.Run("with contract results (direct and indirect descendants)", func(t *testing.T) {
		transactions := []*transaction.ApiTransactionResult{
			{Hash: "aaaa"},
			{Hash: "bbbb"},
		}

		maybeContractResults := []*transaction.ApiTransactionResult{
			{Hash: "aa00", PreviousTransactionHash: "aaaa", SourceShard: selfShard},
			{Hash: "aa11", PreviousTransactionHash: "aaaa", SourceShard: selfShard},
			{Hash: "bb00", PreviousTransactionHash: "bbbb", SourceShard: otherShard},
			{Hash: "bb11", PreviousTransactionHash: "bbbb", SourceShard: selfShard},
			{Hash: "cc00", PreviousTransactionHash: "aa00", SourceShard: selfShard},
			{Hash: "cc11", PreviousTransactionHash: "aa00", SourceShard: selfShard},
			{Hash: "dd00", PreviousTransactionHash: "bb00", SourceShard: otherShard},
			{Hash: "dd11", PreviousTransactionHash: "bb00", SourceShard: selfShard},
		}

		results := findImmediatelyExecutingContractResults(selfShard, transactions, maybeContractResults)
		require.Len(t, results, 5)
		require.Contains(t, results, &transaction.ApiTransactionResult{Hash: "aa00", PreviousTransactionHash: "aaaa", SourceShard: selfShard})
		require.Contains(t, results, &transaction.ApiTransactionResult{Hash: "aa11", PreviousTransactionHash: "aaaa", SourceShard: selfShard})
		require.Contains(t, results, &transaction.ApiTransactionResult{Hash: "bb11", PreviousTransactionHash: "bbbb", SourceShard: selfShard})
		require.Contains(t, results, &transaction.ApiTransactionResult{Hash: "cc00", PreviousTransactionHash: "aa00", SourceShard: selfShard})
		require.Contains(t, results, &transaction.ApiTransactionResult{Hash: "cc11", PreviousTransactionHash: "aa00", SourceShard: selfShard})
	})
}

func TestGatherScheduledTransactions(t *testing.T) {
	block := &api.Block{
		MiniBlocks: []*api.MiniBlock{
			{
				ProcessingType: dataBlock.Scheduled.String(),
				Transactions: []*transaction.ApiTransactionResult{
					{Hash: "aaaa"},
					{Hash: "bbbb"},
				},
			},
			{
				ProcessingType: dataBlock.Normal.String(),
				Transactions: []*transaction.ApiTransactionResult{
					{Hash: "cccc"},
					{Hash: "dddd"},
				},
			},
			{
				Type: dataBlock.Processed.String(),
				Transactions: []*transaction.ApiTransactionResult{
					{Hash: "eeee"},
					{Hash: "ffff"},
				},
			},
		},
	}

	txs := gatherScheduledTransactions(block)
	require.Len(t, txs, 2)
	require.Equal(t, "aaaa", txs[0].Hash)
	require.Equal(t, "bbbb", txs[1].Hash)
}

func TestGatherAllTransactions(t *testing.T) {
	block := &api.Block{
		MiniBlocks: []*api.MiniBlock{
			{
				Transactions: []*transaction.ApiTransactionResult{
					{Hash: "aaaa"},
					{Hash: "bbbb"},
				},
			},
			{
				Transactions: []*transaction.ApiTransactionResult{
					{Hash: "cccc"},
					{Hash: "dddd"},
				},
			},
		},
	}

	txs := gatherAllTransactions(block)
	require.Len(t, txs, 4)
}

func TestGatherAllReceipts(t *testing.T) {
	block := &api.Block{
		MiniBlocks: []*api.MiniBlock{
			{
				Receipts: []*transaction.ApiReceipt{
					{TxHash: "aaaa"},
					{TxHash: "bbbb"},
				},
			},
			{
				Receipts: []*transaction.ApiReceipt{
					{TxHash: "cccc"},
					{TxHash: "dddd"},
				},
			},
		},
	}

	receipts := gatherAllReceipts(block)
	require.Len(t, receipts, 4)
}
