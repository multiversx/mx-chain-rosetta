package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/ElrondNetwork/rosetta/server/factory"
	"github.com/ElrondNetwork/rosetta/version"
	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	cli.AppHelpTemplate = helpTemplate
	app.Name = "Elrond Rosetta CLI App"
	app.Version = version.RosettaMiddlewareVersion
	app.Usage = "This is the entry point for starting a new Elrond Rosetta instance"
	app.Flags = getAllCliFlags()
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
	cliFlags := getParsedCliFlags(ctx)

	fileLogging, err := initializeLogger(cliFlags.logsFolder, cliFlags.logLevel)
	if err != nil {
		return err
	}

	log.Info("Starting Rosetta...", "middleware", version.RosettaMiddlewareVersion, "specification", version.RosettaVersion, "node", version.NodeVersion)

	networkProvider, err := factory.CreateNetworkProvider(factory.ArgsCreateNetworkProvider{
		IsOffline:                   cliFlags.offline,
		NumShards:                   cliFlags.numShards,
		ObservedActualShard:         cliFlags.observerActualShard,
		ObservedProjectedShard:      cliFlags.observerProjectedShard,
		ObservedProjectedShardIsSet: cliFlags.observerProjectedShardIsSet,
		ObserverUrl:                 cliFlags.observerHttpUrl,
		ObserverPubkey:              cliFlags.observerPubkey,
		NetworkID:                   cliFlags.networkID,
		NetworkName:                 cliFlags.networkName,
		GasPerDataByte:              cliFlags.gasPerDataByte,
		MinGasPrice:                 cliFlags.minGasPrice,
		MinGasLimit:                 cliFlags.minGasLimit,
		NativeCurrencySymbol:        cliFlags.nativeCurrencySymbol,
		GenesisBlockHash:            cliFlags.genesisBlock,
		NumHistoricalBlocks:         cliFlags.numHistoricalBlocks,
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
