package resources

import "github.com/ElrondNetwork/elrond-proxy-go/data"

// NetworkConfigApiResponse is an API resource
type NetworkConfigApiResponse struct {
	Data  NetworkConfigApiResponsePayload `json:"data"`
	Error string                          `json:"error"`
	Code  data.ReturnCode                 `json:"code"`
}

// NetworkConfigApiResponsePayload is an API resource
type NetworkConfigApiResponsePayload struct {
	Config NetworkConfig `json:"config"`
}

// NetworkConfig is an API resource
type NetworkConfig struct {
	ChainID        string `json:"erd_chain_id"`
	GasPerDataByte uint64 `json:"erd_gas_per_data_byte"`
	MinGasPrice    uint64 `json:"erd_min_gas_price"`
	MinGasLimit    uint64 `json:"erd_min_gas_limit"`
	MinTxVersion   uint32 `json:"erd_min_transaction_version"`
	StartTime      uint64 `json:"erd_start_time"`
	NodeIsStarting string `json:"error"`
}

// NodeStatusApiResponse is an API resource
type NodeStatusApiResponse struct {
	Data  NodeStatusApiResponsePayload `json:"data"`
	Error string                       `json:"error"`
	Code  data.ReturnCode              `json:"code"`
}

// NodeStatusApiResponsePayload is an API resource
type NodeStatusApiResponsePayload struct {
	Status NodeStatus `json:"status"`
}

// NodeStatus is an API resource
type NodeStatus struct {
	HighestFinalNonce uint64 `json:"erd_highest_final_nonce"`
}

// BlockSummary is an internal resource
type BlockSummary struct {
	Nonce             uint64
	Hash              string
	PreviousBlockHash string
	Timestamp         int64
}
