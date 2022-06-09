package provider

import (
	"errors"

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

	urlPathGetNetworkConfig   = "/network/config"
	urlPathGetNodeStatus      = "/node/status"
	urlPathGetGenesisBalances = "/network/genesis-balances"
)

var log = logger.GetOrCreate("server/provider")

type ArgsNewNetworkProvider struct {
	IsOffline                   bool
	NumShards                   uint32
	ObservedActualShard         uint32
	ObservedProjectedShard      uint32
	ObservedProjectedShardIsSet bool
	ObserverUrl                 string
	ObserverPubkey              string
	ChainID                     string
	GasPerDataByte              uint64
	MinGasPrice                 uint64
	MinGasLimit                 uint64
	NativeCurrencySymbol        string
	GenesisBlockHash            string
	GenesisTimestamp            int64
	OnlyFinalBlocks             bool
}

type networkProvider struct {
	isOffline bool

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
	observerPubkey              string
	nativeCurrencySymbol        string
	genesisBlockHash            string
	genesisTimestamp            int64
	onlyFinalBlocks             bool

	networkConfig *resources.NetworkConfig
}

// TODO: Move constructor calls to /factory. Receive dependencies in constructor.
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
		isOffline: args.IsOffline,

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
		observerPubkey:              args.ObserverPubkey,
		nativeCurrencySymbol:        args.NativeCurrencySymbol,
		genesisBlockHash:            args.GenesisBlockHash,
		genesisTimestamp:            args.GenesisTimestamp,
		onlyFinalBlocks:             args.OnlyFinalBlocks,

		networkConfig: &resources.NetworkConfig{
			ChainID:        args.ChainID,
			GasPerDataByte: args.GasPerDataByte,
			MinGasPrice:    args.MinGasPrice,
			MinGasLimit:    args.MinGasLimit,
		},
	}, nil
}

func (provider *networkProvider) IsOffline() bool {
	return provider.isOffline
}

func (provider *networkProvider) GetBlockchainName() string {
	return resources.BlockchainName
}

func (provider *networkProvider) GetChainID() string {
	return provider.networkConfig.ChainID
}

func (provider *networkProvider) GetNativeCurrency() resources.NativeCurrency {
	return resources.NativeCurrency{
		Symbol:   provider.nativeCurrencySymbol,
		Decimals: int32(nativeCurrencyNumDecimals),
	}
}

// GetObserverPubkey gets the pubkey of the connected observer
func (provider *networkProvider) GetObserverPubkey() string {
	return provider.observerPubkey
}

// GetNetworkConfig gets the network config (once fetched, the network config is indefinitely held in memory)
func (provider *networkProvider) GetNetworkConfig() *resources.NetworkConfig {
	return provider.networkConfig
}

// GetGenesisBlockSummary gets a summary of the genesis block
func (provider *networkProvider) GetGenesisBlockSummary() *resources.BlockSummary {
	return &resources.BlockSummary{
		Nonce:     uint64(genesisBlockNonce),
		Hash:      provider.genesisBlockHash,
		Timestamp: provider.genesisTimestamp,
	}
}

// GetGenesisTimestamp gets the timestamp of the genesis block
func (provider *networkProvider) GetGenesisTimestamp() int64 {
	return provider.genesisTimestamp
}

func (provider *networkProvider) GetGenesisBalances() ([]*resources.GenesisBalance, error) {
	if provider.isOffline {
		return nil, errIsOffline
	}

	response := &resources.GenesisBalancesApiResponse{}

	_, err := provider.baseProcessor.CallGetRestEndPoint(provider.observerUrl, urlPathGetGenesisBalances, &response)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, errors.New(response.Error)
	}

	return response.Data.Balances, nil
}

// GetLatestBlockSummary gets a summary of the latest block
func (provider *networkProvider) GetLatestBlockSummary() (*resources.BlockSummary, error) {
	if provider.isOffline {
		return nil, errIsOffline
	}

	latestBlockNonce, err := provider.getLatestBlockNonce()
	if err != nil {
		return nil, err
	}

	queryOptions := common.BlockQueryOptions{
		WithTransactions: false,
		WithLogs:         false,
	}

	blockResponse, err := provider.blockProcessor.GetBlockByNonce(
		provider.observedActualShard,
		latestBlockNonce,
		queryOptions,
	)
	if err != nil {
		return nil, newErrCannotGetBlockByNonce(latestBlockNonce, err)
	}

	return &resources.BlockSummary{
		Nonce:             blockResponse.Data.Block.Nonce,
		Hash:              blockResponse.Data.Block.Hash,
		PreviousBlockHash: blockResponse.Data.Block.PrevBlockHash,
		Timestamp:         int64(blockResponse.Data.Block.Timestamp),
	}, nil
}

func (provider *networkProvider) getLatestBlockNonce() (uint64, error) {
	nodeStatus, err := provider.getNodeStatus()
	if err != nil {
		return 0, err
	}

	if provider.onlyFinalBlocks {
		return nodeStatus.HighestFinalNonce, nil
	}

	return nodeStatus.HighestNonce, nil
}

func (provider *networkProvider) getNodeStatus() (*resources.NodeStatus, error) {
	if provider.isOffline {
		return nil, errIsOffline
	}

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
	if provider.isOffline {
		return nil, errIsOffline
	}

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
	if provider.isOffline {
		return nil, errIsOffline
	}

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
func (provider *networkProvider) GetAccount(address string) (*data.AccountModel, error) {
	if provider.isOffline {
		return nil, errIsOffline
	}

	options := common.AccountQueryOptions{
		OnFinalBlock: provider.onlyFinalBlocks,
	}

	account, err := provider.accountProcessor.GetAccount(address, options)
	if err != nil {
		return nil, newErrCannotGetAccount(address, err)
	}

	return account, nil
}

func (provider *networkProvider) IsAddressObserved(address string) (bool, error) {
	pubKey, err := provider.ConvertAddressToPubKey(address)
	if err != nil {
		return false, err
	}

	shard, err := provider.baseProcessor.ComputeShardId(pubKey)
	if err != nil {
		return false, err
	}

	isObservedActualShard := shard == provider.observedActualShard
	isObservedProjectedShard := pubKey[len(pubKey)-1] == byte(provider.observedProjectedShard)

	if provider.observedProjectedShardIsSet {
		return isObservedProjectedShard, nil
	}

	return isObservedActualShard, nil
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
func (provider *networkProvider) SendTransaction(tx *data.Transaction) (string, error) {
	if provider.isOffline {
		return "", errIsOffline
	}

	_, hash, err := provider.transactionProcessor.SendTransaction(tx)
	if err != nil {
		return "", err
	}

	return hash, nil
}

// GetMempoolTransactionByHash gets a transaction from the pool
func (provider *networkProvider) GetMempoolTransactionByHash(hash string) (*data.FullTransaction, error) {
	if provider.isOffline {
		return nil, errIsOffline
	}

	tx, _, err := provider.transactionProcessor.GetTransactionByHashAndSenderAddress(hash, "", false)
	if err != nil {
		return nil, newErrCannotGetTransaction(hash, err)
	}

	if tx.Status == transaction.TxStatusPending {
		return tx, nil
	}

	return nil, nil
}

func (provider *networkProvider) LogDescription() {
	log.Info("Description of network provider",
		"isOffline", provider.isOffline,
		"observerUrl", provider.observerUrl,
		"observedActualShard", provider.observedActualShard,
		"observedProjectedShard", provider.observedProjectedShard,
		"observedProjectedShardIsSet", provider.observedProjectedShardIsSet,
	)
}
