package services

import (
	"math/big"

	"github.com/coinbase/rosetta-sdk-go/types"
)

func (service *constructionService) computeFeeComponents(options *constructionOptions, computedData []byte) (*big.Int, uint64, uint64, *types.Error) {
	networkConfig := service.provider.GetNetworkConfig()
	minGasPrice := networkConfig.MinGasPrice
	// TODO: Handle in a future PR
	gasPriceModifier := float64(0.01)

	isForNativeCurrency := service.extension.isNativeCurrencySymbol(options.CurrencySymbol)
	isForCustomCurrency := !isForNativeCurrency
	if isForCustomCurrency {
		// TODO: Handle in a future PR
		return nil, 0, 0, service.errFactory.newErr(ErrNotImplemented)
	}

	movementGasLimit := networkConfig.MinGasLimit + networkConfig.GasPerDataByte*uint64(len(computedData))
	// TODO: Handle in a future PR
	executionGasLimit := uint64(0)
	estimatedGasLimit := movementGasLimit + executionGasLimit

	gasLimit := options.coalesceGasLimit(estimatedGasLimit)
	gasPrice := options.coalesceGasPrice(minGasPrice)

	if gasLimit < estimatedGasLimit {
		return nil, 0, 0, service.errFactory.newErr(ErrInsufficientGasLimit)
	}
	if gasPrice < minGasPrice {
		return nil, 0, 0, service.errFactory.newErr(ErrGasPriceTooLow)
	}

	fee := computeFee(movementGasLimit, executionGasLimit, gasPrice, gasPriceModifier)
	return fee, gasLimit, gasPrice, nil
}

func computeFee(movementGasLimit uint64, executionGasLimit uint64, gasPrice uint64, gasPriceModifier float64) *big.Int {
	movementFee := multiplyUint64(movementGasLimit, gasPrice)
	executionGasPrice := uint64(float64(gasPrice) * gasPriceModifier)
	executionFee := multiplyUint64(executionGasLimit, executionGasPrice)
	computedFee := addBigInt(movementFee, executionFee)
	return computedFee
}
