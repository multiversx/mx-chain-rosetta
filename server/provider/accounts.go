package provider

import "github.com/ElrondNetwork/rosetta/server/resources"

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
func (provider *networkProvider) GetAccountNativeBalance(address string) (*resources.AccountNativeBalance, error) {
	url := buildUrlGetAccountNativeBalance(address)
	response := &resources.AccountNativeBalanceApiResponse{}

	err := provider.getResource(url, response)
	if err != nil {
		return nil, newErrCannotGetAccount(address, err)
	}

	data := &response.Data

	log.Trace("GetAccountNativeBalance()",
		"address", address,
		"balance", data.Balance,
		"block", data.BlockCoordinates.Nonce,
		"blockHash", data.BlockCoordinates.Hash,
		"blockRootHash", data.BlockCoordinates.RootHash,
	)

	return data, nil
}

// GetAccountESDTBalance gets the ESDT balance by address and tokenIdentifier
func (provider *networkProvider) GetAccountESDTBalance(address string, tokenIdentifier string) (*resources.AccountESDTBalance, error) {
	url := buildUrlGetAccountESDTBalance(address, tokenIdentifier)
	response := &resources.AccountESDTBalanceApiResponse{}

	err := provider.getResource(url, response)
	if err != nil {
		return nil, newErrCannotGetAccount(address, err)
	}

	data := &response.Data

	log.Trace("GetAccountESDTBalance()",
		"address", address,
		"balance", data.Balance,
		"block", data.BlockCoordinates.Nonce,
		"blockHash", data.BlockCoordinates.Hash,
		"blockRootHash", data.BlockCoordinates.RootHash,
	)

	return data, nil
}
