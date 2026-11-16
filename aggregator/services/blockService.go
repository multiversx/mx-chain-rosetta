package services

import (
	"context"

	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
)

type blockService struct {
	*base
}

// NewBlockService creates the service behind the /block endpoints
func NewBlockService(args ArgsNewServices) (server.BlockAPIServicer, error) {
	base, err := newBase(args)
	if err != nil {
		return nil, err
	}

	return &blockService{base: base}, nil
}

// Block implements the /block endpoint: it returns a block of the unified chain
func (service *blockService) Block(ctx context.Context, request *types.BlockRequest) (*types.BlockResponse, *types.Error) {
	if service.IsOffline {
		return nil, service.errFactory.newErr(ErrOfflineMode)
	}

	snapshot, err := service.Chain.Snapshot()
	if err != nil {
		return nil, service.errFactory.fromError(err)
	}

	nonce, typedErr := service.resolveBlockNonce(request.BlockIdentifier, snapshot)
	if typedErr != nil {
		return nil, typedErr
	}

	block, err := service.Assembler.GetBlock(ctx, nonce)
	if err != nil {
		return nil, service.errFactory.fromError(err)
	}

	return &types.BlockResponse{Block: block}, nil
}

// BlockTransaction is not implemented, since all transactions are returned by /block
func (service *blockService) BlockTransaction(_ context.Context, _ *types.BlockTransactionRequest) (*types.BlockTransactionResponse, *types.Error) {
	return nil, service.errFactory.newErr(ErrNotImplemented)
}
