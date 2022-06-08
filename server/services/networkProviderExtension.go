package services

import "github.com/coinbase/rosetta-sdk-go/types"

type networkProviderExtension struct {
	provider NetworkProvider
}

func newNetworkProviderExtension(provider NetworkProvider) *networkProviderExtension {
	return &networkProviderExtension{
		provider: provider,
	}
}

func (mapper *networkProviderExtension) getNativeCurrency() *types.Currency {
	currency := mapper.provider.GetNativeCurrency()

	return &types.Currency{
		Symbol:   currency.Symbol,
		Decimals: currency.Decimals,
	}
}
