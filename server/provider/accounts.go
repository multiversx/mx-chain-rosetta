package provider

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/multiversx/mx-chain-rosetta/server/resources"
)

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
	tokenIdentifierParts, err := parseExtendedIdentifierParts(tokenIdentifier)
	if err != nil {
		return nil, err
	}

	url := buildUrlGetAccountESDTBalance(address, tokenIdentifier, options)
	if tokenIdentifierParts.nonce > 0 {
		url = buildUrlGetAccountNFTBalance(address, fmt.Sprintf("%s-%s", tokenIdentifierParts.ticker, tokenIdentifierParts.randomSequence), tokenIdentifierParts.nonce, options)
	}

	response := &resources.AccountESDTBalanceApiResponse{}

	err = provider.getResource(url, response)
	if err != nil {
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

type tokenIdentifierParts struct {
	ticker         string
	randomSequence string
	nonce          uint64
}

func parseExtendedIdentifierParts(tokenIdentifier string) (*tokenIdentifierParts, error) {
	parts := strings.Split(tokenIdentifier, "-")

	if len(parts) == 2 {
		return &tokenIdentifierParts{
			ticker:         parts[0],
			randomSequence: parts[1],
			nonce:          0,
		}, nil
	}

	if len(parts) == 3 {
		nonceHex := parts[2]
		nonce, err := strconv.ParseUint(nonceHex, 16, 64)
		if err != nil {
			return nil, newErrCannotParseTokenIdentifier(tokenIdentifier, err)
		}

		return &tokenIdentifierParts{
			ticker:         parts[0],
			randomSequence: parts[1],
			nonce:          nonce,
		}, nil
	}

	return nil, newErrCannotParseTokenIdentifier(tokenIdentifier, nil)
}
