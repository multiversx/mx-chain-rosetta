package index_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/multiversx/mx-chain-rosetta/aggregator/index"
	"github.com/multiversx/mx-chain-rosetta/aggregator/metachain"
	"github.com/multiversx/mx-chain-rosetta/aggregator/mock"
	"github.com/stretchr/testify/require"
)

func metaHash(nonce uint64) string {
	return mock.FakeHash("meta", nonce)
}

func shardHash(shard uint32, nonce uint64) string {
	return mock.FakeHash("shard", shard, nonce)
}

func newMetaBlock(nonce uint64, notarized ...metachain.NotarizedBlock) *metachain.Block {
	prevHash := "none"
	if nonce > 0 {
		prevHash = metaHash(nonce - 1)
	}

	return &metachain.Block{
		Nonce:       nonce,
		Hash:        metaHash(nonce),
		PrevHash:    prevHash,
		TimestampMs: 1_600_000_000_000 + int64(nonce)*6000,
		Epoch:       uint32(nonce / 10),
		Round:       nonce + 5,
		Notarized:   notarized,
	}
}

func notarized(shard uint32, nonce uint64) metachain.NotarizedBlock {
	return metachain.NotarizedBlock{Shard: shard, Nonce: nonce, Hash: shardHash(shard, nonce)}
}

// A small metachain: metablock 0 is genesis; then shard blocks are notarized at different paces.
func newSmallMetaSource() *mock.MetaSource {
	source := mock.NewMetaSource(2)
	source.AddBlock(newMetaBlock(0))
	source.AddBlock(newMetaBlock(1, notarized(0, 1), notarized(1, 1)))
	source.AddBlock(newMetaBlock(2, notarized(0, 2)))
	source.AddBlock(newMetaBlock(3))
	// Not ordered, and two blocks of the same shard:
	source.AddBlock(newMetaBlock(4, notarized(1, 3), notarized(0, 3), notarized(1, 2)))
	source.AddBlock(newMetaBlock(5, notarized(0, 4), notarized(1, 4)))
	return source
}

func forEachStore(t *testing.T, test func(t *testing.T, store index.Store)) {
	t.Run("memory", func(t *testing.T) {
		test(t, index.NewMemoryStore())
	})

	t.Run("leveldb", func(t *testing.T) {
		store, err := index.NewLevelDBStore(filepath.Join(t.TempDir(), "index"))
		require.NoError(t, err)

		defer func() {
			require.NoError(t, store.Close())
		}()

		test(t, store)
	})
}

func TestIndex_BootstrapAtGenesisAndAppend(t *testing.T) {
	forEachStore(t, func(t *testing.T, store index.Store) {
		ctx := context.Background()
		source := newSmallMetaSource()

		notarizationIndex, err := index.New(store, []uint32{1, 0})
		require.NoError(t, err)

		_, _, hasRecords, err := notarizationIndex.Bounds()
		require.NoError(t, err)
		require.False(t, hasRecords)

		// Appending requires a base record.
		block, _ := source.GetBlockByNonce(ctx, 1)
		require.ErrorContains(t, notarizationIndex.Append(block), "not bootstrapped")

		require.NoError(t, notarizationIndex.Bootstrap(ctx, source, 0, 100))
		require.ErrorContains(t, notarizationIndex.Bootstrap(ctx, source, 0, 100), "already bootstrapped")

		for nonce := uint64(1); nonce <= 5; nonce++ {
			block, err = source.GetBlockByNonce(ctx, nonce)
			require.NoError(t, err)
			require.NoError(t, notarizationIndex.Append(block))
		}

		first, last, hasRecords, err := notarizationIndex.Bounds()
		require.NoError(t, err)
		require.True(t, hasRecords)
		require.Equal(t, uint64(0), first)
		require.Equal(t, uint64(5), last)

		// Genesis: the frontiers are the genesis blocks (unknown hash).
		record, err := notarizationIndex.Get(0)
		require.NoError(t, err)
		require.Equal(t, []index.ShardSpan{{Shard: 0}, {Shard: 1}}, record.Shards)

		// Metablock 3 notarizes nothing: the frontiers do not move.
		record, err = notarizationIndex.Get(3)
		require.NoError(t, err)
		require.Equal(t, metaHash(3), record.MetaHash)
		require.Equal(t, metaHash(2), record.PrevHash)
		require.Equal(t, []index.ShardSpan{
			{Shard: 0, Frontier: index.BlockRef{Nonce: 2, Hash: shardHash(0, 2)}},
			{Shard: 1, Frontier: index.BlockRef{Nonce: 1, Hash: shardHash(1, 1)}},
		}, record.Shards)

		// Metablock 4 notarizes two blocks of shard 1 (received unordered).
		record, err = notarizationIndex.Get(4)
		require.NoError(t, err)
		require.Equal(t, []index.ShardSpan{
			{
				Shard:    0,
				Frontier: index.BlockRef{Nonce: 3, Hash: shardHash(0, 3)},
				New:      []index.BlockRef{{Nonce: 3, Hash: shardHash(0, 3)}},
			},
			{
				Shard:    1,
				Frontier: index.BlockRef{Nonce: 3, Hash: shardHash(1, 3)},
				New:      []index.BlockRef{{Nonce: 2, Hash: shardHash(1, 2)}, {Nonce: 3, Hash: shardHash(1, 3)}},
			},
		}, record.Shards)

		frontier, err := notarizationIndex.Frontier(2, 1)
		require.NoError(t, err)
		require.Equal(t, index.BlockRef{Nonce: 1, Hash: shardHash(1, 1)}, frontier)

		_, err = notarizationIndex.Frontier(2, 7)
		require.ErrorIs(t, err, index.ErrInconsistent)
		_, err = notarizationIndex.Get(6)
		require.ErrorIs(t, err, index.ErrNotFound)

		nonce, found, err := notarizationIndex.GetNonceByHash(metaHash(4))
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, uint64(4), nonce)

		_, found, err = notarizationIndex.GetNonceByHash(metaHash(99))
		require.NoError(t, err)
		require.False(t, found)
	})
}

func TestIndex_BootstrapLaterScansBackwards(t *testing.T) {
	forEachStore(t, func(t *testing.T, store index.Store) {
		ctx := context.Background()
		source := newSmallMetaSource()

		notarizationIndex, err := index.New(store, []uint32{0, 1})
		require.NoError(t, err)

		// Metablock 3 notarizes nothing. The frontiers are found in metablock 2 (shard 0) and in metablock 1 (shard 1).
		require.NoError(t, notarizationIndex.Bootstrap(ctx, source, 3, 100))

		record, err := notarizationIndex.Get(3)
		require.NoError(t, err)
		require.Equal(t, []index.ShardSpan{
			{Shard: 0, Frontier: index.BlockRef{Nonce: 2, Hash: shardHash(0, 2)}},
			{Shard: 1, Frontier: index.BlockRef{Nonce: 1, Hash: shardHash(1, 1)}},
		}, record.Shards)

		// The index continues from the base.
		for nonce := uint64(4); nonce <= 5; nonce++ {
			block, err := source.GetBlockByNonce(ctx, nonce)
			require.NoError(t, err)
			require.NoError(t, notarizationIndex.Append(block))
		}

		first, last, _, err := notarizationIndex.Bounds()
		require.NoError(t, err)
		require.Equal(t, uint64(3), first)
		require.Equal(t, uint64(5), last)

		_, err = notarizationIndex.Get(2)
		require.ErrorIs(t, err, index.ErrNotFound)
	})
}

func TestIndex_BootstrapFailsWhenScanIsTooLong(t *testing.T) {
	ctx := context.Background()

	source := mock.NewMetaSource(1)
	source.AddBlock(newMetaBlock(0))
	source.AddBlock(newMetaBlock(1, notarized(0, 1)))
	for nonce := uint64(2); nonce <= 10; nonce++ {
		source.AddBlock(newMetaBlock(nonce))
	}

	notarizationIndex, err := index.New(index.NewMemoryStore(), []uint32{0})
	require.NoError(t, err)
	require.ErrorContains(t, notarizationIndex.Bootstrap(ctx, source, 10, 3), "no notarized block found")

	// With enough steps, it works.
	require.NoError(t, notarizationIndex.Bootstrap(ctx, source, 10, 9))
	frontier, err := notarizationIndex.Frontier(10, 0)
	require.NoError(t, err)
	require.Equal(t, uint64(1), frontier.Nonce)
}

func TestIndex_AppendRejectsInconsistentData(t *testing.T) {
	ctx := context.Background()
	source := newSmallMetaSource()

	newIndexAt := func(t *testing.T, base uint64) *index.Index {
		notarizationIndex, err := index.New(index.NewMemoryStore(), []uint32{0, 1})
		require.NoError(t, err)
		require.NoError(t, notarizationIndex.Bootstrap(ctx, source, base, 100))
		return notarizationIndex
	}

	t.Run("not the next metablock", func(t *testing.T) {
		err := newIndexAt(t, 1).Append(newMetaBlock(3))
		require.ErrorIs(t, err, index.ErrInconsistent)
		require.ErrorContains(t, err, "does not follow")
	})

	t.Run("broken hash link", func(t *testing.T) {
		block := newMetaBlock(2, notarized(0, 2))
		block.PrevHash = mock.FakeHash("another chain")

		err := newIndexAt(t, 1).Append(block)
		require.ErrorIs(t, err, index.ErrInconsistent)
		require.ErrorContains(t, err, "previous hash")
	})

	t.Run("gap in the notarized shard blocks", func(t *testing.T) {
		err := newIndexAt(t, 1).Append(newMetaBlock(2, notarized(0, 3)))
		require.ErrorIs(t, err, index.ErrInconsistent)
		require.ErrorContains(t, err, "block 2 was expected")
	})

	t.Run("already notarized shard block", func(t *testing.T) {
		err := newIndexAt(t, 1).Append(newMetaBlock(2, notarized(0, 1)))
		require.ErrorIs(t, err, index.ErrInconsistent)
	})

	t.Run("unknown shard", func(t *testing.T) {
		err := newIndexAt(t, 1).Append(newMetaBlock(2, notarized(5, 1)))
		require.ErrorIs(t, err, index.ErrInconsistent)
		require.ErrorContains(t, err, "not served")
	})

	t.Run("notarized block without hash", func(t *testing.T) {
		err := newIndexAt(t, 1).Append(newMetaBlock(2, metachain.NotarizedBlock{Shard: 0, Nonce: 2}))
		require.ErrorIs(t, err, index.ErrInconsistent)
	})

	t.Run("nothing is stored upon failure", func(t *testing.T) {
		notarizationIndex := newIndexAt(t, 1)
		require.Error(t, notarizationIndex.Append(newMetaBlock(2, notarized(0, 3))))

		_, last, _, err := notarizationIndex.Bounds()
		require.NoError(t, err)
		require.Equal(t, uint64(1), last)
	})
}

func TestLevelDBStore_IsPersistent(t *testing.T) {
	ctx := context.Background()
	source := newSmallMetaSource()
	path := filepath.Join(t.TempDir(), "index")

	store, err := index.NewLevelDBStore(path)
	require.NoError(t, err)

	notarizationIndex, err := index.New(store, []uint32{0, 1})
	require.NoError(t, err)
	require.NoError(t, notarizationIndex.Bootstrap(ctx, source, 1, 100))

	for nonce := uint64(2); nonce <= 4; nonce++ {
		block, _ := source.GetBlockByNonce(ctx, nonce)
		require.NoError(t, notarizationIndex.Append(block))
	}

	recordBefore, err := notarizationIndex.Get(4)
	require.NoError(t, err)
	require.NoError(t, notarizationIndex.Close())

	// Reopen.
	store, err = index.NewLevelDBStore(path)
	require.NoError(t, err)

	notarizationIndex, err = index.New(store, []uint32{0, 1})
	require.NoError(t, err)

	defer func() {
		require.NoError(t, notarizationIndex.Close())
	}()

	first, last, hasRecords, err := notarizationIndex.Bounds()
	require.NoError(t, err)
	require.True(t, hasRecords)
	require.Equal(t, uint64(1), first)
	require.Equal(t, uint64(4), last)

	recordAfter, err := notarizationIndex.Get(4)
	require.NoError(t, err)
	require.Equal(t, recordBefore, recordAfter)

	nonce, found, err := notarizationIndex.GetNonceByHash(metaHash(3))
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, uint64(3), nonce)

	// The index continues where it was left.
	block, _ := source.GetBlockByNonce(ctx, 5)
	require.NoError(t, notarizationIndex.Append(block))
	require.ErrorIs(t, notarizationIndex.Append(block), index.ErrInconsistent)
}

func TestStores_RejectGaps(t *testing.T) {
	forEachStore(t, func(t *testing.T, store index.Store) {
		require.NoError(t, store.Put(&index.Record{MetaNonce: 7, MetaHash: metaHash(7)}))
		require.NoError(t, store.Put(&index.Record{MetaNonce: 8, MetaHash: metaHash(8)}))

		err := store.Put(&index.Record{MetaNonce: 10, MetaHash: metaHash(10)})
		require.ErrorIs(t, err, index.ErrInconsistent, fmt.Sprintf("%v", err))

		first, last, ok, err := store.Bounds()
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, uint64(7), first)
		require.Equal(t, uint64(8), last)
	})
}

func TestNew_Errors(t *testing.T) {
	_, err := index.New(nil, []uint32{0})
	require.Error(t, err)

	_, err = index.New(index.NewMemoryStore(), nil)
	require.Error(t, err)
}
