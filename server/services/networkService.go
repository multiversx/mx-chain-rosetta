package services

import (
	"context"

	"github.com/ElrondNetwork/rosetta/version"
	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
)

type networkService struct {
	provider   NetworkProvider
	extension  *networkProviderExtension
	errFactory *errFactory
}

// NewNetworkService creates a new instance of a networkService
func NewNetworkService(networkProvider NetworkProvider) server.NetworkAPIServicer {
	return &networkService{
		provider:   networkProvider,
		extension:  newNetworkProviderExtension(networkProvider),
		errFactory: newErrFactory(),
	}
}

// NetworkList implements the /network/list endpoint
func (service *networkService) NetworkList(
	_ context.Context,
	_ *types.MetadataRequest,
) (*types.NetworkListResponse, *types.Error) {
	network := service.provider.GetNetworkConfig().NetworkName

	return &types.NetworkListResponse{
		NetworkIdentifiers: []*types.NetworkIdentifier{
			{
				Blockchain: service.provider.GetBlockchainName(),
				Network:    network,
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
		return nil, service.errFactory.newErr(ErrOfflineMode)
	}

	nodeStatus, err := service.provider.GetNodeStatus()
	if err != nil {
		return nil, service.errFactory.newErrWithOriginal(ErrUnableToGetNodeStatus, err)
	}

	networkStatusResponse := &types.NetworkStatusResponse{
		CurrentBlockIdentifier: blockSummaryToIdentifier(&nodeStatus.LatestBlock),
		CurrentBlockTimestamp:  timestampInMilliseconds(nodeStatus.LatestBlock.Timestamp),
		GenesisBlockIdentifier: service.extension.getGenesisBlockIdentifier(),
		OldestBlockIdentifier:  blockSummaryToIdentifier(&nodeStatus.OldestBlockWithHistoricalState),
		SyncStatus: &types.SyncStatus{
			Synced: &nodeStatus.Synced,
		},
		Peers: []*types.Peer{
			{
				PeerID: nodeStatus.ObserverPublicKey,
				Metadata: objectsMap{
					"connections": nodeStatus.ConnectedPeersCounts,
				},
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
	nodeVersion, err := service.getNodeVersion()
	if err != nil {
		return nil, err
	}

	return &types.NetworkOptionsResponse{
		Version: &types.Version{
			RosettaVersion: version.RosettaVersion,
			NodeVersion:    nodeVersion,
		},
		Allow: &types.Allow{
			OperationStatuses:       supportedOperationStatuses,
			OperationTypes:          SupportedOperationTypes,
			Errors:                  service.errFactory.getPossibleErrors(),
			HistoricalBalanceLookup: true,
		},
	}, nil
}

func (service *networkService) getNodeVersion() (string, *types.Error) {
	if service.provider.IsOffline() {
		// In offline mode, Rosetta does not interact with the Node.
		return "N / A", nil
	}

	nodeStatus, err := service.provider.GetNodeStatus()
	if err != nil {
		return "", service.errFactory.newErrWithOriginal(ErrUnableToGetNodeStatus, err)
	}

	return nodeStatus.Version, nil
}
