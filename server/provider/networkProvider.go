package provider

import (
	"errors"
	"sync"

	logger "github.com/ElrondNetwork/elrond-go-logger"
	"github.com/ElrondNetwork/elrond-go/core"
	"github.com/ElrondNetwork/elrond-go/core/pubkeyConverter"
	"github.com/ElrondNetwork/elrond-go/data/transaction"
	hasherFactory "github.com/ElrondNetwork/elrond-go/hashing/factory"
	marshalFactory "github.com/ElrondNetwork/elrond-go/marshal/factory"
	"github.com/ElrondNetwork/elrond-go/sharding"
	"github.com/ElrondNetwork/elrond-proxy-go/common"
	"github.com/ElrondNetwork/elrond-proxy-go/data"
	"github.com/ElrondNetwork/elrond-proxy-go/facade"
	"github.com/ElrondNetwork/elrond-proxy-go/observer"
	"github.com/ElrondNetwork/elrond-proxy-go/process"
	"github.com/ElrondNetwork/elrond-proxy-go/process/cache"
	processFactory "github.com/ElrondNetwork/elrond-proxy-go/process/factory"
	"github.com/ElrondNetwork/rosetta/server/resources"
)

var (
	notApplicableConfigurationFilePath   = "not applicable"
	notApplicableFullHistoryNodesMessage = "not applicable"
	notApplicableMaxGasLimitPerBlock     = "0"

	urlPathGetNetworkConfig = "/network/config"
	urlPathGetNodeStatus    = "/node/status"
)

var log = logger.GetOrCreate("server/provider")

type ArgsNewNetworkProvider struct {
	NumShards                   uint32
	ObservedActualShard         uint32
	ObservedProjectedShard      uint32
	ObservedProjectedShardIsSet bool
	ObserverUrl                 string
}

type networkProvider struct {
	pubKeyConverter      core.PubkeyConverter
	baseProcessor        process.Processor
	accountProcessor     facade.AccountProcessor
	transactionProcessor facade.TransactionProcessor
	blockProcessor       facade.BlockProcessor
	nodeStatusProcessor  facade.NodeStatusProcessor

	observedActualShard         uint32
	observedProjectedShard      uint32
	observedProjectedShardIsSet bool
	observerUrl                 string

	networkConfig *resources.NetworkConfig
	mutex         sync.RWMutex
}

func NewNetworkProvider(args ArgsNewNetworkProvider) (*networkProvider, error) {
	shardCoordinator, err := sharding.NewMultiShardCoordinator(args.NumShards, args.ObservedActualShard)
	if err != nil {
		return nil, err
	}

	pubKeyConverter, err := pubkeyConverter.NewBech32PubkeyConverter(pubKeyLength)
	if err != nil {
		return nil, err
	}

	observers := []*data.NodeData{
		{
			ShardId:  args.ObservedActualShard,
			Address:  args.ObserverUrl,
			IsSynced: true,
		},
	}

	observersProvider, err := observer.NewSimpleNodesProvider(observers, notApplicableConfigurationFilePath)
	if err != nil {
		return nil, err
	}

	disabledObserversProvider := observer.NewDisabledNodesProvider(notApplicableFullHistoryNodesMessage)
	if err != nil {
		return nil, err
	}

	baseProcessor, err := process.NewBaseProcessor(
		requestTimeoutInSeconds,
		shardCoordinator,
		observersProvider,
		disabledObserversProvider,
		pubKeyConverter,
	)
	if err != nil {
		return nil, err
	}

	accountProcessor, err := process.NewAccountProcessor(
		baseProcessor,
		pubKeyConverter,
		&disabledExternalStorageConnector{},
	)
	if err != nil {
		return nil, err
	}

	transactionHasher, err := hasherFactory.NewHasher(transactionsHasherType)
	if err != nil {
		return nil, err
	}

	transactionMarshalizer, err := marshalFactory.NewMarshalizer(transactionsMarshalizerType)
	if err != nil {
		return nil, err
	}

	transactionProcessor, err := processFactory.CreateTransactionProcessor(
		baseProcessor,
		pubKeyConverter,
		transactionHasher,
		transactionMarshalizer,
		notApplicableMaxGasLimitPerBlock,
		notApplicableMaxGasLimitPerBlock,
	)
	if err != nil {
		return nil, err
	}

	blockProcessor, err := process.NewBlockProcessor(&disabledExternalStorageConnector{}, baseProcessor)
	if err != nil {
		return nil, err
	}

	economicMetricsCacher := cache.NewGenericApiResponseMemoryCacher()
	nodeStatusProcessor, err := process.NewNodeStatusProcessor(baseProcessor, economicMetricsCacher, nodeStatusCacheDuration)
	if err != nil {
		return nil, err
	}

	return &networkProvider{
		pubKeyConverter:      pubKeyConverter,
		baseProcessor:        baseProcessor,
		accountProcessor:     accountProcessor,
		transactionProcessor: transactionProcessor,
		blockProcessor:       blockProcessor,
		nodeStatusProcessor:  nodeStatusProcessor,

		observedActualShard:         args.ObservedActualShard,
		observedProjectedShard:      args.ObservedProjectedShard,
		observedProjectedShardIsSet: args.ObservedProjectedShardIsSet,
		observerUrl:                 args.ObserverUrl,
	}, nil
}

// GetNetworkConfig gets the network config (once fetched, the network config is indefinitely held in memory)
func (provider *networkProvider) GetNetworkConfig() (*resources.NetworkConfig, error) {
	// We are using "double-checked locking" pattern to lazily initialize the network config object.
	provider.mutex.RLock()
	existing := provider.networkConfig
	provider.mutex.RUnlock()
	if existing != nil {
		return existing, nil
	}

	provider.mutex.Lock()
	defer provider.mutex.Unlock()

	if provider.networkConfig != nil {
		return provider.networkConfig, nil
	}

	networkConfig, err := provider.doGetNetworkConfig()
	if err != nil {
		return nil, err
	}

	provider.networkConfig = networkConfig
	return networkConfig, nil
}

func (provider *networkProvider) doGetNetworkConfig() (*resources.NetworkConfig, error) {
	response := &resources.NetworkConfigApiResponse{}

	_, err := provider.baseProcessor.CallGetRestEndPoint(provider.observerUrl, urlPathGetNetworkConfig, &response)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, errors.New(response.Error)
	}
	if response.Data.Config.NodeIsStarting != "" {
		return nil, errors.New(response.Data.Config.NodeIsStarting)
	}

	return &response.Data.Config, nil
}

// GetLatestBlockSummary gets a summary of the latest block
func (provider *networkProvider) GetLatestBlockSummary() (*resources.BlockSummary, error) {
	nodeStatus, err := provider.getNodeStatus()
	if err != nil {
		return nil, err
	}

	queryOptions := common.BlockQueryOptions{
		WithTransactions: false,
		WithLogs:         false,
	}

	blockResponse, err := provider.blockProcessor.GetBlockByNonce(
		provider.observedActualShard,
		nodeStatus.HighestFinalNonce,
		queryOptions,
	)
	if err != nil {
		return nil, newErrCannotGetBlockByNonce(nodeStatus.HighestFinalNonce, err)
	}

	return &resources.BlockSummary{
		Nonce:             blockResponse.Data.Block.Nonce,
		Hash:              blockResponse.Data.Block.Hash,
		PreviousBlockHash: blockResponse.Data.Block.PrevBlockHash,
		Timestamp:         int64(blockResponse.Data.Block.Timestamp),
	}, nil
}

func (provider *networkProvider) getNodeStatus() (*resources.NodeStatus, error) {
	response := &resources.NodeStatusApiResponse{}

	_, err := provider.baseProcessor.CallGetRestEndPoint(provider.observerUrl, urlPathGetNodeStatus, &response)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, errors.New(response.Error)
	}

	return &response.Data.Status, nil
}

// GetBlockByNonce gets a block by nonce
func (provider *networkProvider) GetBlockByNonce(nonce uint64) (*data.Block, error) {
	queryOptions := common.BlockQueryOptions{
		WithTransactions: true,
		WithLogs:         true,
	}

	response, err := provider.blockProcessor.GetBlockByNonce(provider.observedActualShard, nonce, queryOptions)
	if err != nil {
		return nil, newErrCannotGetBlockByNonce(nonce, err)
	}
	if response.Error != "" {
		return nil, newErrCannotGetBlockByNonce(nonce, errors.New(response.Error))
	}

	return &response.Data.Block, nil
}

// GetBlockByHash gets a block by hash
func (provider *networkProvider) GetBlockByHash(hash string) (*data.Block, error) {
	queryOptions := common.BlockQueryOptions{
		WithTransactions: true,
		WithLogs:         true,
	}

	response, err := provider.blockProcessor.GetBlockByHash(provider.observedActualShard, hash, queryOptions)
	if err != nil {
		return nil, newErrCannotGetBlockByHash(hash, err)
	}
	if response.Error != "" {
		return nil, newErrCannotGetBlockByHash(hash, errors.New(response.Error))
	}

	return &response.Data.Block, nil
}

// GetAccount gets an account by address
func (provider *networkProvider) GetAccount(address string) (*data.Account, error) {
	account, err := provider.accountProcessor.GetAccount(address)
	if err != nil {
		return nil, newErrCannotGetAccount(address, err)
	}

	return account, nil
}

// ConvertPubKeyToAddress converts a public key to an address
func (provider *networkProvider) ConvertPubKeyToAddress(pubkey []byte) string {
	return provider.pubKeyConverter.Encode(pubkey)
}

// ConvertAddressToPubKey converts an address to a pubkey
func (provider *networkProvider) ConvertAddressToPubKey(address string) ([]byte, error) {
	return provider.pubKeyConverter.Decode(address)
}

// ComputeTransactionHash computes the hash of a provided transaction
func (provider *networkProvider) ComputeTransactionHash(tx *data.Transaction) (string, error) {
	return provider.transactionProcessor.ComputeTransactionHash(tx)
}

// SendTransaction broadcasts an already-signed transaction
func (provider *networkProvider) SendTx(tx *data.Transaction) (string, error) {
	_, hash, err := provider.transactionProcessor.SendTransaction(tx)
	if err != nil {
		return "", err
	}

	return hash, nil
}

// GetTransactionByHashFromPool gets a transaction from the pool
func (provider *networkProvider) GetTransactionByHashFromPool(hash string) (*data.FullTransaction, error) {
	tx, _, err := provider.transactionProcessor.GetTransactionByHashAndSenderAddress(hash, "", false)
	if err != nil {
		return nil, newErrCannotGetTransaction(hash, err)
	}

	if tx.Status == transaction.TxStatusPending {
		return tx, nil
	}

	return nil, nil
}
