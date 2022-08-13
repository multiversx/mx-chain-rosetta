package resources

// AccountApiResponse defines an account resource
type AccountApiResponse struct {
	resourceApiResponse
	Data AccountModel `json:"data"`
}

// AccountModel defines an account resource
type AccountModel struct {
	Account          Account                 `json:"account"`
	BlockCoordinates AccountBlockCoordinates `json:"blockInfo"`
}

// Account defines an account resource
type Account struct {
	Address  string `json:"address"`
	Nonce    uint64 `json:"nonce"`
	Balance  string `json:"balance"`
	Username string `json:"username"`
}

// AccountBlockCoordinates defines an account resource
type AccountBlockCoordinates struct {
	Nonce    uint64 `json:"nonce"`
	Hash     string `json:"hash"`
	RootHash string `json:"rootHash"`
}

// AccountNativeBalanceApiResponse defines an account resource
type AccountNativeBalanceApiResponse struct {
	resourceApiResponse
	Data AccountNativeBalance `json:"data"`
}

// AccountNativeBalance defines an account resource
type AccountNativeBalance struct {
	Balance          string                  `json:"balance"`
	BlockCoordinates AccountBlockCoordinates `json:"blockInfo"`
}

// AccountESDTBalanceApiResponse defines an account resource
type AccountESDTBalanceApiResponse struct {
	resourceApiResponse
	Data AccountESDTBalance `json:"data"`
}

// AccountESDTBalance defines an account resource
type AccountESDTBalance struct {
	Balance          string                  `json:"balance"`
	Properties       string                  `json:"properties"`
	BlockCoordinates AccountBlockCoordinates `json:"blockInfo"`
}
