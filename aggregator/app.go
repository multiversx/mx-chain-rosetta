// Package aggregator wires the components of the Rosetta Aggregator: a Rosetta server that presents all shards as one (unified) chain,
// on top of one Rosetta instance per shard and of a metachain observer.
package aggregator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/coinbase/rosetta-sdk-go/asserter"
	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
	logger "github.com/multiversx/mx-chain-logger-go"
	"github.com/multiversx/mx-chain-rosetta/aggregator/backend"
	"github.com/multiversx/mx-chain-rosetta/aggregator/config"
	"github.com/multiversx/mx-chain-rosetta/aggregator/index"
	"github.com/multiversx/mx-chain-rosetta/aggregator/metachain"
	"github.com/multiversx/mx-chain-rosetta/aggregator/services"
	"github.com/multiversx/mx-chain-rosetta/aggregator/shardmap"
	"github.com/multiversx/mx-chain-rosetta/aggregator/unified"
	shardServices "github.com/multiversx/mx-chain-rosetta/server/services"
)

var log = logger.GetOrCreate("aggregator")

// ArgsNewApp holds the arguments for creating the application
type ArgsNewApp struct {
	Config   *config.Config
	Version  string
	Backends *backend.Set

	// The following are not needed in offline mode
	MetaSource metachain.Source
	Store      index.Store
}

// App is the aggregator application
type App struct {
	config   *config.Config
	network  *types.NetworkIdentifier
	backends *backend.Set
	shardMap *shardmap.ShardMap

	metaSource metachain.Source
	index      *index.Index
	chain      *unified.Chain

	routers []server.Router
}

// NewApp creates the application. Nothing is contacted until Start() is called.
func NewApp(args ArgsNewApp) (*App, error) {
	if args.Config == nil {
		return nil, errors.New("nil config")
	}
	if args.Backends == nil {
		return nil, errors.New("nil backends")
	}

	cfg := args.Config
	isOffline := cfg.Server.Offline

	network := &types.NetworkIdentifier{
		Blockchain: cfg.Network.Blockchain,
		Network:    cfg.Network.Name,
	}

	genesisBlockIdentifier := &types.BlockIdentifier{
		Index: 0,
		Hash:  cfg.Network.GenesisBlockHash,
	}

	genesisTimestampMs := cfg.Network.GenesisTimestamp * 1000

	shardMap, err := shardmap.New(cfg.Network.NumShards, args.Backends.Shards(), cfg.Network.HandleContracts, cfg.Network.AddressPrefix)
	if err != nil {
		return nil, err
	}

	app := &App{
		config:   cfg,
		network:  network,
		backends: args.Backends,
		shardMap: shardMap,
	}

	argsServices := services.ArgsNewServices{
		IsOffline:              isOffline,
		Network:                network,
		GenesisBlockIdentifier: genesisBlockIdentifier,
		GenesisTimestampMs:     genesisTimestampMs,
		AggregatorVersion:      args.Version,
		Backends:               args.Backends,
		ShardMap:               shardMap,
	}

	if !isOffline {
		if args.MetaSource == nil {
			return nil, errors.New("nil metachain source")
		}
		if args.Store == nil {
			return nil, errors.New("nil index store")
		}

		app.metaSource = args.MetaSource

		app.index, err = index.New(args.Store, args.Backends.Shards())
		if err != nil {
			return nil, err
		}

		app.chain, err = unified.NewChain(unified.ArgsNewChain{
			Index:            app.index,
			MetaSource:       args.MetaSource,
			Backends:         args.Backends,
			Network:          network,
			StartNonce:       cfg.Index.StartNonce,
			StartEpoch:       cfg.Index.StartEpoch,
			GenesisBlockHash: cfg.Network.GenesisBlockHash,
			FetchConcurrency: cfg.Limits.MaxConcurrentBackendRequests,
		})
		if err != nil {
			return nil, err
		}

		assembler, err := unified.NewAssembler(unified.ArgsNewAssembler{
			Index:                  app.index,
			Chain:                  app.chain,
			Backends:               args.Backends,
			Addresses:              shardMap,
			Network:                network,
			GenesisBlockIdentifier: genesisBlockIdentifier,
			GenesisTimestampMs:     genesisTimestampMs,
			ForwardLinks:           cfg.IsForwardLinksEnabled(),
			BlocksCacheSize:        cfg.Limits.BlockCacheSize,
			MaxConcurrentFetches:   cfg.Limits.MaxConcurrentBackendRequests,
		})
		if err != nil {
			return nil, err
		}

		argsServices.Index = app.index
		argsServices.Chain = app.chain
		argsServices.Assembler = assembler
	}

	app.routers, err = createRouters(argsServices)
	if err != nil {
		return nil, err
	}

	app.routers = append(app.routers, newHealthController(app))

	return app, nil
}

func createRouters(args services.ArgsNewServices) ([]server.Router, error) {
	// The asserter automatically rejects incorrectly formatted requests.
	asserterInstance, err := asserter.NewServer(
		shardServices.SupportedOperationTypes,
		true,
		[]*types.NetworkIdentifier{args.Network},
		nil,
		false,
		"",
	)
	if err != nil {
		return nil, err
	}

	networkService, err := services.NewNetworkService(args)
	if err != nil {
		return nil, err
	}

	blockService, err := services.NewBlockService(args)
	if err != nil {
		return nil, err
	}

	accountService, err := services.NewAccountService(args)
	if err != nil {
		return nil, err
	}

	mempoolService, err := services.NewMempoolService(args)
	if err != nil {
		return nil, err
	}

	constructionService, err := services.NewConstructionService(args)
	if err != nil {
		return nil, err
	}

	return []server.Router{
		server.NewNetworkAPIController(networkService, asserterInstance),
		server.NewBlockAPIController(blockService, asserterInstance),
		server.NewAccountAPIController(accountService, asserterInstance),
		server.NewMempoolAPIController(mempoolService, asserterInstance),
		server.NewConstructionAPIController(constructionService, asserterInstance),
	}, nil
}

// Start checks the backends against the configuration, then starts the unified chain (index bootstrap, first synchronization step).
// In offline mode, it does nothing.
func (app *App) Start(ctx context.Context) error {
	if app.config.Server.Offline {
		log.Info("aggregator started in offline mode")
		return nil
	}

	err := app.checkBackends(ctx)
	if err != nil {
		return fmt.Errorf("start-up check failed: %w", err)
	}

	return app.chain.Start(ctx)
}

// Run keeps the unified chain synchronized, until the context is done. In offline mode, it only waits for the context to be done.
func (app *App) Run(ctx context.Context) {
	if app.config.Server.Offline {
		<-ctx.Done()
		return
	}

	app.chain.Run(ctx, app.config.GetSyncInterval())
}

// Step performs one synchronization step of the unified chain (Run does this periodically). In offline mode, it does nothing.
func (app *App) Step(ctx context.Context) error {
	if app.config.Server.Offline {
		return nil
	}

	return app.chain.Step(ctx)
}

// Close releases the resources held by the application
func (app *App) Close() error {
	if app.index != nil {
		return app.index.Close()
	}

	return nil
}

// checkBackends verifies that the backends are consistent with the configuration (and among themselves)
func (app *App) checkBackends(ctx context.Context) error {
	numShards, err := app.metaSource.GetNumShards(ctx)
	if err != nil {
		return err
	}
	if numShards != app.config.Network.NumShards {
		return fmt.Errorf("the network has %d shards, but %d are configured", numShards, app.config.Network.NumShards)
	}

	var referenceOperationTypes []string

	for _, shard := range app.backends.Shards() {
		shardBackend, _ := app.backends.Get(shard)

		networks, rosettaErr, err := shardBackend.NetworkList(ctx, &types.MetadataRequest{})
		if rosettaErr != nil {
			return fmt.Errorf("shard %d: /network/list returned error %d: %s", shard, rosettaErr.Code, rosettaErr.Message)
		}
		if err != nil {
			return fmt.Errorf("shard %d: %w", shard, err)
		}
		if !containsNetwork(networks, app.network) {
			return fmt.Errorf("shard %d: the shard instance does not serve network %s / %s", shard, app.network.Blockchain, app.network.Network)
		}

		options, rosettaErr, err := shardBackend.NetworkOptions(ctx, &types.NetworkRequest{NetworkIdentifier: app.network})
		if rosettaErr != nil {
			return fmt.Errorf("shard %d: /network/options returned error %d: %s", shard, rosettaErr.Code, rosettaErr.Message)
		}
		if err != nil {
			return fmt.Errorf("shard %d: %w", shard, err)
		}
		if options == nil || options.Allow == nil {
			return fmt.Errorf("shard %d: incomplete network options", shard)
		}

		operationTypes := sortedCopy(options.Allow.OperationTypes)
		if referenceOperationTypes == nil {
			referenceOperationTypes = operationTypes
		} else if strings.Join(referenceOperationTypes, ",") != strings.Join(operationTypes, ",") {
			return fmt.Errorf("shard %d: the shard instances do not support the same operation types (are they running the same version?)", shard)
		}
	}

	return nil
}

func containsNetwork(response *types.NetworkListResponse, network *types.NetworkIdentifier) bool {
	if response == nil {
		return false
	}

	for _, item := range response.NetworkIdentifiers {
		if item != nil && item.Blockchain == network.Blockchain && item.Network == network.Network && item.SubNetworkIdentifier == nil {
			return true
		}
	}

	return false
}

func sortedCopy(items []string) []string {
	result := make([]string, len(items))
	copy(result, items)
	sort.Strings(result)
	return result
}

// Handler returns the HTTP handler of the aggregator: the Rosetta endpoints, plus /health.
// Request bodies are limited in size, and panics are converted to errors.
func (app *App) Handler() http.Handler {
	var handler http.Handler = server.NewRouter(app.routers...)

	if app.config.Server.EnableCors {
		handler = server.CorsMiddleware(handler)
	}

	handler = http.MaxBytesHandler(handler, app.config.Limits.MaxRequestBodyBytes)
	handler = recoverMiddleware(handler)
	return handler
}

// recoverMiddleware answers with a Rosetta error when a handler panics (instead of dropping the connection)
func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}

			log.Error("panic while serving request", "path", r.URL.Path, "panic", fmt.Sprintf("%v", recovered), "stack", string(debug.Stack()))

			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(&types.Error{
				Code:    int32(services.ErrInternal),
				Message: "internal error",
			})
		}()

		next.ServeHTTP(w, r)
	})
}

// healthController exposes the state of the aggregator, for supervisors and for monitoring
type healthController struct {
	app *App
}

func newHealthController(app *App) *healthController {
	return &healthController{app: app}
}

// Routes returns the routes of the controller
func (controller *healthController) Routes() server.Routes {
	return server.Routes{
		{
			Name:        "Health",
			Method:      http.MethodGet,
			Pattern:     "/health",
			HandlerFunc: controller.handle,
		},
	}
}

type healthResponse struct {
	Offline     bool              `json:"offline"`
	Ready       bool              `json:"ready"`
	Synced      bool              `json:"synced"`
	Timestamp   int64             `json:"timestamp"`
	Base        uint64            `json:"base"`
	Tip         uint64            `json:"tip"`
	Oldest      uint64            `json:"oldest"`
	LastIndexed uint64            `json:"lastIndexed"`
	Metachain   healthMetachain   `json:"metachain"`
	Shards      []healthShard     `json:"shards"`
	Anomalies   healthAnomalies   `json:"anomalies"`
	Details     map[string]string `json:"details,omitempty"`
}

type healthMetachain struct {
	Reachable  bool   `json:"reachable"`
	Synced     bool   `json:"synced"`
	FinalNonce uint64 `json:"finalNonce"`
	// Lag is the number of final metablocks that cannot be served yet
	Lag       uint64 `json:"lag"`
	LastError string `json:"lastError,omitempty"`
}

type healthShard struct {
	Shard        uint32 `json:"shard"`
	Reachable    bool   `json:"reachable"`
	Synced       bool   `json:"synced"`
	CurrentIndex int64  `json:"currentIndex"`
	OldestIndex  int64  `json:"oldestIndex"`
	LastError    string `json:"lastError,omitempty"`
}

type healthAnomalies struct {
	// MergedDuplicates counts the transactions that had the same identifier within a unified block (expected: 0)
	MergedDuplicates uint64 `json:"mergedDuplicates"`
}

func (controller *healthController) handle(w http.ResponseWriter, _ *http.Request) {
	response := controller.app.getHealth()

	statusCode := http.StatusOK
	if !response.Offline && !response.Ready {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(response)
}

func (app *App) getHealth() *healthResponse {
	response := &healthResponse{
		Offline:   app.config.Server.Offline,
		Timestamp: time.Now().Unix(),
		Shards:    make([]healthShard, 0),
	}

	if app.config.Server.Offline {
		return response
	}

	snapshot, err := app.chain.Snapshot()
	if err != nil {
		response.Details = map[string]string{"error": err.Error()}
		return response
	}

	response.Ready = true
	response.Synced = snapshot.IsSynced()
	response.Base = snapshot.Base
	response.Tip = snapshot.Tip
	response.Oldest = snapshot.Oldest
	response.LastIndexed = snapshot.LastIndexed
	response.Anomalies.MergedDuplicates = snapshot.NumDuplicatesSeen

	response.Metachain = healthMetachain{
		Reachable:  snapshot.MetaReachable,
		Synced:     snapshot.MetaSynced,
		FinalNonce: snapshot.MetaFinalNonce,
		LastError:  snapshot.MetaLastError,
	}
	if snapshot.MetaFinalNonce > snapshot.Tip {
		response.Metachain.Lag = snapshot.MetaFinalNonce - snapshot.Tip
	}

	for _, shard := range snapshot.Shards {
		response.Shards = append(response.Shards, healthShard{
			Shard:        shard.Shard,
			Reachable:    shard.Reachable,
			Synced:       shard.Synced,
			CurrentIndex: shard.CurrentIndex,
			OldestIndex:  shard.OldestIndex,
			LastError:    shard.LastError,
		})
	}

	return response
}
