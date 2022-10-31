package provider

import (
	"strconv"
	"strings"

	"github.com/ElrondNetwork/rosetta/server/resources"
)

// GetNodeStatus gets an aggregated node status (e.g. current block, oldest available block etc.)
func (provider *networkProvider) GetNodeStatus() (*resources.AggregatedNodeStatus, error) {
	if provider.isOffline {
		return nil, errIsOffline
	}

	plainNodeStatus, err := provider.getPlainNodeStatus()
	if err != nil {
		return nil, err
	}

	latestNonce, err := getLatestNonceGivenHighestFinalNonce(plainNodeStatus.HighestFinalNonce)
	if err != nil {
		return nil, err
	}

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

	peersCounts := parsePeersCounts(plainNodeStatus.ConnectedPeersCounts)

	return &resources.AggregatedNodeStatus{
		Version:                        plainNodeStatus.Version,
		ConnectedPeersCounts:           peersCounts,
		ObserverPublicKey:              plainNodeStatus.ObserverPublicKey,
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
	return getLatestNonceGivenHighestFinalNonce(nodeStatus.HighestFinalNonce)
}

func getLatestNonceGivenHighestFinalNonce(highestFinalNonce uint64) (uint64, error) {
	// Account for rollback-related edge cases while node is syncing (in conjunction with scheduled miniblocks).
	const nonceDelta = 2

	if highestFinalNonce <= nonceDelta {
		return 0, errCannotGetLatestBlockNonce
	}

	return highestFinalNonce - nonceDelta, nil
}

func (provider *networkProvider) getOldestNonceWithHistoricalStateGivenNodeStatus(status *resources.NodeStatus) (uint64, error) {
	oldestEligibleEpoch := provider.getOldestEligibleEpoch(status.CurrentEpoch)
	epochStartInfo, err := provider.getEpochStartInfo(oldestEligibleEpoch)
	if err != nil {
		return 0, err
	}

	return epochStartInfo.Nonce, nil
}

func (provider *networkProvider) getOldestEligibleEpoch(currentEpoch uint32) uint32 {
	oldestEpoch := int(currentEpoch) - int(provider.numHistoricalEpochs)
	if oldestEpoch < int(provider.firstHistoricalEpoch) {
		return provider.firstHistoricalEpoch
	}

	return uint32(oldestEpoch)
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

func parsePeersCounts(csv string) map[string]int {
	if len(csv) == 0 {
		return map[string]int{}
	}

	peersCounts := make(map[string]int)
	parts := strings.Split(csv, ",")

	for _, part := range parts {
		subparts := strings.Split(part, ":")
		if len(subparts) != 2 {
			// We do not report such an error (not important)
			continue
		}

		key := subparts[0]
		countAsString := subparts[1]
		count, err := strconv.ParseInt(countAsString, 10, 32)
		if err != nil {
			// We do not report such an error (not important)
			continue
		}

		peersCounts[key] = int(count)
	}

	return peersCounts
}
