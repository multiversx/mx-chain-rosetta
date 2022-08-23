package provider

import (
	"fmt"

	"github.com/ElrondNetwork/elrond-proxy-go/common"
)

var (
	urlPathGetNodeStatus           = "/node/status"
	urlPathGetGenesisBalances      = "/network/genesis-balances"
	urlPathGetAccount              = "/address/%s"
	urlPathGetAccountNativeBalance = "/address/%s/balance"
	urlPathGetAccountESDTBalance   = "/address/%s/esdt/%s"
)

func buildUrlGetAccount(address string) string {
	options := common.AccountQueryOptions{OnFinalBlock: true}
	return common.BuildUrlWithAccountQueryOptions(fmt.Sprintf(urlPathGetAccount, address), options)
}

func buildUrlGetAccountNativeBalance(address string) string {
	options := common.AccountQueryOptions{OnFinalBlock: true}
	return common.BuildUrlWithAccountQueryOptions(fmt.Sprintf(urlPathGetAccountNativeBalance, address), options)
}

func buildUrlGetAccountESDTBalance(address string, tokenIdentifier string) string {
	options := common.AccountQueryOptions{OnFinalBlock: true}
	return common.BuildUrlWithAccountQueryOptions(fmt.Sprintf(urlPathGetAccountESDTBalance, address, tokenIdentifier), options)
}
