package services

import (
	"math/big"

	"github.com/coinbase/rosetta-sdk-go/types"
)

func (service *constructionService) computeSuggestedFeeAndGas(options *constructionOptions, computedData []byte) (*big.Int, uint64, uint64, *types.Error) {
	networkConfig := service.provider.GetNetworkConfig()
	providedMaxFee := options.getMaxFee()
	providedGasLimit := options.GasLimit
	providedGasPrice := options.GasPrice
	decidedGasLimit := uint64(0)
	decidedGasPrice := uint64(0)
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
	if providedGasLimit > 0 {
		decidedGasLimit = providedGasLimit
	}
	if decidedGasLimit < estimatedGasLimit {
		return nil, 0, 0, service.errFactory.newErr(ErrInsufficientGasLimit)
	}

	if providedGasPrice > 0 {
		decidedGasPrice = providedGasPrice
	} else {
		decidedGasPrice = networkConfig.MinGasPrice
	}

	if options.FeeMultiplier != 0 {
		decidedGasPrice = uint64(options.FeeMultiplier * float64(decidedGasPrice))
	}
	if decidedGasPrice < networkConfig.MinGasPrice {
		return nil, 0, 0, service.errFactory.newErr(ErrGasPriceTooLow)
	}

	movementFee := multiplyUint64(movementGasLimit, decidedGasPrice)
	executionGasPrice := uint64(float64(decidedGasPrice) * gasPriceModifier)
	executionFee := multiplyUint64(executionGasLimit, executionGasPrice)
	computedFee := addBigInt(movementFee, executionFee)

	// In the case that the caller provides both a max fee and a suggested fee multiplier,
	// the max fee will set an upper bound on the suggested fee (regardless of the multiplier provided).
	if computedFee.Cmp(providedMaxFee) > 0 {
		// We are re-computing the decidedGasPrice, as follows:
		// providedMaxFee = movementGasLimit * decidedGasPrice + executionGasLimit * decidedGasPrice * gasPriceModifier
		// =>
		// decidedGasPrice = providedMaxFee / (movementGasLimit + executionGasLimit * gasPriceModifier)

		denominator := big.NewFloat(float64(movementGasLimit) + float64(executionGasLimit)*gasPriceModifier)
		recomputedGasPrice := divideBigIntByBigFloat(providedMaxFee, denominator)
		decidedGasPrice, _ = recomputedGasPrice.Uint64()
	}

	return computedFee, decidedGasPrice, decidedGasLimit, nil
}
