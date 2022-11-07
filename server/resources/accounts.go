package resources

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
