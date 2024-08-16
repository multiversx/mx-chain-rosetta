package main

import (
	"testing"

	"github.com/multiversx/mx-chain-rosetta/server/resources"
	"github.com/stretchr/testify/require"
)

func TestDecideCustomCurrencies(t *testing.T) {
	t.Run("with success (file provided)", func(t *testing.T) {
		customCurrencies, err := decideCustomCurrencies("testdata/custom-currencies.json")
		require.NoError(t, err)
		require.Len(t, customCurrencies, 2)
	})

	t.Run("with success (file not provided)", func(t *testing.T) {
		customCurrencies, err := decideCustomCurrencies("")
		require.NoError(t, err)
		require.Empty(t, customCurrencies)
	})
}

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
		require.ErrorContains(t, err, "error when reading custom currencies config file")
	})

	t.Run("with error (invalid file)", func(t *testing.T) {
		_, err := loadConfigOfCustomCurrencies("testdata/custom-currencies-bad.json")
		require.ErrorContains(t, err, "error when loading custom currencies from file")
	})
}
