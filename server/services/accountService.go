package services

import (
	"context"

	"github.com/ElrondNetwork/elrond-proxy-go/rosetta/configuration"
	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
)

type accountService struct {
	provider NetworkProvider
	config   *configuration.Configuration
}

// NewAccountService will create a new instance of accountService
func NewAccountService(provider NetworkProvider) server.AccountAPIServicer {
	return &accountService{
		provider: provider,
	}
}

// AccountBalance implements the /account/balance endpoint.
func (service *accountService) AccountBalance(
	_ context.Context,
	request *types.AccountBalanceRequest,
) (*types.AccountBalanceResponse, *types.Error) {
	// TODO cannot return balance at a specific nonce right now
	if request.AccountIdentifier.Address == "" {
		return nil, ErrInvalidAccountAddress
	}

	// TODO: Adjust when Account.blockInfo is present.
	latestBlockData, err := service.provider.GetLatestBlockSummary()
	if err != nil {
		return nil, wrapErr(ErrUnableToGetBlock, err)
	}

	account, err := service.provider.GetAccount(request.AccountIdentifier.Address)
	if err != nil {
		return nil, wrapErr(ErrUnableToGetAccount, err)
	}

	response := &types.AccountBalanceResponse{
		BlockIdentifier: &types.BlockIdentifier{
			Index: int64(latestBlockData.Nonce),
			Hash:  latestBlockData.Hash,
		},
		Balances: []*types.Amount{
			{
				Value:    account.Balance,
				Currency: service.config.Currency,
			},
		},
		Metadata: map[string]interface{}{
			"nonce": account.Nonce,
		},
	}

	return response, nil
}

// AccountCoins implements the /account/coins endpoint.
func (service *accountService) AccountCoins(_ context.Context, _ *types.AccountCoinsRequest) (*types.AccountCoinsResponse, *types.Error) {
	return nil, ErrNotImplemented
}
