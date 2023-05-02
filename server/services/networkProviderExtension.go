package services

import (
	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/multiversx/mx-chain-core-go/core"
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
	return &types.Amount{
		Value: value,
		Currency: &types.Currency{
			Symbol: currencySymbol,
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

func (extension *networkProviderExtension) filterObservedOperations(operations []*types.Operation) ([]*types.Operation, error) {
	filtered := make([]*types.Operation, 0, len(operations))

	for _, operation := range operations {
		address := operation.Account.Address

		isObserved, err := extension.provider.IsAddressObserved(address)
		if err != nil {
			return nil, err
		}

		isUserAddress := extension.isUserAddress(address)

		if isObserved && isUserAddress {
			filtered = append(filtered, operation)
		}
	}

	indexOperations(filtered)
	return filtered, nil
}

func (extension *networkProviderExtension) isUserAddress(address string) bool {
	pubkey, err := extension.provider.ConvertAddressToPubKey(address)
	if err != nil {
		// E.g., when address = "metachain"
		return false
	}

	return !core.IsSmartContractAddress(pubkey)
}
