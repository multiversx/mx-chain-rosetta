package resources

// NetworkConfig is a resource
type NetworkConfig struct {
	BlockchainName         string
	NetworkID              string
	NetworkName            string
	MinGasPrice            uint64
	MinGasLimit            uint64
	GasPerDataByte         uint64
	GasPriceModifier       float64
	GasLimitCustomTransfer uint64
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
	Version              string `json:"erd_app_version"`
	ConnectedPeersCounts string `json:"erd_num_connected_peers_classification"`
	ObserverPublicKey    string `json:"erd_public_key_block_sign"`
	IsSyncing            int    `json:"erd_is_syncing"`
	CurrentEpoch         uint32 `json:"erd_epoch_number"`
	HighestNonce         uint64 `json:"erd_nonce"`
	HighestFinalNonce    uint64 `json:"erd_highest_final_nonce"`
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
	Version                        string
	ConnectedPeersCounts           map[string]int
	ObserverPublicKey              string
	Synced                         bool
	LatestBlock                    BlockSummary
	OldestBlockWithHistoricalState BlockSummary
}
