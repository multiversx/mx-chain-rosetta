package services

import (
	"context"
	"testing"

	"github.com/ElrondNetwork/rosetta/testscommon"
	"github.com/ElrondNetwork/rosetta/version"
	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/stretchr/testify/require"
)

func TestNetworkService_NetworkList(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	networkProvider.MockNetworkConfig.ChainID = "T"
	service := NewNetworkService(networkProvider)

	response, err := service.NetworkList(context.Background(), nil)

	require.Nil(t, err)
	require.Equal(t, []*types.NetworkIdentifier{{
		Blockchain: "Elrond",
		Network:    "T",
	}}, response.NetworkIdentifiers)
}

func TestNetworkService_NetworkOptions(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	networkProvider.MockNetworkConfig.ChainID = "T"
	service := NewNetworkService(networkProvider)

	networkOptions, err := service.NetworkOptions(context.Background(), nil)
	require.Nil(t, err)
	require.Equal(t, &types.NetworkOptionsResponse{
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
	}, networkOptions)
}

func TestNetworkService_NetworkStatus(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	networkProvider.MockNetworkConfig.ChainID = "T"
	networkProvider.MockObserverPubkey = "my-computer"
	networkProvider.MockGenesisBlockHash = "genesisHash"
	networkProvider.MockLatestBlockSummary.Nonce = 42
	networkProvider.MockLatestBlockSummary.Hash = "latestHash"
	networkProvider.MockLatestBlockSummary.Timestamp = 123456789

	service := NewNetworkService(networkProvider)

	networkStatusResponse, err := service.NetworkStatus(context.Background(), nil)

	require.Nil(t, err)
	require.Equal(t, &types.NetworkStatusResponse{
		CurrentBlockIdentifier: &types.BlockIdentifier{
			Index: 42,
			Hash:  "latestHash",
		},
		CurrentBlockTimestamp: 123456789000,
		GenesisBlockIdentifier: &types.BlockIdentifier{
			Index: 0,
			Hash:  "genesisHash",
		},
		Peers: []*types.Peer{
			{
				PeerID: "my-computer",
			},
		},
	}, networkStatusResponse)
}
