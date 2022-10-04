package provider

import (
	"errors"
	"testing"

	"github.com/ElrondNetwork/elrond-go-core/data/api"
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
	args.FirstHistoricalEpoch = 2
	args.NumHistoricalEpochs = 8

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
					CurrentEpoch:      11,
				},
			}

			return 0, nil
		}

		// 3 = max(11 - 8, 2)
		if path == "/node/epoch-start/3" {
			value.(*resources.EpochStartApiResponse).Data = resources.EpochStartApiResponsePayload{
				EpochStart: resources.EpochStart{
					Nonce: 300,
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
					Block: api.Block{
						Nonce:         998,
						Hash:          "00000998",
						PrevBlockHash: "00000997",
						Timestamp:     998,
					},
				},
			}, nil
		}

		// Oldest nonce with historical state
		if nonce == 300 {
			return &data.BlockApiResponse{
				Data: data.BlockApiResponsePayload{
					Block: api.Block{
						Nonce:         300,
						Hash:          "00000300",
						PrevBlockHash: "00000299",
						Timestamp:     300,
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
		Nonce:             300,
		Hash:              "00000300",
		PreviousBlockHash: "00000299",
		Timestamp:         300,
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

	observerFacade.CallGetRestEndPointCalled = func(baseUrl, path string, value interface{}) (int, error) {
		if path == "/node/status" {
			value.(*resources.NodeStatusApiResponse).Data = resources.NodeStatusApiResponsePayload{
				Status: resources.NodeStatus{
					HighestFinalNonce: 42,
				},
			}

			return 0, nil
		}

		return 0, errors.New("unexpected request")
	}

	observerFacade.GetBlockByNonceCalled = func(shardID uint32, nonce uint64, options common.BlockQueryOptions) (*data.BlockApiResponse, error) {
		return nil, errors.New("arbitrary error")
	}

	nodeStatus, err := provider.GetNodeStatus()
	require.Nil(t, nodeStatus)
	require.ErrorContains(t, err, "arbitrary error")
}

func TestNetworkProvider_GetLatestBlockNonce(t *testing.T) {
	observerFacade := testscommon.NewObserverFacadeMock()
	args := createDefaultArgsNewNetworkProvider()
	args.ObserverFacade = observerFacade
	args.FirstHistoricalEpoch = 2
	args.NumHistoricalEpochs = 8

	provider, err := NewNetworkProvider(args)
	require.Nil(t, err)
	require.NotNil(t, provider)

	t.Run("when HighestFinalNonce <= 2 (node didn't start syncing)", func(t *testing.T) {
		observerFacade.CallGetRestEndPointCalled = func(baseUrl, path string, value interface{}) (int, error) {
			if path == "/node/status" {
				value.(*resources.NodeStatusApiResponse).Data = resources.NodeStatusApiResponsePayload{
					Status: resources.NodeStatus{
						HighestFinalNonce: 0,
					},
				}

				return 0, nil
			}

			return 0, errors.New("unexpected request")
		}

		nonce, err := provider.getLatestBlockNonce()
		require.Error(t, errCannotGetLatestBlockNonce, err)
		require.Equal(t, uint64(0), nonce)
	})

	t.Run("when HighestFinalNonce > 2", func(t *testing.T) {
		observerFacade.CallGetRestEndPointCalled = func(baseUrl, path string, value interface{}) (int, error) {
			if path == "/node/status" {
				value.(*resources.NodeStatusApiResponse).Data = resources.NodeStatusApiResponsePayload{
					Status: resources.NodeStatus{
						HighestFinalNonce: 42,
					},
				}

				return 0, nil
			}

			return 0, errors.New("unexpected request")
		}

		nonce, err := provider.getLatestBlockNonce()
		require.Nil(t, err)
		require.Equal(t, uint64(40), nonce)
	})
}

func TestGetOldestNonceWithHistoricalStateGivenNodeStatus(t *testing.T) {
	observerFacade := testscommon.NewObserverFacadeMock()
	args := createDefaultArgsNewNetworkProvider()
	args.ObserverFacade = observerFacade
	args.FirstHistoricalEpoch = 2
	args.NumHistoricalEpochs = 8

	provider, err := NewNetworkProvider(args)
	require.Nil(t, err)
	require.NotNil(t, provider)

	observerFacade.CallGetRestEndPointCalled = func(baseUrl, path string, value interface{}) (int, error) {
		// 2 = max(7 - 8, 2)
		if path == "/node/epoch-start/2" {
			value.(*resources.EpochStartApiResponse).Data = resources.EpochStartApiResponsePayload{
				EpochStart: resources.EpochStart{
					Nonce: 200,
				},
			}

			return 0, nil
		}

		// 3 = max(11 - 8, 2)
		if path == "/node/epoch-start/3" {
			value.(*resources.EpochStartApiResponse).Data = resources.EpochStartApiResponsePayload{
				EpochStart: resources.EpochStart{
					Nonce: 300,
				},
			}

			return 0, nil
		}

		// 42 = 50 - 8
		if path == "/node/epoch-start/42" {
			return 0, errors.New("arbitrary error")
		}

		return 0, errors.New("unexpected request")
	}

	oldestNonce, err := provider.getOldestNonceWithHistoricalStateGivenNodeStatus(&resources.NodeStatus{
		CurrentEpoch: 7,
	})
	require.Nil(t, err)
	require.Equal(t, uint64(200), oldestNonce)

	oldestNonce, err = provider.getOldestNonceWithHistoricalStateGivenNodeStatus(&resources.NodeStatus{
		CurrentEpoch: 11,
	})
	require.Nil(t, err)
	require.Equal(t, uint64(300), oldestNonce)

	oldestNonce, err = provider.getOldestNonceWithHistoricalStateGivenNodeStatus(&resources.NodeStatus{
		CurrentEpoch: 50,
	})
	require.Equal(t, uint64(0), oldestNonce)
	require.ErrorContains(t, err, "arbitrary error")
}

func TestGetOldestEligibleEpoch(t *testing.T) {
	observerFacade := testscommon.NewObserverFacadeMock()
	args := createDefaultArgsNewNetworkProvider()
	args.ObserverFacade = observerFacade
	args.FirstHistoricalEpoch = 2
	args.NumHistoricalEpochs = 8

	provider, err := NewNetworkProvider(args)
	require.Nil(t, err)
	require.NotNil(t, provider)

	epoch := provider.getOldestEligibleEpoch(7)
	require.Equal(t, uint32(2), epoch)

	epoch = provider.getOldestEligibleEpoch(11)
	require.Equal(t, uint32(3), epoch)

	epoch = provider.getOldestEligibleEpoch(100)
	require.Equal(t, uint32(92), epoch)
}
