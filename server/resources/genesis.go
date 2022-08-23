package resources

// GenesisBalancesApiResponse is an API resource
type GenesisBalancesApiResponse struct {
	resourceApiResponse
	Data GenesisBalancesApiResponsePayload `json:"data"`
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
