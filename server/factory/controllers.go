package factory

import (
	"github.com/ElrondNetwork/rosetta/server/services"
	"github.com/coinbase/rosetta-sdk-go/asserter"
	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
)

func CreateControllers(networkProvider services.NetworkProvider) ([]server.Router, error) {
	if networkProvider.IsOffline() {
		return createOfflineControllers(networkProvider)
	}

	return createOnlineControllers(networkProvider)
}

func createOfflineControllers(networkProvider services.NetworkProvider) ([]server.Router, error) {
	log.Info("createOfflineControllers()")

	asserterInstance, err := createAsserter(networkProvider)
	if err != nil {
		return nil, err
	}

	offlineService := services.NewOfflineService()

	networkService := services.NewNetworkService(networkProvider)
	networkController := server.NewNetworkAPIController(networkService, asserterInstance)

	accountController := server.NewAccountAPIController(offlineService, asserterInstance)
	blockController := server.NewBlockAPIController(offlineService, asserterInstance)
	mempoolController := server.NewMempoolAPIController(offlineService, asserterInstance)

	constructionService := services.NewConstructionService(networkProvider)
	constructionController := server.NewConstructionAPIController(constructionService, asserterInstance)

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

	asserterInstance, err := createAsserter(networkProvider)
	if err != nil {
		return nil, err
	}

	networkService := services.NewNetworkService(networkProvider)
	networkController := server.NewNetworkAPIController(networkService, asserterInstance)

	accountService := services.NewAccountService(networkProvider)
	accountController := server.NewAccountAPIController(accountService, asserterInstance)

	blockService := services.NewBlockService(networkProvider)
	blockController := server.NewBlockAPIController(blockService, asserterInstance)

	mempoolService := services.NewMempoolService(networkProvider)
	mempoolController := server.NewMempoolAPIController(mempoolService, asserterInstance)

	constructionService := services.NewConstructionService(networkProvider)
	constructionController := server.NewConstructionAPIController(constructionService, asserterInstance)

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
		true, // isHistoricalBalancesLookupEnabled := true
		[]*types.NetworkIdentifier{
			{
				Blockchain: networkProvider.GetBlockchainName(),
				Network:    networkProvider.GetNetworkConfig().NetworkName,
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
