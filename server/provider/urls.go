package provider

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"

	"github.com/multiversx/mx-chain-rosetta/server/resources"
)

var (
	urlPathGetNodeStatus                        = "/node/status"
	urlPathGetEpochStartInfo                    = "/node/epoch-start/%d"
	urlPathGetGenesisBalances                   = "/network/genesis-balances"
	urlPathGetAccount                           = "/address/%s"
	urlPathGetAccountNativeBalance              = "/address/%s"
	urlPathGetAccountESDTBalance                = "/address/%s/esdt/%s"
	urlPathGetAccountNFTBalance                 = "/address/%s/nft/%s/nonce/%d"
	urlParameterAccountQueryOptionsOnFinalBlock = "onFinalBlock"
	urlParameterAccountQueryOptionsBlockNonce   = "blockNonce"
	urlParameterAccountQueryOptionsBlockHash    = "blockHash"
)

func buildUrlGetEpochStartInfo(epoch uint32) string {
	return fmt.Sprintf(urlPathGetEpochStartInfo, epoch)
}

func buildUrlGetAccount(address string) string {
	options := resources.NewAccountQueryOptionsOnFinalBlock()
	return buildUrlWithAccountQueryOptions(fmt.Sprintf(urlPathGetAccount, address), options)
}

func buildUrlGetAccountNativeBalance(address string, options resources.AccountQueryOptions) string {
	return buildUrlWithAccountQueryOptions(fmt.Sprintf(urlPathGetAccountNativeBalance, address), options)
}

func buildUrlGetAccountESDTBalance(address string, tokenIdentifier string, options resources.AccountQueryOptions) string {
	return buildUrlWithAccountQueryOptions(fmt.Sprintf(urlPathGetAccountESDTBalance, address, tokenIdentifier), options)
}

func buildUrlGetAccountNFTBalance(address string, tokenIdentifier string, nonce uint64, options resources.AccountQueryOptions) string {
	return buildUrlWithAccountQueryOptions(fmt.Sprintf(urlPathGetAccountNFTBalance, address, tokenIdentifier, nonce), options)
}

func buildUrlWithAccountQueryOptions(path string, options resources.AccountQueryOptions) string {
	if options.OnFinalBlock {
		return buildUrlWithQueryParameter(path, urlParameterAccountQueryOptionsOnFinalBlock, "true")
	}
	if options.BlockNonce.HasValue {
		return buildUrlWithQueryParameter(path, urlParameterAccountQueryOptionsBlockNonce, strconv.FormatUint(options.BlockNonce.Value, 10))
	}
	if len(options.BlockHash) > 0 {
		return buildUrlWithQueryParameter(path, urlParameterAccountQueryOptionsBlockHash, hex.EncodeToString(options.BlockHash))
	}

	return path
}

func buildUrlWithQueryParameter(path string, key string, value string) string {
	u := url.URL{
		Path: path,
	}

	query := u.Query()
	query.Set(key, value)
	u.RawQuery = query.Encode()
	return u.String()
}
