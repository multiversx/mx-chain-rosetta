package provider

import "github.com/multiversx/mx-chain-rosetta/server/resources"

type currenciesProvider struct {
	nativeCurrency           resources.Currency
	customCurrenciesSymbols  []string
	customCurrencies         []resources.Currency
	customCurrenciesBySymbol map[string]resources.Currency
}

// In the future, we might extract this to a standalone component (separate sub-package).
// For the moment, we keep it as a simple structure, with unexported (future-to-be exported) member functions.
func newCurrenciesProvider(nativeCurrencySymbol string, customCurrencies []resources.Currency) *currenciesProvider {
	customCurrenciesBySymbol := make(map[string]resources.Currency)
	customCurrenciesSymbols := make([]string, 0, len(customCurrencies))

	for _, customCurrency := range customCurrencies {
		customCurrenciesBySymbol[customCurrency.Symbol] = customCurrency
		customCurrenciesSymbols = append(customCurrenciesSymbols, customCurrency.Symbol)
	}

	return &currenciesProvider{
		nativeCurrency: resources.Currency{
			Symbol:   nativeCurrencySymbol,
			Decimals: int32(nativeCurrencyNumDecimals),
		},
		customCurrenciesSymbols:  customCurrenciesSymbols,
		customCurrencies:         customCurrencies,
		customCurrenciesBySymbol: customCurrenciesBySymbol,
	}
}

// GetNativeCurrency gets the native currency (EGLD, 18 decimals)
func (provider *currenciesProvider) GetNativeCurrency() resources.Currency {
	return provider.nativeCurrency
}

// GetCustomCurrencies gets the enabled custom currencies (ESDTs)
func (provider *currenciesProvider) GetCustomCurrencies() []resources.Currency {
	return provider.customCurrencies
}

// GetCustomCurrencyBySymbol gets a custom currency (ESDT) by symbol (identifier)
func (provider *currenciesProvider) GetCustomCurrenciesSymbols() []string {
	return provider.customCurrenciesSymbols
}

// GetCustomCurrencyBySymbol gets a custom currency (ESDT) by symbol (identifier)
func (provider *currenciesProvider) GetCustomCurrencyBySymbol(symbol string) (resources.Currency, bool) {
	currency, ok := provider.customCurrenciesBySymbol[symbol]
	return currency, ok
}

// HasCustomCurrency checks whether a custom currency (ESDT) is enabled (supported)
func (provider *currenciesProvider) HasCustomCurrency(symbol string) bool {
	_, ok := provider.customCurrenciesBySymbol[symbol]
	return ok
}
