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

func (extension *networkProviderExtension) getNativeCurrency() *types.Currency {
	currency := extension.provider.GetNativeCurrency()

	return &types.Currency{
		Symbol:   currency.Symbol,
		Decimals: currency.Decimals,
	}
}

func (extension *networkProviderExtension) getGenesisBlockIdentifier() *types.BlockIdentifier {
	blockSummary := extension.provider.GetGenesisBlockSummary()

	return &types.BlockIdentifier{
		Index: int64(blockSummary.Nonce),
		Hash:  blockSummary.Hash,
	}
}
