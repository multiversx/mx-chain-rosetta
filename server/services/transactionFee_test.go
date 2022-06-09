package services

import (
	"math/big"
	"testing"

	"github.com/ElrondNetwork/rosetta/server/resources"
	"github.com/stretchr/testify/require"
)

func TestEstimateGasLimit(t *testing.T) {
	minGasLimit := uint64(1000)
	gasPerDataByte := uint64(100)
	networkConfig := &resources.NetworkConfig{
		GasPerDataByte: gasPerDataByte,
		MinGasLimit:    minGasLimit,
	}

	dataField := "transaction-data"
	options := objectsMap{
		"data": dataField,
	}

	expectedGasLimit := minGasLimit + uint64(len(dataField))*gasPerDataByte

	gasLimit, err := estimateGasLimit(opTransfer, networkConfig, options)
	require.Nil(t, err)
	require.Equal(t, expectedGasLimit, gasLimit)

	gasLimit, err = estimateGasLimit(opTransfer, networkConfig, nil)
	require.Nil(t, err)
	require.Equal(t, minGasLimit, gasLimit)

	// Unsupported operation type (you cannot estimate gasLimit for e.g. a reward operation)
	gasLimit, err = estimateGasLimit(opReward, networkConfig, nil)
	require.Equal(t, ErrNotImplemented, err)
	require.Equal(t, uint64(0), gasLimit)
}

func TestProvidedGasLimit(t *testing.T) {
	t.Parallel()

	minGasLimit := uint64(1000)
	gasPerDataByte := uint64(100)
	networkConfig := &resources.NetworkConfig{
		GasPerDataByte: gasPerDataByte,
		MinGasLimit:    minGasLimit,
	}

	dataField := "transaction-data"
	options := objectsMap{
		"data": dataField,
	}

	err := checkProvidedGasLimit(uint64(900), opTransfer, options, networkConfig)
	require.Equal(t, ErrInsufficientGasLimit, err)

	err = checkProvidedGasLimit(uint64(900), opReward, options, networkConfig)
	require.Equal(t, ErrNotImplemented, err)

	err = checkProvidedGasLimit(uint64(9000), opTransfer, options, networkConfig)
	require.Nil(t, err)
}

func TestAdjustTxFeeWithFeeMultiplier(t *testing.T) {
	t.Parallel()

	options := objectsMap{
		"feeMultiplier": 1.1,
	}

	expectedGasPrice := uint64(1100)
	expectedFee := "1100"
	suggestedFee := big.NewInt(1000)

	suggestedFeeResult, gasPriceResult := adjustTxFeeWithFeeMultiplier(suggestedFee, 1000, options, 1000)
	require.Equal(t, expectedFee, suggestedFeeResult.String())
	require.Equal(t, expectedGasPrice, gasPriceResult)

	expectedGasPrice = uint64(1000)
	expectedFee = "1000"
	suggestedFeeResult, gasPriceResult = adjustTxFeeWithFeeMultiplier(suggestedFee, 1000, make(objectsMap), 1000)
	require.Equal(t, expectedFee, suggestedFeeResult.String())
	require.Equal(t, expectedGasPrice, gasPriceResult)
}

func TestComputeSuggestedFeeAndGas(t *testing.T) {
	t.Parallel()

	minGasLimit := uint64(1000)
	minGasPrice := uint64(10)
	gasPerDataByte := uint64(100)
	networkConfig := &resources.NetworkConfig{
		GasPerDataByte: gasPerDataByte,
		MinGasLimit:    minGasLimit,
		MinGasPrice:    minGasPrice,
	}

	providedGasPrice := uint64(10)
	options := objectsMap{
		"gasPrice": providedGasPrice,
	}

	suggestedFee, gasPrice, gasLimit, err := computeSuggestedFeeAndGas(opTransfer, options, networkConfig)
	require.Nil(t, err)
	require.Equal(t, minGasLimit, gasLimit)
	require.Equal(t, big.NewInt(10000), suggestedFee)
	require.Equal(t, providedGasPrice, gasPrice)

	// err provided gas price is too low
	options["gasPrice"] = 1
	_, _, _, err = computeSuggestedFeeAndGas(opTransfer, options, networkConfig)
	require.Equal(t, ErrGasPriceTooLow, err)

	// err provided gas limit is too low
	options["gasPrice"] = minGasPrice
	options["gasLimit"] = 1
	_, _, _, err = computeSuggestedFeeAndGas(opTransfer, options, networkConfig)
	require.Equal(t, ErrInsufficientGasLimit, err)

	delete(options, "gasLimit")
	options["gasPrice"] = minGasPrice
	_, _, _, err = computeSuggestedFeeAndGas(opReward, options, networkConfig)
	require.Equal(t, ErrNotImplemented, err)

	//check with fee multiplier
	delete(options, "gasPrice")
	delete(options, "gasLimit")
	options["feeMultiplier"] = 1.1
	expectedSuggestedFee := big.NewInt(11000)
	expectedGasPrice := uint64(11)
	suggestedFee, gasPrice, gasLimit, err = computeSuggestedFeeAndGas(opTransfer, options, networkConfig)
	require.Nil(t, err)
	require.Equal(t, minGasLimit, gasLimit)
	require.Equal(t, expectedSuggestedFee, suggestedFee)
	require.Equal(t, expectedGasPrice, gasPrice)
}

func TestAdjustTxFeeWithFeeMultiplier_FeeMultiplierLessThanOne(t *testing.T) {
	t.Parallel()

	options := objectsMap{
		"feeMultiplier": 0.5,
	}

	expectedFee := "500"
	suggestedFee := big.NewInt(1000)
	gasPrice := uint64(1000)
	minGasPrice := uint64(900)

	suggestedFeeResult, gasPriceResult := adjustTxFeeWithFeeMultiplier(suggestedFee, gasPrice, options, minGasPrice)
	require.Equal(t, expectedFee, suggestedFeeResult.String())
	require.Equal(t, minGasPrice, gasPriceResult)

}
