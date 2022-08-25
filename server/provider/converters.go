package provider

import "encoding/binary"

func blockNonceToBytes(nonce uint64) []byte {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data, nonce)
	return data
}
