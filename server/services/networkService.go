package services

import (
	"context"

	"github.com/ElrondNetwork/rosetta/version"
	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
)

type networkService struct {
	provider NetworkProvider
}

// NewNetworkService creates a new instance of a networkService
func NewNetworkService(networkProvider NetworkProvider) server.NetworkAPIServicer {
	return &networkService{
		provider: networkProvider,
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

	genesisBlockSummary := service.provider.GetGenesisBlockSummary()
	latestBlockSummary, err := service.provider.GetLatestBlockSummary()
	if err != nil {
		return nil, wrapErr(ErrUnableToGetBlock, err)
	}

	networkStatusResponse := &types.NetworkStatusResponse{
		CurrentBlockIdentifier: &types.BlockIdentifier{
			Index: int64(latestBlockSummary.Nonce),
			Hash:  latestBlockSummary.Hash,
		},
		CurrentBlockTimestamp: timestampInMilliseconds(latestBlockSummary.Timestamp),
		GenesisBlockIdentifier: &types.BlockIdentifier{
			Index: int64(genesisBlockSummary.Nonce),
			Hash:  genesisBlockSummary.Hash,
		},
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
