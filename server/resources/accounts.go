package resources

import (
	"github.com/multiversx/mx-chain-core-go/core"
)

// AccountApiResponse is an API resource
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
	Address string `json:"address"`
	Nonce   uint64 `json:"nonce"`
	Balance string `json:"balance"`
}

// AccountESDTBalanceApiResponse is an API resource
type AccountESDTBalanceApiResponse struct {
	resourceApiResponse
	Data AccountESDTBalanceApiResponsePayload `json:"data"`
}

// AccountESDTBalanceApiResponsePayload is an API resource
type AccountESDTBalanceApiResponsePayload struct {
	TokenData        AccountESDTTokenData `json:"tokenData"`
	BlockCoordinates BlockCoordinates     `json:"blockInfo"`
}

// AccountESDTTokenData is an API resource
type AccountESDTTokenData struct {
	Identifier string `json:"tokenIdentifier"`
	Balance    string `json:"balance"`
}

// AccountBalanceOnBlock defines an account resource
type AccountBalanceOnBlock struct {
	Balance          string
	Nonce            core.OptionalUint64
	BlockCoordinates BlockCoordinates
}
