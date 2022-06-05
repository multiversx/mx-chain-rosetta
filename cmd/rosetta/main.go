package main

import (
	"fmt"
	"os"
	"time"

	logger "github.com/ElrondNetwork/elrond-go-logger"
	"github.com/ElrondNetwork/elrond-go/core/pubkeyConverter"
	hasherFactory "github.com/ElrondNetwork/elrond-go/hashing/factory"
	marshalFactory "github.com/ElrondNetwork/elrond-go/marshal/factory"
	"github.com/ElrondNetwork/elrond-go/sharding"
	"github.com/ElrondNetwork/elrond-proxy-go/common"
	"github.com/ElrondNetwork/elrond-proxy-go/data"
	"github.com/ElrondNetwork/elrond-proxy-go/observer"
	"github.com/ElrondNetwork/elrond-proxy-go/process"
	"github.com/ElrondNetwork/elrond-proxy-go/process/cache"
	processFactory "github.com/ElrondNetwork/elrond-proxy-go/process/factory"
	"github.com/urfave/cli"
)

const (
	defaultLogsPath      = "logs"
	logFilePrefix        = "rosetta"
	logFileLifeSpanInSec = 86400
)

var (
	helpTemplate = `NAME:
   {{.Name}} - {{.Usage}}
USAGE:
   {{.HelpName}} {{if .VisibleFlags}}[global options]{{end}}
   {{if len .Authors}}
AUTHOR:
   {{range .Authors}}{{ . }}{{end}}
   {{end}}{{if .Commands}}
GLOBAL OPTIONS:
   {{range .VisibleFlags}}{{.}}
   {{end}}
VERSION:
   {{.Version}}
   {{end}}
`

	log = logger.GetOrCreate("rosetta")

	cliFlagOffline = cli.BoolFlag{
		Name:  "offline",
		Usage: "Starts in offline mode",
	}

	cliParamLogLevel = cli.StringFlag{
		Name: "log-level",
		Usage: "Specifies the logger `level(s)`. It can contain multiple comma-separated value. For example" +
			", if set to *:INFO the logs for all packages will have the INFO level. However, if set to *:INFO,api:DEBUG" +
			" the logs for all packages will have the INFO level, excepting the api package which will receive a DEBUG" +
			" log level.",
		Value: "*:" + logger.LogInfo.String(),
	}

	cliParamLogsFolder = cli.StringFlag{
		Name:  "logs-folder",
		Usage: "Specifies where to save the log files.",
		Value: "",
	}

	cliParamObserveActualShard = cli.UintFlag{
		Name:  "observe-actual-shard",
		Usage: "Specifies the actual shard to observe.",
		Value: 0,
	}

	cliParamObserveProjectedShard = cli.UintFlag{
		Name:  "observe-projected-shard",
		Usage: "Specifies the projected shard to observe.",
		Value: 0,
	}

	cliParamObserver = cli.StringFlag{
		Name:  "observer",
		Usage: "Specifies the URL of the observer.",
		Value: "http://localhost:10100",
	}

	cliParamChainID = cli.StringFlag{
		Name:  "chain-id",
		Usage: "Specifies the Chain ID (the parameter is necessary when constructing transactions in offline mode).",
		Value: "local-testnet",
	}

	cliParamGenesisBlock = cli.StringFlag{
		Name:  "genesis-block",
		Usage: "Specifies the hash of the genesis block, to be returned by network/status. For mainnet, it must be 0xcd229e4ad2753708e4bab01d7f249affe29441829524c9529e84d51b6d12f2a7.",
		Value: "0x0000000000000000000000000000000000000000000000000000000000000000",
	}

	cliParamMinGasPrice = cli.Uint64Flag{
		Name:  "min-gas-price",
		Usage: "Specifies the minimum gas price (the parameter is necessary when constructing transactions in offline mode).",
		Value: 1000000000,
	}

	cliParamMinGasLimit = cli.UintFlag{
		Name:  "min-gas-limit",
		Usage: "Specifies the minimum gas limit (the parameter is necessary when constructing transactions in offline mode).",
		Value: 50000,
	}

	cliNativeCurrencySymbol = cli.StringFlag{
		Name:  "native-currency",
		Usage: "Specifies the symbol of the native currency (must be EGLD for mainnet, XeGLD for testnet and devnet).",
		Value: "XeGLD",
	}
)

func main() {
	app := cli.NewApp()
	cli.AppHelpTemplate = helpTemplate
	app.Name = "Elrond Rosetta CLI App"
	app.Version = "v1.0.0"
	app.Usage = "This is the entry point for starting a new Elrond Rosetta instance"
	app.Flags = []cli.Flag{
		cliFlagOffline,
		cliParamLogLevel,
		cliParamLogsFolder,
		cliParamObserveActualShard,
		cliParamObserveProjectedShard,
		cliParamObserver,
		cliParamChainID,
		cliParamMinGasPrice,
		cliParamMinGasLimit,
	}
	app.Authors = []cli.Author{
		{
			Name:  "The Elrond Team",
			Email: "contact@elrond.com",
		},
	}

	app.Action = startRosetta

	err := app.Run(os.Args)
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}
}

func startRosetta(ctx *cli.Context) error {
	fileLogging, err := initializeLogger(ctx)
	if err != nil {
		return err
	}

	log.Info("Starting Rosetta...")

	pubKeyConverter, err := pubkeyConverter.NewBech32PubkeyConverter(pubKeyLength)
	if err != nil {
		return err
	}

	observerUrl := cliParamObserver.Value
	observersProvider, err := observer.NewSimpleNodesProvider([]*data.NodeData{{ShardId: 0, Address: observerUrl, IsSynced: true}}, "")
	if err != nil {
		return err
	}

	disabledObserversProvider := observer.NewDisabledNodesProvider("full history endpoints are not applicable")
	if err != nil {
		return err
	}

	shardCoordinator, err := sharding.NewMultiShardCoordinator(1, 0)
	if err != nil {
		return err
	}

	baseProcessor, err := process.NewBaseProcessor(
		requestTimeoutInSeconds,
		shardCoordinator,
		observersProvider,
		disabledObserversProvider,
		pubKeyConverter,
	)
	if err != nil {
		return err
	}

	accountProcessor, err := process.NewAccountProcessor(baseProcessor, pubKeyConverter, &disabledExternalStorageConnector{})
	if err != nil {
		return err
	}

	account, err := accountProcessor.GetAccount("erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th")
	if err != nil {
		return err
	}

	fmt.Println(account.Balance)

	transactionHasher, err := hasherFactory.NewHasher(transactionsHasherType)
	if err != nil {
		return err
	}

	transactionMarshalizer, err := marshalFactory.NewMarshalizer(transactionsMarshalizerType)

	transactionProcessor, err := processFactory.CreateTransactionProcessor(
		baseProcessor,
		pubKeyConverter,
		transactionHasher,
		transactionMarshalizer,
		"0",
		"0",
	)
	if err != nil {
		return err
	}

	tx, err := transactionProcessor.GetTransaction("5be93498d366ab14a6794c5c5661c06e70cfef2fbfbd460911c6c924703594ef", true)
	if err != nil {
		return err
	}
	fmt.Println(tx.GasLimit)

	blockProcessor, err := process.NewBlockProcessor(&disabledExternalStorageConnector{}, baseProcessor)
	if err != nil {
		return err
	}

	block, err := blockProcessor.GetBlockByNonce(0, 3, common.BlockQueryOptions{WithTransactions: true})
	if err != nil {
		return err
	}

	fmt.Println(block.Data.Block.Round)

	economicMetricsCacher := cache.NewGenericApiResponseMemoryCacher()
	nodeStatusProcessor, err := process.NewNodeStatusProcessor(baseProcessor, economicMetricsCacher, time.Duration(1*time.Second))
	if err != nil {
		return err
	}

	networkConfig, err := nodeStatusProcessor.GetNetworkConfigMetrics()
	if err != nil {
		return err
	}

	fmt.Println(networkConfig.Data)

	// httpServer, err := startWebServer(versionsRegistry, ctx, generalConfig, *credentialsConfig, statusMetricsProvider, isProfileModeActivated)
	// if err != nil {
	// 	return err
	// }

	// waitForServerShutdown(httpServer, closableComponents)

	// log.Debug("closing proxy")

	_ = fileLogging.Close()

	return nil
}

// func loadMainConfig(filepath string) (*config.Config, error) {
// 	cfg := &config.Config{}
// 	err := core.LoadTomlFile(cfg, filepath)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return cfg, nil
// }

// func createVersionsRegistry(
// 	cfg *config.Config,
// 	configurationFilePath string,
// 	ecConf *erdConfig.EconomicsConfig,
// 	exCfg *erdConfig.ExternalConfig,
// 	statusMetricsHandler data.StatusMetricsProvider,
// 	pemFileLocation string,
// 	apiConfigDirectoryPath string,
// 	closableComponents *data.ClosableComponentsHandler,
// 	isRosettaModeEnabled bool,
// ) (data.VersionsRegistryHandler, error) {
// 	pubKeyConverter, err := factory.NewPubkeyConverter(cfg.AddressPubkeyConverter)
// 	if err != nil {
// 		return nil, err
// 	}

// 	marshalizer, err := marshalFactory.NewMarshalizer(cfg.Marshalizer.Type)
// 	if err != nil {
// 		return nil, err
// 	}
// 	hasher, err := hasherFactory.NewHasher(cfg.Hasher.Type)
// 	if err != nil {
// 		return nil, err
// 	}

// 	shardCoord, err := getShardCoordinator(cfg)
// 	if err != nil {
// 		return nil, err
// 	}

// 	nodesProviderFactory, err := observer.NewNodesProviderFactory(*cfg, configurationFilePath)
// 	if err != nil {
// 		return nil, err
// 	}

// 	observersProvider, err := nodesProviderFactory.CreateObservers()
// 	if err != nil {
// 		return nil, err
// 	}

// 	fullHistoryNodesProvider, err := nodesProviderFactory.CreateFullHistoryNodes()
// 	if err != nil {
// 		if err != observer.ErrEmptyObserversList {
// 			return nil, err
// 		}
// 	}

// 	bp, err := process.NewBaseProcessor(
// 		cfg.GeneralSettings.RequestTimeoutSec,
// 		shardCoord,
// 		observersProvider,
// 		fullHistoryNodesProvider,
// 		pubKeyConverter,
// 	)
// 	if err != nil {
// 		return nil, err
// 	}
// 	bp.StartNodesSyncStateChecks()

// 	connector, err := createElasticSearchConnector(exCfg)
// 	if err != nil {
// 		return nil, err
// 	}

// 	accntProc, err := process.NewAccountProcessor(bp, pubKeyConverter, connector)
// 	if err != nil {
// 		return nil, err
// 	}

// 	txProc, err := processFactory.CreateTransactionProcessor(
// 		bp,
// 		pubKeyConverter,
// 		hasher,
// 		marshalizer,
// 		ecConf.FeeSettings.MaxGasLimitPerBlock,
// 		ecConf.FeeSettings.MaxGasLimitPerMetaBlock,
// 	)
// 	if err != nil {
// 		return nil, err
// 	}

// 	nodeStatusProc, err := process.NewNodeStatusProcessor(bp, economicMetricsCacher, cacheValidity)
// 	if err != nil {
// 		return nil, err
// 	}

// 	closableComponents.Add(htbProc, valStatsProc, nodeStatusProc, bp)

// 	blockProc, err := process.NewBlockProcessor(connector, bp)
// 	if err != nil {
// 		return nil, err
// 	}

// 	blocksPrc, err := process.NewBlocksProcessor(bp)
// 	if err != nil {
// 		return nil, err
// 	}

// 	proofProc, err := process.NewProofProcessor(bp, pubKeyConverter)
// 	if err != nil {
// 		return nil, err
// 	}

// 	esdtSuppliesProc, err := process.NewESDTSupplyProcessor(bp, scQueryProc)
// 	if err != nil {
// 		return nil, err
// 	}

// 	statusProc, err := process.NewStatusProcessor(bp, statusMetricsHandler)
// 	if err != nil {
// 		return nil, err
// 	}

// }

// func getShardCoordinator(cfg *config.Config) (sharding.Coordinator, error) {
// 	maxShardID := uint32(0)
// 	for _, obs := range cfg.Observers {
// 		shardID := obs.ShardId
// 		isMetaChain := shardID == core.MetachainShardId
// 		if maxShardID < shardID && !isMetaChain {
// 			maxShardID = shardID
// 		}
// 	}

// 	shardCoordinator, err := sharding.NewMultiShardCoordinator(maxShardID+1, 0)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return shardCoordinator, nil
// }

// func startWebServer(
// 	versionsRegistry data.VersionsRegistryHandler,
// 	cliContext *cli.Context,
// 	generalConfig *config.Config,
// 	credentialsConfig config.CredentialsConfig,
// 	statusMetricsProvider data.StatusMetricsProvider,
// 	isProfileModeActivated bool,
// ) (*http.Server, error) {
// 	var err error
// 	var httpServer *http.Server

// 	port := generalConfig.GeneralSettings.ServerPort
// 	asRosetta := cliContext.GlobalBool(startAsRosetta.Name)
// 	if asRosetta {
// 		isRosettaOffline := cliContext.GlobalBool(rosettaOffline.Name)
// 		offlineConfigPath := cliContext.GlobalString(rosettaOfflineConfig.Name)
// 		var facades map[string]*data.VersionData
// 		facades, err = versionsRegistry.GetAllVersions()
// 		if err != nil {
// 			return nil, err
// 		}
// 		httpServer, err = rosetta.CreateServer(facades["v1.0"].Facade, generalConfig, port, isRosettaOffline, offlineConfigPath)
// 	} else {
// 		if generalConfig.GeneralSettings.RateLimitWindowDurationSeconds <= 0 {
// 			return nil, fmt.Errorf("invalid value %d for RateLimitWindowDurationSeconds. It must be greater "+
// 				"than zero", generalConfig.GeneralSettings.RateLimitWindowDurationSeconds)
// 		}
// 		httpServer, err = api.CreateServer(
// 			versionsRegistry,
// 			port,
// 			generalConfig.ApiLogging,
// 			credentialsConfig,
// 			statusMetricsProvider,
// 			generalConfig.GeneralSettings.RateLimitWindowDurationSeconds,
// 			isProfileModeActivated,
// 		)
// 	}
// 	if err != nil {
// 		return nil, err
// 	}
// 	go func() {
// 		err = httpServer.ListenAndServe()
// 		if err != nil {
// 			log.Error("cannot ListenAndServe()", "err", err)
// 			os.Exit(1)
// 		}
// 	}()

// 	return httpServer, nil
// }

// func waitForServerShutdown(httpServer *http.Server, closableComponents *data.ClosableComponentsHandler) {
// 	quit := make(chan os.Signal)
// 	signal.Notify(quit, os.Interrupt, os.Kill)
// 	<-quit

// 	closableComponents.Close()

// 	shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
// 	defer cancel()
// 	_ = httpServer.Shutdown(shutdownContext)
// 	_ = httpServer.Close()
// }
