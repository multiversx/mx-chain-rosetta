package services

import (
	"testing"

	"github.com/ElrondNetwork/rosetta/server/resources"
	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/stretchr/testify/require"
)

func TestBlockIdentifierToAccountQueryOptions(t *testing.T) {
	t.Run("with block identifier", func(t *testing.T) {
		options, err := blockIdentifierToAccountQueryOptions(nil)
		require.Equal(t, resources.NewAccountQueryOptionsOnFinalBlock(), options)
		require.Nil(t, err)
	})

	t.Run("with index", func(t *testing.T) {
		options, err := blockIdentifierToAccountQueryOptions(&types.PartialBlockIdentifier{Index: types.Int64(7)})
		require.Equal(t, resources.NewAccountQueryOptionsWithBlockNonce(7), options)
		require.Nil(t, err)
	})

	t.Run("with hash", func(t *testing.T) {
		options, err := blockIdentifierToAccountQueryOptions(&types.PartialBlockIdentifier{Hash: types.String("aabbccdd")})
		require.Equal(t, resources.NewAccountQueryOptionsWithBlockHash([]byte{0xaa, 0xbb, 0xcc, 0xdd}), options)
		require.Nil(t, err)
	})

	t.Run("with bad hash", func(t *testing.T) {
		options, err := blockIdentifierToAccountQueryOptions(&types.PartialBlockIdentifier{Hash: types.String("bad hash")})
		require.Equal(t, resources.AccountQueryOptions{}, options)
		require.ErrorContains(t, err, "encoding/hex: invalid byte")
	})
}
