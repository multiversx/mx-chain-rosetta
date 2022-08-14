package provider

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"

	"github.com/ElrondNetwork/rosetta/server/resources"
)

var (
	urlPathGetNodeStatus                        = "/node/status"
	urlPathGetGenesisBalances                   = "/network/genesis-balances"
	urlPathGetAccount                           = "/address/%s"
	urlPathGetAccountNativeBalance              = "/address/%s/balance"
	urlPathGetAccountESDTBalance                = "/address/%s/esdt/%s"
	urlParameterAccountQueryOptionsOnFinalBlock = "onFinalBlock"
	urlParameterAccountQueryOptionsBlockNonce   = "blockNonce"
	urlParameterAccountQueryOptionsBlockHash    = "blockHash"
)

func buildUrlGetAccount(address string) string {
	options := resources.AccountQueryOptions{OnFinalBlock: true}
	return buildUrlWithAccountQueryOptions(fmt.Sprintf(urlPathGetAccount, address), options)
}

func buildUrlGetAccountNativeBalance(address string, options resources.AccountQueryOptions) string {
	return buildUrlWithAccountQueryOptions(fmt.Sprintf(urlPathGetAccountNativeBalance, address), options)
}

func buildUrlGetAccountESDTBalance(address string, tokenIdentifier string, options resources.AccountQueryOptions) string {
	return buildUrlWithAccountQueryOptions(fmt.Sprintf(urlPathGetAccountESDTBalance, address, tokenIdentifier), options)
}

func buildUrlWithAccountQueryOptions(path string, options resources.AccountQueryOptions) string {
	u := url.URL{Path: path}
	query := u.Query()

	if options.OnFinalBlock {
		query.Set(urlParameterAccountQueryOptionsOnFinalBlock, "true")
	} else if options.BlockNonce.HasValue {
		query.Set(urlParameterAccountQueryOptionsBlockNonce, strconv.FormatUint(options.BlockNonce.Value, 10))
	} else if len(options.BlockHash) > 0 {
		query.Set(urlParameterAccountQueryOptionsBlockHash, hex.EncodeToString(options.BlockHash))
	}

	u.RawQuery = query.Encode()
	return u.String()
}
