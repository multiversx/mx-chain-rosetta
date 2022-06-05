package main

import "github.com/ElrondNetwork/elrond-proxy-go/data"

type disabledExternalStorageConnector struct {
}

func (connector *disabledExternalStorageConnector) GetTransactionsByAddress(address string) ([]data.DatabaseTransaction, error) {
	return make([]data.DatabaseTransaction, 0), nil
}

func (connector *disabledExternalStorageConnector) GetAtlasBlockByShardIDAndNonce(shardID uint32, nonce uint64) (data.AtlasBlock, error) {
	return data.AtlasBlock{}, nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (connector *disabledExternalStorageConnector) IsInterfaceNil() bool {
	return connector == nil
}
