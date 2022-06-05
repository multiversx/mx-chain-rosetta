package services

import (
	"github.com/ElrondNetwork/elrond-proxy-go/data"
	"github.com/ElrondNetwork/rosetta/server/resources"
)

type NetworkProvider interface {
	IsOffline() bool
	GetBlockchainName() string
	GetChainID() (string, error)
	GetNativeCurrency() resources.NativeCurrency
	GetObservedActualShard() uint32
	GetObserverPubkey() string

	GetNetworkConfig() (*resources.NetworkConfig, error)
	GetGenesisBlockSummary() (*resources.BlockSummary, error)
	GetLatestBlockSummary() (*resources.BlockSummary, error)
	GetBlockByNonce(nonce int64) (*data.Block, error)
	GetBlockByHash(hash string) (*data.Block, error)
	GetAccount(address string) (*data.Account, error)
	ConvertPubKeyToAddress(address []byte) (string, error)
	ConvertAddressToPubKey(address string) ([]byte, error)
	SendTransaction(tx *data.Transaction) (string, error)
	ComputeTransactionHash(tx *data.Transaction) (string, error)
	GetTransactionByHashFromPool(hash string) (*data.FullTransaction, bool)
}
