package components

import (
	"github.com/ElrondNetwork/elrond-proxy-go/data"
)

// DisabledExternalStorageConnector is a no-operation external storage connector
type DisabledExternalStorageConnector struct {
}

// GetTransactionsByAddress returns nothing
func (connector *DisabledExternalStorageConnector) GetTransactionsByAddress(address string) ([]data.DatabaseTransaction, error) {
	return make([]data.DatabaseTransaction, 0), nil
}

// GetAtlasBlockByShardIDAndNonce returns nothing
func (connector *DisabledExternalStorageConnector) GetAtlasBlockByShardIDAndNonce(shardID uint32, nonce uint64) (data.AtlasBlock, error) {
	return data.AtlasBlock{}, nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (connector *DisabledExternalStorageConnector) IsInterfaceNil() bool {
	return connector == nil
}
