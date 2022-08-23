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

	oldestNonceWithHistoricalState := provider.getOldestNonceWithHistoricalStateGivenNodeStatus(plainNodeStatus)
	oldestBlockWithHistoricalState, err := provider.getBlockSummaryByNonce(oldestNonceWithHistoricalState)
	if err != nil {
		return nil, err
	}

	return &resources.AggregatedNodeStatus{
		Synced:                         plainNodeStatus.IsSyncing == 0,
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

func (provider *networkProvider) getOldestNonceWithHistoricalStateGivenNodeStatus(status *resources.NodeStatus) uint64 {
	oldest := int64(status.HighestFinalNonce) - int64(provider.numHistoricalBlocks)
	if oldest < int64(oldestPossibleNonceWithHistoricalState) {
		return oldestPossibleNonceWithHistoricalState
	}

	return uint64(oldest)
}
