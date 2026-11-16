package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	logger "github.com/multiversx/mx-chain-logger-go"
	fileLogging "github.com/multiversx/mx-chain-logger-go/file"
	"github.com/multiversx/mx-chain-rosetta/aggregator"
	"github.com/multiversx/mx-chain-rosetta/aggregator/backend"
	"github.com/multiversx/mx-chain-rosetta/aggregator/config"
	"github.com/multiversx/mx-chain-rosetta/aggregator/index"
	"github.com/multiversx/mx-chain-rosetta/aggregator/metachain"
	"github.com/multiversx/mx-chain-rosetta/version"
	"github.com/urfave/cli"
)

const (
	defaultLogsPath      = "logs"
	logFilePrefix        = "rosetta-aggregator"
	logFileLifeSpanInSec = 86400
	logMaxSizeInMB       = 1024

	maxIdleConnectionsPerHost = 512
	startTimeout              = 5 * time.Minute
	shutdownTimeout           = 30 * time.Second
	readHeaderTimeout         = 10 * time.Second
	readTimeout               = 30 * time.Second
	idleTimeout               = 120 * time.Second
	// The write timeout must exceed the time needed to answer a request (which is bounded by the timeout of the backends).
	writeTimeoutMargin = 30 * time.Second
)

var log = logger.GetOrCreate("main")

var (
	cliFlagConfig = cli.StringFlag{
		Name:     "config",
		Usage:    "Specifies the configuration file (TOML).",
		Required: true,
	}

	cliFlagLogLevel = cli.StringFlag{
		Name:  "log-level",
		Usage: "Specifies the logger `level(s)`. For example: *:INFO or *:INFO,aggregator/unified:DEBUG.",
		Value: "*:" + logger.LogInfo.String(),
	}

	cliFlagLogsFolder = cli.StringFlag{
		Name:  "logs-folder",
		Usage: "Specifies where to save the log files.",
		Value: "",
	}
)

func main() {
	app := cli.NewApp()
	app.Name = "MultiversX Rosetta Aggregator"
	app.Version = version.AggregatorVersion
	app.Usage = "Presents all MultiversX shards as one Rosetta chain, on top of one Rosetta instance per shard"
	app.Flags = []cli.Flag{
		cliFlagConfig,
		cliFlagLogLevel,
		cliFlagLogsFolder,
	}
	app.Action = startAggregator

	err := app.Run(os.Args)
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}
}

func startAggregator(cliContext *cli.Context) error {
	logsCloser, err := initializeLogger(cliContext.GlobalString(cliFlagLogsFolder.Name), cliContext.GlobalString(cliFlagLogLevel.Name))
	if err != nil {
		return err
	}

	defer func() {
		_ = logsCloser.Close()
	}()

	cfg, err := config.LoadFromFile(cliContext.GlobalString(cliFlagConfig.Name))
	if err != nil {
		return err
	}

	log.Info("Starting Rosetta Aggregator...",
		"version", version.AggregatorVersion,
		"network", cfg.Network.Name,
		"numShards", cfg.Network.NumShards,
		"offline", cfg.Server.Offline,
	)

	app, err := createApp(cfg)
	if err != nil {
		return err
	}

	defer func() {
		_ = app.Close()
	}()

	// The process stops upon SIGINT or SIGTERM (the latter is sent by Docker, systemd, Kubernetes).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	startContext, cancelStart := context.WithTimeout(ctx, startTimeout)
	err = app.Start(startContext)
	cancelStart()
	if err != nil {
		return err
	}

	httpServer := &http.Server{
		Addr:              net.JoinHostPort(cfg.Server.Host, strconv.Itoa(cfg.Server.Port)),
		Handler:           app.Handler(),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      cfg.GetBackendTimeout() + writeTimeoutMargin,
		IdleTimeout:       idleTimeout,
	}

	// Listening before serving makes a failed bind stop the process (instead of leaving it alive, serving nothing).
	listener, err := net.Listen("tcp", httpServer.Addr)
	if err != nil {
		return fmt.Errorf("cannot listen on %s: %w", httpServer.Addr, err)
	}

	// The unified chain is kept synchronized in the background.
	syncStopped := make(chan struct{})
	go func() {
		app.Run(ctx)
		close(syncStopped)
	}()

	serveErr := make(chan error, 1)
	go func() {
		log.Info("Starting HTTP server...", "address", listener.Addr().String())
		serveErr <- httpServer.Serve(listener)
	}()

	var errServe error

	select {
	case err = <-serveErr:
		errServe = fmt.Errorf("HTTP server stopped unexpectedly: %w", err)
	case <-ctx.Done():
	}

	log.Info("Shutting down...")

	// Stop the synchronization, and wait for the step in progress (if any) to complete, before the index is closed.
	stop()

	select {
	case <-syncStopped:
	case <-time.After(shutdownTimeout):
		log.Warn("the synchronization did not stop in time")
	}

	if errServe != nil {
		return errServe
	}

	shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancelShutdown()

	err = httpServer.Shutdown(shutdownContext)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Warn("HTTP server shutdown", "err", err)
	}

	log.Info("HTTP server stopped")
	return nil
}

func createApp(cfg *config.Config) (*aggregator.App, error) {
	httpClient := createHttpClient(cfg.GetBackendTimeout())

	backendsByShard := make(map[uint32]backend.ShardBackend, len(cfg.Shards))
	for _, shard := range cfg.Shards {
		shardBackend, err := backend.NewHttpBackend(shard.RosettaUrls, httpClient)
		if err != nil {
			return nil, fmt.Errorf("shard %d: %w", shard.ID, err)
		}

		backendsByShard[shard.ID] = shardBackend
	}

	backends, err := backend.NewSet(backendsByShard)
	if err != nil {
		return nil, err
	}

	args := aggregator.ArgsNewApp{
		Config:   cfg,
		Version:  version.AggregatorVersion,
		Backends: backends,
	}

	if !cfg.Server.Offline {
		args.MetaSource, err = metachain.NewObserverSource(cfg.Metachain.ObserverUrls, httpClient)
		if err != nil {
			return nil, err
		}

		args.Store, err = createIndexStore(cfg.Index.Path)
		if err != nil {
			return nil, err
		}
	}

	return aggregator.NewApp(args)
}

func createIndexStore(path string) (index.Store, error) {
	if len(path) == 0 {
		log.Warn("index.path is not set: the notarization index is kept in memory, and rebuilt at each start")
		return index.NewMemoryStore(), nil
	}

	return index.NewLevelDBStore(path)
}

// createHttpClient creates the HTTP client used to reach the shard instances and the metachain observers.
// Many idle connections are allowed per host, so that connections are reused under load (the default is 2).
func createHttpClient(timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConnsPerHost = maxIdleConnectionsPerHost

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
}

func initializeLogger(logsFolder string, logLevel string) (io.Closer, error) {
	if len(logsFolder) == 0 {
		currentDirectory, err := os.Getwd()
		if err != nil {
			return nil, err
		}

		logsFolder = currentDirectory
	}

	err := logger.SetLogLevel(logLevel)
	if err != nil {
		return nil, err
	}

	logsFile, err := fileLogging.NewFileLogging(fileLogging.ArgsFileLogging{
		WorkingDir:      logsFolder,
		DefaultLogsPath: defaultLogsPath,
		LogFilePrefix:   logFilePrefix,
	})
	if err != nil {
		return nil, fmt.Errorf("%w creating a log file", err)
	}

	err = logsFile.ChangeFileLifeSpan(time.Second*time.Duration(logFileLifeSpanInSec), logMaxSizeInMB)
	if err != nil {
		return nil, err
	}

	return logsFile, nil
}
