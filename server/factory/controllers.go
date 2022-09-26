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

	asserter, err := createAsserter(networkProvider)
	if err != nil {
		return nil, err
	}

	offlineService := services.NewOfflineService()

	networkService := services.NewNetworkService(networkProvider)
	networkController := server.NewNetworkAPIController(networkService, asserter)

	accountController := server.NewAccountAPIController(offlineService, asserter)
	blockController := server.NewBlockAPIController(offlineService, asserter)
	mempoolController := server.NewMempoolAPIController(offlineService, asserter)

	constructionService := services.NewConstructionService(networkProvider)
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

	networkService := services.NewNetworkService(networkProvider)
	networkController := server.NewNetworkAPIController(networkService, asserter)

	accountService := services.NewAccountService(networkProvider)
	accountController := server.NewAccountAPIController(accountService, asserter)

	blockService := services.NewBlockService(networkProvider)
	blockController := server.NewBlockAPIController(blockService, asserter)

	mempoolService := services.NewMempoolService(networkProvider)
	mempoolController := server.NewMempoolAPIController(mempoolService, asserter)

	constructionService := services.NewConstructionService(networkProvider)
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
	isHistoricalBalancesLookupEnabled := true

	// The asserter automatically rejects incorrectly formatted requests.
	asserterServer, err := asserter.NewServer(
		services.SupportedOperationTypes,
		isHistoricalBalancesLookupEnabled,
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
