package resources

// NetworkConfig is a resource
type NetworkConfig struct {
	NetworkID      string
	NetworkName    string
	GasPerDataByte uint64
	MinGasPrice    uint64
	MinGasLimit    uint64
}

// NodeStatusApiResponse is an API resource
type NodeStatusApiResponse struct {
	resourceApiResponse
	Data NodeStatusApiResponsePayload `json:"data"`
}

// NodeStatusApiResponsePayload is an API resource
type NodeStatusApiResponsePayload struct {
	Status NodeStatus `json:"metrics"`
}

// NodeStatus is an API resource
type NodeStatus struct {
	IsSyncing         int    `json:"erd_is_syncing"`
	HighestNonce      uint64 `json:"erd_nonce"`
	HighestFinalNonce uint64 `json:"erd_highest_final_nonce"`
	CurrentEpoch      uint32 `json:"erd_epoch_number"`
}

// EpochStartApiResponse is an API resource
type EpochStartApiResponse struct {
	resourceApiResponse
	Data EpochStartApiResponsePayload `json:"data"`
}

// EpochStartApiResponsePayload is an API resource
type EpochStartApiResponsePayload struct {
	EpochStart EpochStart `json:"epochStart"`
}

// EpochStart is an API resource
type EpochStart struct {
	Nonce uint64 `json:"nonce"`
}

// AggregatedNodeStatus is an aggregated resource
type AggregatedNodeStatus struct {
	Synced                         bool
	LatestBlock                    BlockSummary
	OldestBlockWithHistoricalState BlockSummary
}
