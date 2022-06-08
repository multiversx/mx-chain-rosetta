package services

import (
	"context"

	"github.com/ElrondNetwork/elrond-proxy-go/data"
	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
)

type blockService struct {
	provider          NetworkProvider
	extension         *networkProviderExtension
	txsParser         *transactionsParser
	genesisIdentifier *types.BlockIdentifier
}

// NewBlockService will create a new instance of blockService
func NewBlockService(provider NetworkProvider) server.BlockAPIServicer {
	extension := newNetworkProviderExtension(provider)
	genesisIdentifier := blockSummaryToIdentifier(provider.GetGenesisBlockSummary())

	return &blockService{
		provider:          provider,
		extension:         extension,
		txsParser:         newTransactionParser(provider),
		genesisIdentifier: genesisIdentifier,
	}
}

// Block implements the /block endpoint.
func (service *blockService) Block(
	_ context.Context,
	request *types.BlockRequest,
) (*types.BlockResponse, *types.Error) {
	index := request.BlockIdentifier.Index
	hasIndex := index != nil
	hasGenesisIndex := hasIndex && *index == service.genesisIdentifier.Index

	hash := request.BlockIdentifier.Hash
	hasHash := hash != nil
	hasGenesisHash := hasHash && *hash == service.genesisIdentifier.Hash

	isGenesis := hasGenesisIndex || hasGenesisHash

	if isGenesis {
		return &types.BlockResponse{
			Block: &types.Block{
				BlockIdentifier:       service.genesisIdentifier,
				ParentBlockIdentifier: service.genesisIdentifier,
				Timestamp:             timestampInMilliseconds(service.provider.GetGenesisTimestamp()),
			},
		}, nil
	}

	if hasIndex {
		log.Debug("blockService.Block()", "index", *index)
		return service.getBlockByNonce(*index)
	}

	if hasHash {
		log.Debug("blockService.Block()", "hash", *hash)
		return service.getBlockByHash(*hash)
	}

	return nil, ErrMustQueryByIndexOrByHash
}

func (service *blockService) getBlockByNonce(nonce int64) (*types.BlockResponse, *types.Error) {
	block, err := service.provider.GetBlockByNonce(uint64(nonce))
	if err != nil {
		return nil, wrapErr(ErrUnableToGetBlock, err)
	}

	rosettaBlock, err := service.convertToRosettaBlock(block)
	if err != nil {
		return nil, wrapErr(ErrUnableToGetBlock, err)
	}

	return rosettaBlock, nil
}

func (service *blockService) getBlockByHash(hash string) (*types.BlockResponse, *types.Error) {
	block, err := service.provider.GetBlockByHash(hash)
	if err != nil {
		return nil, wrapErr(ErrUnableToGetBlock, err)
	}

	rosettaBlock, err := service.convertToRosettaBlock(block)
	if err != nil {
		return nil, wrapErr(ErrUnableToGetBlock, err)
	}

	return rosettaBlock, nil
}

func (service *blockService) convertToRosettaBlock(block *data.Block) (*types.BlockResponse, error) {
	// Genesis block is handled separately, in Block()
	parentBlockIdentifier := &types.BlockIdentifier{
		Index: int64(block.Nonce - 1),
		Hash:  block.PrevBlockHash,
	}

	transactions, err := service.txsParser.parseTxsFromBlock(block)
	if err != nil {
		return nil, err
	}

	return &types.BlockResponse{
		Block: &types.Block{
			BlockIdentifier:       blockToIdentifier(block),
			ParentBlockIdentifier: parentBlockIdentifier,
			Timestamp:             timestampInMilliseconds(int64(block.Timestamp)),
			Transactions:          transactions,
			Metadata: objectsMap{
				"shard":  block.Shard,
				"epoch":  block.Epoch,
				"round":  block.Round,
				"status": block.Status,
			},
		},
	}, nil
}

// BlockTransaction is not implemented, since all transactions are returned by /block
func (service *blockService) BlockTransaction(
	_ context.Context,
	_ *types.BlockTransactionRequest,
) (*types.BlockTransactionResponse, *types.Error) {
	return nil, ErrNotImplemented
}
