package services

import (
	"math/big"

	"github.com/ElrondNetwork/elrond-go-core/data/transaction"
	"github.com/ElrondNetwork/elrond-proxy-go/data"
	"github.com/ElrondNetwork/rosetta/server/resources"
)

type NetworkProvider interface {
	IsOffline() bool
	GetBlockchainName() string
	GetChainID() string
	GetNativeCurrency() resources.NativeCurrency
	GetObserverPubkey() string
	GetNetworkConfig() *resources.NetworkConfig
	GetGenesisBlockSummary() *resources.BlockSummary
	GetGenesisTimestamp() int64
	GetGenesisBalances() ([]*resources.GenesisBalance, error)
	GetLatestBlockSummary() (*resources.BlockSummary, error)
	GetBlockByNonce(nonce uint64) (*data.Block, error)
	GetBlockByHash(hash string) (*data.Block, error)
	GetAccount(address string) (*resources.AccountModel, error)
	IsAddressObserved(address string) (bool, error)
	ConvertPubKeyToAddress(pubkey []byte) string
	ConvertAddressToPubKey(address string) ([]byte, error)
	SendTransaction(tx *data.Transaction) (string, error)
	ComputeTransactionHash(tx *data.Transaction) (string, error)
	ComputeReceiptHash(apiReceipt *transaction.ApiReceipt) (string, error)
	ComputeTransactionFeeForMoveBalance(tx *data.FullTransaction) *big.Int
	GetMempoolTransactionByHash(hash string) (*data.FullTransaction, error)
}
