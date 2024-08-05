package provider

import (
	"testing"

	"github.com/multiversx/mx-chain-rosetta/server/resources"
	"github.com/stretchr/testify/require"
)

func TestCurrenciesProvider(t *testing.T) {
	t.Parallel()

	provider := newCurrenciesProvider("XeGLD", []resources.Currency{
		{Symbol: "ROSETTA-3a2edf", Decimals: 2},
		{Symbol: "ROSETTA-057ab4", Decimals: 2},
	})

	t.Run("get native", func(t *testing.T) {
		t.Parallel()

		nativeCurrency := provider.GetNativeCurrency()
		require.Equal(t, "XeGLD", nativeCurrency.Symbol)
		require.Equal(t, int32(18), nativeCurrency.Decimals)
	})

	t.Run("get custom", func(t *testing.T) {
		t.Parallel()

		customCurrencies := provider.GetCustomCurrencies()
		require.Equal(t, 2, len(customCurrencies))

		customCurrency, ok := provider.GetCustomCurrencyBySymbol("ROSETTA-3a2edf")
		require.True(t, ok)
		require.Equal(t, "ROSETTA-3a2edf", customCurrency.Symbol)

		customCurrency, ok = provider.GetCustomCurrencyBySymbol("ROSETTA-057ab4")
		require.True(t, ok)
		require.Equal(t, "ROSETTA-057ab4", customCurrency.Symbol)

		customCurrenciesSymbols := provider.GetCustomCurrenciesSymbols()
		require.Equal(t, 2, len(customCurrenciesSymbols))
		require.Equal(t, "ROSETTA-3a2edf", customCurrenciesSymbols[0])
		require.Equal(t, "ROSETTA-057ab4", customCurrenciesSymbols[1])
	})

	t.Run("has custom", func(t *testing.T) {
		t.Parallel()

		require.True(t, provider.HasCustomCurrency("ROSETTA-3a2edf"))
		require.True(t, provider.HasCustomCurrency("ROSETTA-057ab4"))
		require.False(t, provider.HasCustomCurrency("FOO-abcdef"))
		require.False(t, provider.HasCustomCurrency("BAR-abcdef"))
	})
}
