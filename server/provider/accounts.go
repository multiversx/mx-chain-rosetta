package provider

import "github.com/multiversx/mx-chain-rosetta/server/resources"

// TODO: Merge the methods in this file into a single method, e.g. GetAccountWithBalance(address, tokenIdentifier, options), where tokenIdentifier can be the native token or an ESDT.

// GetAccount gets an account by address
func (provider *networkProvider) GetAccount(address string) (*resources.AccountOnBlock, error) {
	url := buildUrlGetAccount(address)
	response := &resources.AccountApiResponse{}

	err := provider.getResource(url, response)
	if err != nil {
		return nil, newErrCannotGetAccount(address, err)
	}

	data := &response.Data

	log.Trace("GetAccount()",
		"address", data.Account.Address,
		"balance", data.Account.Balance,
		"block", data.BlockCoordinates.Nonce,
		"blockHash", data.BlockCoordinates.Hash,
		"blockRootHash", data.BlockCoordinates.RootHash,
	)

	return data, nil
}

// GetAccountNativeBalance gets the native balance by address
func (provider *networkProvider) GetAccountNativeBalance(address string, options resources.AccountQueryOptions) (*resources.AccountOnBlock, error) {
	url := buildUrlGetAccountNativeBalance(address, options)
	response := &resources.AccountApiResponse{}

	err := provider.getResource(url, response)
	if err != nil {
		return nil, newErrCannotGetAccount(address, err)
	}

	data := &response.Data

	log.Trace("GetAccountNativeBalance()",
		"address", address,
		"balance", data.Account.Balance,
		"nonce", data.Account.Nonce,
		"block", data.BlockCoordinates.Nonce,
		"blockHash", data.BlockCoordinates.Hash,
		"blockRootHash", data.BlockCoordinates.RootHash,
	)

	return data, nil
}

// GetAccountESDTBalance gets the ESDT balance by address and tokenIdentifier
// TODO: Return nonce for ESDT, as well (an additional request might be needed).
func (provider *networkProvider) GetAccountESDTBalance(address string, tokenIdentifier string, options resources.AccountQueryOptions) (*resources.AccountESDTBalance, error) {
	url := buildUrlGetAccountESDTBalance(address, tokenIdentifier, options)
	response := &resources.AccountESDTBalanceApiResponse{}

	err := provider.getResource(url, response)
	if err != nil {
		blockCoordinates, ok := parseBlockCoordinatesIfErrAccountNotFoundAtBlock(err)
		if ok {
			return &resources.AccountESDTBalance{
				Balance:          "0",
				BlockCoordinates: blockCoordinates,
			}, nil
		}

		return nil, newErrCannotGetAccount(address, err)
	}

	data := &response.Data

	log.Trace("GetAccountESDTBalance()",
		"address", address,
		"tokenIdentifier", tokenIdentifier,
		"balance", data.TokenData.Balance,
		"block", data.BlockCoordinates.Nonce,
		"blockHash", data.BlockCoordinates.Hash,
		"blockRootHash", data.BlockCoordinates.RootHash,
	)

	return &resources.AccountESDTBalance{
		Balance:          data.TokenData.Balance,
		BlockCoordinates: data.BlockCoordinates,
	}, nil
}
