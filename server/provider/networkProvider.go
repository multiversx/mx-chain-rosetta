package provider

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"github.com/ElrondNetwork/elrond-go-core/core"
	"github.com/ElrondNetwork/elrond-go-core/core/pubkeyConverter"
	"github.com/ElrondNetwork/elrond-go-core/data/receipt"
	"github.com/ElrondNetwork/elrond-go-core/data/transaction"
	"github.com/ElrondNetwork/elrond-go-core/hashing"
	hasherFactory "github.com/ElrondNetwork/elrond-go-core/hashing/factory"
	"github.com/ElrondNetwork/elrond-go-core/marshal"
	marshalFactory "github.com/ElrondNetwork/elrond-go-core/marshal/factory"
	logger "github.com/ElrondNetwork/elrond-go-logger"
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
	ObserveNotFinalBlocks       bool
}

type networkProvider struct {
	isOffline bool

	pubKeyConverter      core.PubkeyConverter
	baseProcessor        process.Processor
	accountProcessor     facade.AccountProcessor
	transactionProcessor facade.TransactionProcessor
	blockProcessor       facade.BlockProcessor
	nodeStatusProcessor  facade.NodeStatusProcessor

	hasher                hashing.Hasher
	marshalizerForHashing marshal.Marshalizer

	observedActualShard         uint32
	observedProjectedShard      uint32
	observedProjectedShardIsSet bool
	observerUrl                 string
	observerPubkey              string
	nativeCurrencySymbol        string
	genesisBlockHash            string
	genesisTimestamp            int64
	observeNotFinalBlocks       bool

	networkConfig *resources.NetworkConfig
}

// TODO: Move constructor calls to /factory. Receive dependencies in constructor.
func NewNetworkProvider(args ArgsNewNetworkProvider) (*networkProvider, error) {
	shardCoordinator, err := sharding.NewMultiShardCoordinator(args.NumShards, args.ObservedActualShard)
	if err != nil {
		return nil, err
	}

	pubKeyConverter, err := pubkeyConverter.NewBech32PubkeyConverter(pubKeyLength, log)
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

	hasher, err := hasherFactory.NewHasher(hasherType)
	if err != nil {
		return nil, err
	}

	marshalizerForHashing, err := marshalFactory.NewMarshalizer(marshalizerForHashingType)
	if err != nil {
		return nil, err
	}

	transactionProcessor, err := processFactory.CreateTransactionProcessor(
		baseProcessor,
		pubKeyConverter,
		hasher,
		marshalizerForHashing,
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

		hasher:                hasher,
		marshalizerForHashing: marshalizerForHashing,

		observedActualShard:         args.ObservedActualShard,
		observedProjectedShard:      args.ObservedProjectedShard,
		observedProjectedShardIsSet: args.ObservedProjectedShardIsSet,
		observerUrl:                 args.ObserverUrl,
		observerPubkey:              args.ObserverPubkey,
		nativeCurrencySymbol:        args.NativeCurrencySymbol,
		genesisBlockHash:            args.GenesisBlockHash,
		genesisTimestamp:            args.GenesisTimestamp,
		observeNotFinalBlocks:       args.ObserveNotFinalBlocks,

		networkConfig: &resources.NetworkConfig{
			ChainID:        args.ChainID,
			GasPerDataByte: args.GasPerDataByte,
			MinGasPrice:    args.MinGasPrice,
			MinGasLimit:    args.MinGasLimit,
		},
	}, nil
}

// IsOffline returns whether the network provider is in the "offline" mode (i.e. no connection to the observer)
func (provider *networkProvider) IsOffline() bool {
	return provider.isOffline
}

// GetBlockchainName returns the name of the network ("Elrond")
func (provider *networkProvider) GetBlockchainName() string {
	return resources.BlockchainName
}

// GetChainID gets the chain identifier ("1" for mainnet, "D" for devnet etc.)
func (provider *networkProvider) GetChainID() string {
	return provider.networkConfig.ChainID
}

// GetNativeCurrency gets the native currency (EGLD, 18 decimals)
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

	log.Debug("GetLatestBlockSummary()", "latestBlockNonce", latestBlockNonce)

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

	if provider.observeNotFinalBlocks {
		return nodeStatus.HighestNonce, nil
	}

	return nodeStatus.HighestFinalNonce, nil
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

	latestNonce, err := provider.getLatestBlockNonce()
	if err != nil {
		return nil, err
	}

	if nonce > latestNonce {
		return nil, errCannotGetBlock
	}

	queryOptions := common.BlockQueryOptions{
		WithTransactions: true,
		WithLogs:         true,
	}

	// TODO: check why the proxy library issues more requests (e.g. 4) instead of 1
	response, err := provider.blockProcessor.GetBlockByNonce(provider.observedActualShard, nonce, queryOptions)
	if err != nil {
		return nil, newErrCannotGetBlockByNonce(nonce, err)
	}
	if response.Error != "" {
		return nil, newErrCannotGetBlockByNonce(nonce, errors.New(response.Error))
	}

	block := &response.Data.Block
	provider.simplifyBlockWithScheduledTransactions(block)

	return block, nil
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

	// TODO: check why the proxy library issues more requests (e.g. 4) instead of 1
	response, err := provider.blockProcessor.GetBlockByHash(provider.observedActualShard, hash, queryOptions)
	if err != nil {
		return nil, newErrCannotGetBlockByHash(hash, err)
	}
	if response.Error != "" {
		return nil, newErrCannotGetBlockByHash(hash, errors.New(response.Error))
	}

	block := &response.Data.Block
	provider.simplifyBlockWithScheduledTransactions(block)

	return block, nil
}

// GetAccount gets an account by address
func (provider *networkProvider) GetAccount(address string) (*data.AccountModel, error) {
	if provider.isOffline {
		return nil, errIsOffline
	}

	onFinalBlock := !provider.observeNotFinalBlocks
	options := common.AccountQueryOptions{OnFinalBlock: onFinalBlock}
	account, err := provider.accountProcessor.GetAccount(address, options)
	if err != nil {
		return nil, newErrCannotGetAccount(address, err)
	}

	log.Trace(fmt.Sprintf("GetAccount(onFinal=%t)", onFinalBlock),
		"address", account.Account.Address,
		"balance", account.Account.Balance,
		"block", account.BlockInfo.Nonce,
		"blockHash", account.BlockInfo.Hash,
		"blockRootHash", account.BlockInfo.RootHash,
	)

	return account, nil
}

// IsAddressObserved returns whether the address is observed (i.e. is located in an observed shard)
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

func (provider *networkProvider) ComputeReceiptHash(apiReceipt *transaction.ApiReceipt) (string, error) {
	txHash, err := hex.DecodeString(apiReceipt.TxHash)
	if err != nil {
		return "", err
	}

	senderPubkey, err := provider.ConvertAddressToPubKey(apiReceipt.SndAddr)
	if err != nil {
		return "", err
	}

	receipt := &receipt.Receipt{
		TxHash:  txHash,
		SndAddr: senderPubkey,
		Value:   apiReceipt.Value,
		Data:    []byte(apiReceipt.Data),
	}

	receiptHash, err := core.CalculateHash(provider.marshalizerForHashing, provider.hasher, receipt)
	if err != nil {
		return "", err
	}

	receiptHashHex := hex.EncodeToString(receiptHash)
	return receiptHashHex, nil
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

func (provider *networkProvider) ComputeTransactionFeeForMoveBalance(tx *data.FullTransaction) *big.Int {
	minGasLimit := provider.networkConfig.MinGasLimit
	gasPerDataByte := provider.networkConfig.GasPerDataByte
	gasLimit := minGasLimit + gasPerDataByte*uint64(len(tx.Data))

	fee := core.SafeMul(gasLimit, tx.GasPrice)
	return fee
}

// LogDescription writes a description of the network provider in the log output
func (provider *networkProvider) LogDescription() {
	log.Info("Description of network provider",
		"isOffline", provider.isOffline,
		"observerUrl", provider.observerUrl,
		"observedActualShard", provider.observedActualShard,
		"observedProjectedShard", provider.observedProjectedShard,
		"observedProjectedShardIsSet", provider.observedProjectedShardIsSet,
		"observeNotFinalBlocks", provider.observeNotFinalBlocks,
		"nativeCurrency", provider.nativeCurrencySymbol,
	)
}
