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

	log = logger.GetOrCreate("rosetta")

	cliFlagPort = cli.IntFlag{
		Name:  "port",
		Usage: "Specifies the TCP port used by Rosetta endpoints.",
		Value: 9090,
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
		Value: "*:" + logger.LogInfo.String(),
	}

	cliFlagLogsFolder = cli.StringFlag{
		Name:  "logs-folder",
		Usage: "Specifies where to save the log files.",
		Value: "",
	}

	cliFlagObserveActualShard = cli.UintFlag{
		Name:  "observe-actual-shard",
		Usage: "Specifies the actual shard to observe.",
		Value: 0,
	}

	cliFlagObserveProjectedShard = cli.UintFlag{
		Name:  "observe-projected-shard",
		Usage: "Specifies the projected shard to observe.",
		Value: 0,
	}

	cliFlagObserver = cli.StringFlag{
		Name:  "observer",
		Usage: "Specifies the URL of the observer.",
		Value: "http://localhost:10100",
	}

	cliFlagChainID = cli.StringFlag{
		Name:  "chain-id",
		Usage: "Specifies the Chain ID (the parameter is necessary when constructing transactions in offline mode).",
		Value: "local-testnet",
	}

	cliFlagNumShards = cli.UintFlag{
		Name:  "num-shards",
		Usage: "Specifies the total number of actual network shards (with the exception of the metachain). Must be 3 for mainnet.",
		Value: 1,
	}

	cliFlagGenesisBlock = cli.StringFlag{
		Name:  "genesis-block",
		Usage: "Specifies the hash of the genesis block, to be returned by network/status. For mainnet, it must be 0xcd229e4ad2753708e4bab01d7f249affe29441829524c9529e84d51b6d12f2a7.",
		Value: "0x0000000000000000000000000000000000000000000000000000000000000000",
	}

	cliFlagMinGasPrice = cli.Uint64Flag{
		Name:  "min-gas-price",
		Usage: "Specifies the minimum gas price (the parameter is necessary when constructing transactions in offline mode).",
		Value: 1000000000,
	}

	cliFlagMinGasLimit = cli.UintFlag{
		Name:  "min-gas-limit",
		Usage: "Specifies the minimum gas limit (the parameter is necessary when constructing transactions in offline mode).",
		Value: 1,
	}

	cliFlagNativeCurrencySymbol = cli.StringFlag{
		Name:  "native-currency",
		Usage: "Specifies the symbol of the native currency (must be EGLD for mainnet, XeGLD for testnet and devnet).",
		Value: "XeGLD",
	}
)

func getAllCliFlags() []cli.Flag {
	return []cli.Flag{
		cliFlagPort,
		cliFlagOffline,
		cliFlagLogLevel,
		cliFlagLogsFolder,
		cliFlagObserveActualShard,
		cliFlagObserveProjectedShard,
		cliFlagObserver,
		cliFlagChainID,
		cliFlagNumShards,
		cliFlagGenesisBlock,
		cliFlagMinGasPrice,
		cliFlagMinGasLimit,
		cliFlagNativeCurrencySymbol,
	}
}

type parsedCliFlags struct {
	port                       int
	offline                    bool
	logLevel                   string
	logsFolder                 string
	observeActualShard         uint32
	observeProjectedShard      uint32
	observeProjectedShardIsSet bool
	observer                   string
	chainID                    string
	numShards                  uint32
	genesisBlock               string
	minGasPrice                uint64
	minGasLimit                uint64
	nativeCurrencySymbol       string
}

func getParsedCliFlags(ctx *cli.Context) parsedCliFlags {
	return parsedCliFlags{
		port:                       ctx.GlobalInt(cliFlagPort.Name),
		offline:                    ctx.GlobalBool(cliFlagOffline.Name),
		logLevel:                   ctx.GlobalString(cliFlagLogLevel.Name),
		logsFolder:                 ctx.GlobalString(cliFlagLogsFolder.Name),
		observeActualShard:         uint32(ctx.GlobalUint(cliFlagObserveActualShard.Name)),
		observeProjectedShard:      uint32(ctx.GlobalUint(cliFlagObserveProjectedShard.Name)),
		observeProjectedShardIsSet: ctx.GlobalIsSet(cliFlagObserveProjectedShard.Name),
		observer:                   ctx.GlobalString(cliFlagObserver.Name),
		chainID:                    ctx.GlobalString(cliFlagChainID.Name),
		numShards:                  uint32(ctx.GlobalUint(cliFlagNumShards.Name)),
		genesisBlock:               ctx.GlobalString(cliFlagGenesisBlock.Name),
		minGasPrice:                ctx.GlobalUint64(cliFlagMinGasPrice.Name),
		minGasLimit:                ctx.GlobalUint64(cliFlagMinGasLimit.Name),
		nativeCurrencySymbol:       ctx.GlobalString(cliFlagNativeCurrencySymbol.Name),
	}
}
