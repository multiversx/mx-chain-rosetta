package provider

import (
	"testing"

	"github.com/multiversx/mx-chain-rosetta/server/resources"
	"github.com/stretchr/testify/require"
)

func TestNewCurrenciesProvider(t *testing.T) {
	t.Run("with success", func(t *testing.T) {
		t.Parallel()

		provider, err := newCurrenciesProvider("XeGLD", []resources.Currency{
			{Symbol: "ROSETTA-3a2edf", Decimals: 2},
			{Symbol: "ROSETTA-057ab4", Decimals: 2},
		})

		require.NoError(t, err)
		require.NotNil(t, provider)
	})

	t.Run("with success (empty or nil array of custom currencies)", func(t *testing.T) {
		t.Parallel()

		provider, err := newCurrenciesProvider("XeGLD", []resources.Currency{})
		require.NoError(t, err)
		require.NotNil(t, provider)

		provider, err = newCurrenciesProvider("XeGLD", nil)
		require.NoError(t, err)
		require.NotNil(t, provider)
	})

	t.Run("with invalid custom currency symbol", func(t *testing.T) {
		t.Parallel()

		_, err := newCurrenciesProvider("XeGLD", []resources.Currency{
			{Symbol: "", Decimals: 2},
		})

		require.ErrorIs(t, err, errInvalidCustomCurrencySymbol)
		require.Equal(t, "invalid custom currency symbol, index = 0", err.Error())
	})
}

func TestCurrenciesProvider_NativeCurrency(t *testing.T) {
	t.Parallel()

	provider, err := newCurrenciesProvider("XeGLD", []resources.Currency{
		{Symbol: "ROSETTA-3a2edf", Decimals: 2},
	})

	require.NoError(t, err)

	nativeCurrency := provider.GetNativeCurrency()
	require.Equal(t, "XeGLD", nativeCurrency.Symbol)
	require.Equal(t, int32(18), nativeCurrency.Decimals)
}

func TestCurrenciesProvider_CustomCurrencies(t *testing.T) {
	provider, err := newCurrenciesProvider("XeGLD", []resources.Currency{
		{Symbol: "ROSETTA-3a2edf", Decimals: 2},
		{Symbol: "ROSETTA-057ab4", Decimals: 2},
	})

	require.NoError(t, err)

	t.Run("check has", func(t *testing.T) {
		t.Parallel()

		require.True(t, provider.HasCustomCurrency("ROSETTA-3a2edf"))
		require.True(t, provider.HasCustomCurrency("ROSETTA-057ab4"))
		require.False(t, provider.HasCustomCurrency("FOO-abcdef"))
		require.False(t, provider.HasCustomCurrency("BAR-abcdef"))
		require.False(t, provider.HasCustomCurrency(""))
	})

	t.Run("get all", func(t *testing.T) {
		t.Parallel()

		customCurrencies := provider.GetCustomCurrencies()
		require.Equal(t, 2, len(customCurrencies))
	})

	t.Run("get all symbols", func(t *testing.T) {
		t.Parallel()

		customCurrenciesSymbols := provider.GetCustomCurrenciesSymbols()
		require.Equal(t, 2, len(customCurrenciesSymbols))
		require.Equal(t, "ROSETTA-3a2edf", customCurrenciesSymbols[0])
		require.Equal(t, "ROSETTA-057ab4", customCurrenciesSymbols[1])
	})

	t.Run("get by symbol", func(t *testing.T) {
		t.Parallel()

		customCurrency, ok := provider.GetCustomCurrencyBySymbol("ROSETTA-3a2edf")
		require.True(t, ok)
		require.Equal(t, "ROSETTA-3a2edf", customCurrency.Symbol)

		customCurrency, ok = provider.GetCustomCurrencyBySymbol("ROSETTA-057ab4")
		require.True(t, ok)
		require.Equal(t, "ROSETTA-057ab4", customCurrency.Symbol)
	})
}
