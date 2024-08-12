package provider

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseTokenIdentifierIntoParts(t *testing.T) {
	t.Run("with fungible token", func(t *testing.T) {
		parts, err := parseTokenIdentifierIntoParts("ROSETTA-2c0a37")
		require.Nil(t, err)
		require.NotNil(t, parts)
		require.Equal(t, "ROSETTA", parts.ticker)
		require.Equal(t, "2c0a37", parts.randomSequence)
		require.Equal(t, "ROSETTA-2c0a37", parts.tickerWithRandomSequence)
		require.Equal(t, uint64(0), parts.nonce)
	})

	t.Run("with non-fungible token", func(t *testing.T) {
		parts, err := parseTokenIdentifierIntoParts("EXAMPLE-453bec-0a")
		require.Nil(t, err)
		require.NotNil(t, parts)
		require.Equal(t, "EXAMPLE", parts.ticker)
		require.Equal(t, "453bec", parts.randomSequence)
		require.Equal(t, "EXAMPLE-453bec", parts.tickerWithRandomSequence)
		require.Equal(t, uint64(10), parts.nonce)
	})

	t.Run("with invalid custom token identifier", func(t *testing.T) {
		parts, err := parseTokenIdentifierIntoParts("token")
		require.ErrorIs(t, err, errCannotParseTokenIdentifier)
		require.Nil(t, parts)
	})
}
