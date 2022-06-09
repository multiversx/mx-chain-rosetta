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

func (extension *networkProviderExtension) getNativeAmount(value string) *types.Amount {
	return &types.Amount{
		Value:    value,
		Currency: extension.getNativeCurrency(),
	}
}

func (extension *networkProviderExtension) getNativeCurrency() *types.Currency {
	currency := extension.provider.GetNativeCurrency()

	return &types.Currency{
		Symbol:   currency.Symbol,
		Decimals: currency.Decimals,
	}
}

func (extension *networkProviderExtension) getAccountIdentifier(address string) *types.AccountIdentifier {
	return &types.AccountIdentifier{
		Address: address,
	}
}

func (extension *networkProviderExtension) getTransactionIdentifier(hash string) *types.TransactionIdentifier {
	return &types.TransactionIdentifier{
		Hash: hash,
	}
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
		if isObserved {
			filtered = append(filtered, operation)
		}
	}

	extension.indexOperations(filtered)
	return filtered, nil
}

func (extension *networkProviderExtension) indexOperations(operations []*types.Operation) []*types.Operation {
	for index, operation := range operations {
		operation.OperationIdentifier = &types.OperationIdentifier{Index: int64(index)}
	}

	return operations
}
