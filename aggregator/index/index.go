package index

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/multiversx/mx-chain-rosetta/aggregator/metachain"
)

// Index is the notarization index. It validates the metachain data before storing it.
type Index struct {
	store  Store
	shards []uint32

	// Protects appending (the store itself is safe for concurrent reads)
	mutexAppend sync.Mutex
}

// New creates an index for the given (served) shards, on top of a store
func New(store Store, shards []uint32) (*Index, error) {
	if store == nil {
		return nil, errors.New("nil store")
	}
	if len(shards) == 0 {
		return nil, errors.New("no shards")
	}

	sortedShards := make([]uint32, len(shards))
	copy(sortedShards, shards)
	sort.Slice(sortedShards, func(i, j int) bool { return sortedShards[i] < sortedShards[j] })

	return &Index{
		store:  store,
		shards: sortedShards,
	}, nil
}

// Bounds returns the nonces of the first (base) and of the last record. "ok" is false if the index is empty.
func (index *Index) Bounds() (uint64, uint64, bool, error) {
	return index.store.Bounds()
}

// Get returns the record of a metablock, or ErrNotFound
func (index *Index) Get(metaNonce uint64) (*Record, error) {
	return index.store.Get(metaNonce)
}

// GetNonceByHash returns the nonce of the metablock with the given hash, if indexed
func (index *Index) GetNonceByHash(metaHash string) (uint64, bool, error) {
	return index.store.GetNonceByHash(metaHash)
}

// Frontier returns the last block of a shard notarized up to (and including) the given metablock
func (index *Index) Frontier(metaNonce uint64, shard uint32) (BlockRef, error) {
	record, err := index.store.Get(metaNonce)
	if err != nil {
		return BlockRef{}, err
	}

	frontier, ok := record.Frontier(shard)
	if !ok {
		return BlockRef{}, fmt.Errorf("%w: shard %d is missing from record %d", ErrInconsistent, shard, metaNonce)
	}

	return frontier, nil
}

// Bootstrap creates the first record of the index (the "base"), for the metablock with the given nonce.
// The frontier of each shard is found by scanning the metachain backwards from the base, until a notarized block of that shard is seen
// (or until the genesis is reached). The base record is only used as a reference: the first block that can be served is the one after it.
func (index *Index) Bootstrap(ctx context.Context, source metachain.Source, baseNonce uint64, maxBackwardSteps int) error {
	index.mutexAppend.Lock()
	defer index.mutexAppend.Unlock()

	_, _, hasRecords, err := index.store.Bounds()
	if err != nil {
		return err
	}
	if hasRecords {
		return errors.New("the notarization index is already bootstrapped")
	}

	baseBlock, err := source.GetBlockByNonce(ctx, baseNonce)
	if err != nil {
		return err
	}

	frontiers := make(map[uint32]BlockRef, len(index.shards))
	block := baseBlock
	steps := 0

	for {
		for _, shard := range index.shards {
			if _, found := frontiers[shard]; found {
				continue
			}

			notarized := block.NotarizedOfShard(shard)
			if len(notarized) > 0 {
				latest := notarized[len(notarized)-1]
				frontiers[shard] = BlockRef{Nonce: latest.Nonce, Hash: latest.Hash}
			}
		}

		if len(frontiers) == len(index.shards) {
			break
		}
		if block.Nonce == 0 {
			// Genesis reached: the remaining shards have no notarized block yet (their frontier is their genesis block).
			break
		}

		steps++
		if steps > maxBackwardSteps {
			return fmt.Errorf("cannot bootstrap the notarization index at metablock %d: no notarized block found for all shards within %d metablocks", baseNonce, maxBackwardSteps)
		}

		block, err = source.GetBlockByNonce(ctx, block.Nonce-1)
		if err != nil {
			return err
		}
	}

	spans := make([]ShardSpan, 0, len(index.shards))
	for _, shard := range index.shards {
		// For shards not found, the frontier is the zero value (genesis, unknown hash).
		spans = append(spans, ShardSpan{Shard: shard, Frontier: frontiers[shard]})
	}

	return index.store.Put(&Record{
		MetaNonce:   baseBlock.Nonce,
		MetaHash:    baseBlock.Hash,
		PrevHash:    baseBlock.PrevHash,
		TimestampMs: baseBlock.TimestampMs,
		Epoch:       baseBlock.Epoch,
		Round:       baseBlock.Round,
		Shards:      spans,
	})
}

// Append validates a metablock against the last record, then adds its record.
// The metablock must be final: records are never rewritten.
func (index *Index) Append(block *metachain.Block) error {
	index.mutexAppend.Lock()
	defer index.mutexAppend.Unlock()

	_, last, hasRecords, err := index.store.Bounds()
	if err != nil {
		return err
	}
	if !hasRecords {
		return errors.New("the notarization index is not bootstrapped")
	}

	previous, err := index.store.Get(last)
	if err != nil {
		return err
	}

	if block.Nonce != previous.MetaNonce+1 {
		return fmt.Errorf("%w: metablock %d does not follow the last indexed metablock (%d)", ErrInconsistent, block.Nonce, previous.MetaNonce)
	}
	if normalizeHash(block.PrevHash) != normalizeHash(previous.MetaHash) {
		return fmt.Errorf("%w: metablock %d has previous hash %s, but the indexed metablock %d has hash %s",
			ErrInconsistent, block.Nonce, block.PrevHash, previous.MetaNonce, previous.MetaHash)
	}

	for _, notarized := range block.Notarized {
		if !index.hasShard(notarized.Shard) {
			return fmt.Errorf("%w: metablock %d notarizes a block of shard %d, which is not served (is the number of shards configured correctly?)",
				ErrInconsistent, block.Nonce, notarized.Shard)
		}
	}

	spans := make([]ShardSpan, 0, len(index.shards))

	for _, shard := range index.shards {
		previousFrontier, ok := previous.Frontier(shard)
		if !ok {
			return fmt.Errorf("%w: shard %d is missing from record %d", ErrInconsistent, shard, previous.MetaNonce)
		}

		notarized := block.NotarizedOfShard(shard)
		span := ShardSpan{Shard: shard, Frontier: previousFrontier}

		for i, item := range notarized {
			expectedNonce := previousFrontier.Nonce + 1 + uint64(i)
			if item.Nonce != expectedNonce {
				return fmt.Errorf("%w: metablock %d notarizes block %d of shard %d, but block %d was expected",
					ErrInconsistent, block.Nonce, item.Nonce, shard, expectedNonce)
			}
			if len(item.Hash) == 0 {
				return fmt.Errorf("%w: metablock %d notarizes block %d of shard %d without a hash", ErrInconsistent, block.Nonce, item.Nonce, shard)
			}

			ref := BlockRef{Nonce: item.Nonce, Hash: item.Hash}
			span.New = append(span.New, ref)
			span.Frontier = ref
		}

		spans = append(spans, span)
	}

	return index.store.Put(&Record{
		MetaNonce:   block.Nonce,
		MetaHash:    block.Hash,
		PrevHash:    block.PrevHash,
		TimestampMs: block.TimestampMs,
		Epoch:       block.Epoch,
		Round:       block.Round,
		Shards:      spans,
	})
}

func (index *Index) hasShard(shard uint32) bool {
	for _, item := range index.shards {
		if item == shard {
			return true
		}
	}

	return false
}

// Close closes the underlying store
func (index *Index) Close() error {
	return index.store.Close()
}
