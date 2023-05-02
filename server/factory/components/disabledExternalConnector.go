package components

import (
	"github.com/multiversx/mx-chain-proxy-go/data"
)

// DisabledExternalStorageConnector is a no-operation external storage connector
type DisabledExternalStorageConnector struct {
}

// GetTransactionsByAddress returns nothing
func (connector *DisabledExternalStorageConnector) GetTransactionsByAddress(_ string) ([]data.DatabaseTransaction, error) {
	return make([]data.DatabaseTransaction, 0), nil
}

// GetAtlasBlockByShardIDAndNonce returns nothing
func (connector *DisabledExternalStorageConnector) GetAtlasBlockByShardIDAndNonce(_ uint32, _ uint64) (data.AtlasBlock, error) {
	return data.AtlasBlock{}, nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (connector *DisabledExternalStorageConnector) IsInterfaceNil() bool {
	return connector == nil
}
