package services

import (
	"context"

	"github.com/ElrondNetwork/rosetta/server/resources"
	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
)

type accountService struct {
	provider   NetworkProvider
	extension  *networkProviderExtension
	errFactory *errFactory
}

// NewAccountService will create a new instance of accountService
func NewAccountService(provider NetworkProvider) server.AccountAPIServicer {
	return &accountService{
		provider:   provider,
		extension:  newNetworkProviderExtension(provider),
		errFactory: newErrFactory(),
	}
}

// AccountBalance implements the /account/balance endpoint.
func (service *accountService) AccountBalance(
	_ context.Context,
	request *types.AccountBalanceRequest,
) (*types.AccountBalanceResponse, *types.Error) {
	options, err := blockIdentifierToAccountQueryOptions(request.BlockIdentifier)
	if err != nil {
		return nil, service.errFactory.newErrWithOriginal(ErrUnableToGetAccount, err)
	}

	address := request.AccountIdentifier.Address
	if address == "" {
		return nil, service.errFactory.newErr(ErrInvalidAccountAddress)
	}

	isForOneCurrency := len(request.Currencies) == 1
	if !isForOneCurrency {
		return nil, service.errFactory.newErr(ErrNotImplemented)
	}

	currencySymbol := request.Currencies[0].Symbol
	amount, blockCoordinates, err := service.getBalance(address, currencySymbol, options)
	if err != nil {
		return nil, service.errFactory.newErrWithOriginal(ErrUnableToGetAccount, err)
	}

	response := &types.AccountBalanceResponse{
		BlockIdentifier: accountBlockCoordinatesToIdentifier(blockCoordinates),
		Balances:        []*types.Amount{amount},
	}

	return response, nil
}

func (service *accountService) getBalance(address string, currencySymbol string, options resources.AccountQueryOptions) (*types.Amount, resources.AccountBlockCoordinates, error) {
	isForNative := currencySymbol == service.provider.GetNativeCurrency().Symbol
	if isForNative {
		accountBalance, err := service.provider.GetAccountNativeBalance(address, options)
		if err != nil {
			return nil, resources.AccountBlockCoordinates{}, err
		}

		amount := service.extension.valueToNativeAmount(accountBalance.Balance)
		return amount, accountBalance.BlockCoordinates, nil
	}

	accountBalance, err := service.provider.GetAccountESDTBalance(address, currencySymbol, options)
	if err != nil {
		return nil, resources.AccountBlockCoordinates{}, err
	}

	amount := service.extension.valueToCustomAmount(accountBalance.Balance, currencySymbol)
	return amount, accountBalance.BlockCoordinates, nil
}

// AccountCoins implements the /account/coins endpoint.
func (service *accountService) AccountCoins(_ context.Context, _ *types.AccountCoinsRequest) (*types.AccountCoinsResponse, *types.Error) {
	return nil, service.errFactory.newErr(ErrNotImplemented)
}
