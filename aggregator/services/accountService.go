package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/multiversx/mx-chain-rosetta/aggregator/backend"
	"github.com/multiversx/mx-chain-rosetta/aggregator/index"
)

type accountService struct {
	*base
}

// NewAccountService creates the service behind the /account endpoints
func NewAccountService(args ArgsNewServices) (server.AccountAPIServicer, error) {
	base, err := newBase(args)
	if err != nil {
		return nil, err
	}

	return &accountService{base: base}, nil
}

// AccountBalance implements the /account/balance endpoint.
// The balance of an account at a unified block is its balance (in its own shard) at the frontier:
// the last block of that shard notarized up to the unified block.
func (service *accountService) AccountBalance(ctx context.Context, request *types.AccountBalanceRequest) (*types.AccountBalanceResponse, *types.Error) {
	if service.IsOffline {
		return nil, service.errFactory.newErr(ErrOfflineMode)
	}
	if request.AccountIdentifier == nil {
		return nil, service.errFactory.newErr(ErrInvalidAddress)
	}

	shard, typedErr := service.getServedShardOfAddress(request.AccountIdentifier.Address)
	if typedErr != nil {
		return nil, typedErr
	}

	snapshot, err := service.Chain.Snapshot()
	if err != nil {
		return nil, service.errFactory.fromError(err)
	}

	// When no block is requested, the balance is pinned to the tip of the unified chain (a block that the client can also fetch).
	nonce, typedErr := service.resolveBlockNonce(request.BlockIdentifier, snapshot)
	if typedErr != nil {
		return nil, typedErr
	}
	if nonce > snapshot.Tip {
		return nil, service.errFactory.newErr(ErrBlockNotYetAvailable)
	}
	if nonce != 0 && nonce < snapshot.Base {
		return nil, service.errFactory.newErr(ErrBlockTooOld)
	}

	// For the genesis block, the frontier of any shard is its own genesis block (whose hash is not known to the aggregator).
	var record *index.Record
	frontier := index.BlockRef{}

	if nonce != 0 {
		record, err = service.Index.Get(nonce)
		if err != nil {
			if errors.Is(err, index.ErrNotFound) {
				return nil, service.errFactory.newErr(ErrBlockNotYetAvailable)
			}
			return nil, service.errFactory.fromError(err)
		}

		var hasFrontier bool
		frontier, hasFrontier = record.Frontier(shard)
		if !hasFrontier {
			return nil, service.errFactory.newErrWithDetails(ErrInternal, fmt.Sprintf("shard %d is missing from record %d", shard, nonce))
		}
	}

	frontierIndex := int64(frontier.Nonce)
	shardRequest := &types.AccountBalanceRequest{
		NetworkIdentifier: request.NetworkIdentifier,
		AccountIdentifier: request.AccountIdentifier,
		BlockIdentifier:   &types.PartialBlockIdentifier{Index: &frontierIndex},
		Currencies:        request.Currencies,
	}

	shardResponse, typedErr := callBackendOfShard(service.base, shard, func(shardBackend backend.ShardBackend) (*types.AccountBalanceResponse, *types.Error, error) {
		return shardBackend.AccountBalance(ctx, shardRequest)
	})
	if typedErr != nil {
		return nil, typedErr
	}
	if shardResponse == nil || shardResponse.BlockIdentifier == nil {
		return nil, service.errFactory.newErrWithDetails(ErrBackendInconsistent, fmt.Sprintf("shard %d: empty account balance response", shard))
	}

	// The balance must be the one at the frontier (and on the chain notarized by the metachain).
	if shardResponse.BlockIdentifier.Index != frontierIndex {
		return nil, service.errFactory.newErrWithDetails(ErrBackendInconsistent,
			fmt.Sprintf("shard %d: requested balance at block %d, received balance at block %d", shard, frontierIndex, shardResponse.BlockIdentifier.Index))
	}
	if len(frontier.Hash) > 0 && !strings.EqualFold(shardResponse.BlockIdentifier.Hash, frontier.Hash) {
		return nil, service.errFactory.newErrWithDetails(ErrBackendInconsistent,
			fmt.Sprintf("shard %d: block %d has hash %s, but the metachain has notarized %s", shard, frontierIndex, shardResponse.BlockIdentifier.Hash, frontier.Hash))
	}

	metadata := make(map[string]interface{}, len(shardResponse.Metadata)+2)
	for key, value := range shardResponse.Metadata {
		metadata[key] = value
	}

	metadata["shard"] = shard
	metadata["shardBlock"] = map[string]interface{}{
		"nonce": shardResponse.BlockIdentifier.Index,
		"hash":  shardResponse.BlockIdentifier.Hash,
	}

	return &types.AccountBalanceResponse{
		BlockIdentifier: service.getBlockIdentifier(nonce, record),
		Balances:        shardResponse.Balances,
		Metadata:        metadata,
	}, nil
}

// AccountCoins is not implemented (as in the shard instances)
func (service *accountService) AccountCoins(_ context.Context, _ *types.AccountCoinsRequest) (*types.AccountCoinsResponse, *types.Error) {
	return nil, service.errFactory.newErr(ErrNotImplemented)
}

// AllAccountBalances is not implemented (as in the shard instances)
func (service *accountService) AllAccountBalances(_ context.Context, _ *types.AllAccountBalancesRequest) (*types.AllAccountBalancesResponse, *types.Error) {
	return nil, service.errFactory.newErr(ErrNotImplemented)
}
