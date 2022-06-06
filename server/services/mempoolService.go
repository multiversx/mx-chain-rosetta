package services

import (
	"context"

	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
)

type mempoolService struct {
	provider  NetworkProvider
	txsParser *transactionsParser
}

// NewMempoolService will create a new instance of mempoolAPIService
func NewMempoolService(provider NetworkProvider) server.MempoolAPIServicer {
	return &mempoolService{
		provider:  provider,
		txsParser: newTransactionParser(provider),
	}
}

// Mempool is not implemented yet
func (service *mempoolService) Mempool(context.Context, *types.NetworkRequest) (*types.MempoolResponse, *types.Error) {
	return nil, ErrNotImplemented
}

// MempoolTransaction will return operations for a transaction that is in pool
func (service *mempoolService) MempoolTransaction(
	_ context.Context,
	request *types.MempoolTransactionRequest,
) (*types.MempoolTransactionResponse, *types.Error) {
	tx, err := service.provider.GetTransactionByHashFromPool(request.TransactionIdentifier.Hash)
	if err != nil {
		return nil, wrapErr(ErrCannotParsePoolTransaction, err)
	}
	if tx == nil {
		return nil, ErrTransactionIsNotInPool
	}

	rosettaTx, err := service.txsParser.parseTx(tx, true)
	if err != nil {
		return nil, ErrCannotParsePoolTransaction
	}

	return &types.MempoolTransactionResponse{
		Transaction: rosettaTx,
	}, nil

}
