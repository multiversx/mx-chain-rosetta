package services

import (
	"context"

	"github.com/coinbase/rosetta-sdk-go/types"
)

type offlineService struct {
	errFactory *errFactory
}

// NewOfflineService will create a new instance of offlineService
func NewOfflineService() *offlineService {
	return &offlineService{}
}

// AccountBalance implements the /account/balance endpoint.
func (service *offlineService) AccountBalance(
	_ context.Context,
	_ *types.AccountBalanceRequest,
) (*types.AccountBalanceResponse, *types.Error) {
	return nil, service.errFactory.newErr(ErrOfflineMode)
}

// AccountCoins implements the /account/coins endpoint.
func (service *offlineService) AccountCoins(_ context.Context, _ *types.AccountCoinsRequest) (*types.AccountCoinsResponse, *types.Error) {
	return nil, service.errFactory.newErr(ErrOfflineMode)
}

// Block implements the /block endpoint.
func (service *offlineService) Block(
	_ context.Context,
	_ *types.BlockRequest,
) (*types.BlockResponse, *types.Error) {
	return nil, service.errFactory.newErr(ErrOfflineMode)
}

// BlockTransaction - not implemented
// We don't need this method because all transactions are returned by method Block
func (service *offlineService) BlockTransaction(
	_ context.Context,
	_ *types.BlockTransactionRequest,
) (*types.BlockTransactionResponse, *types.Error) {
	return nil, service.errFactory.newErr(ErrOfflineMode)
}

// Mempool is not implemented yet
func (service *offlineService) Mempool(context.Context, *types.NetworkRequest) (*types.MempoolResponse, *types.Error) {
	return nil, service.errFactory.newErr(ErrOfflineMode)
}

// MempoolTransaction will return operations for a transaction that is in pool
func (service *offlineService) MempoolTransaction(
	_ context.Context,
	_ *types.MempoolTransactionRequest,
) (*types.MempoolTransactionResponse, *types.Error) {
	return nil, service.errFactory.newErr(ErrOfflineMode)
}

// NetworkStatus implements the /network/status endpoint.
func (service *offlineService) NetworkStatus(
	_ context.Context,
	_ *types.NetworkRequest,
) (*types.NetworkStatusResponse, *types.Error) {
	return nil, service.errFactory.newErr(ErrOfflineMode)
}

// NetworkOptions implements the /network/options endpoint.
func (service *offlineService) NetworkOptions(
	_ context.Context,
	_ *types.NetworkRequest,
) (*types.NetworkOptionsResponse, *types.Error) {
	return nil, service.errFactory.newErr(ErrOfflineMode)
}

// NetworkList implements the /network/list endpoint
func (service *offlineService) NetworkList(
	_ context.Context,
	_ *types.MetadataRequest,
) (*types.NetworkListResponse, *types.Error) {
	return nil, service.errFactory.newErr(ErrOfflineMode)
}
