package resources

import "github.com/ElrondNetwork/rosetta/common"

// AccountApiResponse defines an account resource
type AccountApiResponse struct {
	resourceApiResponse
	Data AccountOnBlock `json:"data"`
}

// AccountOnBlock defines an account resource
type AccountOnBlock struct {
	Account          Account          `json:"account"`
	BlockCoordinates BlockCoordinates `json:"blockInfo"`
}

// Account defines an account resource
type Account struct {
	Address  string `json:"address"`
	Nonce    uint64 `json:"nonce"`
	Balance  string `json:"balance"`
	Username string `json:"username"`
}

// AccountQueryOptions defines an (internal) account resource
type AccountQueryOptions struct {
	OnFinalBlock bool
	BlockNonce   common.OptionalUint64
	BlockHash    []byte
}

// AccountNativeBalanceApiResponse defines an account resource
type AccountNativeBalanceApiResponse struct {
	resourceApiResponse
	Data AccountNativeBalance `json:"data"`
}

// AccountNativeBalance defines an account resource
type AccountNativeBalance struct {
	Balance          string           `json:"balance"`
	BlockCoordinates BlockCoordinates `json:"blockInfo"`
}

// AccountESDTBalanceApiResponse defines an account resource
type AccountESDTBalanceApiResponse struct {
	resourceApiResponse
	Data AccountESDTBalance `json:"data"`
}

// AccountESDTBalance defines an account resource
type AccountESDTBalance struct {
	Balance          string           `json:"balance"`
	Properties       string           `json:"properties"`
	BlockCoordinates BlockCoordinates `json:"blockInfo"`
}
