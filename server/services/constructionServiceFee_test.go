package services

import (
	"math/big"
	"testing"

	"github.com/multiversx/mx-chain-rosetta/testscommon"
	"github.com/stretchr/testify/require"
)

func TestConstructionService_ComputeFeeComponents(t *testing.T) {
	t.Parallel()

	networkProvider := testscommon.NewNetworkProviderMock()
	service := NewConstructionService(networkProvider).(*constructionService)

	t.Run("native transfer", func(t *testing.T) {
		fee, gasLimit, gasPrice, err := service.computeFeeComponents(&constructionOptions{
			GasLimit:       50000,
			GasPrice:       1000000000,
			CurrencySymbol: "XeGLD",
		}, []byte{})

		require.Nil(t, err)
		require.Equal(t, "50000000000000", fee.String())
		require.Equal(t, uint64(50000), gasLimit)
		require.Equal(t, uint64(1000000000), gasPrice)
	})

	t.Run("native transfer, with computed data", func(t *testing.T) {
		fee, gasLimit, gasPrice, err := service.computeFeeComponents(&constructionOptions{
			GasLimit:       53000,
			GasPrice:       1000000000,
			CurrencySymbol: "XeGLD",
		}, []byte{0xaa, 0xbb})

		require.Nil(t, err)
		require.Equal(t, "53000000000000", fee.String())
		require.Equal(t, uint64(53000), gasLimit)
		require.Equal(t, uint64(1000000000), gasPrice)
	})

	t.Run("native transfer, with insufficient gas limit", func(t *testing.T) {
		fee, gasLimit, gasPrice, err := service.computeFeeComponents(&constructionOptions{
			GasLimit:       40000,
			GasPrice:       1000000000,
			CurrencySymbol: "XeGLD",
		}, []byte{})

		require.Equal(t, int32(ErrInsufficientGasLimit), err.Code)
		require.Nil(t, fee)
		require.Equal(t, uint64(0), gasLimit)
		require.Equal(t, uint64(0), gasPrice)
	})

	t.Run("native transfer, with more gas limit than necessary", func(t *testing.T) {
		fee, gasLimit, gasPrice, err := service.computeFeeComponents(&constructionOptions{
			GasLimit:       70000,
			GasPrice:       1000000000,
			CurrencySymbol: "XeGLD",
		}, []byte{})

		require.Nil(t, err)
		require.Equal(t, "50000000000000", fee.String())
		require.Equal(t, uint64(70000), gasLimit)
		require.Equal(t, uint64(1000000000), gasPrice)
	})

	t.Run("native transfer, with gas price too low", func(t *testing.T) {
		fee, gasLimit, gasPrice, err := service.computeFeeComponents(&constructionOptions{
			GasLimit:       50000,
			GasPrice:       500000000,
			CurrencySymbol: "XeGLD",
		}, []byte{})

		require.Equal(t, int32(ErrGasPriceTooLow), err.Code)
		require.Nil(t, fee)
		require.Equal(t, uint64(0), gasLimit)
		require.Equal(t, uint64(0), gasPrice)
	})

	t.Run("native transfer, with gas price higher than necessary", func(t *testing.T) {
		fee, gasLimit, gasPrice, err := service.computeFeeComponents(&constructionOptions{
			GasLimit:       50000,
			GasPrice:       2000000000,
			CurrencySymbol: "XeGLD",
		}, []byte{})

		require.Nil(t, err)
		require.Equal(t, "100000000000000", fee.String())
		require.Equal(t, uint64(50000), gasLimit)
		require.Equal(t, uint64(2000000000), gasPrice)
	})
}

func TestComputeFee(t *testing.T) {
	t.Parallel()

	require.Equal(t, big.NewInt(50000000000000), computeFee(50000, 0, 1000000000, 0.01))
	require.Equal(t, big.NewInt(50000000000000), computeFee(50000, 0, 1000000000, 0.02))
	require.Equal(t, big.NewInt(100000000000000), computeFee(50000, 0, 2000000000, 0.01))
	require.Equal(t, big.NewInt(70000000000000), computeFee(70000, 0, 1000000000, 0.01))
	require.Equal(t, big.NewInt(60000000000000), computeFee(50000, 1000000, 1000000000, 0.01))
}
