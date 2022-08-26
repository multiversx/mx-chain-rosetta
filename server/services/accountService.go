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

	// The specification states:
	// > If the currencies field is populated, only balances for the specified currencies will be returned.
	// > If not populated, all available balances will be returned.
	// https://www.rosetta-api.org/docs/models/AccountBalanceRequest.html

	// However, we cannot fully implement this requirement at the moment.
	var currencySymbol string

	if len(request.Currencies) == 0 {
		// For the moment, we only return the native currency when "request.Currencies" is empty.
		currencySymbol = service.getNativeSymbol()
	} else if len(request.Currencies) == 1 {
		currencySymbol = request.Currencies[0].Symbol
	} else {
		// For the moment, we cannot atomically fetch multiple currencies at "the latest final block",
		// thus we postpone the implementation of this feature (doable in a future PR).
		return nil, service.errFactory.newErr(ErrNotImplemented)
	}

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

func (service *accountService) getBalance(address string, currencySymbol string, options resources.AccountQueryOptions) (*types.Amount, resources.BlockCoordinates, error) {
	isForNative := currencySymbol == service.getNativeSymbol()
	if isForNative {
		accountBalance, err := service.provider.GetAccountNativeBalance(address, options)
		if err != nil {
			return nil, resources.BlockCoordinates{}, err
		}

		amount := service.extension.valueToNativeAmount(accountBalance.Balance)
		return amount, accountBalance.BlockCoordinates, nil
	}

	accountBalance, err := service.provider.GetAccountESDTBalance(address, currencySymbol, options)
	if err != nil {
		return nil, resources.BlockCoordinates{}, err
	}

	amount := service.extension.valueToCustomAmount(accountBalance.Balance, currencySymbol)
	return amount, accountBalance.BlockCoordinates, nil
}

func (service *accountService) getNativeSymbol() string {
	return service.provider.GetNativeCurrency().Symbol
}

// AccountCoins implements the /account/coins endpoint.
func (service *accountService) AccountCoins(_ context.Context, _ *types.AccountCoinsRequest) (*types.AccountCoinsResponse, *types.Error) {
	return nil, service.errFactory.newErr(ErrNotImplemented)
}
