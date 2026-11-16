package services

import (
	"context"
	"fmt"

	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
)

type mempoolService struct {
	*base
}

// NewMempoolService creates the service behind the /mempool endpoints
func NewMempoolService(args ArgsNewServices) (server.MempoolAPIServicer, error) {
	base, err := newBase(args)
	if err != nil {
		return nil, err
	}

	return &mempoolService{base: base}, nil
}

// Mempool is not implemented (as in the shard instances)
func (service *mempoolService) Mempool(_ context.Context, _ *types.NetworkRequest) (*types.MempoolResponse, *types.Error) {
	return nil, service.errFactory.newErr(ErrNotImplemented)
}

// MempoolTransaction implements the /mempool/transaction endpoint.
// The shard of the transaction cannot be told from its hash, thus all shard instances are asked: the first one that knows the transaction answers.
func (service *mempoolService) MempoolTransaction(ctx context.Context, request *types.MempoolTransactionRequest) (*types.MempoolTransactionResponse, *types.Error) {
	if service.IsOffline {
		return nil, service.errFactory.newErr(ErrOfflineMode)
	}

	var lastRosettaErr *types.Error
	var lastErr error

	for _, shard := range service.Backends.Shards() {
		shardBackend, _ := service.Backends.Get(shard)

		response, rosettaErr, err := shardBackend.MempoolTransaction(ctx, request)
		if rosettaErr != nil {
			// E.g. "transaction is not in pool": continue with the other shards.
			lastRosettaErr = rosettaErr
			continue
		}
		if err != nil {
			lastErr = fmt.Errorf("shard %d: %w", shard, err)
			continue
		}
		if response != nil && response.Transaction != nil {
			return response, nil
		}
	}

	if lastRosettaErr != nil {
		return nil, lastRosettaErr
	}
	if lastErr != nil {
		return nil, service.errFactory.newErrWithDetails(ErrBackendUnavailable, lastErr.Error())
	}

	return nil, service.errFactory.newErrWithDetails(ErrInternal, "no shard backend answered")
}
