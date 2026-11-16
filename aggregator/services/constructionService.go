package services

import (
	"context"
	"encoding/json"

	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/multiversx/mx-chain-rosetta/aggregator/backend"
)

const optionsKeySender = "sender"

type constructionService struct {
	*base
}

// NewConstructionService creates the service behind the /construction endpoints.
// The endpoints that do not need access to the chain are answered by any shard instance.
// The ones that do (metadata, submit) are routed to the shard of the sender.
func NewConstructionService(args ArgsNewServices) (server.ConstructionAPIServicer, error) {
	base, err := newBase(args)
	if err != nil {
		return nil, err
	}

	return &constructionService{base: base}, nil
}

// ConstructionDerive implements the /construction/derive endpoint
func (service *constructionService) ConstructionDerive(ctx context.Context, request *types.ConstructionDeriveRequest) (*types.ConstructionDeriveResponse, *types.Error) {
	return callAnyBackend(ctx, service.base, func(shardBackend backend.ShardBackend) (*types.ConstructionDeriveResponse, *types.Error, error) {
		return shardBackend.ConstructionDerive(ctx, request)
	})
}

// ConstructionPreprocess implements the /construction/preprocess endpoint
func (service *constructionService) ConstructionPreprocess(ctx context.Context, request *types.ConstructionPreprocessRequest) (*types.ConstructionPreprocessResponse, *types.Error) {
	return callAnyBackend(ctx, service.base, func(shardBackend backend.ShardBackend) (*types.ConstructionPreprocessResponse, *types.Error, error) {
		return shardBackend.ConstructionPreprocess(ctx, request)
	})
}

// ConstructionPreprocessOperations implements the /construction/preprocess_operations endpoint
func (service *constructionService) ConstructionPreprocessOperations(ctx context.Context, request *types.ConstructionPreprocessOperationsRequest) (*types.ConstructionPreprocessOperationsResponse, *types.Error) {
	return callAnyBackend(ctx, service.base, func(shardBackend backend.ShardBackend) (*types.ConstructionPreprocessOperationsResponse, *types.Error, error) {
		return shardBackend.ConstructionPreprocessOperations(ctx, request)
	})
}

// ConstructionMetadata implements the /construction/metadata endpoint.
// The metadata includes the account nonce of the sender, therefore the request is routed to the shard of the sender.
func (service *constructionService) ConstructionMetadata(ctx context.Context, request *types.ConstructionMetadataRequest) (*types.ConstructionMetadataResponse, *types.Error) {
	if service.IsOffline {
		return nil, service.errFactory.newErr(ErrOfflineMode)
	}

	sender, _ := request.Options[optionsKeySender].(string)
	if len(sender) == 0 {
		return nil, service.errFactory.newErrWithDetails(ErrInvalidRequest, "missing option: 'sender'")
	}

	shard, typedErr := service.getServedShardOfAddress(sender)
	if typedErr != nil {
		return nil, typedErr
	}

	return callBackendOfShard(service.base, shard, func(shardBackend backend.ShardBackend) (*types.ConstructionMetadataResponse, *types.Error, error) {
		return shardBackend.ConstructionMetadata(ctx, request)
	})
}

// ConstructionPayloads implements the /construction/payloads endpoint
func (service *constructionService) ConstructionPayloads(ctx context.Context, request *types.ConstructionPayloadsRequest) (*types.ConstructionPayloadsResponse, *types.Error) {
	return callAnyBackend(ctx, service.base, func(shardBackend backend.ShardBackend) (*types.ConstructionPayloadsResponse, *types.Error, error) {
		return shardBackend.ConstructionPayloads(ctx, request)
	})
}

// ConstructionParse implements the /construction/parse endpoint
func (service *constructionService) ConstructionParse(ctx context.Context, request *types.ConstructionParseRequest) (*types.ConstructionParseResponse, *types.Error) {
	return callAnyBackend(ctx, service.base, func(shardBackend backend.ShardBackend) (*types.ConstructionParseResponse, *types.Error, error) {
		return shardBackend.ConstructionParse(ctx, request)
	})
}

// ConstructionCombine implements the /construction/combine endpoint
func (service *constructionService) ConstructionCombine(ctx context.Context, request *types.ConstructionCombineRequest) (*types.ConstructionCombineResponse, *types.Error) {
	return callAnyBackend(ctx, service.base, func(shardBackend backend.ShardBackend) (*types.ConstructionCombineResponse, *types.Error, error) {
		return shardBackend.ConstructionCombine(ctx, request)
	})
}

// ConstructionHash implements the /construction/hash endpoint
func (service *constructionService) ConstructionHash(ctx context.Context, request *types.ConstructionHashRequest) (*types.TransactionIdentifierResponse, *types.Error) {
	return callAnyBackend(ctx, service.base, func(shardBackend backend.ShardBackend) (*types.TransactionIdentifierResponse, *types.Error, error) {
		return shardBackend.ConstructionHash(ctx, request)
	})
}

// ConstructionSubmit implements the /construction/submit endpoint.
// A transaction must be broadcast in the shard of its sender, therefore the request is routed to that shard.
func (service *constructionService) ConstructionSubmit(ctx context.Context, request *types.ConstructionSubmitRequest) (*types.TransactionIdentifierResponse, *types.Error) {
	if service.IsOffline {
		return nil, service.errFactory.newErr(ErrOfflineMode)
	}

	sender, err := getSenderOfSignedTransaction(request.SignedTransaction)
	if err != nil {
		return nil, service.errFactory.newErrWithDetails(ErrInvalidRequest, err.Error())
	}
	if len(sender) == 0 {
		return nil, service.errFactory.newErrWithDetails(ErrInvalidRequest, "the signed transaction has no sender")
	}

	shard, typedErr := service.getServedShardOfAddress(sender)
	if typedErr != nil {
		return nil, typedErr
	}

	return callBackendOfShard(service.base, shard, func(shardBackend backend.ShardBackend) (*types.TransactionIdentifierResponse, *types.Error, error) {
		return shardBackend.ConstructionSubmit(ctx, request)
	})
}

// getSenderOfSignedTransaction reads the sender from a signed transaction (JSON), as produced by /construction/combine
func getSenderOfSignedTransaction(signedTransaction string) (string, error) {
	var tx struct {
		Sender string `json:"sender"`
	}

	err := json.Unmarshal([]byte(signedTransaction), &tx)
	if err != nil {
		return "", err
	}

	return tx.Sender, nil
}
