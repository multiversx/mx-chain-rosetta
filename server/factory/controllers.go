package factory

import (
	logger "github.com/ElrondNetwork/elrond-go-logger"
	"github.com/ElrondNetwork/rosetta/server/services"
	"github.com/ElrondNetwork/rosetta/server/services/offline"
	"github.com/coinbase/rosetta-sdk-go/asserter"
	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
)

var log = logger.GetOrCreate("server/factory")

func CreateControllers(networkProvider services.NetworkProvider) ([]server.Router, error) {
	if networkProvider.IsOffline() {
		return createOfflineControllers(networkProvider)
	}

	return createOnlineControllers(networkProvider)
}

func createOfflineControllers(networkProvider services.NetworkProvider) ([]server.Router, error) {
	log.Info("createOfflineControllers()")

	asserter, err := createAsserter(networkProvider)
	if err != nil {
		return nil, err
	}

	offlineService := offline.NewOfflineService()

	networkService := services.NewNetworkAPIService(networkProvider)
	networkController := server.NewNetworkAPIController(networkService, asserter)

	accountController := server.NewAccountAPIController(offlineService, asserter)
	blockController := server.NewBlockAPIController(offlineService, asserter)
	mempoolController := server.NewMempoolAPIController(offlineService, asserter)

	constructionService := services.NewConstructionAPIService(networkProvider)
	constructionController := server.NewConstructionAPIController(constructionService, asserter)

	return []server.Router{
		networkController,
		accountController,
		blockController,
		mempoolController,
		constructionController,
	}, nil
}

func createOnlineControllers(networkProvider services.NetworkProvider) ([]server.Router, error) {
	log.Info("createOnlineControllers()")

	asserter, err := createAsserter(networkProvider)
	if err != nil {
		return nil, err
	}

	networkService := services.NewNetworkAPIService(networkProvider)
	networkController := server.NewNetworkAPIController(networkService, asserter)

	accountService := services.NewAccountService(networkProvider)
	accountController := server.NewAccountAPIController(accountService, asserter)

	blockService := services.NewBlockService(networkProvider)
	blockController := server.NewBlockAPIController(blockService, asserter)

	mempoolService := services.NewMempoolApiService(networkProvider)
	mempoolController := server.NewMempoolAPIController(mempoolService, asserter)

	constructionService := services.NewConstructionAPIService(networkProvider)
	constructionController := server.NewConstructionAPIController(constructionService, asserter)

	return []server.Router{
		networkController,
		accountController,
		blockController,
		mempoolController,
		constructionController,
	}, nil
}

func createAsserter(networkProvider services.NetworkProvider) (*asserter.Asserter, error) {
	// The asserter automatically rejects incorrectly formatted requests.
	asserterServer, err := asserter.NewServer(
		services.SupportedOperationTypes,
		false,
		[]*types.NetworkIdentifier{
			{
				Blockchain: networkProvider.GetBlockchainName(),
				Network:    networkProvider.GetChainID(),
				// TODO: Perhaps add subnetwork identifier, as well?
			},
		},
		nil,
		false,
		"",
	)
	if err != nil {
		return nil, err
	}

	return asserterServer, nil
}
