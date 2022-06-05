package provider

import (
	"github.com/ElrondNetwork/elrond-go/core/pubkeyConverter"
	hasherFactory "github.com/ElrondNetwork/elrond-go/hashing/factory"
	marshalFactory "github.com/ElrondNetwork/elrond-go/marshal/factory"
	"github.com/ElrondNetwork/elrond-go/sharding"
	"github.com/ElrondNetwork/elrond-proxy-go/data"
	"github.com/ElrondNetwork/elrond-proxy-go/facade"
	"github.com/ElrondNetwork/elrond-proxy-go/observer"
	"github.com/ElrondNetwork/elrond-proxy-go/process"
	"github.com/ElrondNetwork/elrond-proxy-go/process/cache"
	processFactory "github.com/ElrondNetwork/elrond-proxy-go/process/factory"
)

var (
	notApplicableConfigurationFilePath   = "not applicable"
	notApplicableFullHistoryNodesMessage = "not applicable"
	notApplicableMaxGasLimitPerBlock     = "0"
)

type ArgsNewNetworkProvider struct {
	NumShards                   uint32
	ObservedActualShard         uint32
	ObservedProjectedShard      uint32
	ObservedProjectedShardIsSet bool
	ObserverUrl                 string
}

type networkProvider struct {
	baseProcessor        process.Processor
	accountProcessor     facade.AccountProcessor
	transactionProcessor facade.TransactionProcessor
	blockProcessor       facade.BlockProcessor
	nodeStatusProcessor  facade.NodeStatusProcessor
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
		baseProcessor:        baseProcessor,
		accountProcessor:     accountProcessor,
		transactionProcessor: transactionProcessor,
		blockProcessor:       blockProcessor,
		nodeStatusProcessor:  nodeStatusProcessor,
	}, nil
}

func (provider *networkProvider) SynchronizeNetworkConfig() {

}

// type NetworkProviderHandler interface {
// 	GetNetworkConfig() (*NetworkConfig, error)
// 	GetLatestBlockData() (*BlockData, error)
// 	GetBlockByNonce(nonce int64) (*data.Hyperblock, error)
// 	GetBlockByHash(hash string) (*data.Hyperblock, error)
// 	GetAccount(address string) (*data.Account, error)
// 	EncodeAddress(address []byte) (string, error)
// 	DecodeAddress(address string) ([]byte, error)
// 	SendTx(tx *data.Transaction) (string, error)
// 	ComputeTransactionHash(tx *data.Transaction) (string, error)
// 	GetTransactionByHashFromPool(txHash string) (*data.FullTransaction, bool)
// }
