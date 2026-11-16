package aggregator

import (
	"fmt"
	"math/big"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/multiversx/mx-chain-rosetta/aggregator/backend"
	"github.com/multiversx/mx-chain-rosetta/aggregator/metachain"
	"github.com/multiversx/mx-chain-rosetta/aggregator/mock"
	"github.com/multiversx/mx-chain-rosetta/aggregator/unified"
	"github.com/stretchr/testify/require"
)

// locatedTransaction is a transaction of the unified chain, together with the block that holds it
type locatedTransaction struct {
	blockIndex int64
	tx         *types.Transaction
}

// unifiedChainView is what a Rosetta consumer sees after syncing the unified chain
type unifiedChainView struct {
	blocks       []*types.Block
	transactions map[string]locatedTransaction
}

// checkUnifiedChain syncs the unified chain as a Rosetta consumer would (block by block, starting with "firstBlock"), and checks that:
//   - blocks are valid Rosetta blocks, and are linked to their parents;
//   - no transaction identifier is repeated (neither within a block, nor across blocks);
//   - each block holds exactly the shard blocks notarized by its metablock;
//   - for every account and every block, the balance changes by the sum of the (successful) operations of the block.
//
// The last check is the one performed by the reconciler of the Mesh CLI ("check:data").
func checkUnifiedChain(h *harness, firstBlock int64) *unifiedChainView {
	t := h.t
	tip := int64(h.tip())
	accounts := h.simulator.Accounts()
	metaBlocks := h.simulator.MetaBlocks()

	view := &unifiedChainView{
		blocks:       make([]*types.Block, tip+1),
		transactions: make(map[string]locatedTransaction),
	}

	// Balances at the block before the first one (the baseline).
	balances := make(map[string]*big.Int, len(accounts))
	if firstBlock == 0 {
		for _, account := range accounts {
			balances[account.Address] = big.NewInt(0)
		}
	} else {
		for _, account := range accounts {
			balances[account.Address] = h.getBalance(account.Address, firstBlock-1)
		}
	}

	var previousBlock *types.Block

	for blockIndex := firstBlock; blockIndex <= tip; blockIndex++ {
		block := h.getBlock(blockIndex)
		view.blocks[blockIndex] = block

		// Identity: the unified block is the metablock.
		require.Equal(t, blockIndex, block.BlockIdentifier.Index)
		require.Equal(t, metaBlocks[blockIndex].Hash, block.BlockIdentifier.Hash)

		if blockIndex > 0 {
			require.Equal(t, blockIndex-1, block.ParentBlockIdentifier.Index)
			require.Equal(t, metaBlocks[blockIndex-1].Hash, block.ParentBlockIdentifier.Hash)
			require.Equal(t, metaBlocks[blockIndex].TimestampMs, block.Timestamp)
		}
		if previousBlock != nil {
			require.Equal(t, previousBlock.BlockIdentifier, block.ParentBlockIdentifier)
		}

		// Content: exactly the shard blocks notarized by the metablock.
		if blockIndex > 0 {
			checkShardBlocksOfUnifiedBlock(h, block, blockIndex)
		}

		// Transaction identifiers are unique (within the block, and on the whole chain).
		for _, tx := range block.Transactions {
			hash := tx.TransactionIdentifier.Hash
			existing, isDuplicate := view.transactions[hash]
			require.False(t, isDuplicate, "transaction %s is in block %d and in block %d", hash, existing.blockIndex, blockIndex)
			require.NotContains(t, hash, "/")

			view.transactions[hash] = locatedTransaction{blockIndex: blockIndex, tx: tx}
		}

		// Reconciliation, for all accounts (including the ones without operations in this block).
		deltas := sumSuccessfulOperations(t, block)
		for address := range deltas {
			_, isKnownAccount := balances[address]
			require.True(t, isKnownAccount, "block %d holds an operation of unknown account %s", blockIndex, address)
		}

		for _, account := range accounts {
			expected := big.NewInt(0).Set(balances[account.Address])
			if delta, hasOperations := deltas[account.Address]; hasOperations {
				expected.Add(expected, delta)
			}

			actual := h.getBalance(account.Address, blockIndex)
			require.Equal(t, expected.String(), actual.String(),
				"block %d, account %s (shard %d): the balance does not match the operations", blockIndex, account.Address, account.Shard)

			balances[account.Address] = actual
		}

		previousBlock = block
	}

	// At the tip, the balances are the ones of the shard chains at their frontier.
	lastRecord, err := h.app.index.Get(uint64(tip))
	if tip > 0 {
		require.NoError(t, err)

		for _, account := range accounts {
			frontier, ok := lastRecord.Frontier(account.Shard)
			require.True(t, ok)

			expected, ok := h.simulator.Chain(account.Shard).GetBalance(account.Address, int64(frontier.Nonce))
			require.True(t, ok)
			require.Equal(t, expected.String(), balances[account.Address].String())
		}
	}

	return view
}

// checkShardBlocksOfUnifiedBlock checks that the block metadata lists exactly the shard blocks notarized by the metablock,
// and that the block holds all (and only) their transactions.
func checkShardBlocksOfUnifiedBlock(h *harness, block *types.Block, blockIndex int64) {
	t := h.t
	metaBlock := h.simulator.MetaBlocks()[blockIndex]

	expectedShardBlocks := make(map[string]struct{})
	expectedNumTransactions := 0

	for _, notarized := range metaBlock.Notarized {
		expectedShardBlocks[fmt.Sprintf("%d/%d/%s", notarized.Shard, notarized.Nonce, notarized.Hash)] = struct{}{}

		shardBlock, ok := h.simulator.Chain(notarized.Shard).GetBlock(int64(notarized.Nonce))
		require.True(t, ok)
		expectedNumTransactions += len(shardBlock.Transactions)
	}

	shardBlocks, ok := block.Metadata["shardBlocks"].([]interface{})
	require.True(t, ok)
	require.Len(t, shardBlocks, len(expectedShardBlocks))

	previousKey := ""
	for _, item := range shardBlocks {
		shardBlock := item.(map[string]interface{})
		key := fmt.Sprintf("%d/%d/%s", uint32(shardBlock["shard"].(float64)), uint64(shardBlock["nonce"].(float64)), shardBlock["hash"])
		require.Contains(t, expectedShardBlocks, key)

		// Ordered by shard, then by nonce.
		sortableKey := fmt.Sprintf("%05d/%012d", uint32(shardBlock["shard"].(float64)), uint64(shardBlock["nonce"].(float64)))
		require.Greater(t, sortableKey, previousKey)
		previousKey = sortableKey
	}

	require.Len(t, block.Transactions, expectedNumTransactions)
}

func hasRelated(tx *types.Transaction, hash string, direction types.Direction) bool {
	for _, item := range tx.RelatedTransactions {
		if item.TransactionIdentifier.Hash == hash && item.Direction == direction && item.NetworkIdentifier == nil {
			return true
		}
	}

	return false
}

func findOperation(tx *types.Transaction, address string, value *big.Int) *types.Operation {
	for _, operation := range tx.Operations {
		if operation.Account.Address == address && operation.Amount.Value == value.String() {
			return operation
		}
	}

	return nil
}

func getCrossShardMetadata(t *testing.T, tx *types.Transaction) map[string]interface{} {
	crossShard, ok := tx.Metadata["crossShard"].(map[string]interface{})
	require.True(t, ok, "transaction %s has no cross-shard metadata", tx.TransactionIdentifier.Hash)
	return crossShard
}

// checkCrossShardTransfers checks how the cross-shard transfers generated by the simulator are presented on the unified chain:
// two linked transactions (legs), in different blocks, each holding the operations of its own shard.
// All transfers are expected to be settled (executed at destination, refunded if failed).
func checkCrossShardTransfers(h *harness, view *unifiedChainView, expectForwardLinks bool) {
	t := h.t
	transfers := h.simulator.CrossShardTransfers()
	require.NotEmpty(t, transfers)

	numFailed := 0

	for _, transfer := range transfers {
		require.True(t, transfer.ExecutedAtDestination, "the simulation is not settled")

		// Source leg: keeps the hash of the transaction, holds the debit of the sender (and the fee).
		sourceLeg, ok := view.transactions[transfer.Hash]
		require.True(t, ok, "source leg of %s not found", transfer.Hash)

		crossShard := getCrossShardMetadata(t, sourceLeg.tx)
		require.Equal(t, "source", crossShard["leg"])
		require.Equal(t, float64(transfer.SourceShard), crossShard["sourceShard"])
		require.Equal(t, float64(transfer.DestinationShard), crossShard["destinationShard"])
		require.NotContains(t, crossShard, "hash")

		debit := findOperation(sourceLeg.tx, transfer.Sender, big.NewInt(0).Neg(transfer.Value))
		require.NotNil(t, debit)
		require.Equal(t, mock.OpStatusSuccess, *debit.Status)
		require.Nil(t, findOperation(sourceLeg.tx, transfer.Receiver, transfer.Value), "the source leg must not hold the credit of the receiver")

		// Destination leg: derived identifier, holds the credit of the receiver, in a later block.
		destinationHash := transfer.Hash + unified.DestinationLegSuffix
		destinationLeg, ok := view.transactions[destinationHash]
		require.True(t, ok, "destination leg of %s not found", transfer.Hash)
		require.Greater(t, destinationLeg.blockIndex, sourceLeg.blockIndex, "the destination leg must be in a later block")

		crossShard = getCrossShardMetadata(t, destinationLeg.tx)
		require.Equal(t, "destination", crossShard["leg"])
		require.Equal(t, transfer.Hash, crossShard["hash"])

		credit := findOperation(destinationLeg.tx, transfer.Receiver, transfer.Value)
		require.NotNil(t, credit)
		require.Len(t, destinationLeg.tx.Operations, 1)

		// Links: backward (always), forward (when enabled; the destination leg of a transfer of value is certain).
		require.True(t, hasRelated(destinationLeg.tx, transfer.Hash, types.Backward))
		require.Equal(t, expectForwardLinks, hasRelated(sourceLeg.tx, destinationHash, types.Forward))

		if !transfer.FailsAtDestination {
			require.Equal(t, mock.OpStatusSuccess, *credit.Status)
			require.Empty(t, transfer.RefundHash)
			continue
		}

		// The transfer failed at destination: the credit is reported as failed, and the value is refunded to the sender.
		numFailed++
		require.Equal(t, mock.OpStatusFailure, *credit.Status)
		require.True(t, transfer.RefundExecuted, "the simulation is not settled")

		// The refund is, itself, a cross-shard item (a smart contract result), travelling in the opposite direction.
		refundSourceLeg, ok := view.transactions[transfer.RefundHash]
		require.True(t, ok)
		require.Equal(t, destinationLeg.blockIndex, refundSourceLeg.blockIndex)
		require.Equal(t, "source", getCrossShardMetadata(t, refundSourceLeg.tx)["leg"])
		require.True(t, hasRelated(refundSourceLeg.tx, transfer.Hash, types.Backward), "the refund must be linked to the original transaction")
		for _, item := range refundSourceLeg.tx.RelatedTransactions {
			require.NotEqual(t, types.Forward, item.Direction, "no forward link is promised for smart contract results")
		}

		refundDestinationLeg, ok := view.transactions[transfer.RefundHash+unified.DestinationLegSuffix]
		require.True(t, ok)
		require.Greater(t, refundDestinationLeg.blockIndex, refundSourceLeg.blockIndex)
		require.True(t, hasRelated(refundDestinationLeg.tx, transfer.RefundHash, types.Backward))
		require.True(t, hasRelated(refundDestinationLeg.tx, transfer.Hash, types.Backward))

		refund := findOperation(refundDestinationLeg.tx, transfer.Sender, transfer.Value)
		require.NotNil(t, refund)
		require.Equal(t, mock.OpStatusSuccess, *refund.Status)
	}

	// Following the links from a transaction reaches all the transactions it has caused (what the "related transactions" are for).
	for hash, located := range view.transactions {
		for _, item := range located.tx.RelatedTransactions {
			relatedTx, ok := view.transactions[item.TransactionIdentifier.Hash]
			require.True(t, ok, "transaction %s is linked to %s, which is not on the chain", hash, item.TransactionIdentifier.Hash)

			if item.Direction == types.Forward {
				require.Greater(t, relatedTx.blockIndex, located.blockIndex)
			} else {
				require.LessOrEqual(t, relatedTx.blockIndex, located.blockIndex)
			}
		}

		// Only destination legs have derived identifiers.
		if strings.HasSuffix(hash, unified.DestinationLegSuffix) {
			require.Equal(t, "destination", getCrossShardMetadata(t, located.tx)["leg"])
		}
	}

	t.Logf("checked %d cross-shard transfers (%d failed at destination), %d transactions, %d blocks",
		len(transfers), numFailed, len(view.transactions), len(view.blocks))
}

func TestSimulation_UnifiedChainReconciles(t *testing.T) {
	testCases := []struct {
		numShards uint32
		seed      int64
		numRounds int
	}{
		{numShards: 3, seed: 1, numRounds: 150},
		{numShards: 3, seed: 2, numRounds: 150},
		{numShards: 3, seed: 3, numRounds: 150},
		{numShards: 2, seed: 4, numRounds: 120},
		{numShards: 5, seed: 5, numRounds: 120},
		{numShards: 4, seed: 6, numRounds: 120},
	}

	for _, testCase := range testCases {
		t.Run(fmt.Sprintf("shards=%d,seed=%d", testCase.numShards, testCase.seed), func(t *testing.T) {
			simulator := mock.NewSimulator(mock.ArgsNewSimulator{
				NumShards:           testCase.numShards,
				NumAccountsPerShard: 3,
				Seed:                testCase.seed,
			})

			h := newHarness(t, simulator)

			// The aggregator follows the network while it advances (as it does when running).
			for i := 0; i < testCase.numRounds/10; i++ {
				simulator.RunRounds(10)
				require.NoError(t, h.step())
			}

			simulator.Settle()
			h.sync()

			view := checkUnifiedChain(h, 0)
			checkCrossShardTransfers(h, view, true)

			health, statusCode := h.getHealth()
			require.Equal(t, 200, statusCode)
			require.True(t, health.Ready)
			require.True(t, health.Synced)
			require.Equal(t, uint64(0), health.Anomalies.MergedDuplicates)
			require.Equal(t, uint64(0), health.Metachain.Lag)
		})
	}
}

func TestSimulation_SingleShard(t *testing.T) {
	// With one shard there are no cross-shard transactions: the unified chain holds the transactions of the shard, regrouped by metablock.
	simulator := mock.NewSimulator(mock.ArgsNewSimulator{NumShards: 1, NumAccountsPerShard: 4, Seed: 7})
	h := newHarness(t, simulator)

	simulator.RunRounds(80)
	simulator.Settle()
	h.sync()

	view := checkUnifiedChain(h, 0)
	require.Empty(t, simulator.CrossShardTransfers())

	for hash, located := range view.transactions {
		require.False(t, strings.HasSuffix(hash, unified.DestinationLegSuffix))
		require.NotContains(t, located.tx.Metadata, "crossShard")
	}
}

func TestSimulation_ForwardLinksCanBeDisabled(t *testing.T) {
	simulator := mock.NewSimulator(mock.ArgsNewSimulator{NumShards: 3, NumAccountsPerShard: 3, Seed: 8})

	h := newHarness(t, simulator, func(setup *harnessSetup) {
		disabled := false
		setup.config.CrossShard.ForwardLinks = &disabled
	})

	simulator.RunRounds(80)
	simulator.Settle()
	h.sync()

	view := checkUnifiedChain(h, 0)
	checkCrossShardTransfers(h, view, false)
}

// TestSimulation_OverHttp runs the simulation with the real HTTP clients of the aggregator on both sides:
// the shard instances and the metachain observer of the simulated network are served over HTTP (on the loopback interface).
// This covers what the in-memory backends do not: the wire format (e.g. numbers within the metadata are decoded as floats).
func TestSimulation_OverHttp(t *testing.T) {
	simulator := mock.NewSimulator(mock.ArgsNewSimulator{NumShards: 3, NumAccountsPerShard: 3, Seed: 9})

	metaObserver := httptest.NewServer(mock.NewObserverHandler(simulator.MetaSource()))
	t.Cleanup(metaObserver.Close)

	observerSource, err := metachain.NewObserverSource([]string{metaObserver.URL}, metaObserver.Client())
	require.NoError(t, err)

	backendsByShard := make(map[uint32]backend.ShardBackend)
	for shard := uint32(0); shard < simulator.NumShards(); shard++ {
		handler, err := mock.NewShardInstanceHandler(simulator.Backend(shard))
		require.NoError(t, err)

		shardInstance := httptest.NewServer(handler)
		t.Cleanup(shardInstance.Close)

		backendsByShard[shard], err = backend.NewHttpBackend([]string{shardInstance.URL}, shardInstance.Client())
		require.NoError(t, err)
	}

	backends, err := backend.NewSet(backendsByShard)
	require.NoError(t, err)

	h := newHarness(t, simulator, func(setup *harnessSetup) {
		setup.backends = backends
		setup.metaSource = observerSource
	})

	for i := 0; i < 8; i++ {
		simulator.RunRounds(10)
		require.NoError(t, h.step())
	}

	simulator.Settle()
	h.sync()

	view := checkUnifiedChain(h, 0)
	checkCrossShardTransfers(h, view, true)

	health, _ := h.getHealth()
	require.True(t, health.Synced)
	require.Equal(t, uint64(0), health.Anomalies.MergedDuplicates)
}

// TestSimulation_ManySeeds repeats the reconciliation check over many (smaller) simulated networks, to cover more interleavings
// of shard blocks, metablocks and cross-shard legs.
func TestSimulation_ManySeeds(t *testing.T) {
	if testing.Short() {
		t.Skip("long")
	}

	for seed := int64(100); seed < 130; seed++ {
		numShards := uint32(2 + seed%3)

		t.Run(fmt.Sprintf("shards=%d,seed=%d", numShards, seed), func(t *testing.T) {
			simulator := mock.NewSimulator(mock.ArgsNewSimulator{NumShards: numShards, NumAccountsPerShard: 2, Seed: seed})

			// Vary the pace of the chains: slower shards, or a slower metachain (which then notarizes several blocks at once).
			simulator.ProbabilityShardBlock = 0.5 + float64(seed%5)*0.1
			simulator.ProbabilityMetaBlock = 0.4 + float64(seed%7)*0.1

			h := newHarness(t, simulator)

			for i := 0; i < 6; i++ {
				simulator.RunRounds(10)
				require.NoError(t, h.step())
			}

			simulator.Settle()
			h.sync()

			view := checkUnifiedChain(h, 0)
			if len(simulator.CrossShardTransfers()) > 0 {
				checkCrossShardTransfers(h, view, true)
			}

			health, _ := h.getHealth()
			require.Equal(t, uint64(0), health.Anomalies.MergedDuplicates)
		})
	}
}
