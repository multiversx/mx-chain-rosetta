package provider

import (
	"testing"

	dataBlock "github.com/ElrondNetwork/elrond-go-core/data/block"
	"github.com/ElrondNetwork/elrond-proxy-go/data"
	"github.com/stretchr/testify/require"
)

func TestGatherInvalidTransactions(t *testing.T) {
	// Block N-1
	previousBlock := &data.Block{
		MiniBlocks: []*data.MiniBlock{
			{
				Type: dataBlock.InvalidBlock.String(),
				Transactions: []*data.FullTransaction{
					{Hash: "aaaa"},
				},
			},
		},
	}

	// Block N
	block := &data.Block{
		MiniBlocks: []*data.MiniBlock{
			{
				ProcessingType: string(Scheduled),
				Transactions: []*data.FullTransaction{
					{Hash: "bbbb"},
					{Hash: "cccc"},
				},
			},
			{
				Type: dataBlock.InvalidBlock.String(),
				Transactions: []*data.FullTransaction{
					{Hash: "bbbb"},
				},
			},
		},
	}

	// Block N+1
	nextBlock := &data.Block{
		MiniBlocks: []*data.MiniBlock{
			{
				ProcessingType: string(Processed),
				Transactions: []*data.FullTransaction{
					{Hash: "cccc"},
				},
			},
			{
				Type: dataBlock.InvalidBlock.String(),
				Transactions: []*data.FullTransaction{
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
	previousBlock := &data.Block{
		MiniBlocks: []*data.MiniBlock{
			{
				ProcessingType: string(Scheduled),
				Transactions: []*data.FullTransaction{
					{Hash: "aaaa"},
				},
			},
			// "aaaa" is an invalid transaction that produces effects in N-1,
			// but the intra-shard, "invalid" miniblock is missing (not provided by the API).
		},
	}

	// Block N
	block := &data.Block{
		MiniBlocks: []*data.MiniBlock{
			{
				ProcessingType: string(Scheduled),
				Transactions: []*data.FullTransaction{
					{Hash: "abab"},
					{Hash: "cccc"},
				},
			},
			{
				Type: dataBlock.InvalidBlock.String(),
				Transactions: []*data.FullTransaction{
					{Hash: "aaaa"},
				},
			},
			{
				// Intra-shard miniblock, holds both "aaaa" (scheduled in N-1, with effects in N)
				// and "abab" (scheduled in N, with effects in N)
				Type: dataBlock.InvalidBlock.String(),
				Transactions: []*data.FullTransaction{
					{Hash: "aaaa"},
					{Hash: "abab"},
				},
			},
		},
	}

	// Block N+1
	nextBlock := &data.Block{
		MiniBlocks: []*data.MiniBlock{
			{
				ProcessingType: string(Processed),
				Transactions: []*data.FullTransaction{
					{Hash: "cccc"},
				},
			},
			{
				Type: dataBlock.InvalidBlock.String(),
				Transactions: []*data.FullTransaction{
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
