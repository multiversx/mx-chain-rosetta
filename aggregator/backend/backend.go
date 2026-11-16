// Package backend holds the clients used by the aggregator to talk to the shard instances (one Rosetta instance per shard).
package backend

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/coinbase/rosetta-sdk-go/types"
)

// ShardBackend is the Rosetta API of a shard instance, as used by the aggregator.
// Each call returns either a response, or a Rosetta error (returned by the shard instance), or a transport error (the instance could not be reached).
// When a Rosetta error is returned, the third result may also be set (it is then informative only).
type ShardBackend interface {
	NetworkList(ctx context.Context, request *types.MetadataRequest) (*types.NetworkListResponse, *types.Error, error)
	NetworkOptions(ctx context.Context, request *types.NetworkRequest) (*types.NetworkOptionsResponse, *types.Error, error)
	NetworkStatus(ctx context.Context, request *types.NetworkRequest) (*types.NetworkStatusResponse, *types.Error, error)
	Block(ctx context.Context, request *types.BlockRequest) (*types.BlockResponse, *types.Error, error)
	AccountBalance(ctx context.Context, request *types.AccountBalanceRequest) (*types.AccountBalanceResponse, *types.Error, error)
	MempoolTransaction(ctx context.Context, request *types.MempoolTransactionRequest) (*types.MempoolTransactionResponse, *types.Error, error)
	ConstructionCombine(ctx context.Context, request *types.ConstructionCombineRequest) (*types.ConstructionCombineResponse, *types.Error, error)
	ConstructionDerive(ctx context.Context, request *types.ConstructionDeriveRequest) (*types.ConstructionDeriveResponse, *types.Error, error)
	ConstructionHash(ctx context.Context, request *types.ConstructionHashRequest) (*types.TransactionIdentifierResponse, *types.Error, error)
	ConstructionMetadata(ctx context.Context, request *types.ConstructionMetadataRequest) (*types.ConstructionMetadataResponse, *types.Error, error)
	ConstructionParse(ctx context.Context, request *types.ConstructionParseRequest) (*types.ConstructionParseResponse, *types.Error, error)
	ConstructionPayloads(ctx context.Context, request *types.ConstructionPayloadsRequest) (*types.ConstructionPayloadsResponse, *types.Error, error)
	ConstructionPreprocess(ctx context.Context, request *types.ConstructionPreprocessRequest) (*types.ConstructionPreprocessResponse, *types.Error, error)
	ConstructionPreprocessOperations(ctx context.Context, request *types.ConstructionPreprocessOperationsRequest) (*types.ConstructionPreprocessOperationsResponse, *types.Error, error)
	ConstructionSubmit(ctx context.Context, request *types.ConstructionSubmitRequest) (*types.TransactionIdentifierResponse, *types.Error, error)
}

// Set holds the backends, by shard
type Set struct {
	byShard map[uint32]ShardBackend
	shards  []uint32
}

// NewSet creates a set of backends
func NewSet(byShard map[uint32]ShardBackend) (*Set, error) {
	if len(byShard) == 0 {
		return nil, errors.New("no shard backends")
	}

	shards := make([]uint32, 0, len(byShard))
	for shard, backend := range byShard {
		if backend == nil {
			return nil, fmt.Errorf("nil backend for shard %d", shard)
		}

		shards = append(shards, shard)
	}

	sort.Slice(shards, func(i, j int) bool { return shards[i] < shards[j] })

	return &Set{
		byShard: byShard,
		shards:  shards,
	}, nil
}

// Shards returns the shards that have a backend, in ascending order
func (set *Set) Shards() []uint32 {
	result := make([]uint32, len(set.shards))
	copy(result, set.shards)
	return result
}

// Get returns the backend of a shard
func (set *Set) Get(shard uint32) (ShardBackend, bool) {
	backend, ok := set.byShard[shard]
	return backend, ok
}
