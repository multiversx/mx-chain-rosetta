package main

import (
	"testing"

	"github.com/multiversx/mx-chain-rosetta/server/resources"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigOfCustomCurrencies(t *testing.T) {
	t.Run("with success", func(t *testing.T) {
		customCurrencies, err := loadConfigOfCustomCurrencies("testdata/custom-currencies.json")
		require.NoError(t, err)
		require.NoError(t, err)
		require.Equal(t, []resources.Currency{
			{
				Symbol:   "WEGLD-bd4d79",
				Decimals: 18,
			},
			{
				Symbol:   "USDC-c76f1f",
				Decimals: 6,
			},
		}, customCurrencies)
	})

	t.Run("with error (missing file)", func(t *testing.T) {
		_, err := loadConfigOfCustomCurrencies("testdata/missing-file.json")
		require.Error(t, err)
	})

	t.Run("with error (invalid file)", func(t *testing.T) {
		_, err := loadConfigOfCustomCurrencies("testdata/custom-currencies-bad.json")
		require.Error(t, err)
	})
}
