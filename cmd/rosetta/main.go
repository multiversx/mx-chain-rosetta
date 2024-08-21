package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/multiversx/mx-chain-rosetta/server/factory"
	"github.com/multiversx/mx-chain-rosetta/version"
	"github.com/urfave/cli"
)

func main() {
	// This is a workaround, so that TCP sockets opened by rosetta (as a TCP client) to read data from the observer can be reused more easily,
	// once their TCP TIME_WAIT expires.
	// The default value would have been very small (e.g. 2), thus forcing the TCP client (rosetta)
	// to << read, CLOSING, TIME_WAIT, CLOSED >>, without any reuse - thus easily exhausting all available sockets in a short amount of time,
	// under heavy load.
	// References: https://github.com/golang/go/issues/16012
	http.DefaultTransport.(*http.Transport).MaxIdleConnsPerHost = 512

	app := cli.NewApp()
	cli.AppHelpTemplate = helpTemplate
	app.Name = "MultiversX Rosetta CLI App"
	app.Version = version.RosettaMiddlewareVersion
	app.Usage = "This is the entry point for starting a new MultiversX Rosetta instance"
	app.Flags = getAllCliFlags()
	app.Authors = []cli.Author{
		{
			Name:  "The MultiversX Team",
			Email: "contact@multiversx.com",
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
	cliFlags := getParsedCliFlags(ctx)

	fileLogging, err := initializeLogger(cliFlags.logsFolder, cliFlags.logLevel)
	if err != nil {
		return err
	}

	customCurrencies, err := decideCustomCurrencies(cliFlags.configFileCustomCurrencies)
	if err != nil {
		return err
	}

	log.Info("Starting Rosetta...", "middleware", version.RosettaMiddlewareVersion, "specification", version.RosettaVersion)

	networkProvider, err := factory.CreateNetworkProvider(factory.ArgsCreateNetworkProvider{
		IsOffline:                   cliFlags.offline,
		NumShards:                   cliFlags.numShards,
		ObservedActualShard:         cliFlags.observerActualShard,
		ObservedProjectedShard:      cliFlags.observerProjectedShard,
		ObservedProjectedShardIsSet: cliFlags.observerProjectedShardIsSet,
		ObserverUrl:                 cliFlags.observerHttpUrl,
		BlockchainName:              cliFlags.blockchainName,
		NetworkID:                   cliFlags.networkID,
		NetworkName:                 cliFlags.networkName,
		GasPerDataByte:              cliFlags.gasPerDataByte,
		GasPriceModifier:            cliFlags.gasPriceModifier,
		GasLimitCustomTransfer:      cliFlags.gasLimitCustomTransfer,
		MinGasPrice:                 cliFlags.minGasPrice,
		MinGasLimit:                 cliFlags.minGasLimit,
		ExtraGasLimitGuardedTx:      cliFlags.extraGasLimitGuardedTx,
		NativeCurrencySymbol:        cliFlags.nativeCurrencySymbol,
		CustomCurrencies:            customCurrencies,
		GenesisBlockHash:            cliFlags.genesisBlock,
		FirstHistoricalEpoch:        cliFlags.firstHistoricalEpoch,
		NumHistoricalEpochs:         cliFlags.numHistoricalEpochs,
		ShouldHandleContracts:       cliFlags.shouldHandleContracts,
		ActivationEpochSpica:        cliFlags.activationEpochSpica,
	})
	if err != nil {
		return err
	}

	networkProvider.LogDescription()

	controllers, err := factory.CreateControllers(networkProvider)
	if err != nil {
		return err
	}

	httpServer, err := createHttpServer(cliFlags.port, controllers...)
	if err != nil {
		return err
	}

	go func() {
		log.Info("Starting HTTP server...", "address", httpServer.Addr)
		err := httpServer.ListenAndServe()
		if err == http.ErrServerClosed {
			log.Info("HTTP server stopped")
		} else {
			log.Error("Unexpected HTTP server error", "err", err)
		}
	}()

	// Set up signal capturing
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, os.Kill)
	<-stop

	shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownContext)
	_ = httpServer.Close()
	_ = fileLogging.Close()

	return nil
}

func createHttpServer(port int, routers ...server.Router) (*http.Server, error) {
	router := server.NewRouter(
		routers...,
	)

	corsRouter := server.CorsMiddleware(router)

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: corsRouter,
	}

	return httpServer, nil
}
