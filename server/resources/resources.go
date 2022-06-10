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
}

// NodeStatusApiResponse is an API resource
type NodeStatusApiResponse struct {
	Data  NodeStatusApiResponsePayload `json:"data"`
	Error string                       `json:"error"`
	Code  data.ReturnCode              `json:"code"`
}

// NodeStatusApiResponsePayload is an API resource
type NodeStatusApiResponsePayload struct {
	Status NodeStatus `json:"metrics"`
}

// NodeStatus is an API resource
type NodeStatus struct {
	HighestNonce      uint64 `json:"erd_nonce"`
	HighestFinalNonce uint64 `json:"erd_highest_final_nonce"`
}

// BlockSummary is an internal resource
type BlockSummary struct {
	Nonce             uint64
	Hash              string
	PreviousBlockHash string
	Timestamp         int64
}

// NativeCurrency is an internal resource
type NativeCurrency struct {
	Symbol   string
	Decimals int32
}

// GenesisBalancesApiResponse is an API resource
type GenesisBalancesApiResponse struct {
	Data  GenesisBalancesApiResponsePayload `json:"data"`
	Error string                            `json:"error"`
	Code  data.ReturnCode                   `json:"code"`
}

// GenesisBalancesApiResponsePayload is an API resource
type GenesisBalancesApiResponsePayload struct {
	Balances []*GenesisBalance `json:"balances"`
}

// GenesisBalance is an API resource
type GenesisBalance struct {
	Address      string                   `json:"address"`
	Supply       string                   `json:"supply"`
	Balance      string                   `json:"balance"`
	StakingValue string                   `json:"stakingvalue"`
	Delegation   GenesisBalanceDelegation `json:"delegation"`
}

// GenesisBalanceDelegation is an API resource
type GenesisBalanceDelegation struct {
	Address string `json:"address"`
	Value   string `json:"value"`
}
