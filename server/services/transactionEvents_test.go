package services

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEventESDT(t *testing.T) {
	t.Run("without nonce (fungible)", func(t *testing.T) {
		event := eventESDT{
			identifier:   "FOO-abcdef",
			nonceAsBytes: nil,
		}

		require.Equal(t, "FOO-abcdef", event.getBaseIdentifier())
		require.Equal(t, "FOO-abcdef", event.getExtendedIdentifier())

		event = eventESDT{
			identifier:   "FOO-abcdef",
			nonceAsBytes: numberToBytesWithoutLeadingZeros(0),
		}

		require.Equal(t, "FOO-abcdef", event.getBaseIdentifier())
		require.Equal(t, "FOO-abcdef", event.getExtendedIdentifier())
	})

	t.Run("with nonce (SFT, MetaESDT, NFT)", func(t *testing.T) {
		event := eventESDT{
			identifier:   "FOO-abcdef",
			nonceAsBytes: numberToBytesWithoutLeadingZeros(42),
		}

		require.Equal(t, "FOO-abcdef", event.getBaseIdentifier())
		require.Equal(t, "FOO-abcdef-2a", event.getExtendedIdentifier())
	})
}

func numberToBytesWithoutLeadingZeros(number uint64) []byte {
	data := make([]byte, 8)
	binary.BigEndian.PutUint64(data, number)
	data = bytes.Trim(data, "\x00")
	return data
}
