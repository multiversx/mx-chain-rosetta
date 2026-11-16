package unified

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coinbase/rosetta-sdk-go/types"
	logger "github.com/multiversx/mx-chain-logger-go"
	"github.com/multiversx/mx-chain-rosetta/aggregator/backend"
	"github.com/multiversx/mx-chain-rosetta/aggregator/index"
	"github.com/multiversx/mx-chain-rosetta/aggregator/metachain"
)

var log = logger.GetOrCreate("aggregator/unified")

const (
	defaultMaxBootstrapBackwardSteps = 10000
	defaultMaxRecordsPerStep         = 2048
	defaultFetchConcurrency          = 16
)

// ShardStatus is the status of a shard instance, as last seen by the aggregator
type ShardStatus struct {
	Shard uint32
	// HasStatus is true if the status of the shard instance was fetched (at least once)
	HasStatus bool
	// Reachable is true if the most recent attempt to fetch the status succeeded
	Reachable    bool
	Synced       bool
	CurrentIndex int64
	OldestIndex  int64
	Peers        []*types.Peer
	LastError    string
}

// Snapshot is a consistent view over the state of the unified chain
type Snapshot struct {
	// Base is the nonce of the first record of the index. Blocks above the base (and the genesis block) can be served.
	Base uint64
	// Tip is the highest unified block that can be served
	Tip       uint64
	TipRecord *index.Record
	// Oldest is the oldest unified block for which all shard instances hold historical balances
	Oldest       uint64
	OldestRecord *index.Record

	LastIndexed       uint64
	MetaReachable     bool
	MetaSynced        bool
	MetaFinalNonce    uint64
	MetaLastError     string
	Shards            []ShardStatus
	NumDuplicatesSeen uint64
}

// IsSynced returns whether the metachain source and all shard instances are reachable and synchronized
func (snapshot *Snapshot) IsSynced() bool {
	if !snapshot.MetaReachable || !snapshot.MetaSynced {
		return false
	}

	for _, shard := range snapshot.Shards {
		if !shard.Reachable || !shard.Synced {
			return false
		}
	}

	return true
}

// ArgsNewChain holds the arguments for creating a Chain
type ArgsNewChain struct {
	Index      *index.Index
	MetaSource metachain.Source
	Backends   *backend.Set
	Network    *types.NetworkIdentifier

	// StartNonce is the nonce of the metablock where the index starts (the "base"), when the index is empty.
	// It is ignored if "StartEpoch" is set.
	StartNonce uint64
	// StartEpoch, if set, makes the index start at the first metablock of the given epoch
	StartEpoch *uint32
	// GenesisBlockHash is checked against the metachain, when the index starts at genesis
	GenesisBlockHash string

	MaxBootstrapBackwardSteps int
	MaxRecordsPerStep         int
	FetchConcurrency          int
}

// Chain follows the metachain and the shard instances, maintains the notarization index and decides which unified blocks can be served
type Chain struct {
	index      *index.Index
	metaSource metachain.Source
	backends   *backend.Set
	network    *types.NetworkIdentifier

	startNonce       uint64
	startEpoch       *uint32
	genesisBlockHash string

	maxBootstrapBackwardSteps int
	maxRecordsPerStep         int
	fetchConcurrency          int

	mutexStep     sync.Mutex
	snapshot      atomic.Pointer[Snapshot]
	oldestKey     string
	oldestCached  uint64
	numDuplicates atomic.Uint64
	lastLoggedErr string
}

// NewChain creates a Chain
func NewChain(args ArgsNewChain) (*Chain, error) {
	if args.Index == nil {
		return nil, errors.New("nil index")
	}
	if args.MetaSource == nil {
		return nil, errors.New("nil metachain source")
	}
	if args.Backends == nil {
		return nil, errors.New("nil backends")
	}
	if args.Network == nil {
		return nil, errors.New("nil network identifier")
	}

	chain := &Chain{
		index:                     args.Index,
		metaSource:                args.MetaSource,
		backends:                  args.Backends,
		network:                   args.Network,
		startNonce:                args.StartNonce,
		startEpoch:                args.StartEpoch,
		genesisBlockHash:          args.GenesisBlockHash,
		maxBootstrapBackwardSteps: positiveOrDefault(args.MaxBootstrapBackwardSteps, defaultMaxBootstrapBackwardSteps),
		maxRecordsPerStep:         positiveOrDefault(args.MaxRecordsPerStep, defaultMaxRecordsPerStep),
		fetchConcurrency:          positiveOrDefault(args.FetchConcurrency, defaultFetchConcurrency),
	}

	return chain, nil
}

func positiveOrDefault(value int, defaultValue int) int {
	if value > 0 {
		return value
	}

	return defaultValue
}

// Start bootstraps the index (if empty) and performs the first synchronization step
func (chain *Chain) Start(ctx context.Context) error {
	err := chain.bootstrapIfNeeded(ctx)
	if err != nil {
		return err
	}

	base, last, _, err := chain.index.Bounds()
	if err != nil {
		return err
	}

	baseRecord, err := chain.index.Get(base)
	if err != nil {
		return err
	}

	if base == 0 && len(chain.genesisBlockHash) > 0 && !strings.EqualFold(baseRecord.MetaHash, chain.genesisBlockHash) {
		return fmt.Errorf("the configured genesis block hash (%s) does not match the hash of the first metablock (%s)", chain.genesisBlockHash, baseRecord.MetaHash)
	}

	shards := make([]ShardStatus, 0)
	for _, shard := range chain.backends.Shards() {
		shards = append(shards, ShardStatus{Shard: shard})
	}

	// Initially, nothing above the base can be served.
	chain.snapshot.Store(&Snapshot{
		Base:         base,
		Tip:          base,
		TipRecord:    baseRecord,
		Oldest:       base,
		OldestRecord: baseRecord,
		LastIndexed:  last,
		Shards:       shards,
	})

	log.Info("unified chain started", "base", base, "lastIndexed", last)

	// Errors of the first step are not fatal (e.g. a shard instance might be temporarily unreachable).
	err = chain.Step(ctx)
	if err != nil {
		log.Warn("first synchronization step", "err", err)
	}

	return nil
}

func (chain *Chain) bootstrapIfNeeded(ctx context.Context) error {
	_, _, hasRecords, err := chain.index.Bounds()
	if err != nil {
		return err
	}
	if hasRecords {
		return nil
	}

	baseNonce := chain.startNonce
	if chain.startEpoch != nil {
		baseNonce, err = chain.metaSource.GetEpochStartNonce(ctx, *chain.startEpoch)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrMetachainUnavailable, err)
		}
	}

	log.Info("bootstrapping the notarization index", "base", baseNonce)

	err = chain.index.Bootstrap(ctx, chain.metaSource, baseNonce, chain.maxBootstrapBackwardSteps)
	if err != nil {
		return fmt.Errorf("cannot bootstrap the notarization index: %w", err)
	}

	return nil
}

// Snapshot returns the current view over the unified chain, or ErrNotReady if Start() did not complete
func (chain *Chain) Snapshot() (*Snapshot, error) {
	snapshot := chain.snapshot.Load()
	if snapshot == nil {
		return nil, ErrNotReady
	}

	result := *snapshot
	result.NumDuplicatesSeen = chain.numDuplicates.Load()
	return &result, nil
}

func (chain *Chain) reportDuplicates(numDuplicates int) {
	if numDuplicates > 0 {
		chain.numDuplicates.Add(uint64(numDuplicates))
	}
}

// Run performs synchronization steps periodically, until the context is done
func (chain *Chain) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			err := chain.Step(ctx)
			chain.logStepError(err)
		}
	}
}

// logStepError avoids flooding the log with the same error, at each step
func (chain *Chain) logStepError(err error) {
	message := ""
	if err != nil {
		message = err.Error()
	}

	if message == chain.lastLoggedErr {
		return
	}

	chain.lastLoggedErr = message

	if err != nil {
		log.Warn("synchronization step", "err", err)
	} else {
		log.Info("synchronization step: recovered")
	}
}

// Step performs one synchronization step: extends the index with the new (final) metablocks,
// refreshes the status of the shard instances, then advances the tip of the unified chain.
// A failure of one part does not prevent the others from progressing.
func (chain *Chain) Step(ctx context.Context) error {
	chain.mutexStep.Lock()
	defer chain.mutexStep.Unlock()

	previous := chain.snapshot.Load()
	if previous == nil {
		return ErrNotReady
	}

	var errs []error
	next := *previous

	// (1) Metachain: status, then index the new final metablocks.
	metaStatus, err := chain.metaSource.GetStatus(ctx)
	if err != nil {
		next.MetaReachable = false
		next.MetaLastError = err.Error()
		errs = append(errs, fmt.Errorf("%w: %v", ErrMetachainUnavailable, err))
	} else {
		next.MetaReachable = true
		next.MetaSynced = metaStatus.IsSynced
		next.MetaFinalNonce = metaStatus.HighestFinalNonce
		next.MetaLastError = ""

		err = chain.indexUpTo(ctx, metaStatus.HighestFinalNonce)
		if err != nil {
			next.MetaLastError = err.Error()
			errs = append(errs, err)
		}
	}

	_, last, _, err := chain.index.Bounds()
	if err != nil {
		return err
	}

	next.LastIndexed = last

	// (2) Shard instances: status.
	next.Shards = chain.fetchShardStatuses(ctx, previous.Shards)
	for _, shard := range next.Shards {
		if !shard.Reachable {
			errs = append(errs, newErrBackendUnavailable(shard.Shard, errors.New(shard.LastError)))
		}
	}

	// (3) Tip: only moves forward.
	tip, err := chain.computeTip(previous.Tip, last, next.Shards)
	if err != nil {
		errs = append(errs, err)
	} else if tip != previous.Tip {
		tipRecord, errGet := chain.index.Get(tip)
		if errGet != nil {
			errs = append(errs, errGet)
		} else {
			next.Tip = tip
			next.TipRecord = tipRecord
		}
	}

	// (4) Oldest block with historical balances.
	oldest, err := chain.computeOldest(next.Base, next.Tip, next.Shards)
	if err != nil {
		errs = append(errs, err)
	} else if oldest != previous.Oldest || next.OldestRecord == nil {
		oldestRecord, errGet := chain.index.Get(oldest)
		if errGet != nil {
			errs = append(errs, errGet)
		} else {
			next.Oldest = oldest
			next.OldestRecord = oldestRecord
		}
	}

	chain.snapshot.Store(&next)

	return errors.Join(errs...)
}

// indexUpTo appends the records of the metablocks (last indexed, target] to the index. Metablocks are fetched concurrently, then appended in order.
func (chain *Chain) indexUpTo(ctx context.Context, target uint64) error {
	_, last, _, err := chain.index.Bounds()
	if err != nil {
		return err
	}

	numAppended := 0

	for last < target && numAppended < chain.maxRecordsPerStep {
		batchSize := uint64(chain.fetchConcurrency)
		if target-last < batchSize {
			batchSize = target - last
		}
		if remaining := uint64(chain.maxRecordsPerStep - numAppended); remaining < batchSize {
			batchSize = remaining
		}

		blocks := make([]*metachain.Block, batchSize)
		fetchErrs := make([]error, batchSize)

		var wg sync.WaitGroup
		for i := uint64(0); i < batchSize; i++ {
			wg.Add(1)

			go func(position uint64) {
				defer wg.Done()
				blocks[position], fetchErrs[position] = chain.metaSource.GetBlockByNonce(ctx, last+1+position)
			}(i)
		}
		wg.Wait()

		for i := uint64(0); i < batchSize; i++ {
			if fetchErrs[i] != nil {
				return fmt.Errorf("%w: %v", ErrMetachainUnavailable, fetchErrs[i])
			}

			err = chain.index.Append(blocks[i])
			if err != nil {
				return err
			}

			last++
			numAppended++
		}
	}

	return nil
}

func (chain *Chain) fetchShardStatuses(ctx context.Context, previous []ShardStatus) []ShardStatus {
	shards := chain.backends.Shards()
	statuses := make([]ShardStatus, len(shards))

	var wg sync.WaitGroup
	for i, shard := range shards {
		wg.Add(1)

		go func(position int, shard uint32) {
			defer wg.Done()
			statuses[position] = chain.fetchShardStatus(ctx, shard, findShardStatus(previous, shard))
		}(i, shard)
	}
	wg.Wait()

	return statuses
}

func findShardStatus(statuses []ShardStatus, shard uint32) ShardStatus {
	for _, status := range statuses {
		if status.Shard == shard {
			return status
		}
	}

	return ShardStatus{Shard: shard}
}

// fetchShardStatus refreshes the status of a shard instance. Upon failures, the last known values are kept (and the shard is marked as unreachable).
func (chain *Chain) fetchShardStatus(ctx context.Context, shard uint32, previous ShardStatus) ShardStatus {
	status := previous
	status.Shard = shard
	status.Reachable = false

	shardBackend, ok := chain.backends.Get(shard)
	if !ok {
		status.LastError = "no backend"
		return status
	}

	response, rosettaErr, err := shardBackend.NetworkStatus(ctx, &types.NetworkRequest{NetworkIdentifier: chain.network})
	if rosettaErr != nil {
		status.LastError = fmt.Sprintf("error %d: %s", rosettaErr.Code, rosettaErr.Message)
		return status
	}
	if err != nil {
		status.LastError = err.Error()
		return status
	}
	if response == nil || response.CurrentBlockIdentifier == nil {
		status.LastError = "empty network status"
		return status
	}

	status.HasStatus = true
	status.Reachable = true
	status.LastError = ""
	status.CurrentIndex = response.CurrentBlockIdentifier.Index
	status.Peers = response.Peers

	status.Synced = true
	if response.SyncStatus != nil && response.SyncStatus.Synced != nil {
		status.Synced = *response.SyncStatus.Synced
	}

	status.OldestIndex = 0
	if response.OldestBlockIdentifier != nil {
		status.OldestIndex = response.OldestBlockIdentifier.Index
	}

	return status
}

// computeTip returns the highest indexed metablock (in the range [currentTip, lastIndexed]) for which all shard instances can serve the notarized shard blocks.
// Since the frontiers never decrease, a binary search is used.
func (chain *Chain) computeTip(currentTip uint64, lastIndexed uint64, shards []ShardStatus) (uint64, error) {
	for _, shard := range shards {
		if !shard.HasStatus {
			// Without knowing the current block of all shard instances, the tip cannot advance.
			return currentTip, nil
		}
	}

	isServable := func(metaNonce uint64) (bool, error) {
		record, err := chain.index.Get(metaNonce)
		if err != nil {
			return false, err
		}

		for _, shard := range shards {
			frontier, ok := record.Frontier(shard.Shard)
			if !ok {
				return false, fmt.Errorf("%w: shard %d is missing from record %d", index.ErrInconsistent, shard.Shard, metaNonce)
			}
			if shard.CurrentIndex < 0 || frontier.Nonce > uint64(shard.CurrentIndex) {
				return false, nil
			}
		}

		return true, nil
	}

	low := currentTip
	high := lastIndexed

	for low < high {
		middle := low + (high-low+1)/2

		ok, err := isServable(middle)
		if err != nil {
			return currentTip, err
		}

		if ok {
			low = middle
		} else {
			high = middle - 1
		}
	}

	return low, nil
}

// computeOldest returns the oldest unified block for which all shard instances hold historical balances:
// the smallest block (above the base) whose frontiers are, for all shards, at or above the oldest block of the shard instance.
func (chain *Chain) computeOldest(base uint64, tip uint64, shards []ShardStatus) (uint64, error) {
	if tip <= base {
		return base, nil
	}

	key := fmt.Sprintf("%d", base)
	allFromGenesis := true
	for _, shard := range shards {
		key += fmt.Sprintf("|%d:%d", shard.Shard, shard.OldestIndex)
		if shard.OldestIndex > 0 {
			allFromGenesis = false
		}
	}

	if base == 0 && allFromGenesis {
		return 0, nil
	}
	if key == chain.oldestKey && chain.oldestCached <= tip {
		return chain.oldestCached, nil
	}

	hasHistory := func(metaNonce uint64) (bool, error) {
		record, err := chain.index.Get(metaNonce)
		if err != nil {
			return false, err
		}

		for _, shard := range shards {
			frontier, ok := record.Frontier(shard.Shard)
			if !ok {
				return false, fmt.Errorf("%w: shard %d is missing from record %d", index.ErrInconsistent, shard.Shard, metaNonce)
			}
			if shard.OldestIndex > 0 && frontier.Nonce < uint64(shard.OldestIndex) {
				return false, nil
			}
		}

		return true, nil
	}

	low := base + 1
	high := tip

	ok, err := hasHistory(high)
	if err != nil {
		return 0, err
	}
	if !ok {
		// Not even the tip is covered (the shard instances are ahead). This is not cached, since it will change as the tip advances.
		return tip, nil
	}

	for low < high {
		middle := low + (high-low)/2

		ok, err = hasHistory(middle)
		if err != nil {
			return 0, err
		}

		if ok {
			high = middle
		} else {
			low = middle + 1
		}
	}

	chain.oldestKey = key
	chain.oldestCached = low
	return low, nil
}
