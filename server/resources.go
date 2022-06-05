package server

const (
	MetachainID = 4294967295
)

// NetworkConfig is the struct used to store network config information
type NetworkConfig struct {
	ChainID        string `json:"erd_chain_id"`
	GasPerDataByte uint64 `json:"erd_gas_per_data_byte"`
	MinGasPrice    uint64 `json:"erd_min_gas_price"`
	MinGasLimit    uint64 `json:"erd_min_gas_limit"`
	MinTxVersion   uint32 `json:"erd_min_transaction_version"`
	StartTime      uint64 `json:"erd_start_time"`
	NodeIsStarting string `json:"error"`
}

// BlockData is the struct used to store information about a block
type BlockData struct {
	Nonce         uint64
	Hash          string
	PrevBlockHash string
	Timestamp     int64
}
