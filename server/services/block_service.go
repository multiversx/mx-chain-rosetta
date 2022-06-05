package services

import (
	"context"

	"github.com/ElrondNetwork/elrond-proxy-go/data"
	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
)

type blockAPIService struct {
	provider  NetworkProvider
	txsParser *transactionsParser
}

// NewBlockAPIService will create a new instance of blockAPIService
func NewBlockAPIService(provider NetworkProvider) server.BlockAPIServicer {
	return &blockAPIService{
		provider:  provider,
		txsParser: newTransactionParser(provider),
	}
}

// Block implements the /block endpoint.
func (service *blockAPIService) Block(
	_ context.Context,
	request *types.BlockRequest,
) (*types.BlockResponse, *types.Error) {
	if request.BlockIdentifier.Index != nil {
		return service.getBlockByNonce(*request.BlockIdentifier.Index)
	}

	if request.BlockIdentifier.Hash != nil {
		return service.getBlockByHash(*request.BlockIdentifier.Hash)
	}

	return nil, ErrMustQueryByIndexOrByHash
}

func (service *blockAPIService) getBlockByNonce(nonce int64) (*types.BlockResponse, *types.Error) {
	block, err := service.provider.GetBlockByNonce(nonce)
	if err != nil {
		return nil, wrapErr(ErrUnableToGetBlock, err)
	}

	rosettaBlock, err := service.parseBlock(block)
	if err != nil {
		return nil, wrapErr(ErrUnableToGetBlock, err)
	}

	return rosettaBlock, nil
}

func (service *blockAPIService) getBlockByHash(hash string) (*types.BlockResponse, *types.Error) {
	block, err := service.provider.GetBlockByHash(hash)
	if err != nil {
		return nil, wrapErr(ErrUnableToGetBlock, err)
	}

	rosettaBlock, err := service.parseBlock(block)
	if err != nil {
		return nil, wrapErr(ErrUnableToGetBlock, err)
	}

	return rosettaBlock, nil
}

func (service *blockAPIService) parseBlock(block *data.Block) (*types.BlockResponse, error) {
	var parentBlockIdentifier *types.BlockIdentifier
	if block.Nonce != 0 {
		parentBlockIdentifier = &types.BlockIdentifier{
			Index: int64(block.Nonce - 1),
			Hash:  block.PrevBlockHash,
		}
	}

	transactions, err := service.txsParser.parseTxsFromBlock(block)
	if err != nil {
		return nil, err
	}

	return &types.BlockResponse{
		Block: &types.Block{
			BlockIdentifier: &types.BlockIdentifier{
				Index: int64(block.Nonce),
				Hash:  block.Hash,
			},
			ParentBlockIdentifier: parentBlockIdentifier,
			Timestamp:             int64(block.Timestamp),
			Transactions:          transactions,
			Metadata: objectsMap{
				"epoch": block.Epoch,
				"round": block.Round,
			},
		},
	}, nil
}

// BlockTransaction - not implemented
// We dont need this method because all transactions are returned by method Block
func (service *blockAPIService) BlockTransaction(
	_ context.Context,
	_ *types.BlockTransactionRequest,
) (*types.BlockTransactionResponse, *types.Error) {
	return nil, ErrNotImplemented
}
