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
					HighestNonce:      1005,
					HighestFinalNonce: 1000,
					OldestKeptEpoch:   3,
				},
			}

			return 0, nil
		}

		// 5 = OldestKeptEpoch + 2
		if path == "/node/epoch-start/5" {
			value.(*resources.EpochStartApiResponse).Data = resources.EpochStartApiResponsePayload{
				EpochStart: resources.EpochStart{
					Nonce: 500,
				},
			}

			return 0, nil
		}

		return 0, errors.New("unexpected request")
	}

	observerFacade.GetBlockByNonceCalled = func(shardID uint32, nonce uint64, options common.BlockQueryOptions) (*data.BlockApiResponse, error) {
		// 998 = HighestFinalNonce - 2
		if nonce == 998 {
			return &data.BlockApiResponse{
				Data: data.BlockApiResponsePayload{
					Block: data.Block{
						Nonce:         998,
						Hash:          "00000998",
						PrevBlockHash: "00000997",
						Timestamp:     998,
					},
				},
			}, nil
		}

		// Oldest nonce with historical state
		if nonce == 500 {
			return &data.BlockApiResponse{
				Data: data.BlockApiResponsePayload{
					Block: data.Block{
						Nonce:         500,
						Hash:          "00000500",
						PrevBlockHash: "00000499",
						Timestamp:     500,
					},
				},
			}, nil
		}

		return nil, errors.New("unexpected request")
	}

	expectedSummaryOfLatest := resources.BlockSummary{
		Nonce:             998,
		Hash:              "00000998",
		PreviousBlockHash: "00000997",
		Timestamp:         998,
	}

	expectedSummaryOfOldest := resources.BlockSummary{
		Nonce:             500,
		Hash:              "00000500",
		PreviousBlockHash: "00000499",
		Timestamp:         500,
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
	observerFacade := testscommon.NewObserverFacadeMock()
	args := createDefaultArgsNewNetworkProvider()
	args.ObserverFacade = observerFacade

	provider, err := NewNetworkProvider(args)
	require.Nil(t, err)
	require.NotNil(t, provider)

	observerFacade.CallGetRestEndPointCalled = func(baseUrl, path string, value interface{}) (int, error) {
		if path == "/node/epoch-start/5" {
			value.(*resources.EpochStartApiResponse).Data = resources.EpochStartApiResponsePayload{
				EpochStart: resources.EpochStart{
					Nonce: 500,
				},
			}

			return 0, nil
		}

		if path == "/node/epoch-start/3" {
			return 0, errors.New("arbitrary error")
		}

		return 0, errors.New("unexpected request")
	}

	oldestNonce, err := provider.getOldestNonceWithHistoricalStateGivenNodeStatus(&resources.NodeStatus{
		OldestKeptEpoch: 3,
	})
	require.Nil(t, err)
	require.Equal(t, uint64(500), oldestNonce)

	oldestNonce, err = provider.getOldestNonceWithHistoricalStateGivenNodeStatus(&resources.NodeStatus{
		OldestKeptEpoch: 1,
	})
	require.Equal(t, uint64(0), oldestNonce)
	require.ErrorContains(t, err, "arbitrary error")
}
