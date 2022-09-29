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
	networkProvider.MockNetworkConfig.NetworkName = "testnet"
	service := NewNetworkService(networkProvider)

	response, err := service.NetworkList(context.Background(), nil)

	require.Nil(t, err)
	require.Equal(t, []*types.NetworkIdentifier{{
		Blockchain: "Elrond",
		Network:    "testnet",
	}}, response.NetworkIdentifiers)
}

func TestNetworkService_NetworkOptions(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	service := NewNetworkService(networkProvider)

	networkOptions, err := service.NetworkOptions(context.Background(), nil)
	require.Nil(t, err)
	require.Equal(t, &types.NetworkOptionsResponse{
		Version: &types.Version{
			RosettaVersion: version.RosettaVersion,
			NodeVersion:    version.NodeVersion,
		},
		Allow: &types.Allow{
			HistoricalBalanceLookup: true,
			OperationStatuses:       supportedOperationStatuses,
			OperationTypes:          SupportedOperationTypes,
			Errors:                  newErrFactory().getPossibleErrors(),
		},
	}, networkOptions)
}

func TestNetworkService_NetworkStatus(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	networkProvider.MockObserverPubkey = "my-computer"
	networkProvider.MockGenesisBlockHash = "genesisHash"
	networkProvider.MockNodeStatus.LatestBlock.Nonce = 42
	networkProvider.MockNodeStatus.LatestBlock.Hash = "latestHash"
	networkProvider.MockNodeStatus.LatestBlock.Timestamp = 123456789
	networkProvider.MockNodeStatus.OldestBlockWithHistoricalState.Nonce = 7
	networkProvider.MockNodeStatus.OldestBlockWithHistoricalState.Hash = "oldestHash"
	networkProvider.MockNodeStatus.Synced = true

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
		OldestBlockIdentifier: &types.BlockIdentifier{
			Index: 7,
			Hash:  "oldestHash",
		},
		SyncStatus: &types.SyncStatus{
			Synced: types.Bool(true),
		},
		Peers: []*types.Peer{
			{
				PeerID: "my-computer",
			},
		},
	}, networkStatusResponse)
}
