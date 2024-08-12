package provider

import (
	"github.com/multiversx/mx-chain-core-go/core"
	"github.com/multiversx/mx-chain-rosetta/server/resources"
)

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
		"native balance", data.Account.Balance,
		"nonce", data.Account.Nonce,
		"block", data.BlockCoordinates.Nonce,
		"blockHash", data.BlockCoordinates.Hash,
	)

	return data, nil
}

// GetAccountNativeBalance gets the native balance by address
func (provider *networkProvider) GetAccountBalance(address string, tokenIdentifier string, options resources.AccountQueryOptions) (*resources.AccountBalanceOnBlock, error) {
	isNativeBalance := tokenIdentifier == provider.nativeCurrency.Symbol
	if isNativeBalance {
		return provider.getNativeBalance(address, options)
	}

	return provider.getCustomTokenBalance(address, tokenIdentifier, options)
}

func (provider *networkProvider) getNativeBalance(address string, options resources.AccountQueryOptions) (*resources.AccountBalanceOnBlock, error) {
	url := buildUrlGetAccountNativeBalance(address, options)
	response := &resources.AccountApiResponse{}

	err := provider.getResource(url, response)
	if err != nil {
		return nil, newErrCannotGetAccount(address, err)
	}

	data := &response.Data

	log.Trace("networkProvider.getNativeBalance()",
		"address", address,
		"balance", data.Account.Balance,
		"nonce", data.Account.Nonce,
		"block", data.BlockCoordinates.Nonce,
		"blockHash", data.BlockCoordinates.Hash,
	)

	// Here, we also return the account nonce (directly available).
	return &resources.AccountBalanceOnBlock{
		Balance:          data.Account.Balance,
		Nonce:            core.OptionalUint64{Value: data.Account.Nonce, HasValue: true},
		BlockCoordinates: data.BlockCoordinates,
	}, nil
}

func (provider *networkProvider) getCustomTokenBalance(address string, tokenIdentifier string, options resources.AccountQueryOptions) (*resources.AccountBalanceOnBlock, error) {
	url, err := decideCustomTokenBalanceUrl(address, tokenIdentifier, options)
	if err != nil {
		return nil, err
	}

	response := &resources.AccountESDTBalanceApiResponse{}

	err = provider.getResource(url, response)
	if err != nil {
		return nil, newErrCannotGetAccount(address, err)
	}

	data := &response.Data

	log.Trace("networkProvider.getCustomTokenBalance()",
		"address", address,
		"tokenIdentifier", tokenIdentifier,
		"balance", data.TokenData.Balance,
		"block", data.BlockCoordinates.Nonce,
		"blockHash", data.BlockCoordinates.Hash,
	)

	return &resources.AccountBalanceOnBlock{
		Balance:          data.TokenData.Balance,
		BlockCoordinates: data.BlockCoordinates,
	}, nil
}

func decideCustomTokenBalanceUrl(address string, tokenIdentifier string, options resources.AccountQueryOptions) (string, error) {
	tokenIdentifierParts, err := parseTokenIdentifierIntoParts(tokenIdentifier)
	if err != nil {
		return "", err
	}

	isFungible := tokenIdentifierParts.nonce == 0
	if isFungible {
		return buildUrlGetAccountFungibleTokenBalance(address, tokenIdentifier, options), nil
	}

	return buildUrlGetAccountNonFungibleTokenBalance(address, tokenIdentifierParts.tickerWithRandomSequence, tokenIdentifierParts.nonce, options), nil
}
