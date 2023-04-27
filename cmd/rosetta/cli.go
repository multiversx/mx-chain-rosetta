package main

import (
	logger "github.com/ElrondNetwork/elrond-go-logger"
	"github.com/urfave/cli"
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

	cliFlagPort = cli.IntFlag{
		Name:  "port",
		Usage: "Specifies the TCP port used by Rosetta endpoints.",
		Value: 8091,
	}

	cliFlagOffline = cli.BoolFlag{
		Name:  "offline",
		Usage: "Starts in offline mode",
	}

	cliFlagLogLevel = cli.StringFlag{
		Name: "log-level",
		Usage: "Specifies the logger `level(s)`. It can contain multiple comma-separated value. For example" +
			", if set to *:INFO the logs for all packages will have the INFO level. However, if set to *:INFO,api:DEBUG" +
			" the logs for all packages will have the INFO level, excepting the api package which will receive a DEBUG" +
			" log level.",
		Value: "*:" + logger.LogDebug.String(),
	}

	cliFlagLogsFolder = cli.StringFlag{
		Name:  "logs-folder",
		Usage: "Specifies where to save the log files.",
		Value: "",
	}

	cliFlagObserverActualShard = cli.UintFlag{
		Name:  "observer-actual-shard",
		Usage: "Specifies the actual shard to observe.",
		Value: 0,
	}

	cliFlagObserverProjectedShard = cli.UintFlag{
		Name:  "observer-projected-shard",
		Usage: "Specifies the projected shard to observe.",
		Value: 0,
	}

	cliFlagObserverHttpUrl = cli.StringFlag{
		Name:  "observer-http-url",
		Usage: "Specifies the URL of the observer.",
		Value: "http://nowhere.localhost.local",
	}

	cliFlagBlockchainName = cli.StringFlag{
		Name:  "blockchain",
		Usage: "Specifies the blockchain name (e.g. MultiversX).",
		Value: "MultiversX",
	}

	cliFlagNetworkID = cli.StringFlag{
		Name:     "network-id",
		Usage:    "Specifies the network ID (e.g. 1, D, T)",
		Required: true,
	}

	cliFlagNetworkName = cli.StringFlag{
		Name:     "network-name",
		Usage:    "Specifies the network name (e.g. mainnet, devnet, testnet).",
		Required: true,
	}

	cliFlagNumShards = cli.UintFlag{
		Name:  "num-shards",
		Usage: "Specifies the total number of actual network shards (with the exception of the metachain). Must be 3 for mainnet.",
		Value: 3,
	}

	cliFlagGenesisBlock = cli.StringFlag{
		Name:  "genesis-block",
		Usage: "Specifies the hash of the genesis block, to be returned by network/status. For mainnet, it must be cd229e4ad2753708e4bab01d7f249affe29441829524c9529e84d51b6d12f2a7.",
		Value: "cd229e4ad2753708e4bab01d7f249affe29441829524c9529e84d51b6d12f2a7",
	}

	cliFlagGenesisTimestamp = cli.Int64Flag{
		Name:  "genesis-timestamp",
		Usage: "Specifies the timestamp of the genesis block. For mainnet, it must be 1596117600 (Thursday, July 30, 2020 14:00:00 UTC).",
		Value: 1596117600,
	}

	cliFlagMinGasPrice = cli.Uint64Flag{
		Name:  "min-gas-price",
		Usage: "Specifies the minimum gas price (required in offline mode).",
		Value: 1000000000,
	}

	cliFlagMinGasLimit = cli.UintFlag{
		Name:  "min-gas-limit",
		Usage: "Specifies the minimum gas limit (required in offline mode).",
		Value: 50000,
	}

	cliFlagGasPerDataByte = cli.UintFlag{
		Name:  "gas-per-data-byte",
		Usage: "Specifies the gas required per data byte (required in offline mode).",
		Value: 1500,
	}

	cliFlagNativeCurrencySymbol = cli.StringFlag{
		Name:  "native-currency",
		Usage: "Specifies the symbol of the native currency (must be EGLD for mainnet, XeGLD for testnet and devnet).",
		Value: "EGLD",
	}

	cliFlagFirstHistoricalEpoch = cli.UintFlag{
		Name:     "first-historical-epoch",
		Usage:    "Specifies the first epoch with historical data available in Observer's database.",
		Required: true,
	}

	cliFlagNumHistoricalEpochs = cli.UintFlag{
		Name:     "num-historical-epochs",
		Usage:    "Provides a hint for the number of historical epochs to be kept.",
		Required: true,
	}

	cliFlagCustomCurrenciesSymbols = cli.StringSliceFlag{
		Name:  "custom-currencies",
		Usage: "Specifies the symbols of enabled custom currencies (i.e. ESDT identifiers).",
		Value: &cli.StringSlice{},
	}
)

func getAllCliFlags() []cli.Flag {
	return []cli.Flag{
		cliFlagPort,
		cliFlagOffline,
		cliFlagLogLevel,
		cliFlagLogsFolder,
		cliFlagObserverActualShard,
		cliFlagObserverProjectedShard,
		cliFlagObserverHttpUrl,
		cliFlagBlockchainName,
		cliFlagNetworkID,
		cliFlagNetworkName,
		cliFlagNumShards,
		cliFlagGenesisBlock,
		cliFlagGenesisTimestamp,
		cliFlagMinGasPrice,
		cliFlagMinGasLimit,
		cliFlagGasPerDataByte,
		cliFlagNativeCurrencySymbol,
		cliFlagFirstHistoricalEpoch,
		cliFlagNumHistoricalEpochs,
		cliFlagCustomCurrenciesSymbols,
	}
}

type parsedCliFlags struct {
	port                        int
	offline                     bool
	logLevel                    string
	logsFolder                  string
	observerActualShard         uint32
	observerProjectedShard      uint32
	observerProjectedShardIsSet bool
	observerHttpUrl             string
	blockchainName              string
	networkID                   string
	networkName                 string
	numShards                   uint32
	genesisBlock                string
	genesisTimestamp            int64
	minGasPrice                 uint64
	minGasLimit                 uint64
	gasPerDataByte              uint64
	nativeCurrencySymbol        string
	firstHistoricalEpoch        uint32
	numHistoricalEpochs         uint32
	customCurrenciesSymbols     []string
}

func getParsedCliFlags(ctx *cli.Context) parsedCliFlags {
	return parsedCliFlags{
		port:                        ctx.GlobalInt(cliFlagPort.Name),
		offline:                     ctx.GlobalBool(cliFlagOffline.Name),
		logLevel:                    ctx.GlobalString(cliFlagLogLevel.Name),
		logsFolder:                  ctx.GlobalString(cliFlagLogsFolder.Name),
		observerActualShard:         uint32(ctx.GlobalUint(cliFlagObserverActualShard.Name)),
		observerProjectedShard:      uint32(ctx.GlobalUint(cliFlagObserverProjectedShard.Name)),
		observerProjectedShardIsSet: ctx.GlobalIsSet(cliFlagObserverProjectedShard.Name),
		observerHttpUrl:             ctx.GlobalString(cliFlagObserverHttpUrl.Name),
		blockchainName:              ctx.GlobalString(cliFlagBlockchainName.Name),
		networkID:                   ctx.GlobalString(cliFlagNetworkID.Name),
		networkName:                 ctx.GlobalString(cliFlagNetworkName.Name),
		numShards:                   uint32(ctx.GlobalUint(cliFlagNumShards.Name)),
		genesisBlock:                ctx.GlobalString(cliFlagGenesisBlock.Name),
		genesisTimestamp:            ctx.GlobalInt64(cliFlagGenesisTimestamp.Name),
		minGasPrice:                 ctx.GlobalUint64(cliFlagMinGasPrice.Name),
		minGasLimit:                 ctx.GlobalUint64(cliFlagMinGasLimit.Name),
		gasPerDataByte:              ctx.GlobalUint64(cliFlagGasPerDataByte.Name),
		nativeCurrencySymbol:        ctx.GlobalString(cliFlagNativeCurrencySymbol.Name),
		firstHistoricalEpoch:        uint32(ctx.GlobalUint(cliFlagFirstHistoricalEpoch.Name)),
		numHistoricalEpochs:         uint32(ctx.GlobalUint(cliFlagNumHistoricalEpochs.Name)),
		customCurrenciesSymbols:     ctx.GlobalStringSlice(cliFlagCustomCurrenciesSymbols.Name),
	}
}
