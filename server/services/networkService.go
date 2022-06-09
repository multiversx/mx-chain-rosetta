package services

import (
	"context"

	"github.com/ElrondNetwork/rosetta/version"
	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
)

type networkService struct {
	provider  NetworkProvider
	extension *networkProviderExtension
}

// NewNetworkService creates a new instance of a networkService
func NewNetworkService(networkProvider NetworkProvider) server.NetworkAPIServicer {
	return &networkService{
		provider:  networkProvider,
		extension: newNetworkProviderExtension(networkProvider),
	}
}

// NetworkList implements the /network/list endpoint
func (service *networkService) NetworkList(
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
func (service *networkService) NetworkStatus(
	_ context.Context,
	_ *types.NetworkRequest,
) (*types.NetworkStatusResponse, *types.Error) {
	if service.provider.IsOffline() {
		return nil, ErrOfflineMode
	}

	latestBlockSummary, err := service.provider.GetLatestBlockSummary()
	if err != nil {
		return nil, wrapErr(ErrUnableToGetBlock, err)
	}

	networkStatusResponse := &types.NetworkStatusResponse{
		CurrentBlockIdentifier: blockSummaryToIdentifier(latestBlockSummary),
		CurrentBlockTimestamp:  timestampInMilliseconds(latestBlockSummary.Timestamp),
		GenesisBlockIdentifier: service.extension.getGenesisBlockIdentifier(),
		Peers: []*types.Peer{
			{
				PeerID: service.provider.GetObserverPubkey(),
			},
		},
	}

	return networkStatusResponse, nil
}

// NetworkOptions implements the /network/options endpoint.
func (service *networkService) NetworkOptions(
	_ context.Context,
	_ *types.NetworkRequest,
) (*types.NetworkOptionsResponse, *types.Error) {
	return &types.NetworkOptionsResponse{
		Version: &types.Version{
			RosettaVersion: version.RosettaVersion,
			NodeVersion:    version.NodeVersion,
		},
		Allow: &types.Allow{
			OperationStatuses: []*types.OperationStatus{
				{
					Status:     OpStatusSuccess,
					Successful: true,
				},
				// TODO: Possibly remove this?
				{
					Status:     OpStatusFailed,
					Successful: false,
				},
				// TODO: Should we add anything else here?
			},
			OperationTypes: SupportedOperationTypes,
			Errors:         Errors,
		},
	}, nil
}
