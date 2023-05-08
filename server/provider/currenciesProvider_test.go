package provider

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCurrenciesProvider(t *testing.T) {
	t.Parallel()

	provider := newCurrenciesProvider("XeGLD", []string{"ROSETTA-3a2edf", "ROSETTA-057ab4"})

	t.Run("get native", func(t *testing.T) {
		t.Parallel()

		nativeCurrency := provider.getNativeCurrency()
		require.Equal(t, "XeGLD", nativeCurrency.Symbol)
		require.Equal(t, int32(18), nativeCurrency.Decimals)
	})

	t.Run("get custom", func(t *testing.T) {
		t.Parallel()

		customCurrencies := provider.getCustomCurrencies()
		require.Equal(t, 2, len(customCurrencies))

		customCurrency, ok := provider.getCustomCurrencyBySymbol("ROSETTA-3a2edf")
		require.True(t, ok)
		require.Equal(t, "ROSETTA-3a2edf", customCurrency.Symbol)

		customCurrency, ok = provider.getCustomCurrencyBySymbol("ROSETTA-057ab4")
		require.True(t, ok)
		require.Equal(t, "ROSETTA-057ab4", customCurrency.Symbol)

		customCurrenciesSymbols := provider.getCustomCurrenciesSymbols()
		require.Equal(t, 2, len(customCurrenciesSymbols))
		require.Equal(t, "ROSETTA-3a2edf", customCurrenciesSymbols[0])
		require.Equal(t, "ROSETTA-057ab4", customCurrenciesSymbols[1])
	})

	t.Run("has custom", func(t *testing.T) {
		t.Parallel()

		require.True(t, provider.hasCustomCurrency("ROSETTA-3a2edf"))
		require.True(t, provider.hasCustomCurrency("ROSETTA-057ab4"))
		require.False(t, provider.hasCustomCurrency("FOO-abcdef"))
		require.False(t, provider.hasCustomCurrency("BAR-abcdef"))
	})
}
