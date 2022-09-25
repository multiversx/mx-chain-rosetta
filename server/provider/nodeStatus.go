package provider

import "github.com/ElrondNetwork/rosetta/server/resources"

// GetNodeStatus gets an aggregated node status (e.g. current block, oldest available block etc.)
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

	oldestNonceWithHistoricalState, err := provider.getOldestNonceWithHistoricalStateGivenNodeStatus(plainNodeStatus)
	if err != nil {
		return nil, err
	}

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
	// Account for rollback-related edge cases while node is syncing (in conjunction with scheduled miniblocks).
	return highestFinalNonce - 2
}

func (provider *networkProvider) getOldestNonceWithHistoricalStateGivenNodeStatus(status *resources.NodeStatus) (uint64, error) {
	// Avoid eventual snapshotting-related edge-cases by not considering the 2 oldest kept epochs.
	oldestEligibleEpoch := status.OldestKeptEpoch + 2
	epochStartInfo, err := provider.getEpochStartInfo(oldestEligibleEpoch)
	if err != nil {
		return 0, err
	}

	return epochStartInfo.Nonce, nil
}

func (provider *networkProvider) getEpochStartInfo(epoch uint32) (*resources.EpochStart, error) {
	if provider.isOffline {
		return nil, errIsOffline
	}

	url := buildUrlGetEpochStartInfo(epoch)
	response := &resources.EpochStartApiResponse{}
	err := provider.getResource(url, response)
	if err != nil {
		return nil, err
	}

	return &response.Data.EpochStart, nil
}
