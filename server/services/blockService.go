package services

import (
	"context"
	"sync"

	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/multiversx/mx-chain-core-go/data/api"
	"github.com/multiversx/mx-chain-rosetta/server/resources"
)

type blockService struct {
	provider       NetworkProvider
	extension      *networkProviderExtension
	errFactory     *errFactory
	txsTransformer *transactionsTransformer

	genesisBlock      *types.BlockResponse
	genesisBlockMutex sync.RWMutex
}

// NewBlockService will create a new instance of blockService
func NewBlockService(provider NetworkProvider) server.BlockAPIServicer {
	extension := newNetworkProviderExtension(provider)

	return &blockService{
		provider:       provider,
		extension:      extension,
		errFactory:     newErrFactory(),
		txsTransformer: newTransactionsTransformer(provider),
	}
}

// Block implements the /block endpoint.
func (service *blockService) Block(
	_ context.Context,
	request *types.BlockRequest,
) (*types.BlockResponse, *types.Error) {
	response, err := service.doGetBlock(request)
	if err != nil {
		return nil, err
	}

	traceBlockResponse(response)
	return response, nil
}

func (service *blockService) doGetBlock(request *types.BlockRequest) (*types.BlockResponse, *types.Error) {
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
		log.Trace("blockService.Block()", "index", *index)
		return service.getBlockByNonce(*index)
	}

	if hasHash {
		log.Trace("blockService.Block()", "hash", *hash)
		return service.getBlockByHash(*hash)
	}

	return nil, service.errFactory.newErr(ErrMustQueryByIndexOrByHash)
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
		return nil, service.errFactory.newErrWithOriginal(ErrUnableToGetGenesisBlock, err)
	}

	operations, err := service.createGenesisOperations(genesisBalances)
	if err != nil {
		return nil, service.errFactory.newErrWithOriginal(ErrUnableToGetGenesisBlock, err)
	}

	genesisTransaction := &types.Transaction{
		TransactionIdentifier: hashToTransactionIdentifier(emptyHash),
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
			Account: addressToAccountIdentifier(balance.Address),
			Amount:  service.extension.valueToNativeAmount(balance.Balance),
		}

		operations = append(operations, operation)
	}

	operations, err := filterOperationsByAddress(operations, service.provider.IsAddressObserved)
	if err != nil {
		return nil, err
	}

	applyDefaultStatusOnOperations(operations)

	return operations, nil
}

func (service *blockService) getBlockByNonce(nonce int64) (*types.BlockResponse, *types.Error) {
	block, err := service.provider.GetBlockByNonce(uint64(nonce))
	if err != nil {
		return nil, service.errFactory.newErrWithOriginal(ErrUnableToGetBlock, err)
	}

	rosettaBlock, err := service.convertToRosettaBlock(block)
	if err != nil {
		return nil, service.errFactory.newErrWithOriginal(ErrUnableToGetBlock, err)
	}

	return rosettaBlock, nil
}

func (service *blockService) getBlockByHash(hash string) (*types.BlockResponse, *types.Error) {
	block, err := service.provider.GetBlockByHash(hash)
	if err != nil {
		return nil, service.errFactory.newErrWithOriginal(ErrUnableToGetBlock, err)
	}

	rosettaBlock, err := service.convertToRosettaBlock(block)
	if err != nil {
		return nil, service.errFactory.newErrWithOriginal(ErrUnableToGetBlock, err)
	}

	return rosettaBlock, nil
}

func (service *blockService) convertToRosettaBlock(block *api.Block) (*types.BlockResponse, error) {
	// Genesis block is handled separately, in Block()
	parentBlockIdentifier := &types.BlockIdentifier{
		Index: int64(block.Nonce - 1),
		Hash:  block.PrevBlockHash,
	}

	// Link the second block to genesis
	if block.Nonce == 1 {
		parentBlockIdentifier = service.extension.getGenesisBlockIdentifier()
	}

	transactions, err := service.txsTransformer.transformBlockTxs(block)
	if err != nil {
		return nil, err
	}

	response := &types.BlockResponse{
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
	}

	return response, nil
}

// BlockTransaction is not implemented, since all transactions are returned by /block
func (service *blockService) BlockTransaction(
	_ context.Context,
	_ *types.BlockTransactionRequest,
) (*types.BlockTransactionResponse, *types.Error) {
	return nil, service.errFactory.newErr(ErrNotImplemented)
}
