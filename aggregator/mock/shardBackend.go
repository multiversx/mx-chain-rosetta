package mock

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sync"

	"github.com/coinbase/rosetta-sdk-go/types"
	shardServices "github.com/multiversx/mx-chain-rosetta/server/services"
)

const (
	// OpStatusSuccess is the status of successful operations
	OpStatusSuccess = "Success"
	// OpStatusFailure is the status of failed operations
	OpStatusFailure = "Failure"

	errCodeUnableToGetAccount     = 2
	errCodeUnableToGetBlock       = 4
	errCodeTransactionIsNotInPool = 15
)

// NativeCurrency is the currency used by the mocks
var NativeCurrency = &types.Currency{Symbol: "EGLD", Decimals: 18}

var errBackendDown = errors.New("connection refused (mock)")

// ShardChain is an in-memory chain of a shard, as a shard instance would present it:
// blocks (with Rosetta transactions and operations for the accounts of the shard) and the balances after each block.
type ShardChain struct {
	Shard uint32

	mutex    sync.RWMutex
	blocks   []*types.Block
	balances []map[string]*big.Int
}

// NewShardChain creates an empty chain
func NewShardChain(shard uint32) *ShardChain {
	return &ShardChain{
		Shard:    shard,
		blocks:   make([]*types.Block, 0),
		balances: make([]map[string]*big.Int, 0),
	}
}

// AddBlock appends a block. The balances are updated by applying the successful operations of the block.
func (chain *ShardChain) AddBlock(block *types.Block) {
	chain.mutex.Lock()
	defer chain.mutex.Unlock()

	if block.BlockIdentifier.Index != int64(len(chain.blocks)) {
		panic(fmt.Sprintf("mock.ShardChain: expected block %d, got %d", len(chain.blocks), block.BlockIdentifier.Index))
	}

	balances := make(map[string]*big.Int)
	if len(chain.balances) > 0 {
		for address, balance := range chain.balances[len(chain.balances)-1] {
			balances[address] = balance
		}
	}

	for _, tx := range block.Transactions {
		for _, operation := range tx.Operations {
			if operation.Status == nil || *operation.Status != OpStatusSuccess {
				continue
			}

			delta, ok := big.NewInt(0).SetString(operation.Amount.Value, 10)
			if !ok {
				panic("mock.ShardChain: bad amount " + operation.Amount.Value)
			}

			address := operation.Account.Address
			current, exists := balances[address]
			if !exists {
				current = big.NewInt(0)
			}

			updated := big.NewInt(0).Add(current, delta)
			if updated.Sign() < 0 {
				panic(fmt.Sprintf("mock.ShardChain: negative balance for %s in block %d of shard %d", address, block.BlockIdentifier.Index, chain.Shard))
			}

			balances[address] = updated
		}
	}

	chain.blocks = append(chain.blocks, block)
	chain.balances = append(chain.balances, balances)
}

// NumBlocks returns the number of blocks (including the genesis block)
func (chain *ShardChain) NumBlocks() int {
	chain.mutex.RLock()
	defer chain.mutex.RUnlock()

	return len(chain.blocks)
}

// LatestIndex returns the index of the latest block
func (chain *ShardChain) LatestIndex() int64 {
	return int64(chain.NumBlocks()) - 1
}

// GetBlock returns a block, by index
func (chain *ShardChain) GetBlock(index int64) (*types.Block, bool) {
	chain.mutex.RLock()
	defer chain.mutex.RUnlock()

	if index < 0 || index >= int64(len(chain.blocks)) {
		return nil, false
	}

	return chain.blocks[index], true
}

// GetBalance returns the balance of an address after the block with the given index
func (chain *ShardChain) GetBalance(address string, index int64) (*big.Int, bool) {
	chain.mutex.RLock()
	defer chain.mutex.RUnlock()

	if index < 0 || index >= int64(len(chain.balances)) {
		return nil, false
	}

	balance, exists := chain.balances[index][address]
	if !exists {
		return big.NewInt(0), true
	}

	return big.NewInt(0).Set(balance), true
}

// ShardBackend is a fake shard instance, backed by an in-memory chain
type ShardBackend struct {
	Network *types.NetworkIdentifier
	Chain   *ShardChain

	mutex sync.Mutex
	// servedTip limits the blocks that the backend serves (to simulate a shard instance that lags). Negative means no limit.
	servedTip   int64
	oldestIndex int64
	isDown      bool
	isSynced    bool
	// tamperHashes makes the backend return blocks with altered hashes (to simulate a backend on a different chain)
	tamperHashes bool

	mempoolTransactions   map[string]*types.Transaction
	submittedTransactions []string
	calls                 map[string]int
}

// NewShardBackend creates a fake shard instance
func NewShardBackend(network *types.NetworkIdentifier, chain *ShardChain) *ShardBackend {
	return &ShardBackend{
		Network:             network,
		Chain:               chain,
		servedTip:           -1,
		isSynced:            true,
		mempoolTransactions: make(map[string]*types.Transaction),
		calls:               make(map[string]int),
	}
}

// SetServedTip limits the blocks served by the backend (pass a negative value to remove the limit)
func (shardBackend *ShardBackend) SetServedTip(index int64) {
	shardBackend.mutex.Lock()
	defer shardBackend.mutex.Unlock()

	shardBackend.servedTip = index
}

// SetOldestIndex sets the oldest block (with historical balances) reported by the backend
func (shardBackend *ShardBackend) SetOldestIndex(index int64) {
	shardBackend.mutex.Lock()
	defer shardBackend.mutex.Unlock()

	shardBackend.oldestIndex = index
}

// SetDown makes the backend unreachable (or reachable again)
func (shardBackend *ShardBackend) SetDown(isDown bool) {
	shardBackend.mutex.Lock()
	defer shardBackend.mutex.Unlock()

	shardBackend.isDown = isDown
}

// SetTamperHashes makes the backend return blocks with altered hashes
func (shardBackend *ShardBackend) SetTamperHashes(tamper bool) {
	shardBackend.mutex.Lock()
	defer shardBackend.mutex.Unlock()

	shardBackend.tamperHashes = tamper
}

// AddMempoolTransaction adds a transaction to the (fake) pool
func (shardBackend *ShardBackend) AddMempoolTransaction(tx *types.Transaction) {
	shardBackend.mutex.Lock()
	defer shardBackend.mutex.Unlock()

	shardBackend.mempoolTransactions[tx.TransactionIdentifier.Hash] = tx
}

// NumCalls returns how many times an endpoint was called
func (shardBackend *ShardBackend) NumCalls(endpoint string) int {
	shardBackend.mutex.Lock()
	defer shardBackend.mutex.Unlock()

	return shardBackend.calls[endpoint]
}

// SubmittedTransactions returns the transactions received by /construction/submit
func (shardBackend *ShardBackend) SubmittedTransactions() []string {
	shardBackend.mutex.Lock()
	defer shardBackend.mutex.Unlock()

	result := make([]string, len(shardBackend.submittedTransactions))
	copy(result, shardBackend.submittedTransactions)
	return result
}

// enter records a call and returns the state needed to answer it
func (shardBackend *ShardBackend) enter(endpoint string) (currentIndex int64, err error) {
	shardBackend.mutex.Lock()
	defer shardBackend.mutex.Unlock()

	shardBackend.calls[endpoint]++

	if shardBackend.isDown {
		return 0, errBackendDown
	}

	currentIndex = shardBackend.Chain.LatestIndex()
	if shardBackend.servedTip >= 0 && shardBackend.servedTip < currentIndex {
		currentIndex = shardBackend.servedTip
	}

	return currentIndex, nil
}

func newRosettaError(code int32, message string, retriable bool) *types.Error {
	return &types.Error{Code: code, Message: message, Retriable: retriable}
}

func (shardBackend *ShardBackend) getBlockIdentifier(index int64) *types.BlockIdentifier {
	block, _ := shardBackend.Chain.GetBlock(index)
	identifier := *block.BlockIdentifier

	shardBackend.mutex.Lock()
	tamper := shardBackend.tamperHashes
	shardBackend.mutex.Unlock()

	if tamper && index > 0 {
		identifier.Hash = FakeHash("tampered", identifier.Hash)
	}

	return &identifier
}

// NetworkList implements /network/list
func (shardBackend *ShardBackend) NetworkList(_ context.Context, _ *types.MetadataRequest) (*types.NetworkListResponse, *types.Error, error) {
	_, err := shardBackend.enter("/network/list")
	if err != nil {
		return nil, nil, err
	}

	return &types.NetworkListResponse{NetworkIdentifiers: []*types.NetworkIdentifier{shardBackend.Network}}, nil, nil
}

// NetworkOptions implements /network/options
func (shardBackend *ShardBackend) NetworkOptions(_ context.Context, _ *types.NetworkRequest) (*types.NetworkOptionsResponse, *types.Error, error) {
	_, err := shardBackend.enter("/network/options")
	if err != nil {
		return nil, nil, err
	}

	middlewareVersion := "v0.0.0-mock"

	return &types.NetworkOptionsResponse{
		Version: &types.Version{
			RosettaVersion:    "1.4.12",
			NodeVersion:       "mock",
			MiddlewareVersion: &middlewareVersion,
		},
		Allow: &types.Allow{
			OperationStatuses: []*types.OperationStatus{
				{Status: OpStatusSuccess, Successful: true},
				{Status: OpStatusFailure, Successful: false},
			},
			OperationTypes: shardServices.SupportedOperationTypes,
			Errors: []*types.Error{
				newRosettaError(errCodeUnableToGetAccount, "unable to get account", true),
				newRosettaError(errCodeUnableToGetBlock, "unable to get block", true),
				newRosettaError(errCodeTransactionIsNotInPool, "transaction is not in pool", true),
			},
			HistoricalBalanceLookup: true,
		},
	}, nil, nil
}

// NetworkStatus implements /network/status
func (shardBackend *ShardBackend) NetworkStatus(_ context.Context, _ *types.NetworkRequest) (*types.NetworkStatusResponse, *types.Error, error) {
	currentIndex, err := shardBackend.enter("/network/status")
	if err != nil {
		return nil, nil, err
	}

	shardBackend.mutex.Lock()
	oldestIndex := shardBackend.oldestIndex
	isSynced := shardBackend.isSynced
	shardBackend.mutex.Unlock()

	currentBlock, _ := shardBackend.Chain.GetBlock(currentIndex)

	return &types.NetworkStatusResponse{
		CurrentBlockIdentifier: shardBackend.getBlockIdentifier(currentIndex),
		CurrentBlockTimestamp:  currentBlock.Timestamp,
		GenesisBlockIdentifier: shardBackend.getBlockIdentifier(0),
		OldestBlockIdentifier:  shardBackend.getBlockIdentifier(oldestIndex),
		SyncStatus:             &types.SyncStatus{Synced: &isSynced},
		Peers: []*types.Peer{
			{PeerID: fmt.Sprintf("observer-of-shard-%d", shardBackend.Chain.Shard), Metadata: map[string]interface{}{"version": "mock"}},
		},
	}, nil, nil
}

// Block implements /block (by index only, as used by the aggregator)
func (shardBackend *ShardBackend) Block(_ context.Context, request *types.BlockRequest) (*types.BlockResponse, *types.Error, error) {
	currentIndex, err := shardBackend.enter("/block")
	if err != nil {
		return nil, nil, err
	}

	if request.BlockIdentifier == nil || request.BlockIdentifier.Index == nil {
		return nil, newRosettaError(errCodeUnableToGetBlock, "unable to get block", true), nil
	}

	index := *request.BlockIdentifier.Index
	block, ok := shardBackend.Chain.GetBlock(index)
	if !ok || index > currentIndex {
		return nil, newRosettaError(errCodeUnableToGetBlock, "unable to get block", true), nil
	}

	blockCopy := *block
	blockCopy.BlockIdentifier = shardBackend.getBlockIdentifier(index)
	return &types.BlockResponse{Block: &blockCopy}, nil, nil
}

// AccountBalance implements /account/balance (native currency only)
func (shardBackend *ShardBackend) AccountBalance(_ context.Context, request *types.AccountBalanceRequest) (*types.AccountBalanceResponse, *types.Error, error) {
	currentIndex, err := shardBackend.enter("/account/balance")
	if err != nil {
		return nil, nil, err
	}

	index := currentIndex
	if request.BlockIdentifier != nil && request.BlockIdentifier.Index != nil {
		index = *request.BlockIdentifier.Index
	}

	balance, ok := shardBackend.Chain.GetBalance(request.AccountIdentifier.Address, index)
	if !ok || index > currentIndex {
		return nil, newRosettaError(errCodeUnableToGetAccount, "unable to get account", true), nil
	}

	return &types.AccountBalanceResponse{
		BlockIdentifier: shardBackend.getBlockIdentifier(index),
		Balances: []*types.Amount{
			{Value: balance.String(), Currency: NativeCurrency},
		},
		Metadata: map[string]interface{}{"nonce": 0},
	}, nil, nil
}

// MempoolTransaction implements /mempool/transaction
func (shardBackend *ShardBackend) MempoolTransaction(_ context.Context, request *types.MempoolTransactionRequest) (*types.MempoolTransactionResponse, *types.Error, error) {
	_, err := shardBackend.enter("/mempool/transaction")
	if err != nil {
		return nil, nil, err
	}

	shardBackend.mutex.Lock()
	tx, ok := shardBackend.mempoolTransactions[request.TransactionIdentifier.Hash]
	shardBackend.mutex.Unlock()

	if !ok {
		return nil, newRosettaError(errCodeTransactionIsNotInPool, "transaction is not in pool", true), nil
	}

	return &types.MempoolTransactionResponse{Transaction: tx}, nil, nil
}

// ConstructionCombine implements /construction/combine
func (shardBackend *ShardBackend) ConstructionCombine(_ context.Context, request *types.ConstructionCombineRequest) (*types.ConstructionCombineResponse, *types.Error, error) {
	_, err := shardBackend.enter("/construction/combine")
	if err != nil {
		return nil, nil, err
	}

	return &types.ConstructionCombineResponse{SignedTransaction: request.UnsignedTransaction}, nil, nil
}

// ConstructionDerive implements /construction/derive
func (shardBackend *ShardBackend) ConstructionDerive(_ context.Context, _ *types.ConstructionDeriveRequest) (*types.ConstructionDeriveResponse, *types.Error, error) {
	_, err := shardBackend.enter("/construction/derive")
	if err != nil {
		return nil, nil, err
	}

	return &types.ConstructionDeriveResponse{AccountIdentifier: &types.AccountIdentifier{Address: "derived"}}, nil, nil
}

// ConstructionHash implements /construction/hash
func (shardBackend *ShardBackend) ConstructionHash(_ context.Context, request *types.ConstructionHashRequest) (*types.TransactionIdentifierResponse, *types.Error, error) {
	_, err := shardBackend.enter("/construction/hash")
	if err != nil {
		return nil, nil, err
	}

	return &types.TransactionIdentifierResponse{
		TransactionIdentifier: &types.TransactionIdentifier{Hash: FakeHash("tx", request.SignedTransaction)},
	}, nil, nil
}

// ConstructionMetadata implements /construction/metadata
func (shardBackend *ShardBackend) ConstructionMetadata(_ context.Context, request *types.ConstructionMetadataRequest) (*types.ConstructionMetadataResponse, *types.Error, error) {
	_, err := shardBackend.enter("/construction/metadata")
	if err != nil {
		return nil, nil, err
	}

	return &types.ConstructionMetadataResponse{
		Metadata: map[string]interface{}{
			"sender":      request.Options["sender"],
			"nonce":       7,
			"servedShard": shardBackend.Chain.Shard,
		},
	}, nil, nil
}

// ConstructionParse implements /construction/parse
func (shardBackend *ShardBackend) ConstructionParse(_ context.Context, _ *types.ConstructionParseRequest) (*types.ConstructionParseResponse, *types.Error, error) {
	_, err := shardBackend.enter("/construction/parse")
	if err != nil {
		return nil, nil, err
	}

	return &types.ConstructionParseResponse{Operations: []*types.Operation{}}, nil, nil
}

// ConstructionPayloads implements /construction/payloads
func (shardBackend *ShardBackend) ConstructionPayloads(_ context.Context, _ *types.ConstructionPayloadsRequest) (*types.ConstructionPayloadsResponse, *types.Error, error) {
	_, err := shardBackend.enter("/construction/payloads")
	if err != nil {
		return nil, nil, err
	}

	return &types.ConstructionPayloadsResponse{UnsignedTransaction: "{}", Payloads: []*types.SigningPayload{}}, nil, nil
}

// ConstructionPreprocess implements /construction/preprocess
func (shardBackend *ShardBackend) ConstructionPreprocess(_ context.Context, _ *types.ConstructionPreprocessRequest) (*types.ConstructionPreprocessResponse, *types.Error, error) {
	_, err := shardBackend.enter("/construction/preprocess")
	if err != nil {
		return nil, nil, err
	}

	return &types.ConstructionPreprocessResponse{Options: map[string]interface{}{}}, nil, nil
}

// ConstructionPreprocessOperations implements /construction/preprocess_operations
func (shardBackend *ShardBackend) ConstructionPreprocessOperations(_ context.Context, _ *types.ConstructionPreprocessOperationsRequest) (*types.ConstructionPreprocessOperationsResponse, *types.Error, error) {
	_, err := shardBackend.enter("/construction/preprocess_operations")
	if err != nil {
		return nil, nil, err
	}

	return &types.ConstructionPreprocessOperationsResponse{Operations: []*types.Operation{}}, nil, nil
}

// ConstructionSubmit implements /construction/submit
func (shardBackend *ShardBackend) ConstructionSubmit(_ context.Context, request *types.ConstructionSubmitRequest) (*types.TransactionIdentifierResponse, *types.Error, error) {
	_, err := shardBackend.enter("/construction/submit")
	if err != nil {
		return nil, nil, err
	}

	shardBackend.mutex.Lock()
	shardBackend.submittedTransactions = append(shardBackend.submittedTransactions, request.SignedTransaction)
	shardBackend.mutex.Unlock()

	return &types.TransactionIdentifierResponse{
		TransactionIdentifier: &types.TransactionIdentifier{Hash: FakeHash("tx", request.SignedTransaction)},
	}, nil, nil
}
