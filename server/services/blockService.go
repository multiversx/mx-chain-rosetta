package services

import (
	"context"
	"sync"

	"github.com/ElrondNetwork/elrond-proxy-go/data"
	"github.com/ElrondNetwork/rosetta/server/resources"
	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
)

type blockService struct {
	provider  NetworkProvider
	extension *networkProviderExtension
	txsParser *transactionsParser

	genesisBlock      *types.BlockResponse
	genesisBlockMutex sync.RWMutex
}

// NewBlockService will create a new instance of blockService
func NewBlockService(provider NetworkProvider) server.BlockAPIServicer {
	extension := newNetworkProviderExtension(provider)

	return &blockService{
		provider:  provider,
		extension: extension,
		txsParser: newTransactionParser(provider),
	}
}

// Block implements the /block endpoint.
func (service *blockService) Block(
	_ context.Context,
	request *types.BlockRequest,
) (*types.BlockResponse, *types.Error) {
	genesisBlockIdentifier := service.extension.getGenesisBlockIdentifier()

	index := request.BlockIdentifier.Index
	hasIndex := index != nil
	hasGenesisIndex := hasIndex && *index == genesisBlockIdentifier.Index

	hash := request.BlockIdentifier.Hash
	hasHash := hash != nil
	hasGenesisHash := hasHash && *hash == genesisBlockIdentifier.Hash

	isGenesis := hasGenesisIndex || hasGenesisHash
	if isGenesis {
		return service.getGenesisBlock()
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

// getGenesisBlock returns or lazily fetches the genesis block (using "double-checked locking" pattern)
func (service *blockService) getGenesisBlock() (*types.BlockResponse, *types.Error) {
	log.Debug("blockService.getGenesisBlock()")

	service.genesisBlockMutex.RLock()
	block := service.genesisBlock
	service.genesisBlockMutex.RUnlock()

	if block != nil {
		return block, nil
	}

	service.genesisBlockMutex.Lock()
	defer service.genesisBlockMutex.Unlock()

	if service.genesisBlock != nil {
		return service.genesisBlock, nil
	}

	fetchedBlock, err := service.doGetGenesisBlock()
	if err != nil {
		return nil, err
	}

	service.genesisBlock = fetchedBlock
	return fetchedBlock, nil
}

func (service *blockService) doGetGenesisBlock() (*types.BlockResponse, *types.Error) {
	log.Debug("blockService.doGetGenesisBlock()")

	genesisBlockIdentifier := service.extension.getGenesisBlockIdentifier()
	genesisBalances, err := service.provider.GetGenesisBalances()
	if err != nil {
		return nil, wrapErr(ErrUnableToGetGenesisBlock, err)
	}

	operations, err := service.createGenesisOperations(genesisBalances)
	if err != nil {
		return nil, wrapErr(ErrUnableToGetGenesisBlock, err)
	}

	genesisTransaction := &types.Transaction{
		TransactionIdentifier: service.extension.getTransactionIdentifier(emptyHash),
		Operations:            operations,
	}

	return &types.BlockResponse{
		Block: &types.Block{
			BlockIdentifier:       genesisBlockIdentifier,
			ParentBlockIdentifier: genesisBlockIdentifier,
			Timestamp:             timestampInMilliseconds(service.provider.GetGenesisTimestamp()),
			Transactions:          []*types.Transaction{genesisTransaction},
		},
	}, nil
}

func (service *blockService) createGenesisOperations(balances []*resources.GenesisBalance) ([]*types.Operation, error) {
	operations := make([]*types.Operation, 0, len(balances))

	for _, balance := range balances {
		operation := &types.Operation{
			Type:    opGenesisBalanceMovement,
			Status:  &OpStatusSuccess,
			Account: service.extension.getAccountIdentifier(balance.Address),
			Amount:  service.extension.getNativeAmount(balance.Balance),
		}

		operations = append(operations, operation)
	}

	return service.extension.filterObservedOperations(operations)
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

	// Link the second block to genesis
	if block.Nonce == 1 {
		parentBlockIdentifier = service.extension.getGenesisBlockIdentifier()
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