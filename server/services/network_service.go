package services

import (
	"context"

	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
)

// NetworkAPIService implements the server.NetworkAPIServicer interface.
type networkAPIService struct {
	provider NetworkProvider
}

// NewNetworkAPIService creates a new instance of a NetworkAPIService.
func NewNetworkAPIService(networkProvider NetworkProvider) server.NetworkAPIServicer {
	return &networkAPIService{
		provider: networkProvider,
	}
}

// NetworkList implements the /network/list endpoint
func (service *networkAPIService) NetworkList(
	_ context.Context,
	_ *types.MetadataRequest,
) (*types.NetworkListResponse, *types.Error) {
	chainID := service.provider.GetChainID()

	return &types.NetworkListResponse{
		NetworkIdentifiers: []*types.NetworkIdentifier{
			{
				Blockchain: service.provider.GetBlockchainName(),
				Network:    chainID,
			},
		},
	}, nil
}

// NetworkStatus implements the /network/status endpoint.
func (service *networkAPIService) NetworkStatus(
	_ context.Context,
	_ *types.NetworkRequest,
) (*types.NetworkStatusResponse, *types.Error) {
	if service.provider.IsOffline() {
		return nil, ErrOfflineMode
	}

	genesisBlockSummary, err := service.provider.GetGenesisBlockSummary()
	if err != nil {
		return nil, wrapErr(ErrUnableToGetBlock, err)
	}

	latestBlockSummary, err := service.provider.GetLatestBlockSummary()
	if err != nil {
		return nil, wrapErr(ErrUnableToGetBlock, err)
	}

	networkStatusResponse := &types.NetworkStatusResponse{
		CurrentBlockIdentifier: &types.BlockIdentifier{
			Index: int64(latestBlockSummary.Nonce),
			Hash:  latestBlockSummary.Hash,
		},
		CurrentBlockTimestamp: latestBlockSummary.Timestamp,
		GenesisBlockIdentifier: &types.BlockIdentifier{
			Index: int64(genesisBlockSummary.Nonce),
			Hash:  genesisBlockSummary.Hash,
		},
		Peers: []*types.Peer{
			&types.Peer{
				PeerID: service.provider.GetObserverPubkey(),
			},
		},
	}

	return networkStatusResponse, nil
}

// NetworkOptions implements the /network/options endpoint.
func (service *networkAPIService) NetworkOptions(
	_ context.Context,
	_ *types.NetworkRequest,
) (*types.NetworkOptionsResponse, *types.Error) {
	return &types.NetworkOptionsResponse{
		Version: &types.Version{
			RosettaVersion: "TBD/TODO",
			NodeVersion:    "TBD/TODO",
		},
		Allow: &types.Allow{
			OperationStatuses: []*types.OperationStatus{
				{
					Status:     OpStatusSuccess,
					Successful: true,
				},
				{
					Status:     OpStatusFailed,
					Successful: false,
				},
			},
			OperationTypes: SupportedOperationTypes,
			Errors:         Errors,
		},
	}, nil
}
