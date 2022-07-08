package factory

import (
	"time"

	"github.com/ElrondNetwork/elrond-go-core/core/pubkeyConverter"
	hasherFactory "github.com/ElrondNetwork/elrond-go-core/hashing/factory"
	marshalFactory "github.com/ElrondNetwork/elrond-go-core/marshal/factory"
	"github.com/ElrondNetwork/elrond-go/sharding"
	"github.com/ElrondNetwork/elrond-proxy-go/data"
	"github.com/ElrondNetwork/elrond-proxy-go/observer"
	"github.com/ElrondNetwork/elrond-proxy-go/process"
	processFactory "github.com/ElrondNetwork/elrond-proxy-go/process/factory"
	"github.com/ElrondNetwork/rosetta/server/provider"
)

const (
	hasherType                = "blake2b"
	marshalizerForHashingType = "gogo protobuf"
	pubKeyLength              = 32

	notApplicableConfigurationFilePath   = "not applicable"
	notApplicableFullHistoryNodesMessage = "not applicable"

	requestTimeoutInSeconds = 60
)

var (
	nodeStatusCacheDuration = time.Duration(1 * time.Second)
)

type ArgsCreateNetworkProvider struct {
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

func CreateNetworkProvider(args ArgsCreateNetworkProvider) (networkProvider, error) {
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

	return provider.NewNetworkProvider(provider.ArgsNewNetworkProvider{
		IsOffline:                   args.IsOffline,
		ObservedActualShard:         args.ObservedActualShard,
		ObservedProjectedShard:      args.ObservedProjectedShard,
		ObservedProjectedShardIsSet: args.ObservedProjectedShardIsSet,
		ObserverUrl:                 args.ObserverUrl,
		ObserverPubkey:              args.ObserverPubkey,
		ChainID:                     args.ChainID,
		GasPerDataByte:              args.GasPerDataByte,
		MinGasPrice:                 args.MinGasPrice,
		MinGasLimit:                 args.MinGasLimit,
		NativeCurrencySymbol:        args.NativeCurrencySymbol,
		GenesisBlockHash:            args.GenesisBlockHash,
		GenesisTimestamp:            args.GenesisTimestamp,
		ObserveNotFinalBlocks:       args.ObserveNotFinalBlocks,

		BaseProcessor:        baseProcessor,
		AccountProcessor:     accountProcessor,
		TransactionProcessor: transactionProcessor,
		BlockProcessor:       blockProcessor,

		Hasher:                hasher,
		MarshalizerForHashing: marshalizerForHashing,
		PubKeyConverter:       pubKeyConverter,
	})
}
