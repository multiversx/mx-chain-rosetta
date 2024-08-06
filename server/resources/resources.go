package resources

import "github.com/multiversx/mx-chain-proxy-go/data"

type resourceApiResponse struct {
	Error string          `json:"error"`
	Code  data.ReturnCode `json:"code"`
}

// GetErrorMessage gets the error message
func (resource *resourceApiResponse) GetErrorMessage() string {
	return resource.Error
}

// BlockSummary is an internal resource
type BlockSummary struct {
	Nonce             uint64
	Hash              string
	PreviousBlockHash string
	Timestamp         int64
}

// Currency is an internal resource
type Currency struct {
	Symbol   string `json:"symbol"`
	Decimals int32  `json:"decimals"`
}

// BlockCoordinates is an API resource
type BlockCoordinates struct {
	Nonce    uint64 `json:"nonce"`
	Hash     string `json:"hash"`
	RootHash string `json:"rootHash"`
}
