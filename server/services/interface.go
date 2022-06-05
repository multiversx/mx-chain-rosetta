package server

import "github.com/ElrondNetwork/elrond-proxy-go/data"

type NetworkProviderHandler interface {
	GetNetworkConfig() (*NetworkConfig, error)
	GetLatestBlockData() (*BlockData, error)
	GetBlockByNonce(nonce int64) (*data.Hyperblock, error)
	GetBlockByHash(hash string) (*data.Hyperblock, error)
	GetAccount(address string) (*data.Account, error)
	EncodeAddress(address []byte) (string, error)
	DecodeAddress(address string) ([]byte, error)
	SendTx(tx *data.Transaction) (string, error)
	ComputeTransactionHash(tx *data.Transaction) (string, error)
	GetTransactionByHashFromPool(txHash string) (*data.FullTransaction, bool)
}
