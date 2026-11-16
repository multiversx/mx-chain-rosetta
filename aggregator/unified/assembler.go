package unified

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/multiversx/mx-chain-rosetta/aggregator/backend"
	"github.com/multiversx/mx-chain-rosetta/aggregator/index"
	"github.com/multiversx/mx-chain-storage-go/lrucache"
)

const (
	defaultBlocksCacheSize      = 256
	defaultMaxConcurrentFetches = 32

	blockMetadataKeyShard = "shard"
)

type blocksCache interface {
	Get(key []byte) (value interface{}, ok bool)
	Put(key []byte, value interface{}, size int) (evicted bool)
}

// ArgsNewAssembler holds the arguments for creating an Assembler
type ArgsNewAssembler struct {
	Index     *index.Index
	Chain     *Chain
	Backends  *backend.Set
	Addresses addressesObserver
	Network   *types.NetworkIdentifier

	GenesisBlockIdentifier *types.BlockIdentifier
	GenesisTimestampMs     int64

	// ForwardLinks enables the "forward" links from source legs to destination legs (when the destination leg is certain)
	ForwardLinks bool

	BlocksCacheSize      int
	MaxConcurrentFetches int
}

// Assembler builds unified blocks out of the blocks returned by the shard instances
type Assembler struct {
	index      *index.Index
	chain      *Chain
	backends   *backend.Set
	network    *types.NetworkIdentifier
	classifier *legsClassifier

	genesisBlockIdentifier *types.BlockIdentifier
	genesisTimestampMs     int64

	cache          blocksCache
	fetchSemaphore chan struct{}

	mutexInFlight sync.Mutex
	inFlight      map[uint64]*inFlightAssembly

	mutexGenesis sync.Mutex
	genesisBlock *types.Block
}

type inFlightAssembly struct {
	done  chan struct{}
	block *types.Block
	err   error
}

// NewAssembler creates an Assembler
func NewAssembler(args ArgsNewAssembler) (*Assembler, error) {
	if args.Index == nil {
		return nil, errors.New("nil index")
	}
	if args.Chain == nil {
		return nil, errors.New("nil chain")
	}
	if args.Backends == nil {
		return nil, errors.New("nil backends")
	}
	if args.Addresses == nil {
		return nil, errors.New("nil addresses observer")
	}
	if args.Network == nil {
		return nil, errors.New("nil network identifier")
	}
	if args.GenesisBlockIdentifier == nil {
		return nil, errors.New("nil genesis block identifier")
	}

	cache, err := lrucache.NewCache(positiveOrDefault(args.BlocksCacheSize, defaultBlocksCacheSize))
	if err != nil {
		return nil, err
	}

	return &Assembler{
		index:                  args.Index,
		chain:                  args.Chain,
		backends:               args.Backends,
		network:                args.Network,
		classifier:             newLegsClassifier(args.Addresses, args.ForwardLinks),
		genesisBlockIdentifier: args.GenesisBlockIdentifier,
		genesisTimestampMs:     args.GenesisTimestampMs,
		cache:                  cache,
		fetchSemaphore:         make(chan struct{}, positiveOrDefault(args.MaxConcurrentFetches, defaultMaxConcurrentFetches)),
		inFlight:               make(map[uint64]*inFlightAssembly),
	}, nil
}

// GetBlock returns the unified block with the given index.
// The returned block is shared (cached): it must not be modified by the caller.
func (assembler *Assembler) GetBlock(ctx context.Context, nonce uint64) (*types.Block, error) {
	if nonce == 0 {
		return assembler.GetGenesisBlock(ctx)
	}

	snapshot, err := assembler.chain.Snapshot()
	if err != nil {
		return nil, err
	}
	if nonce > snapshot.Tip {
		return nil, ErrBlockNotYetAvailable
	}
	if nonce <= snapshot.Base {
		// The base record is only a reference (the range of shard blocks that it notarizes is not known).
		return nil, ErrBlockTooOld
	}

	cacheKey := encodeNonce(nonce)
	cached, ok := assembler.cache.Get(cacheKey)
	if ok {
		block, isBlock := cached.(*types.Block)
		if isBlock {
			return block, nil
		}
	}

	// Concurrent requests for the same block share one assembly.
	assembler.mutexInFlight.Lock()
	assembly, isInFlight := assembler.inFlight[nonce]
	if !isInFlight {
		assembly = &inFlightAssembly{done: make(chan struct{})}
		assembler.inFlight[nonce] = assembly
	}
	assembler.mutexInFlight.Unlock()

	if isInFlight {
		select {
		case <-assembly.done:
			return assembly.block, assembly.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	assembly.block, assembly.err = assembler.assemble(ctx, nonce)
	if assembly.err == nil {
		// Assembled blocks are immutable (they are built from final data), thus the cache needs no invalidation.
		assembler.cache.Put(cacheKey, assembly.block, 1)
	}

	assembler.mutexInFlight.Lock()
	delete(assembler.inFlight, nonce)
	assembler.mutexInFlight.Unlock()
	close(assembly.done)

	return assembly.block, assembly.err
}

type blockPart struct {
	shard uint32
	ref   index.BlockRef
	block *types.Block
	err   error
}

func (assembler *Assembler) assemble(ctx context.Context, nonce uint64) (*types.Block, error) {
	record, err := assembler.index.Get(nonce)
	if err != nil {
		if errors.Is(err, index.ErrNotFound) {
			return nil, ErrBlockNotYetAvailable
		}
		return nil, err
	}

	parentIdentifier, err := assembler.getBlockIdentifier(nonce - 1)
	if err != nil {
		return nil, err
	}

	// The parts are ordered by shard, then by nonce: this gives the order of the transactions within the unified block.
	parts := make([]*blockPart, 0)
	for _, span := range record.Shards {
		for _, ref := range span.New {
			parts = append(parts, &blockPart{shard: span.Shard, ref: ref})
		}
	}

	assembler.fetchParts(ctx, parts)

	transactions := make([]*types.Transaction, 0)
	shardBlocks := make([]interface{}, 0, len(parts))

	for _, part := range parts {
		if part.err != nil {
			return nil, part.err
		}

		for _, tx := range part.block.Transactions {
			if tx == nil || tx.TransactionIdentifier == nil {
				return nil, newErrBackendInconsistent(part.shard, fmt.Sprintf("block %d holds a transaction without identifier", part.ref.Nonce))
			}

			transactions = append(transactions, assembler.classifier.apply(part.shard, tx))
		}

		shardBlocks = append(shardBlocks, map[string]interface{}{
			"shard": part.shard,
			"nonce": part.ref.Nonce,
			"hash":  part.ref.Hash,
		})
	}

	// The two legs of a cross-shard transaction are never notarized by the same metablock (and their identifiers differ),
	// thus no duplicates are expected. Still, a block with duplicated transaction identifiers would be rejected by Rosetta clients.
	transactions, numDuplicates := mergeDuplicates(transactions)
	if numDuplicates > 0 {
		log.Warn("duplicated transaction identifiers within a unified block (merged)", "block", nonce, "numDuplicates", numDuplicates)
		assembler.chain.reportDuplicates(numDuplicates)
	}

	return &types.Block{
		BlockIdentifier: &types.BlockIdentifier{
			Index: int64(record.MetaNonce),
			Hash:  record.MetaHash,
		},
		ParentBlockIdentifier: parentIdentifier,
		Timestamp:             record.TimestampMs,
		Transactions:          transactions,
		Metadata: map[string]interface{}{
			"epoch":       record.Epoch,
			"round":       record.Round,
			"shardBlocks": shardBlocks,
		},
	}, nil
}

// getBlockIdentifier returns the identifier of a unified block (given its index)
func (assembler *Assembler) getBlockIdentifier(nonce uint64) (*types.BlockIdentifier, error) {
	if nonce == 0 {
		return assembler.genesisBlockIdentifier, nil
	}

	record, err := assembler.index.Get(nonce)
	if err != nil {
		return nil, err
	}

	return &types.BlockIdentifier{
		Index: int64(record.MetaNonce),
		Hash:  record.MetaHash,
	}, nil
}

// fetchParts fetches the shard blocks concurrently (bounded by a global limit, shared among all requests)
func (assembler *Assembler) fetchParts(ctx context.Context, parts []*blockPart) {
	var wg sync.WaitGroup

	for _, part := range parts {
		wg.Add(1)

		go func(part *blockPart) {
			defer wg.Done()

			select {
			case assembler.fetchSemaphore <- struct{}{}:
			case <-ctx.Done():
				part.err = ctx.Err()
				return
			}

			defer func() {
				<-assembler.fetchSemaphore
			}()

			part.block, part.err = assembler.fetchShardBlock(ctx, part.shard, part.ref)
		}(part)
	}

	wg.Wait()
}

// fetchShardBlock fetches a block from a shard instance, and checks it against what the metachain has notarized
func (assembler *Assembler) fetchShardBlock(ctx context.Context, shard uint32, ref index.BlockRef) (*types.Block, error) {
	shardBackend, ok := assembler.backends.Get(shard)
	if !ok {
		return nil, newErrBackendUnavailable(shard, errors.New("no backend"))
	}

	blockIndex := int64(ref.Nonce)
	response, rosettaErr, err := shardBackend.Block(ctx, &types.BlockRequest{
		NetworkIdentifier: assembler.network,
		BlockIdentifier:   &types.PartialBlockIdentifier{Index: &blockIndex},
	})
	if rosettaErr != nil {
		return nil, &BackendRosettaError{Shard: shard, RosettaError: rosettaErr}
	}
	if err != nil {
		return nil, newErrBackendUnavailable(shard, err)
	}
	if response == nil || response.Block == nil || response.Block.BlockIdentifier == nil {
		return nil, newErrBackendInconsistent(shard, fmt.Sprintf("empty response for block %d", ref.Nonce))
	}

	block := response.Block

	if block.BlockIdentifier.Index != blockIndex {
		return nil, newErrBackendInconsistent(shard, fmt.Sprintf("requested block %d, received block %d", blockIndex, block.BlockIdentifier.Index))
	}
	if len(ref.Hash) > 0 && !strings.EqualFold(block.BlockIdentifier.Hash, ref.Hash) {
		return nil, newErrBackendInconsistent(shard, fmt.Sprintf("block %d has hash %s, but the metachain has notarized %s", blockIndex, block.BlockIdentifier.Hash, ref.Hash))
	}

	// Shard instances set the shard in the block metadata. This catches backends wired to the wrong shard.
	actualShard, hasShard := readUint32(block.Metadata[blockMetadataKeyShard])
	if hasShard && actualShard != shard {
		return nil, newErrBackendInconsistent(shard, fmt.Sprintf("block %d belongs to shard %d", blockIndex, actualShard))
	}

	return block, nil
}

// GetGenesisBlock returns the genesis block of the unified chain, which holds the genesis operations of all served shards
func (assembler *Assembler) GetGenesisBlock(ctx context.Context) (*types.Block, error) {
	assembler.mutexGenesis.Lock()
	defer assembler.mutexGenesis.Unlock()

	if assembler.genesisBlock != nil {
		return assembler.genesisBlock, nil
	}

	parts := make([]*blockPart, 0)
	for _, shard := range assembler.backends.Shards() {
		// The hash of the genesis block of a shard is not known to the aggregator (thus, not checked).
		parts = append(parts, &blockPart{shard: shard, ref: index.BlockRef{Nonce: 0}})
	}

	assembler.fetchParts(ctx, parts)

	transactions := make([]*types.Transaction, 0)
	for _, part := range parts {
		if part.err != nil {
			return nil, part.err
		}

		for _, tx := range part.block.Transactions {
			if tx == nil || tx.TransactionIdentifier == nil {
				return nil, newErrBackendInconsistent(part.shard, "genesis block holds a transaction without identifier")
			}

			transactions = append(transactions, copyTransaction(tx))
		}
	}

	// Each shard instance reports the genesis balances of its own shard within a transaction that has the same identifier on all shards.
	// These are merged into one transaction (this is expected, and not counted as an anomaly).
	transactions, _ = mergeDuplicates(transactions)

	assembler.genesisBlock = &types.Block{
		BlockIdentifier:       assembler.genesisBlockIdentifier,
		ParentBlockIdentifier: assembler.genesisBlockIdentifier,
		Timestamp:             assembler.genesisTimestampMs,
		Transactions:          transactions,
	}

	return assembler.genesisBlock, nil
}

func encodeNonce(nonce uint64) []byte {
	encoded := make([]byte, 8)
	binary.BigEndian.PutUint64(encoded, nonce)
	return encoded
}
