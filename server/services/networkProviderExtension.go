package services

import (
	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/multiversx/mx-chain-core-go/core"
	"github.com/multiversx/mx-chain-rosetta/server/resources"
)

type networkProviderExtension struct {
	provider NetworkProvider
}

func newNetworkProviderExtension(provider NetworkProvider) *networkProviderExtension {
	return &networkProviderExtension{
		provider: provider,
	}
}

func (extension *networkProviderExtension) valueToNativeAmount(value string) *types.Amount {
	return &types.Amount{
		Value:    value,
		Currency: extension.getNativeCurrency(),
	}
}

func (extension *networkProviderExtension) valueToCustomAmount(value string, currencySymbol string) *types.Amount {
	currency, ok := extension.provider.GetCustomCurrencyBySymbol(currencySymbol)
	if !ok {
		log.Warn("valueToCustomAmount(): unknown currency", "symbol", currencySymbol)

		currency = resources.Currency{
			Symbol:   currencySymbol,
			Decimals: 0,
		}
	}

	return &types.Amount{
		Value: value,
		Currency: &types.Currency{
			Symbol:   currency.Symbol,
			Decimals: currency.Decimals,
		},
	}
}

func (extension *networkProviderExtension) getNativeCurrency() *types.Currency {
	currency := extension.provider.GetNativeCurrency()

	return &types.Currency{
		Symbol:   currency.Symbol,
		Decimals: currency.Decimals,
	}
}

func (extension *networkProviderExtension) getNativeCurrencySymbol() string {
	return extension.provider.GetNativeCurrency().Symbol
}

func (extension *networkProviderExtension) isNativeCurrency(currency *types.Currency) bool {
	nativeCurrency := extension.provider.GetNativeCurrency()
	return currency.Symbol == nativeCurrency.Symbol && currency.Decimals == nativeCurrency.Decimals
}

func (extension *networkProviderExtension) isNativeCurrencySymbol(symbol string) bool {
	return symbol == extension.getNativeCurrencySymbol()
}

func (extension *networkProviderExtension) getGenesisBlockIdentifier() *types.BlockIdentifier {
	summary := extension.provider.GetGenesisBlockSummary()
	return blockSummaryToIdentifier(summary)
}

func (extension *networkProviderExtension) isAddressObserved(address string) (bool, error) {
	belongsToObservedShard, err := extension.provider.IsAddressObserved(address)
	if err != nil {
		return false, err
	}

	isUserAddress := extension.isUserAddress(address)
	return belongsToObservedShard && isUserAddress, nil
}

func (extension *networkProviderExtension) isUserAddress(address string) bool {
	pubkey, err := extension.provider.ConvertAddressToPubKey(address)
	if err != nil {
		// E.g., when address = "metachain"
		return false
	}

	return !core.IsSmartContractAddress(pubkey)
}
