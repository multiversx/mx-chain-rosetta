package mock

import (
	"context"
	"net/http"

	"github.com/coinbase/rosetta-sdk-go/asserter"
	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
	shardServices "github.com/multiversx/mx-chain-rosetta/server/services"
)

const errCodeMockUnavailable = 99

// NewShardInstanceHandler creates an HTTP handler that serves a fake shard instance through the Rosetta API (as a real shard instance does).
// It allows testing the real HTTP client of the aggregator against the simulated network.
func NewShardInstanceHandler(shardBackend *ShardBackend) (http.Handler, error) {
	asserterInstance, err := asserter.NewServer(
		shardServices.SupportedOperationTypes,
		true,
		[]*types.NetworkIdentifier{shardBackend.Network},
		nil,
		false,
		"",
	)
	if err != nil {
		return nil, err
	}

	servicer := &shardInstanceServicer{backend: shardBackend}

	return server.NewRouter(
		server.NewNetworkAPIController(servicer, asserterInstance),
		server.NewBlockAPIController(servicer, asserterInstance),
		server.NewAccountAPIController(servicer, asserterInstance),
		server.NewMempoolAPIController(servicer, asserterInstance),
		server.NewConstructionAPIController(servicer, asserterInstance),
	), nil
}

// shardInstanceServicer adapts a fake shard backend to the interfaces of the Rosetta server
type shardInstanceServicer struct {
	backend *ShardBackend
}

func adapt[T any](response T, rosettaErr *types.Error, err error) (T, *types.Error) {
	var zero T

	if rosettaErr != nil {
		return zero, rosettaErr
	}
	if err != nil {
		return zero, &types.Error{Code: errCodeMockUnavailable, Message: err.Error(), Retriable: true}
	}

	return response, nil
}

func newNotImplementedError() *types.Error {
	return &types.Error{Code: 5, Message: "operation not implemented"}
}

func (servicer *shardInstanceServicer) NetworkList(ctx context.Context, request *types.MetadataRequest) (*types.NetworkListResponse, *types.Error) {
	return adapt(servicer.backend.NetworkList(ctx, request))
}

func (servicer *shardInstanceServicer) NetworkOptions(ctx context.Context, request *types.NetworkRequest) (*types.NetworkOptionsResponse, *types.Error) {
	return adapt(servicer.backend.NetworkOptions(ctx, request))
}

func (servicer *shardInstanceServicer) NetworkStatus(ctx context.Context, request *types.NetworkRequest) (*types.NetworkStatusResponse, *types.Error) {
	return adapt(servicer.backend.NetworkStatus(ctx, request))
}

func (servicer *shardInstanceServicer) Block(ctx context.Context, request *types.BlockRequest) (*types.BlockResponse, *types.Error) {
	return adapt(servicer.backend.Block(ctx, request))
}

func (servicer *shardInstanceServicer) BlockTransaction(_ context.Context, _ *types.BlockTransactionRequest) (*types.BlockTransactionResponse, *types.Error) {
	return nil, newNotImplementedError()
}

func (servicer *shardInstanceServicer) AccountBalance(ctx context.Context, request *types.AccountBalanceRequest) (*types.AccountBalanceResponse, *types.Error) {
	return adapt(servicer.backend.AccountBalance(ctx, request))
}

func (servicer *shardInstanceServicer) AccountCoins(_ context.Context, _ *types.AccountCoinsRequest) (*types.AccountCoinsResponse, *types.Error) {
	return nil, newNotImplementedError()
}

func (servicer *shardInstanceServicer) AllAccountBalances(_ context.Context, _ *types.AllAccountBalancesRequest) (*types.AllAccountBalancesResponse, *types.Error) {
	return nil, newNotImplementedError()
}

func (servicer *shardInstanceServicer) Mempool(_ context.Context, _ *types.NetworkRequest) (*types.MempoolResponse, *types.Error) {
	return nil, newNotImplementedError()
}

func (servicer *shardInstanceServicer) MempoolTransaction(ctx context.Context, request *types.MempoolTransactionRequest) (*types.MempoolTransactionResponse, *types.Error) {
	return adapt(servicer.backend.MempoolTransaction(ctx, request))
}

func (servicer *shardInstanceServicer) ConstructionCombine(ctx context.Context, request *types.ConstructionCombineRequest) (*types.ConstructionCombineResponse, *types.Error) {
	return adapt(servicer.backend.ConstructionCombine(ctx, request))
}

func (servicer *shardInstanceServicer) ConstructionDerive(ctx context.Context, request *types.ConstructionDeriveRequest) (*types.ConstructionDeriveResponse, *types.Error) {
	return adapt(servicer.backend.ConstructionDerive(ctx, request))
}

func (servicer *shardInstanceServicer) ConstructionHash(ctx context.Context, request *types.ConstructionHashRequest) (*types.TransactionIdentifierResponse, *types.Error) {
	return adapt(servicer.backend.ConstructionHash(ctx, request))
}

func (servicer *shardInstanceServicer) ConstructionMetadata(ctx context.Context, request *types.ConstructionMetadataRequest) (*types.ConstructionMetadataResponse, *types.Error) {
	return adapt(servicer.backend.ConstructionMetadata(ctx, request))
}

func (servicer *shardInstanceServicer) ConstructionParse(ctx context.Context, request *types.ConstructionParseRequest) (*types.ConstructionParseResponse, *types.Error) {
	return adapt(servicer.backend.ConstructionParse(ctx, request))
}

func (servicer *shardInstanceServicer) ConstructionPayloads(ctx context.Context, request *types.ConstructionPayloadsRequest) (*types.ConstructionPayloadsResponse, *types.Error) {
	return adapt(servicer.backend.ConstructionPayloads(ctx, request))
}

func (servicer *shardInstanceServicer) ConstructionPreprocess(ctx context.Context, request *types.ConstructionPreprocessRequest) (*types.ConstructionPreprocessResponse, *types.Error) {
	return adapt(servicer.backend.ConstructionPreprocess(ctx, request))
}

func (servicer *shardInstanceServicer) ConstructionPreprocessOperations(ctx context.Context, request *types.ConstructionPreprocessOperationsRequest) (*types.ConstructionPreprocessOperationsResponse, *types.Error) {
	return adapt(servicer.backend.ConstructionPreprocessOperations(ctx, request))
}

func (servicer *shardInstanceServicer) ConstructionSubmit(ctx context.Context, request *types.ConstructionSubmitRequest) (*types.TransactionIdentifierResponse, *types.Error) {
	return adapt(servicer.backend.ConstructionSubmit(ctx, request))
}
