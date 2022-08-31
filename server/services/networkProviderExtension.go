package services

import (
	"github.com/ElrondNetwork/elrond-go-core/core"
	"github.com/coinbase/rosetta-sdk-go/types"
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
			// Currently, we hardcode numDecimals to zero for custom currencies.
			// TODO: Fix this once we have the information from the metachain.
			Decimals: 0,
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

func (extension *networkProviderExtension) isNativeCurrency(currency *types.Currency) bool {
	nativeCurrency := extension.provider.GetNativeCurrency()
	return currency.Symbol == nativeCurrency.Symbol && currency.Decimals == nativeCurrency.Decimals
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
