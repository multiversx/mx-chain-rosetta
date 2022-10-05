package services

import (
	"math/big"

	"github.com/ElrondNetwork/rosetta/server/resources"
	"github.com/coinbase/rosetta-sdk-go/types"
)

func (service *constructionService) computeSuggestedFeeAndGas(txType string, options *constructionOptions, networkConfig *resources.NetworkConfig) (*big.Int, uint64, uint64, *types.Error) {
	gasLimit := options.GasLimit
	gasPrice := options.GasPrice

	if gasLimit > 0 {
		err := service.checkProvidedGasLimit(gasLimit, txType, options, networkConfig)
		if err != nil {
			return nil, 0, 0, err
		}
	} else {
		// if gas limit is not provided, we estimate it
		estimatedGasLimit, err := service.estimateGasLimit(txType, networkConfig, options)
		if err != nil {
			return nil, 0, 0, err
		}

		gasLimit = estimatedGasLimit
	}

	if gasPrice > 0 {
		if gasPrice < networkConfig.MinGasPrice {
			return nil, 0, 0, service.errFactory.newErr(ErrGasPriceTooLow)
		}
	} else {
		// if gas price is not provided, we set it to minGasPrice
		gasPrice = networkConfig.MinGasPrice
	}

	if options.FeeMultiplier != 0 {
		gasPrice = uint64(options.FeeMultiplier * float64(gasPrice))

		if gasPrice < networkConfig.MinGasPrice {
			gasPrice = networkConfig.MinGasPrice
		}
	}

	fee := big.NewInt(0).Mul(
		big.NewInt(0).SetUint64(gasPrice),
		big.NewInt(0).SetUint64(gasLimit),
	)

	// TODO: In the case that the caller provides both a max fee and a suggested fee multiplier, the max fee will set an upper bound on the suggested fee (regardless of the multiplier provided).

	return fee, gasPrice, gasLimit, nil
}

func (service *constructionService) estimateGasLimit(operationType string, networkConfig *resources.NetworkConfig, options *constructionOptions) (uint64, *types.Error) {
	gasForDataField := networkConfig.GasPerDataByte * uint64(len(options.Data))

	switch operationType {
	case opTransfer:
		return networkConfig.MinGasLimit + gasForDataField, nil
	default:
		//  we do not support this yet other operation types, but we might support it in the future
		return 0, service.errFactory.newErr(ErrNotImplemented)
	}
}

func (service *constructionService) checkProvidedGasLimit(providedGasLimit uint64, txType string, options *constructionOptions, networkConfig *resources.NetworkConfig) *types.Error {
	estimatedGasLimit, err := service.estimateGasLimit(txType, networkConfig, options)
	if err != nil {
		return err
	}

	if providedGasLimit < estimatedGasLimit {
		return service.errFactory.newErr(ErrInsufficientGasLimit)
	}

	return nil
}
