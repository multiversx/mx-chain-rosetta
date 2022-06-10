package services

import (
	"context"

	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
)

type accountService struct {
	provider  NetworkProvider
	extension *networkProviderExtension
}

// NewAccountService will create a new instance of accountService
func NewAccountService(provider NetworkProvider) server.AccountAPIServicer {
	return &accountService{
		provider:  provider,
		extension: newNetworkProviderExtension(provider),
	}
}

// AccountBalance implements the /account/balance endpoint.
func (service *accountService) AccountBalance(
	_ context.Context,
	request *types.AccountBalanceRequest,
) (*types.AccountBalanceResponse, *types.Error) {
	if request.AccountIdentifier.Address == "" {
		return nil, ErrInvalidAccountAddress
	}

	accountModel, err := service.provider.GetAccount(request.AccountIdentifier.Address)
	if err != nil {
		return nil, wrapErr(ErrUnableToGetAccount, err)
	}

	response := &types.AccountBalanceResponse{
		BlockIdentifier: blockInfoToIdentifier(accountModel.BlockInfo),
		Balances: []*types.Amount{
			service.extension.valueToNativeAmount(accountModel.Account.Balance),
		},
		Metadata: map[string]interface{}{
			"nonce":    accountModel.Account.Nonce,
			"username": accountModel.Account.Username,
		},
	}

	return response, nil
}

// AccountCoins implements the /account/coins endpoint.
func (service *accountService) AccountCoins(_ context.Context, _ *types.AccountCoinsRequest) (*types.AccountCoinsResponse, *types.Error) {
	return nil, ErrNotImplemented
}
