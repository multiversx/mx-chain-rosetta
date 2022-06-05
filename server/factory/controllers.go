package controllers

import (
	"github.com/ElrondNetwork/rosetta/server/services"
	"github.com/ElrondNetwork/rosetta/server/services/offline"
	"github.com/coinbase/rosetta-sdk-go/asserter"
	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
)

func CreateOfflineControllers(networkProvider services.NetworkProvider) ([]server.Router, error) {
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

func CreateOnlineControllers(networkProvider services.NetworkProvider) ([]server.Router, error) {
	asserter, err := createAsserter(networkProvider)
	if err != nil {
		return nil, err
	}

	networkService := services.NewNetworkAPIService(networkProvider)
	networkController := server.NewNetworkAPIController(networkService, asserter)

	accountService := services.NewAccountAPIService(networkProvider)
	accountController := server.NewAccountAPIController(accountService, asserter)

	blockService := services.NewBlockAPIService(networkProvider)
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
	chainID, err := networkProvider.GetChainID()
	if err != nil {
		return nil, err
	}

	// The asserter automatically rejects incorrectly formatted requests.
	asserterServer, err := asserter.NewServer(
		services.SupportedOperationTypes,
		false,
		[]*types.NetworkIdentifier{
			{
				Blockchain: networkProvider.GetBlockchainName(),
				Network:    chainID,
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
