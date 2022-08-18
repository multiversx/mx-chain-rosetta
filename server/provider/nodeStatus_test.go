package provider

import (
	"errors"
	"testing"

	"github.com/ElrondNetwork/elrond-proxy-go/common"
	"github.com/ElrondNetwork/elrond-proxy-go/data"
	"github.com/ElrondNetwork/rosetta/server/resources"
	"github.com/ElrondNetwork/rosetta/testscommon"
	"github.com/stretchr/testify/require"
)

func TestNetworkProvider_GetNodeStatusWithSuccess(t *testing.T) {
	observerFacade := testscommon.NewObserverFacadeMock()
	args := createDefaultArgsNewNetworkProvider()
	args.ObserverFacade = observerFacade

	provider, err := NewNetworkProvider(args)
	require.Nil(t, err)
	require.NotNil(t, provider)

	observerFacade.CallGetRestEndPointCalled = func(baseUrl, path string, value interface{}) (int, error) {
		if path == "/node/status" {
			value.(*resources.NodeStatusApiResponse).Data = resources.NodeStatusApiResponsePayload{
				Status: resources.NodeStatus{
					IsSyncing:         1,
					HighestNonce:      7,
					HighestFinalNonce: 5,
					NonceAtEpochStart: 0,
					RoundsPerEpoch:    100,
				},
			}
		}

		return 0, nil
	}

	observerFacade.GetBlockByNonceCalled = func(shardID uint32, nonce uint64, options common.BlockQueryOptions) (*data.BlockApiResponse, error) {
		// The one before "HighestFinalNonce"
		if nonce == 4 {
			return &data.BlockApiResponse{
				Data: data.BlockApiResponsePayload{
					Block: data.Block{
						Nonce:         4,
						Hash:          "0004",
						PrevBlockHash: "0003",
						Timestamp:     4,
					},
				},
			}, nil
		}

		// Oldest nonce with historical state (as considered by Rosetta)
		// max(1, NonceAtEpochStart - RoundsPerEpoch)
		if nonce == 1 {
			return &data.BlockApiResponse{
				Data: data.BlockApiResponsePayload{
					Block: data.Block{
						Nonce:         1,
						Hash:          "0001",
						PrevBlockHash: "0000",
						Timestamp:     1,
					},
				},
			}, nil
		}

		panic("unexpected request")
	}

	expectedSummaryOfLatest := resources.BlockSummary{
		Nonce:             4,
		Hash:              "0004",
		PreviousBlockHash: "0003",
		Timestamp:         4,
	}

	expectedSummaryOfOldest := resources.BlockSummary{
		Nonce:             1,
		Hash:              "0001",
		PreviousBlockHash: "0000",
		Timestamp:         1,
	}

	nodeStatus, err := provider.GetNodeStatus()
	require.Nil(t, err)
	require.False(t, nodeStatus.Synced)
	require.Equal(t, expectedSummaryOfLatest, nodeStatus.LatestBlock)
	require.Equal(t, expectedSummaryOfOldest, nodeStatus.OldestBlockWithHistoricalState)
}

func TestNetworkProvider_GetNodeStatusWithError(t *testing.T) {
	observerFacade := testscommon.NewObserverFacadeMock()
	args := createDefaultArgsNewNetworkProvider()
	args.ObserverFacade = observerFacade

	provider, err := NewNetworkProvider(args)
	require.Nil(t, err)
	require.NotNil(t, provider)

	observerFacade.GetBlockByNonceCalled = func(shardID uint32, nonce uint64, options common.BlockQueryOptions) (*data.BlockApiResponse, error) {
		return nil, errors.New("arbitrary error")
	}

	nodeStatus, err := provider.GetNodeStatus()
	require.Nil(t, nodeStatus)
	require.ErrorContains(t, err, "arbitrary error")
}

func TestGetOldestNonceWithHistoricalStateGivenNodeStatus(t *testing.T) {
	oldestNonce := getOldestNonceWithHistoricalStateGivenNodeStatus(&resources.NodeStatus{
		NonceAtEpochStart: 0,
		RoundsPerEpoch:    14400,
	})

	require.Equal(t, uint64(1), oldestNonce)

	oldestNonce = getOldestNonceWithHistoricalStateGivenNodeStatus(&resources.NodeStatus{
		NonceAtEpochStart: 50000,
		RoundsPerEpoch:    14400,
	})

	require.Equal(t, uint64(50000-14400), oldestNonce)
}
