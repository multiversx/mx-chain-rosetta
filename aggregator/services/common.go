package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/multiversx/mx-chain-rosetta/aggregator/backend"
	"github.com/multiversx/mx-chain-rosetta/aggregator/index"
	"github.com/multiversx/mx-chain-rosetta/aggregator/shardmap"
	"github.com/multiversx/mx-chain-rosetta/aggregator/unified"
)

// ArgsNewServices holds the dependencies shared by the services of the aggregator
type ArgsNewServices struct {
	IsOffline              bool
	Network                *types.NetworkIdentifier
	GenesisBlockIdentifier *types.BlockIdentifier
	GenesisTimestampMs     int64
	AggregatorVersion      string

	Backends *backend.Set
	ShardMap *shardmap.ShardMap

	// The following are not needed in offline mode
	Index     *index.Index
	Chain     *unified.Chain
	Assembler *unified.Assembler
}

func (args *ArgsNewServices) check() error {
	if args.Network == nil {
		return errors.New("nil network identifier")
	}
	if args.GenesisBlockIdentifier == nil {
		return errors.New("nil genesis block identifier")
	}
	if args.Backends == nil {
		return errors.New("nil backends")
	}
	if args.ShardMap == nil {
		return errors.New("nil shard map")
	}

	if args.IsOffline {
		return nil
	}

	if args.Index == nil {
		return errors.New("nil index")
	}
	if args.Chain == nil {
		return errors.New("nil chain")
	}
	if args.Assembler == nil {
		return errors.New("nil assembler")
	}

	return nil
}

// base holds what is common to all services
type base struct {
	ArgsNewServices
	errFactory *errFactory
}

func newBase(args ArgsNewServices) (*base, error) {
	err := args.check()
	if err != nil {
		return nil, err
	}

	return &base{
		ArgsNewServices: args,
		errFactory:      newErrFactory(),
	}, nil
}

// getBlockIdentifier returns the identifier of a unified block, given its index and its index record (not needed for the genesis block)
func (service *base) getBlockIdentifier(nonce uint64, record *index.Record) *types.BlockIdentifier {
	if nonce == 0 || record == nil {
		return service.GenesisBlockIdentifier
	}

	return &types.BlockIdentifier{
		Index: int64(record.MetaNonce),
		Hash:  record.MetaHash,
	}
}

// resolveBlockNonce translates a (partial) block identifier, as received from the client, to the index of a unified block.
// When no block is specified, the tip of the unified chain is returned.
func (service *base) resolveBlockNonce(identifier *types.PartialBlockIdentifier, snapshot *unified.Snapshot) (uint64, *types.Error) {
	if identifier == nil || (identifier.Index == nil && identifier.Hash == nil) {
		return snapshot.Tip, nil
	}

	if identifier.Hash != nil {
		nonce, typedErr := service.resolveBlockHash(*identifier.Hash)
		if typedErr != nil {
			return 0, typedErr
		}

		if identifier.Index != nil && *identifier.Index != int64(nonce) {
			return 0, service.errFactory.newErrWithDetails(ErrInvalidRequest, "block index and block hash do not refer to the same block")
		}

		return nonce, nil
	}

	if *identifier.Index < 0 {
		return 0, service.errFactory.newErrWithDetails(ErrInvalidRequest, "negative block index")
	}

	return uint64(*identifier.Index), nil
}

func (service *base) resolveBlockHash(hash string) (uint64, *types.Error) {
	if strings.EqualFold(hash, service.GenesisBlockIdentifier.Hash) {
		return 0, nil
	}

	nonce, found, err := service.Index.GetNonceByHash(hash)
	if err != nil {
		return 0, service.errFactory.fromError(err)
	}
	if !found {
		// The hash is either unknown, or it belongs to a metablock that is not indexed yet.
		return 0, service.errFactory.newErrWithDetails(ErrBlockNotYetAvailable, fmt.Sprintf("block with hash %s is not indexed", hash))
	}

	return nonce, nil
}

// callAnyBackend invokes "fn" against the shard backends, in order, until one of them can be reached.
// It is used for requests that can be answered by any shard instance (they do not depend on the state of a particular shard).
func callAnyBackend[T any](
	ctx context.Context,
	service *base,
	fn func(shardBackend backend.ShardBackend) (T, *types.Error, error),
) (T, *types.Error) {
	var zero T
	var lastErr error

	for _, shard := range service.Backends.Shards() {
		shardBackend, _ := service.Backends.Get(shard)

		response, rosettaErr, err := fn(shardBackend)
		if rosettaErr != nil {
			return zero, rosettaErr
		}
		if err == nil {
			return response, nil
		}

		lastErr = fmt.Errorf("shard %d: %w", shard, err)

		if ctx.Err() != nil {
			break
		}
	}

	return zero, service.errFactory.newErrWithDetails(ErrBackendUnavailable, lastErr.Error())
}

// callBackendOfShard invokes "fn" against the backend of a given shard
func callBackendOfShard[T any](
	service *base,
	shard uint32,
	fn func(shardBackend backend.ShardBackend) (T, *types.Error, error),
) (T, *types.Error) {
	var zero T

	shardBackend, ok := service.Backends.Get(shard)
	if !ok {
		return zero, service.errFactory.newErrWithDetails(ErrAddressNotServed, fmt.Sprintf("shard %d", shard))
	}

	response, rosettaErr, err := fn(shardBackend)
	if rosettaErr != nil {
		return zero, rosettaErr
	}
	if err != nil {
		return zero, service.errFactory.newErrWithDetails(ErrBackendUnavailable, fmt.Sprintf("shard %d: %v", shard, err))
	}

	return response, nil
}

// getServedShardOfAddress returns the shard of an address, if a backend exists for that shard
func (service *base) getServedShardOfAddress(address string) (uint32, *types.Error) {
	shard, err := service.ShardMap.ShardOfAddress(address)
	if err != nil {
		return 0, service.errFactory.newErrWithDetails(ErrInvalidAddress, err.Error())
	}

	_, hasBackend := service.Backends.Get(shard)
	if !service.ShardMap.IsServed(shard) || !hasBackend {
		return 0, service.errFactory.newErrWithDetails(ErrAddressNotServed, fmt.Sprintf("address %s belongs to shard %d", address, shard))
	}

	return shard, nil
}
