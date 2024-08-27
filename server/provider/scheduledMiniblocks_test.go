package provider

import (
	"errors"
	"testing"

	"github.com/multiversx/mx-chain-core-go/data/api"
	dataBlock "github.com/multiversx/mx-chain-core-go/data/block"
	"github.com/multiversx/mx-chain-core-go/data/transaction"
	"github.com/multiversx/mx-chain-proxy-go/common"
	"github.com/multiversx/mx-chain-proxy-go/data"
	"github.com/multiversx/mx-chain-rosetta/testscommon"
	"github.com/stretchr/testify/require"
)

func TestNetworkProviderSimplifyBlockWithScheduledTransactions(t *testing.T) {
	observerFacade := testscommon.NewObserverFacadeMock()
	args := createDefaultArgsNewNetworkProvider()
	args.ObserverFacade = observerFacade

	provider, err := NewNetworkProvider(args)
	require.Nil(t, err)
	require.NotNil(t, provider)

	tx_a := &transaction.ApiTransactionResult{Hash: "aaaa"}
	tx_b := &transaction.ApiTransactionResult{Hash: "bbbb"}
	tx_c := &transaction.ApiTransactionResult{Hash: "cccc"}
	tx_d := &transaction.ApiTransactionResult{Hash: "dddd"}

	blocks := []*api.Block{
		{
			Nonce: 0,
			MiniBlocks: []*api.MiniBlock{
				{
					Type: dataBlock.InvalidBlock.String(),
					Transactions: []*transaction.ApiTransactionResult{
						tx_a,
					},
				},
			},
		},
		{
			Nonce: 1,
			MiniBlocks: []*api.MiniBlock{
				{
					ProcessingType: dataBlock.Scheduled.String(),
					Transactions: []*transaction.ApiTransactionResult{
						tx_b,
						tx_c,
					},
				},
				{
					Type: dataBlock.InvalidBlock.String(),
					Transactions: []*transaction.ApiTransactionResult{
						tx_b,
					},
				},
			},
		},
		{
			Nonce: 2,
			MiniBlocks: []*api.MiniBlock{
				{
					ProcessingType: dataBlock.Processed.String(),
					Transactions: []*transaction.ApiTransactionResult{
						tx_c,
					},
				},
				{
					Type: dataBlock.InvalidBlock.String(),
					Transactions: []*transaction.ApiTransactionResult{
						tx_b,
						tx_d,
					},
				},
			},
		},
	}

	observerFacade.GetBlockByNonceCalled = func(shardID uint32, nonce uint64, options common.BlockQueryOptions) (*data.BlockApiResponse, error) {
		if int(nonce) < len(blocks) {
			return &data.BlockApiResponse{
				Data: data.BlockApiResponsePayload{
					Block: *blocks[nonce],
				},
			}, nil
		}

		return nil, errors.New("unexpected request")
	}

	err = provider.simplifyBlockWithScheduledTransactions(blocks[1])
	require.Nil(t, err)

	require.Len(t, blocks[1].MiniBlocks, 2)
	require.Equal(t, miniblockTypeArtificial, blocks[1].MiniBlocks[0].Type)
	require.Equal(t, miniblockTypeArtificial, blocks[1].MiniBlocks[1].Type)

	txs := blocks[1].MiniBlocks[0].Transactions
	require.Len(t, txs, 2)
	require.Contains(t, txs, tx_b)
	require.Contains(t, txs, tx_c)
}

func TestGatherEffectiveTransactions(t *testing.T) {
	selfShard := uint32(1)

	tx_a := &transaction.ApiTransactionResult{Hash: "aaaa"}
	tx_b := &transaction.ApiTransactionResult{Hash: "bbbb"}
	tx_c := &transaction.ApiTransactionResult{Hash: "cccc"}
	tx_d := &transaction.ApiTransactionResult{Hash: "dddd"}
	tx_e := &transaction.ApiTransactionResult{Hash: "eeee"}
	tx_f := &transaction.ApiTransactionResult{Hash: "ffff"}
	tx_g_result_of_e := &transaction.ApiTransactionResult{Hash: "ffaa", PreviousTransactionHash: tx_e.Hash, SourceShard: selfShard}
	tx_i_result_of_f := &transaction.ApiTransactionResult{Hash: "ffbb", PreviousTransactionHash: tx_f.Hash, SourceShard: selfShard}

	block_0 := &api.Block{
		MiniBlocks: []*api.MiniBlock{
			{
				Type: dataBlock.InvalidBlock.String(),
				Transactions: []*transaction.ApiTransactionResult{
					tx_a,
				},
			},
		},
	}

	block_1 := &api.Block{
		MiniBlocks: []*api.MiniBlock{
			{
				ProcessingType: dataBlock.Scheduled.String(),
				Transactions: []*transaction.ApiTransactionResult{
					tx_b,
					tx_c,
				},
			},
			{
				Type: dataBlock.InvalidBlock.String(),
				Transactions: []*transaction.ApiTransactionResult{
					tx_b,
				},
			},
		},
	}

	block_2 := &api.Block{
		MiniBlocks: []*api.MiniBlock{
			{
				ProcessingType: dataBlock.Processed.String(),
				Transactions: []*transaction.ApiTransactionResult{
					tx_c,
				},
			},
			{
				Type: dataBlock.InvalidBlock.String(),
				Transactions: []*transaction.ApiTransactionResult{
					tx_b,
					tx_d,
				},
			},
		},
	}

	block_3 := &api.Block{
		MiniBlocks: []*api.MiniBlock{},
	}

	block_4 := &api.Block{
		MiniBlocks: []*api.MiniBlock{
			{
				ProcessingType: dataBlock.Scheduled.String(),
				Transactions: []*transaction.ApiTransactionResult{
					tx_e,
				},
			},
		},
	}

	block_5 := &api.Block{
		MiniBlocks: []*api.MiniBlock{
			{
				Transactions: []*transaction.ApiTransactionResult{
					tx_f,
				},
			},
			{
				Transactions: []*transaction.ApiTransactionResult{
					tx_g_result_of_e,
					tx_i_result_of_f,
				},
			},
		},
	}

	block_6 := &api.Block{
		MiniBlocks: []*api.MiniBlock{},
	}

	// Current block is 1
	txs := gatherEffectiveTransactions(selfShard, block_0, block_1, block_2)
	require.Len(t, txs, 2)
	require.Contains(t, txs, tx_b)
	require.Contains(t, txs, tx_c)

	// Current block is 2
	txs = gatherEffectiveTransactions(selfShard, block_1, block_2, block_3)
	require.Len(t, txs, 1)
	require.Contains(t, txs, tx_d)

	// Current block is 3
	txs = gatherEffectiveTransactions(selfShard, block_2, block_3, block_4)
	require.Len(t, txs, 0)

	// Current block is 4
	txs = gatherEffectiveTransactions(selfShard, block_3, block_4, block_5)
	require.Len(t, txs, 2)
	require.Contains(t, txs, tx_e)
	require.Contains(t, txs, tx_g_result_of_e)

	// Current block is 5
	txs = gatherEffectiveTransactions(selfShard, block_4, block_5, block_6)
	require.Len(t, txs, 2)
	require.Contains(t, txs, tx_f)
	require.Contains(t, txs, tx_i_result_of_f)
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
