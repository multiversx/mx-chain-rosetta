package provider

import "github.com/ElrondNetwork/rosetta/server/resources"

func (provider *networkProvider) GetNodeStatus() (*resources.AggregatedNodeStatus, error) {
	if provider.isOffline {
		return nil, errIsOffline
	}

	plainNodeStatus, err := provider.getPlainNodeStatus()
	if err != nil {
		return nil, err
	}

	latestNonce := getLatestNonceGivenHighestFinalNonce(plainNodeStatus.HighestFinalNonce)
	latestBlockSummary, err := provider.getBlockSummaryByNonce(latestNonce)
	if err != nil {
		return nil, err
	}

	oldestNonceWithHistoricalState := getOldestNonceWithHistoricalStateGivenNodeStatus(plainNodeStatus)
	oldestBlockWithHistoricalState, err := provider.getBlockSummaryByNonce(oldestNonceWithHistoricalState)
	if err != nil {
		return nil, err
	}

	return &resources.AggregatedNodeStatus{
		Synced:                         !plainNodeStatus.IsSyncing,
		LatestBlock:                    latestBlockSummary,
		OldestBlockWithHistoricalState: oldestBlockWithHistoricalState,
	}, nil
}

func (provider *networkProvider) getPlainNodeStatus() (*resources.NodeStatus, error) {
	if provider.isOffline {
		return nil, errIsOffline
	}

	response := &resources.NodeStatusApiResponse{}
	err := provider.getResource(urlPathGetNodeStatus, response)
	if err != nil {
		return nil, err
	}

	return &response.Data.Status, nil
}

func (provider *networkProvider) getLatestBlockNonce() (uint64, error) {
	nodeStatus, err := provider.getPlainNodeStatus()
	if err != nil {
		return 0, err
	}

	// In the context of scheduled transactions, make sure the N+1 block is final, as well.
	return getLatestNonceGivenHighestFinalNonce(nodeStatus.HighestFinalNonce), nil
}

func getLatestNonceGivenHighestFinalNonce(highestFinalNonce uint64) uint64 {
	return highestFinalNonce - 1
}

func getOldestNonceWithHistoricalStateGivenNodeStatus(status *resources.NodeStatus) uint64 {
	if status.NonceAtEpochStart < uint64(status.RoundsPerEpoch) {
		return 0
	}

	// Workaround: this is only a heuristic (a pessimistic, restrictive one).
	// This will be improved once the Node API provides the nonce at startOfEpoch(currentEpoch - 3).
	oldestNonce := uint64(status.NonceAtEpochStart) - uint64(status.RoundsPerEpoch)
	return oldestNonce
}
