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

func (extension *networkProviderExtension) valueToAmount(value string, currencySymbol string) *types.Amount {
	if extension.isNativeCurrencySymbol(currencySymbol) {
		return extension.valueToNativeAmount(value)
	}

	return extension.valueToCustomAmount(value, currencySymbol)
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

func (extension *networkProviderExtension) isContractAddress(address string) bool {
	return !extension.isUserAddress(address)
}

func (extension *networkProviderExtension) isUserAddress(address string) bool {
	pubKey, err := extension.provider.ConvertAddressToPubKey(address)
	if err != nil {
		// E.g., when address = "metachain"
		return false
	}

	return extension.isUserPubKey(pubKey)
}

func (extension *networkProviderExtension) isUserPubKey(pubKey []byte) bool {
	return !core.IsSmartContractAddress(pubKey)
}
