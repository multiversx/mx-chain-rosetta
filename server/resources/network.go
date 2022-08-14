package resources

// NetworkConfig is an API resource
type NetworkConfig struct {
	ChainID        string `json:"erd_chain_id"`
	GasPerDataByte uint64 `json:"erd_gas_per_data_byte"`
	MinGasPrice    uint64 `json:"erd_min_gas_price"`
	MinGasLimit    uint64 `json:"erd_min_gas_limit"`
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
	NonceAtEpochStart uint64 `json:"erd_nonce_at_epoch_start"`
	RoundsPerEpoch    uint32 `json:"erd_rounds_per_epoch"`
}

// AggregatedNodeStatus is an aggregated resource
type AggregatedNodeStatus struct {
	Synced                         bool
	LatestBlock                    BlockSummary
	OldestBlockWithHistoricalState BlockSummary
}
