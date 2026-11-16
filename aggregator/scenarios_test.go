package aggregator

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coinbase/rosetta-sdk-go/client"
	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/multiversx/mx-chain-core-go/core/pubkeyConverter"
	"github.com/multiversx/mx-chain-rosetta/aggregator/backend"
	"github.com/multiversx/mx-chain-rosetta/aggregator/index"
	"github.com/multiversx/mx-chain-rosetta/aggregator/metachain"
	"github.com/multiversx/mx-chain-rosetta/aggregator/mock"
	"github.com/multiversx/mx-chain-rosetta/aggregator/services"
	"github.com/stretchr/testify/require"
)

func newSettledSimulator(seed int64, numRounds int) *mock.Simulator {
	simulator := mock.NewSimulator(mock.ArgsNewSimulator{NumShards: 3, NumAccountsPerShard: 3, Seed: seed})
	simulator.RunRounds(numRounds)
	simulator.Settle()
	return simulator
}

// newTestPublicKeyBytes returns the bytes of a (dummy) public key. The request asserter rejects keys made of zeros only.
func newTestPublicKeyBytes() []byte {
	publicKey := make([]byte, 32)
	for i := range publicKey {
		publicKey[i] = byte(i + 1)
	}

	return publicKey
}

func requireErrorCode(t *testing.T, expected int32, rosettaErr *types.Error) {
	require.NotNil(t, rosettaErr, "expected error %d, got none", expected)
	require.Equal(t, expected, rosettaErr.Code, types.PrintStruct(rosettaErr))
}

// newAddressOfMetachainContract creates the address of a (system) smart contract that lives on the metachain
func newAddressOfMetachainContract(t *testing.T) string {
	converter, err := pubkeyConverter.NewBech32PubkeyConverter(32, "erd")
	require.NoError(t, err)

	pubKey := make([]byte, 32)
	pubKey[9] = 1
	pubKey[30] = 0xff
	pubKey[31] = 0xff

	address, err := converter.Encode(pubKey)
	require.NoError(t, err)
	return address
}

// findBlockWithShard returns the index of a unified block (in the given range) that holds at least one block of the given shard
func findBlockWithShard(h *harness, shard uint32, from uint64, to uint64) int64 {
	for nonce := from; nonce <= to; nonce++ {
		record, err := h.app.index.Get(nonce)
		require.NoError(h.t, err)

		span, ok := record.Span(shard)
		require.True(h.t, ok)
		if len(span.New) > 0 {
			return int64(nonce)
		}
	}

	require.Fail(h.t, "no unified block holds a block of the shard", "shard = %d", shard)
	return 0
}

func TestNetworkEndpoints(t *testing.T) {
	simulator := newSettledSimulator(11, 60)
	h := newHarness(t, simulator)
	h.sync()

	ctx := context.Background()
	lastMetaBlock := simulator.MetaBlocks()[len(simulator.MetaBlocks())-1]

	t.Run("list", func(t *testing.T) {
		response, rosettaErr, err := h.client.NetworkAPI.NetworkList(ctx, &types.MetadataRequest{})
		require.NoError(t, err)
		require.Nil(t, rosettaErr)
		require.Equal(t, []*types.NetworkIdentifier{simulator.Network()}, response.NetworkIdentifiers)
	})

	t.Run("options", func(t *testing.T) {
		response, rosettaErr := h.options()
		require.Nil(t, rosettaErr)

		require.Equal(t, "v0.0.0-test", *response.Version.MiddlewareVersion)
		require.Equal(t, "v0.0.0-mock", response.Version.Metadata["shardMiddlewareVersion"])
		require.Equal(t, "mock", response.Version.NodeVersion)
		require.True(t, response.Allow.HistoricalBalanceLookup)
		require.Contains(t, response.Allow.OperationTypes, "Transfer")
		require.Len(t, response.Allow.OperationStatuses, 2)

		// Errors of the shard instances, and errors of the aggregator (sorted by code, no duplicates).
		codes := make([]int32, 0)
		for _, item := range response.Allow.Errors {
			codes = append(codes, item.Code)
		}
		require.Equal(t, []int32{2, 4, 15, 120, 121, 122, 123, 124, 125, 126, 127, 128, 129, 130}, codes)
	})

	t.Run("status", func(t *testing.T) {
		response, rosettaErr := h.status()
		require.Nil(t, rosettaErr)

		require.Equal(t, int64(lastMetaBlock.Nonce), response.CurrentBlockIdentifier.Index)
		require.Equal(t, lastMetaBlock.Hash, response.CurrentBlockIdentifier.Hash)
		require.Equal(t, lastMetaBlock.TimestampMs, response.CurrentBlockTimestamp)
		require.Equal(t, &types.BlockIdentifier{Index: 0, Hash: simulator.GenesisBlockHash()}, response.GenesisBlockIdentifier)
		// The shard instances hold the whole history, and the index starts at genesis.
		require.Equal(t, response.GenesisBlockIdentifier, response.OldestBlockIdentifier)

		require.True(t, *response.SyncStatus.Synced)
		require.Equal(t, int64(lastMetaBlock.Nonce), *response.SyncStatus.CurrentIndex)
		require.Equal(t, int64(lastMetaBlock.Nonce), *response.SyncStatus.TargetIndex)

		require.Len(t, response.Peers, 3)
		for shard, peer := range response.Peers {
			require.Equal(t, fmt.Sprintf("observer-of-shard-%d", shard), peer.PeerID)
			require.Equal(t, float64(shard), peer.Metadata["shard"])
			require.Equal(t, "mock", peer.Metadata["version"])
		}
	})

	t.Run("responses are valid for Rosetta consumers", func(t *testing.T) {
		require.NotNil(t, h.getAsserter())
	})

	t.Run("oldest block follows the history held by the shard instances", func(t *testing.T) {
		// Each shard instance holds historical balances starting with a different block.
		oldestByShard := map[uint32]int64{0: 7, 1: 3, 2: 12}
		for shard, oldest := range oldestByShard {
			simulator.Backend(shard).SetOldestIndex(oldest)
		}

		require.NoError(t, h.step())

		response, rosettaErr := h.status()
		require.Nil(t, rosettaErr)

		oldest := uint64(response.OldestBlockIdentifier.Index)
		require.Greater(t, oldest, uint64(0))
		require.Equal(t, simulator.MetaBlocks()[oldest].Hash, response.OldestBlockIdentifier.Hash)

		// It is the first unified block whose frontiers are, for all shards, covered by the history of the shard instances.
		isCovered := func(nonce uint64) bool {
			record, err := h.app.index.Get(nonce)
			require.NoError(t, err)

			for shard, oldestOfShard := range oldestByShard {
				frontier, _ := record.Frontier(shard)
				if int64(frontier.Nonce) < oldestOfShard {
					return false
				}
			}

			return true
		}

		require.True(t, isCovered(oldest))
		require.False(t, isCovered(oldest-1))

		health, _ := h.getHealth()
		require.Equal(t, oldest, health.Oldest)
	})
}

func TestBlockEndpoint(t *testing.T) {
	simulator := newSettledSimulator(12, 40)
	h := newHarness(t, simulator)
	h.sync()

	tip := int64(h.tip())

	t.Run("genesis block holds the genesis balances of all shards", func(t *testing.T) {
		block := h.getBlock(0)
		require.Equal(t, &types.BlockIdentifier{Index: 0, Hash: simulator.GenesisBlockHash()}, block.BlockIdentifier)
		require.Equal(t, block.BlockIdentifier, block.ParentBlockIdentifier)
		require.Equal(t, mock.GenesisTimestamp*1000, block.Timestamp)

		// The genesis transactions of the shards (which share the identifier) are merged into one.
		require.Len(t, block.Transactions, 1)
		require.Len(t, block.Transactions[0].Operations, len(simulator.Accounts()))

		accounts := make(map[string]struct{})
		for i, operation := range block.Transactions[0].Operations {
			require.Equal(t, int64(i), operation.OperationIdentifier.Index)
			require.Equal(t, "GenesisBalanceMovement", operation.Type)
			accounts[operation.Account.Address] = struct{}{}
		}
		require.Len(t, accounts, len(simulator.Accounts()))

		// Fetching the genesis block is not counted as an anomaly, even if transactions are merged.
		health, _ := h.getHealth()
		require.Equal(t, uint64(0), health.Anomalies.MergedDuplicates)

		genesisHash := simulator.GenesisBlockHash()
		byHash, rosettaErr := h.tryGetBlock(&types.PartialBlockIdentifier{Hash: &genesisHash})
		require.Nil(t, rosettaErr)
		require.Equal(t, block, byHash)
	})

	t.Run("by hash", func(t *testing.T) {
		hash := simulator.MetaBlocks()[5].Hash
		byHash, rosettaErr := h.tryGetBlock(&types.PartialBlockIdentifier{Hash: &hash})
		require.Nil(t, rosettaErr)
		require.Equal(t, h.getBlock(5), byHash)

		// Hashes are not case-sensitive.
		upperHash := strings.ToUpper(hash)
		byHash, rosettaErr = h.tryGetBlock(&types.PartialBlockIdentifier{Hash: &upperHash})
		require.Nil(t, rosettaErr)
		require.Equal(t, int64(5), byHash.BlockIdentifier.Index)
	})

	t.Run("index and hash must refer to the same block", func(t *testing.T) {
		hash := simulator.MetaBlocks()[5].Hash
		blockIndex := int64(6)
		_, rosettaErr := h.tryGetBlock(&types.PartialBlockIdentifier{Index: &blockIndex, Hash: &hash})
		requireErrorCode(t, int32(services.ErrInvalidRequest), rosettaErr)

		blockIndex = 5
		block, rosettaErr := h.tryGetBlock(&types.PartialBlockIdentifier{Index: &blockIndex, Hash: &hash})
		require.Nil(t, rosettaErr)
		require.Equal(t, int64(5), block.BlockIdentifier.Index)
	})

	t.Run("unknown hash", func(t *testing.T) {
		hash := mock.FakeHash("unknown")
		_, rosettaErr := h.tryGetBlock(&types.PartialBlockIdentifier{Hash: &hash})
		requireErrorCode(t, int32(services.ErrBlockNotYetAvailable), rosettaErr)
		require.True(t, rosettaErr.Retriable)
	})

	t.Run("above the tip", func(t *testing.T) {
		_, rosettaErr := h.tryGetBlockByIndex(tip + 1)
		requireErrorCode(t, int32(services.ErrBlockNotYetAvailable), rosettaErr)
		require.True(t, rosettaErr.Retriable)
	})

	t.Run("blocks are assembled once, then served from cache", func(t *testing.T) {
		blockIndex := findBlockWithShard(h, 1, 10, uint64(tip))
		numCallsBefore := simulator.Backend(1).NumCalls("/block")

		first := h.getBlock(blockIndex)
		numCallsAfterFirst := simulator.Backend(1).NumCalls("/block")
		require.Greater(t, numCallsAfterFirst, numCallsBefore)

		second := h.getBlock(blockIndex)
		require.Equal(t, first, second)
		require.Equal(t, numCallsAfterFirst, simulator.Backend(1).NumCalls("/block"))
	})

	t.Run("/block/transaction is not implemented", func(t *testing.T) {
		_, rosettaErr, _ := h.client.BlockAPI.BlockTransaction(context.Background(), &types.BlockTransactionRequest{
			NetworkIdentifier:     h.network,
			BlockIdentifier:       &types.BlockIdentifier{Index: 1, Hash: simulator.MetaBlocks()[1].Hash},
			TransactionIdentifier: &types.TransactionIdentifier{Hash: "aa"},
		})
		requireErrorCode(t, int32(services.ErrNotImplemented), rosettaErr)
	})

	t.Run("malformed requests are rejected with a Rosetta error", func(t *testing.T) {
		statusCode, body := h.post("/block", `{}`)
		require.Equal(t, http.StatusInternalServerError, statusCode)
		require.Contains(t, body, `"code"`)

		// Another network.
		statusCode, body = h.post("/block", `{"network_identifier":{"blockchain":"MultiversX","network":"another"},"block_identifier":{"index":1}}`)
		require.Equal(t, http.StatusInternalServerError, statusCode)
		require.Contains(t, body, `"code"`)
	})

	t.Run("oversized requests are rejected", func(t *testing.T) {
		huge := `{"network_identifier":{"blockchain":"MultiversX","network":"simulated"},"block_identifier":{"index":1},"padding":"` +
			strings.Repeat("x", int(h.config.Limits.MaxRequestBodyBytes)) + `"}`

		statusCode, _ := h.post("/block", huge)
		require.NotEqual(t, http.StatusOK, statusCode)
	})
}

func TestAccountBalanceEndpoint(t *testing.T) {
	simulator := newSettledSimulator(13, 40)
	h := newHarness(t, simulator)
	h.sync()

	tip := int64(h.tip())
	account := simulator.AccountsOfShard(1)[0]

	t.Run("without block: pinned to the tip of the unified chain", func(t *testing.T) {
		response, rosettaErr := h.tryGetBalance(account.Address, nil)
		require.Nil(t, rosettaErr)

		require.Equal(t, tip, response.BlockIdentifier.Index)
		require.Equal(t, simulator.MetaBlocks()[tip].Hash, response.BlockIdentifier.Hash)

		// The balance is the one in the shard of the account, at the frontier.
		frontier, err := h.app.index.Frontier(uint64(tip), account.Shard)
		require.NoError(t, err)

		expected, _ := simulator.Chain(account.Shard).GetBalance(account.Address, int64(frontier.Nonce))
		require.Equal(t, expected.String(), response.Balances[0].Value)
		require.Equal(t, mock.NativeCurrency, response.Balances[0].Currency)

		require.Equal(t, float64(account.Shard), response.Metadata["shard"])
		require.Equal(t, map[string]interface{}{"nonce": float64(frontier.Nonce), "hash": frontier.Hash}, response.Metadata["shardBlock"])
		// The metadata of the shard instance (e.g. the account nonce) is preserved.
		require.Contains(t, response.Metadata, "nonce")
	})

	t.Run("by hash", func(t *testing.T) {
		hash := simulator.MetaBlocks()[8].Hash
		response, rosettaErr := h.tryGetBalance(account.Address, &types.PartialBlockIdentifier{Hash: &hash})
		require.Nil(t, rosettaErr)
		require.Equal(t, int64(8), response.BlockIdentifier.Index)
		require.Equal(t, h.getBalance(account.Address, 8).String(), response.Balances[0].Value)
	})

	t.Run("at genesis", func(t *testing.T) {
		response, rosettaErr := h.tryGetBalance(account.Address, &types.PartialBlockIdentifier{Index: new(int64)})
		require.Nil(t, rosettaErr)
		require.Equal(t, &types.BlockIdentifier{Index: 0, Hash: simulator.GenesisBlockHash()}, response.BlockIdentifier)

		expected, _ := simulator.Chain(account.Shard).GetBalance(account.Address, 0)
		require.Equal(t, expected.String(), response.Balances[0].Value)
	})

	t.Run("above the tip", func(t *testing.T) {
		blockIndex := tip + 1
		_, rosettaErr := h.tryGetBalance(account.Address, &types.PartialBlockIdentifier{Index: &blockIndex})
		requireErrorCode(t, int32(services.ErrBlockNotYetAvailable), rosettaErr)
	})

	t.Run("address on the metachain", func(t *testing.T) {
		_, rosettaErr := h.tryGetBalance(newAddressOfMetachainContract(t), nil)
		requireErrorCode(t, int32(services.ErrAddressNotServed), rosettaErr)
		require.False(t, rosettaErr.Retriable)
	})

	t.Run("invalid address", func(t *testing.T) {
		for _, address := range []string{"foobar", "erd1foobar", "../node/status", "erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6tx"} {
			_, rosettaErr := h.tryGetBalance(address, nil)
			requireErrorCode(t, int32(services.ErrInvalidAddress), rosettaErr)
		}

		// Nothing reaches the shard instances.
		for shard := uint32(0); shard < 3; shard++ {
			numCalls := simulator.Backend(shard).NumCalls("/account/balance")
			_, _ = h.tryGetBalance("../node/status", nil)
			require.Equal(t, numCalls, simulator.Backend(shard).NumCalls("/account/balance"))
		}
	})

	t.Run("not implemented endpoints", func(t *testing.T) {
		_, rosettaErr, _ := h.client.AccountAPI.AccountCoins(context.Background(), &types.AccountCoinsRequest{
			NetworkIdentifier: h.network,
			AccountIdentifier: &types.AccountIdentifier{Address: account.Address},
		})
		requireErrorCode(t, int32(services.ErrNotImplemented), rosettaErr)
	})
}

func TestTipFollowsSlowestShardInstance(t *testing.T) {
	simulator := newSettledSimulator(14, 60)

	// The instance of shard 1 lags: it only serves its first 5 blocks.
	simulator.Backend(1).SetServedTip(5)

	h := newHarness(t, simulator)
	require.NoError(t, h.step())

	tip := h.tip()
	lastMetaNonce := uint64(len(simulator.MetaBlocks()) - 1)
	require.Less(t, tip, lastMetaNonce)

	// The tip is the last unified block that does not need more than what shard 1 can serve.
	frontierAtTip, err := h.app.index.Frontier(tip, 1)
	require.NoError(t, err)
	require.LessOrEqual(t, frontierAtTip.Nonce, uint64(5))

	frontierAfterTip, err := h.app.index.Frontier(tip+1, 1)
	require.NoError(t, err)
	require.Greater(t, frontierAfterTip.Nonce, uint64(5))

	status, rosettaErr := h.status()
	require.Nil(t, rosettaErr)
	require.Equal(t, int64(tip), status.CurrentBlockIdentifier.Index)
	require.Equal(t, int64(lastMetaNonce), *status.SyncStatus.TargetIndex)

	_, rosettaErr = h.tryGetBlockByIndex(int64(tip) + 1)
	requireErrorCode(t, int32(services.ErrBlockNotYetAvailable), rosettaErr)

	health, _ := h.getHealth()
	require.Equal(t, lastMetaNonce-tip, health.Metachain.Lag)
	require.Equal(t, lastMetaNonce, health.LastIndexed)

	// What is below the tip reconciles.
	checkUnifiedChain(h, 0)

	// The instance catches up: the tip advances (and never goes back).
	simulator.Backend(1).SetServedTip(-1)
	h.sync()
	require.Equal(t, lastMetaNonce, h.tip())

	view := checkUnifiedChain(h, 0)
	checkCrossShardTransfers(h, view, true)
}

func TestShardInstanceUnavailable(t *testing.T) {
	simulator := newSettledSimulator(15, 40)
	h := newHarness(t, simulator)
	h.sync()

	tipBefore := h.tip()
	cachedBlockIndex := findBlockWithShard(h, 2, 1, tipBefore/2)
	notCachedBlockIndex := findBlockWithShard(h, 2, tipBefore/2+1, tipBefore)
	cachedBlock := h.getBlock(cachedBlockIndex)

	accountOfShard0 := simulator.AccountsOfShard(0)[0]
	accountOfShard2 := simulator.AccountsOfShard(2)[0]

	// The instance of shard 2 goes down, while the network advances.
	simulator.Backend(2).SetDown(true)
	simulator.RunRounds(20)
	simulator.Settle()

	err := h.step()
	require.ErrorContains(t, err, "shard 2")

	// The tip stops where shard 2 was last known to be.
	tipWhileDown := h.tip()
	require.Less(t, tipWhileDown, uint64(len(simulator.MetaBlocks())-1))
	require.GreaterOrEqual(t, tipWhileDown, tipBefore)

	// Blocks already assembled are still served. The others, which need shard 2, are not.
	require.Equal(t, cachedBlock, h.getBlock(cachedBlockIndex))
	_, rosettaErr := h.tryGetBlockByIndex(notCachedBlockIndex)
	requireErrorCode(t, int32(services.ErrBackendUnavailable), rosettaErr)
	require.True(t, rosettaErr.Retriable)

	// Accounts of the other shards are not affected.
	_, rosettaErr = h.tryGetBalance(accountOfShard0.Address, nil)
	require.Nil(t, rosettaErr)
	_, rosettaErr = h.tryGetBalance(accountOfShard2.Address, nil)
	requireErrorCode(t, int32(services.ErrBackendUnavailable), rosettaErr)

	status, rosettaErr := h.status()
	require.Nil(t, rosettaErr)
	require.False(t, *status.SyncStatus.Synced)
	require.Equal(t, int64(tipWhileDown), status.CurrentBlockIdentifier.Index)

	health, statusCode := h.getHealth()
	require.Equal(t, http.StatusOK, statusCode)
	require.False(t, health.Synced)
	require.False(t, health.Shards[2].Reachable)
	require.NotEmpty(t, health.Shards[2].LastError)
	require.True(t, health.Shards[0].Reachable)

	// The instance comes back: everything resumes, by itself.
	simulator.Backend(2).SetDown(false)
	h.sync()

	view := checkUnifiedChain(h, 0)
	checkCrossShardTransfers(h, view, true)

	health, _ = h.getHealth()
	require.True(t, health.Synced)
}

func TestShardInstanceOnAnotherChain(t *testing.T) {
	simulator := newSettledSimulator(16, 40)
	h := newHarness(t, simulator)
	h.sync()

	tip := h.tip()
	blockIndex := findBlockWithShard(h, 1, 1, tip)
	account := simulator.AccountsOfShard(1)[0]

	// The instance of shard 1 returns blocks whose hashes differ from the ones notarized by the metachain
	// (e.g. it observes another network, or it is on a fork).
	simulator.Backend(1).SetTamperHashes(true)

	_, rosettaErr := h.tryGetBlockByIndex(blockIndex)
	requireErrorCode(t, int32(services.ErrBackendInconsistent), rosettaErr)
	require.Contains(t, rosettaErr.Details["details"], "shard 1")

	_, rosettaErr = h.tryGetBalance(account.Address, nil)
	requireErrorCode(t, int32(services.ErrBackendInconsistent), rosettaErr)

	// Nothing wrong was cached.
	simulator.Backend(1).SetTamperHashes(false)
	checkUnifiedChain(h, 0)
}

func TestMetachainUnavailable(t *testing.T) {
	simulator := newSettledSimulator(17, 40)
	h := newHarness(t, simulator)
	h.sync()

	tipBefore := h.tip()

	simulator.MetaSource().SetNextError(errors.New("metachain observer is down"))
	simulator.RunRounds(10)

	err := h.step()
	require.ErrorContains(t, err, "metachain observer is down")
	require.Equal(t, tipBefore, h.tip())

	// What is indexed is still served.
	checkUnifiedChain(h, 0)

	status, rosettaErr := h.status()
	require.Nil(t, rosettaErr)
	require.False(t, *status.SyncStatus.Synced)

	health, _ := h.getHealth()
	require.False(t, health.Metachain.Reachable)
	require.Contains(t, health.Metachain.LastError, "metachain observer is down")

	simulator.MetaSource().SetNextError(nil)
	simulator.Settle()
	h.sync()
	require.Greater(t, h.tip(), tipBefore)
	checkUnifiedChain(h, 0)
}

// tamperingMetaSource alters the metachain data, to simulate an inconsistent source
type tamperingMetaSource struct {
	metachain.Source
	breakLinkAtNonce uint64
	numShards        *uint32
}

func (source *tamperingMetaSource) GetBlockByNonce(ctx context.Context, nonce uint64) (*metachain.Block, error) {
	block, err := source.Source.GetBlockByNonce(ctx, nonce)
	if err != nil {
		return nil, err
	}

	if source.breakLinkAtNonce > 0 && nonce == source.breakLinkAtNonce {
		tampered := *block
		tampered.PrevHash = mock.FakeHash("another chain")
		return &tampered, nil
	}

	return block, nil
}

func (source *tamperingMetaSource) GetNumShards(ctx context.Context) (uint32, error) {
	if source.numShards != nil {
		return *source.numShards, nil
	}

	return source.Source.GetNumShards(ctx)
}

func TestInconsistentMetachainDataIsNotIndexed(t *testing.T) {
	simulator := newSettledSimulator(18, 40)
	source := &tamperingMetaSource{Source: simulator.MetaSource(), breakLinkAtNonce: 12}

	h := newHarness(t, simulator, func(setup *harnessSetup) {
		setup.metaSource = source
	})

	// Indexing stops before the metablock that does not link to its parent.
	err := h.step()
	require.ErrorIs(t, err, index.ErrInconsistent)
	require.Equal(t, uint64(11), h.tip())

	health, _ := h.getHealth()
	require.Equal(t, uint64(11), health.LastIndexed)
	require.Contains(t, health.Metachain.LastError, "previous hash")

	_, rosettaErr := h.tryGetBlockByIndex(12)
	requireErrorCode(t, int32(services.ErrBlockNotYetAvailable), rosettaErr)

	// Once the source is consistent again, indexing resumes.
	source.breakLinkAtNonce = 0
	h.sync()
	checkUnifiedChain(h, 0)
}

func TestConstructionEndpoints(t *testing.T) {
	simulator := newSettledSimulator(19, 20)
	h := newHarness(t, simulator)
	h.sync()

	ctx := context.Background()

	numCalls := func(endpoint string) []int {
		return []int{
			simulator.Backend(0).NumCalls(endpoint),
			simulator.Backend(1).NumCalls(endpoint),
			simulator.Backend(2).NumCalls(endpoint),
		}
	}

	t.Run("metadata is routed to the shard of the sender", func(t *testing.T) {
		for shard := uint32(0); shard < 3; shard++ {
			sender := simulator.AccountsOfShard(shard)[0].Address
			before := numCalls("/construction/metadata")

			response, rosettaErr, _ := h.client.ConstructionAPI.ConstructionMetadata(ctx, &types.ConstructionMetadataRequest{
				NetworkIdentifier: h.network,
				Options:           map[string]interface{}{"sender": sender, "receiver": "anyone"},
			})
			require.Nil(t, rosettaErr)
			require.Equal(t, float64(shard), response.Metadata["servedShard"])
			require.Equal(t, sender, response.Metadata["sender"])

			after := numCalls("/construction/metadata")
			for otherShard := uint32(0); otherShard < 3; otherShard++ {
				expectedDelta := 0
				if otherShard == shard {
					expectedDelta = 1
				}
				require.Equal(t, expectedDelta, after[otherShard]-before[otherShard])
			}
		}
	})

	t.Run("metadata: bad sender", func(t *testing.T) {
		request := func(options map[string]interface{}) *types.Error {
			_, rosettaErr, _ := h.client.ConstructionAPI.ConstructionMetadata(ctx, &types.ConstructionMetadataRequest{
				NetworkIdentifier: h.network,
				Options:           options,
			})
			return rosettaErr
		}

		requireErrorCode(t, int32(services.ErrInvalidRequest), request(map[string]interface{}{"receiver": "anyone"}))
		requireErrorCode(t, int32(services.ErrInvalidRequest), request(map[string]interface{}{"sender": 42}))
		requireErrorCode(t, int32(services.ErrInvalidAddress), request(map[string]interface{}{"sender": "../node/status"}))
		requireErrorCode(t, int32(services.ErrAddressNotServed), request(map[string]interface{}{"sender": newAddressOfMetachainContract(t)}))
	})

	t.Run("submit is routed to the shard of the sender", func(t *testing.T) {
		sender := simulator.AccountsOfShard(1)[1].Address
		signedTransaction := fmt.Sprintf(`{"nonce":7,"value":"1","receiver":"anyone","sender":"%s","signature":"aa"}`, sender)

		response, rosettaErr, _ := h.client.ConstructionAPI.ConstructionSubmit(ctx, &types.ConstructionSubmitRequest{
			NetworkIdentifier: h.network,
			SignedTransaction: signedTransaction,
		})
		require.Nil(t, rosettaErr)
		require.NotEmpty(t, response.TransactionIdentifier.Hash)

		require.Empty(t, simulator.Backend(0).SubmittedTransactions())
		require.Equal(t, []string{signedTransaction}, simulator.Backend(1).SubmittedTransactions())
		require.Empty(t, simulator.Backend(2).SubmittedTransactions())
	})

	t.Run("submit: bad transaction", func(t *testing.T) {
		request := func(signedTransaction string) *types.Error {
			_, rosettaErr, _ := h.client.ConstructionAPI.ConstructionSubmit(ctx, &types.ConstructionSubmitRequest{
				NetworkIdentifier: h.network,
				SignedTransaction: signedTransaction,
			})
			return rosettaErr
		}

		requireErrorCode(t, int32(services.ErrInvalidRequest), request("not json"))
		requireErrorCode(t, int32(services.ErrInvalidRequest), request(`{"nonce":7}`))
		requireErrorCode(t, int32(services.ErrInvalidAddress), request(`{"sender":"foobar"}`))
		requireErrorCode(t, int32(services.ErrAddressNotServed), request(fmt.Sprintf(`{"sender":"%s"}`, newAddressOfMetachainContract(t))))
	})

	publicKey := &types.PublicKey{Bytes: newTestPublicKeyBytes(), CurveType: types.Edwards25519}
	account := &types.AccountIdentifier{Address: simulator.AccountsOfShard(0)[0].Address}
	operations := []*types.Operation{
		{
			OperationIdentifier: &types.OperationIdentifier{Index: 0},
			Type:                "Transfer",
			Account:             account,
			Amount:              &types.Amount{Value: "-1", Currency: mock.NativeCurrency},
		},
	}

	// These endpoints do not depend on the state of a shard: any shard instance can answer.
	stateless := map[string]func() *types.Error{
		"/construction/derive": func() *types.Error {
			_, rosettaErr, _ := h.client.ConstructionAPI.ConstructionDerive(ctx, &types.ConstructionDeriveRequest{NetworkIdentifier: h.network, PublicKey: publicKey})
			return rosettaErr
		},
		"/construction/preprocess": func() *types.Error {
			_, rosettaErr, _ := h.client.ConstructionAPI.ConstructionPreprocess(ctx, &types.ConstructionPreprocessRequest{NetworkIdentifier: h.network, Operations: operations})
			return rosettaErr
		},
		"/construction/preprocess_operations": func() *types.Error {
			_, rosettaErr, _ := h.client.ConstructionAPI.ConstructionPreprocessOperations(ctx, &types.ConstructionPreprocessOperationsRequest{NetworkIdentifier: h.network, ConstructOp: "transfer"})
			return rosettaErr
		},
		"/construction/payloads": func() *types.Error {
			_, rosettaErr, _ := h.client.ConstructionAPI.ConstructionPayloads(ctx, &types.ConstructionPayloadsRequest{NetworkIdentifier: h.network, Operations: operations})
			return rosettaErr
		},
		"/construction/parse": func() *types.Error {
			_, rosettaErr, _ := h.client.ConstructionAPI.ConstructionParse(ctx, &types.ConstructionParseRequest{NetworkIdentifier: h.network, Transaction: "{}"})
			return rosettaErr
		},
		"/construction/combine": func() *types.Error {
			_, rosettaErr, _ := h.client.ConstructionAPI.ConstructionCombine(ctx, &types.ConstructionCombineRequest{
				NetworkIdentifier:   h.network,
				UnsignedTransaction: "{}",
				Signatures: []*types.Signature{
					{
						SigningPayload: &types.SigningPayload{AccountIdentifier: account, Bytes: []byte("payload"), SignatureType: types.Ed25519},
						PublicKey:      publicKey,
						SignatureType:  types.Ed25519,
						Bytes:          append(newTestPublicKeyBytes(), newTestPublicKeyBytes()...),
					},
				},
			})
			return rosettaErr
		},
		"/construction/hash": func() *types.Error {
			_, rosettaErr, _ := h.client.ConstructionAPI.ConstructionHash(ctx, &types.ConstructionHashRequest{NetworkIdentifier: h.network, SignedTransaction: "{}"})
			return rosettaErr
		},
	}

	for endpoint, call := range stateless {
		t.Run(endpoint+" is answered by any shard instance", func(t *testing.T) {
			before := numCalls(endpoint)
			require.Nil(t, call())
			after := numCalls(endpoint)
			require.Equal(t, []int{before[0] + 1, before[1], before[2]}, after)

			// When the first instance cannot be reached, the next one answers.
			simulator.Backend(0).SetDown(true)
			require.Nil(t, call())
			simulator.Backend(0).SetDown(false)
			require.Equal(t, after[1]+1, simulator.Backend(1).NumCalls(endpoint))
		})
	}

	t.Run("all shard instances are down", func(t *testing.T) {
		for shard := uint32(0); shard < 3; shard++ {
			simulator.Backend(shard).SetDown(true)
		}

		defer func() {
			for shard := uint32(0); shard < 3; shard++ {
				simulator.Backend(shard).SetDown(false)
			}
		}()

		requireErrorCode(t, int32(services.ErrBackendUnavailable), stateless["/construction/derive"]())

		_, rosettaErr, _ := h.client.ConstructionAPI.ConstructionMetadata(ctx, &types.ConstructionMetadataRequest{
			NetworkIdentifier: h.network,
			Options:           map[string]interface{}{"sender": account.Address},
		})
		requireErrorCode(t, int32(services.ErrBackendUnavailable), rosettaErr)
	})
}

func TestMempoolEndpoints(t *testing.T) {
	simulator := newSettledSimulator(20, 20)
	h := newHarness(t, simulator)
	h.sync()

	ctx := context.Background()

	pendingTx := &types.Transaction{
		TransactionIdentifier: &types.TransactionIdentifier{Hash: mock.FakeHash("pending")},
		Operations:            []*types.Operation{},
	}
	simulator.Backend(2).AddMempoolTransaction(pendingTx)

	request := func(hash string) (*types.MempoolTransactionResponse, *types.Error) {
		response, rosettaErr, _ := h.client.MempoolAPI.MempoolTransaction(ctx, &types.MempoolTransactionRequest{
			NetworkIdentifier:     h.network,
			TransactionIdentifier: &types.TransactionIdentifier{Hash: hash},
		})
		return response, rosettaErr
	}

	// The shard of a pending transaction is not known: all shard instances are asked.
	response, rosettaErr := request(pendingTx.TransactionIdentifier.Hash)
	require.Nil(t, rosettaErr)
	require.Equal(t, pendingTx.TransactionIdentifier, response.Transaction.TransactionIdentifier)

	// Not in any pool: the error of the shard instances is passed through.
	_, rosettaErr = request(mock.FakeHash("unknown"))
	requireErrorCode(t, 15, rosettaErr)
	require.Equal(t, "transaction is not in pool", rosettaErr.Message)

	// A shard instance that cannot be reached does not prevent the others from answering.
	simulator.Backend(0).SetDown(true)
	response, rosettaErr = request(pendingTx.TransactionIdentifier.Hash)
	require.Nil(t, rosettaErr)
	require.NotNil(t, response.Transaction)
	simulator.Backend(0).SetDown(false)

	_, rosettaErr, _ = h.client.MempoolAPI.Mempool(ctx, &types.NetworkRequest{NetworkIdentifier: h.network})
	requireErrorCode(t, int32(services.ErrNotImplemented), rosettaErr)
}

func TestIndexStartsLater(t *testing.T) {
	const base = 20

	check := func(t *testing.T, simulator *mock.Simulator, h *harness) {
		h.sync()

		// The base is only a reference: the first block that can be served is the one after it.
		for _, blockIndex := range []int64{base, base - 1, 5, 1} {
			_, rosettaErr := h.tryGetBlockByIndex(blockIndex)
			requireErrorCode(t, int32(services.ErrBlockTooOld), rosettaErr)
			require.False(t, rosettaErr.Retriable)
		}

		firstBlock := h.getBlock(base + 1)
		require.Equal(t, &types.BlockIdentifier{Index: base, Hash: simulator.MetaBlocks()[base].Hash}, firstBlock.ParentBlockIdentifier)

		// The genesis block is always available.
		require.Equal(t, int64(0), h.getBlock(0).BlockIdentifier.Index)

		status, rosettaErr := h.status()
		require.Nil(t, rosettaErr)
		require.Equal(t, int64(base+1), status.OldestBlockIdentifier.Index)
		require.Equal(t, simulator.MetaBlocks()[base+1].Hash, status.OldestBlockIdentifier.Hash)

		// Balances are available at the base (they are the baseline for reconciling the first block), not before.
		account := simulator.Accounts()[0]
		_, rosettaErr = h.tryGetBalance(account.Address, &types.PartialBlockIdentifier{Index: func() *int64 { v := int64(base - 1); return &v }()})
		requireErrorCode(t, int32(services.ErrBlockTooOld), rosettaErr)

		// Blocks before the base are not indexed (thus, not fetched from the metachain) - except for the ones needed to find the frontiers.
		health, _ := h.getHealth()
		require.Equal(t, uint64(base), health.Base)

		// A consumer that starts syncing after the base reconciles.
		checkUnifiedChain(h, base+1)
	}

	t.Run("start_nonce", func(t *testing.T) {
		simulator := newSettledSimulator(21, 80)
		h := newHarness(t, simulator, func(setup *harnessSetup) {
			setup.config.Index.StartNonce = base
		})

		check(t, simulator, h)
	})

	t.Run("start_epoch", func(t *testing.T) {
		simulator := newSettledSimulator(22, 80)
		simulator.MetaSource().SetEpochStartNonce(3, base)

		h := newHarness(t, simulator, func(setup *harnessSetup) {
			epoch := uint32(3)
			setup.config.Index.StartEpoch = &epoch
			// Ignored, since the epoch is set:
			setup.config.Index.StartNonce = 2
		})

		check(t, simulator, h)
	})

	t.Run("unknown start_epoch", func(t *testing.T) {
		simulator := newSettledSimulator(23, 20)

		_, err := tryNewHarness(t, simulator, func(setup *harnessSetup) {
			epoch := uint32(99)
			setup.config.Index.StartEpoch = &epoch
		})
		require.ErrorContains(t, err, "epoch 99")
	})
}

func TestRestartWithPersistentIndex(t *testing.T) {
	simulator := newSettledSimulator(24, 50)
	path := filepath.Join(t.TempDir(), "index")

	openStore := func(setup *harnessSetup) {
		store, err := index.NewLevelDBStore(path)
		require.NoError(t, err)
		setup.store = store
	}

	// First run.
	first := newHarness(t, simulator, openStore)
	first.sync()
	tipOfFirstRun := first.tip()
	blockOfFirstRun := first.getBlock(10)

	first.server.Close()
	require.NoError(t, first.app.Close())

	// The network advances while the aggregator is stopped.
	simulator.RunRounds(30)
	simulator.Settle()

	// Second run: the index is reused (the metablocks already indexed are not fetched again).
	numBlockRequestsBefore := simulator.MetaSource().NumBlockRequests
	second := newHarness(t, simulator, openStore)
	second.sync()

	numNewMetaBlocks := len(simulator.MetaBlocks()) - 1 - int(tipOfFirstRun)
	require.Greater(t, numNewMetaBlocks, 0)
	require.Equal(t, numNewMetaBlocks, simulator.MetaSource().NumBlockRequests-numBlockRequestsBefore)

	require.Equal(t, blockOfFirstRun, second.getBlock(10))

	view := checkUnifiedChain(second, 0)
	checkCrossShardTransfers(second, view, true)
}

func TestOfflineMode(t *testing.T) {
	simulator := newSettledSimulator(25, 10)

	// In offline mode, a single (offline) shard instance is sufficient, and the metachain is not needed.
	cfg := newConfigForSimulator(simulator)
	cfg.Server.Offline = true
	cfg.Shards = cfg.Shards[:1]
	cfg.Metachain.ObserverUrls = nil
	require.NoError(t, cfg.Validate())

	backends, err := backend.NewSet(map[uint32]backend.ShardBackend{0: simulator.Backend(0)})
	require.NoError(t, err)

	app, err := NewApp(ArgsNewApp{Config: cfg, Version: "v0.0.0-test", Backends: backends})
	require.NoError(t, err)
	require.NoError(t, app.Start(context.Background()))
	require.NoError(t, app.Step(context.Background()))

	server := httptest.NewServer(app.Handler())
	defer server.Close()

	h := &harness{
		t:         t,
		simulator: simulator,
		config:    cfg,
		app:       app,
		server:    server,
		network:   simulator.Network(),
		client:    client.NewAPIClient(client.NewConfiguration(server.URL, "test", server.Client())),
	}

	ctx := context.Background()
	offline := int32(services.ErrOfflineMode)
	account := simulator.AccountsOfShard(0)[0].Address

	// Endpoints that need the chain are not available.
	_, rosettaErr := h.status()
	requireErrorCode(t, offline, rosettaErr)

	_, rosettaErr, _ = h.client.BlockAPI.Block(ctx, &types.BlockRequest{NetworkIdentifier: h.network, BlockIdentifier: &types.PartialBlockIdentifier{Index: new(int64)}})
	requireErrorCode(t, offline, rosettaErr)

	_, rosettaErr = h.tryGetBalance(account, nil)
	requireErrorCode(t, offline, rosettaErr)

	_, rosettaErr, _ = h.client.MempoolAPI.MempoolTransaction(ctx, &types.MempoolTransactionRequest{NetworkIdentifier: h.network, TransactionIdentifier: &types.TransactionIdentifier{Hash: "aa"}})
	requireErrorCode(t, offline, rosettaErr)

	_, rosettaErr, _ = h.client.ConstructionAPI.ConstructionMetadata(ctx, &types.ConstructionMetadataRequest{NetworkIdentifier: h.network, Options: map[string]interface{}{"sender": account}})
	requireErrorCode(t, offline, rosettaErr)

	_, rosettaErr, _ = h.client.ConstructionAPI.ConstructionSubmit(ctx, &types.ConstructionSubmitRequest{NetworkIdentifier: h.network, SignedTransaction: fmt.Sprintf(`{"sender":"%s"}`, account)})
	requireErrorCode(t, offline, rosettaErr)

	// Endpoints that do not need the chain are available.
	networks, rosettaErr, _ := h.client.NetworkAPI.NetworkList(ctx, &types.MetadataRequest{})
	require.Nil(t, rosettaErr)
	require.Len(t, networks.NetworkIdentifiers, 1)

	options, rosettaErr := h.options()
	require.Nil(t, rosettaErr)
	require.Equal(t, "v0.0.0-test", *options.Version.MiddlewareVersion)

	derived, rosettaErr, _ := h.client.ConstructionAPI.ConstructionDerive(ctx, &types.ConstructionDeriveRequest{
		NetworkIdentifier: h.network,
		PublicKey:         &types.PublicKey{Bytes: newTestPublicKeyBytes(), CurveType: types.Edwards25519},
	})
	require.Nil(t, rosettaErr)
	require.NotNil(t, derived.AccountIdentifier)

	_, rosettaErr, _ = h.client.ConstructionAPI.ConstructionHash(ctx, &types.ConstructionHashRequest{NetworkIdentifier: h.network, SignedTransaction: "{}"})
	require.Nil(t, rosettaErr)

	health, statusCode := h.getHealth()
	require.Equal(t, http.StatusOK, statusCode)
	require.True(t, health.Offline)

	// The metachain was never contacted.
	require.Equal(t, 0, simulator.MetaSource().NumBlockRequests)
}

func TestStartUpChecks(t *testing.T) {
	t.Run("number of shards differs from the one of the network", func(t *testing.T) {
		simulator := newSettledSimulator(26, 10)
		numShards := uint32(4)

		_, err := tryNewHarness(t, simulator, func(setup *harnessSetup) {
			setup.metaSource = &tamperingMetaSource{Source: simulator.MetaSource(), numShards: &numShards}
		})
		require.ErrorContains(t, err, "the network has 4 shards, but 3 are configured")
	})

	t.Run("a shard instance serves another network", func(t *testing.T) {
		simulator := newSettledSimulator(27, 10)
		anotherNetwork := &types.NetworkIdentifier{Blockchain: "MultiversX", Network: "another"}

		_, err := tryNewHarness(t, simulator, func(setup *harnessSetup) {
			backends, errSet := backend.NewSet(map[uint32]backend.ShardBackend{
				0: simulator.Backend(0),
				1: mock.NewShardBackend(anotherNetwork, simulator.Chain(1)),
				2: simulator.Backend(2),
			})
			require.NoError(t, errSet)
			setup.backends = backends
		})
		require.ErrorContains(t, err, "shard 1: the shard instance does not serve network")
	})

	t.Run("a shard instance cannot be reached", func(t *testing.T) {
		simulator := newSettledSimulator(28, 10)
		simulator.Backend(2).SetDown(true)

		_, err := tryNewHarness(t, simulator)
		require.ErrorContains(t, err, "shard 2")
	})

	t.Run("the metachain cannot be reached", func(t *testing.T) {
		simulator := newSettledSimulator(29, 10)
		simulator.MetaSource().SetNextError(errors.New("metachain observer is down"))

		_, err := tryNewHarness(t, simulator)
		require.ErrorContains(t, err, "metachain observer is down")
	})

	t.Run("genesis block hash differs from the one of the metachain", func(t *testing.T) {
		simulator := newSettledSimulator(30, 10)

		_, err := tryNewHarness(t, simulator, func(setup *harnessSetup) {
			setup.config.Network.GenesisBlockHash = mock.FakeHash("another genesis")
		})
		require.ErrorContains(t, err, "genesis block hash")
	})

	t.Run("missing dependencies", func(t *testing.T) {
		simulator := newSettledSimulator(31, 5)
		cfg := newConfigForSimulator(simulator)

		_, err := NewApp(ArgsNewApp{Config: cfg, Backends: simulator.Backends(), Store: index.NewMemoryStore()})
		require.ErrorContains(t, err, "nil metachain source")

		_, err = NewApp(ArgsNewApp{Config: cfg, Backends: simulator.Backends(), MetaSource: simulator.MetaSource()})
		require.ErrorContains(t, err, "nil index store")

		_, err = NewApp(ArgsNewApp{Config: cfg, MetaSource: simulator.MetaSource(), Store: index.NewMemoryStore()})
		require.ErrorContains(t, err, "nil backends")

		_, err = NewApp(ArgsNewApp{Backends: simulator.Backends()})
		require.ErrorContains(t, err, "nil config")
	})
}

func TestRecoverMiddleware(t *testing.T) {
	handler := recoverMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		panic("something went wrong")
	}))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/block", nil))

	// The client receives a Rosetta error (instead of a dropped connection).
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.JSONEq(t, `{"code":130,"message":"internal error","retriable":false}`, recorder.Body.String())
}

func TestCorsIsOptIn(t *testing.T) {
	simulator := newSettledSimulator(32, 5)

	getCorsHeader := func(h *harness) string {
		httpResponse, err := h.server.Client().Post(h.server.URL+"/network/list", "application/json", strings.NewReader("{}"))
		require.NoError(t, err)
		_ = httpResponse.Body.Close()
		return httpResponse.Header.Get("Access-Control-Allow-Origin")
	}

	require.Empty(t, getCorsHeader(newHarness(t, simulator)))

	withCors := newHarness(t, simulator, func(setup *harnessSetup) {
		setup.config.Server.EnableCors = true
	})
	require.Equal(t, "*", getCorsHeader(withCors))
}

func TestRun_FollowsTheNetwork(t *testing.T) {
	simulator := newSettledSimulator(33, 10)
	h := newHarness(t, simulator, func(setup *harnessSetup) {
		setup.config.Limits.SyncIntervalMilliseconds = 2
	})

	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})

	// As in the binary: the unified chain is kept synchronized in the background.
	go func() {
		h.app.Run(ctx)
		close(stopped)
	}()

	for i := 0; i < 5; i++ {
		simulator.RunRounds(10)
		simulator.Settle()

		lastMetaNonce := uint64(len(simulator.MetaBlocks()) - 1)
		require.Eventually(t, func() bool {
			return h.tip() == lastMetaNonce
		}, 5*time.Second, 2*time.Millisecond)

		// Requests are served while the chain advances.
		status, rosettaErr := h.status()
		require.Nil(t, rosettaErr)
		require.GreaterOrEqual(t, status.CurrentBlockIdentifier.Index, int64(lastMetaNonce))
	}

	cancel()

	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		require.Fail(t, "Run() did not stop when the context was canceled")
	}

	view := checkUnifiedChain(h, 0)
	checkCrossShardTransfers(h, view, true)
}

func TestConcurrentRequestsShareOneAssembly(t *testing.T) {
	simulator := newSettledSimulator(34, 40)
	h := newHarness(t, simulator)
	h.sync()

	tip := h.tip()
	blockIndex := findBlockWithShard(h, 0, tip/2, tip)

	record, err := h.app.index.Get(uint64(blockIndex))
	require.NoError(t, err)
	span, _ := record.Span(0)

	// Make sure the asserter (which is lazily created, and not safe for concurrent creation) exists.
	h.getAsserter()

	numCallsBefore := simulator.Backend(0).NumCalls("/block")

	const numClients = 32
	blocks := make([]*types.Block, numClients)

	var wg sync.WaitGroup
	for i := 0; i < numClients; i++ {
		wg.Add(1)

		go func(position int) {
			defer wg.Done()

			response, rosettaErr, errRequest := h.client.BlockAPI.Block(context.Background(), &types.BlockRequest{
				NetworkIdentifier: h.network,
				BlockIdentifier:   &types.PartialBlockIdentifier{Index: &blockIndex},
			})
			if rosettaErr == nil && errRequest == nil {
				blocks[position] = response.Block
			}
		}(i)
	}
	wg.Wait()

	for _, block := range blocks {
		require.NotNil(t, block)
		require.Equal(t, blocks[0], block)
	}

	// The shard blocks were fetched once, not once per client.
	require.Equal(t, len(span.New), simulator.Backend(0).NumCalls("/block")-numCallsBefore)
}

func TestHealth_NotReadyBeforeStart(t *testing.T) {
	simulator := newSettledSimulator(35, 5)

	app, err := NewApp(ArgsNewApp{
		Config:     newConfigForSimulator(simulator),
		Backends:   simulator.Backends(),
		MetaSource: simulator.MetaSource(),
		Store:      index.NewMemoryStore(),
	})
	require.NoError(t, err)

	// Start() was not called: the application answers, but reports that it is not ready.
	server := httptest.NewServer(app.Handler())
	defer server.Close()

	httpResponse, err := server.Client().Get(server.URL + "/health")
	require.NoError(t, err)
	_ = httpResponse.Body.Close()
	require.Equal(t, http.StatusServiceUnavailable, httpResponse.StatusCode)

	h := &harness{
		t:       t,
		server:  server,
		network: simulator.Network(),
		client:  client.NewAPIClient(client.NewConfiguration(server.URL, "test", server.Client())),
	}

	_, rosettaErr := h.status()
	requireErrorCode(t, int32(services.ErrBlockNotYetAvailable), rosettaErr)
	require.True(t, rosettaErr.Retriable)

	require.ErrorContains(t, app.Step(context.Background()), "not ready")
}

// TestDuplicatedIdentifiersWithinBlockAreMerged covers a situation that is not expected on the real network:
// two shard blocks, notarized by the same metablock, hold a transaction with the same identifier.
// Rosetta consumers reject blocks with duplicated transaction identifiers, thus the aggregator merges them (and reports the anomaly).
func TestDuplicatedIdentifiersWithinBlockAreMerged(t *testing.T) {
	network := &types.NetworkIdentifier{Blockchain: "MultiversX", Network: "handmade"}
	addresses := mock.NewSimulator(mock.ArgsNewSimulator{NumShards: 2, NumAccountsPerShard: 1, Seed: 1})
	alice := addresses.NewAddressInShard(0)
	bob := addresses.NewAddressInShard(1)

	status := mock.OpStatusSuccess
	newCredit := func(address string, value string) *types.Transaction {
		return &types.Transaction{
			TransactionIdentifier: &types.TransactionIdentifier{Hash: mock.FakeHash("same identifier")},
			Operations: []*types.Operation{
				{
					OperationIdentifier: &types.OperationIdentifier{Index: 0},
					Type:                "Reward",
					Status:              &status,
					Account:             &types.AccountIdentifier{Address: address},
					Amount:              &types.Amount{Value: value, Currency: mock.NativeCurrency},
				},
			},
		}
	}

	newChain := func(shard uint32, tx *types.Transaction) *mock.ShardChain {
		chain := mock.NewShardChain(shard)
		genesis := &types.BlockIdentifier{Index: 0, Hash: mock.FakeHash("handmade", shard, 0)}

		chain.AddBlock(&types.Block{BlockIdentifier: genesis, ParentBlockIdentifier: genesis, Timestamp: mock.GenesisTimestamp * 1000})
		chain.AddBlock(&types.Block{
			BlockIdentifier:       &types.BlockIdentifier{Index: 1, Hash: mock.FakeHash("handmade", shard, 1)},
			ParentBlockIdentifier: genesis,
			Timestamp:             mock.GenesisTimestamp*1000 + 6000,
			Transactions:          []*types.Transaction{tx},
			Metadata:              map[string]interface{}{"shard": shard},
		})

		return chain
	}

	chains := []*mock.ShardChain{newChain(0, newCredit(alice, "5")), newChain(1, newCredit(bob, "7"))}

	backends, err := backend.NewSet(map[uint32]backend.ShardBackend{
		0: mock.NewShardBackend(network, chains[0]),
		1: mock.NewShardBackend(network, chains[1]),
	})
	require.NoError(t, err)

	metaSource := mock.NewMetaSource(2)
	metaSource.AddBlock(&metachain.Block{Nonce: 0, Hash: mock.FakeHash("handmade-meta", 0), TimestampMs: mock.GenesisTimestamp * 1000})
	metaSource.AddBlock(&metachain.Block{
		Nonce:       1,
		Hash:        mock.FakeHash("handmade-meta", 1),
		PrevHash:    mock.FakeHash("handmade-meta", 0),
		TimestampMs: mock.GenesisTimestamp*1000 + 6000,
		Notarized: []metachain.NotarizedBlock{
			{Shard: 0, Nonce: 1, Hash: mock.FakeHash("handmade", 0, 1)},
			{Shard: 1, Nonce: 1, Hash: mock.FakeHash("handmade", 1, 1)},
		},
	})
	metaSource.SetHighestFinalNonce(1)

	h := newHarness(t, addresses, func(setup *harnessSetup) {
		setup.config.Network.Name = network.Network
		setup.config.Network.NumShards = 2
		setup.config.Network.GenesisBlockHash = mock.FakeHash("handmade-meta", 0)
		setup.backends = backends
		setup.metaSource = metaSource
	})
	require.NoError(t, h.step())

	// One transaction, holding the operations of both (re-indexed). The block is valid for Rosetta consumers (checked by the harness).
	block := h.getBlock(1)
	require.Len(t, block.Transactions, 1)
	require.Len(t, block.Transactions[0].Operations, 2)
	require.Equal(t, alice, block.Transactions[0].Operations[0].Account.Address)
	require.Equal(t, int64(0), block.Transactions[0].Operations[0].OperationIdentifier.Index)
	require.Equal(t, bob, block.Transactions[0].Operations[1].Account.Address)
	require.Equal(t, int64(1), block.Transactions[0].Operations[1].OperationIdentifier.Index)

	// Balances still reconcile.
	require.Equal(t, "5", h.getBalance(alice, 1).String())
	require.Equal(t, "7", h.getBalance(bob, 1).String())

	// The anomaly is reported (once: the block is cached).
	_ = h.getBlock(1)
	health, _ := h.getHealth()
	require.Equal(t, uint64(1), health.Anomalies.MergedDuplicates)

	// A block of another shard than the one configured for the backend is rejected.
	// (here, the instance of shard 1 is asked for the blocks of shard 0, and the other way around)
	swapped, err := backend.NewSet(map[uint32]backend.ShardBackend{
		0: mock.NewShardBackend(network, chains[1]),
		1: mock.NewShardBackend(network, chains[0]),
	})
	require.NoError(t, err)

	wronglyWired := newHarness(t, addresses, func(setup *harnessSetup) {
		setup.config.Network.Name = network.Network
		setup.config.Network.NumShards = 2
		setup.config.Network.GenesisBlockHash = mock.FakeHash("handmade-meta", 0)
		setup.backends = swapped
		setup.metaSource = metaSource
	})
	require.NoError(t, wronglyWired.step())

	_, rosettaErr := wronglyWired.tryGetBlockByIndex(1)
	requireErrorCode(t, int32(services.ErrBackendInconsistent), rosettaErr)
}
