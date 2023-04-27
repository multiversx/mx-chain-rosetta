package factory

import (
	"math/big"

	"github.com/multiversx/mx-chain-core-go/data/api"
	"github.com/multiversx/mx-chain-core-go/data/transaction"
	"github.com/multiversx/mx-chain-proxy-go/data"
	"github.com/multiversx/mx-chain-rosetta/server/resources"
)

// NetworkProvider defines the actions that need to be performed by the component that handles network data fetching
type NetworkProvider interface {
	IsOffline() bool
	GetBlockchainName() string
	GetNativeCurrency() resources.NativeCurrency
	GetNetworkConfig() *resources.NetworkConfig
	GetGenesisBlockSummary() *resources.BlockSummary
	GetGenesisTimestamp() int64
	GetGenesisBalances() ([]*resources.GenesisBalance, error)
	GetNodeStatus() (*resources.AggregatedNodeStatus, error)
	GetBlockByNonce(nonce uint64) (*api.Block, error)
	GetBlockByHash(hash string) (*api.Block, error)
	GetAccount(address string) (*resources.AccountOnBlock, error)
	GetAccountNativeBalance(address string, options resources.AccountQueryOptions) (*resources.AccountOnBlock, error)
	GetAccountESDTBalance(address string, tokenIdentifier string, options resources.AccountQueryOptions) (*resources.AccountESDTBalance, error)
	IsAddressObserved(address string) (bool, error)
	ConvertPubKeyToAddress(pubkey []byte) string
	ConvertAddressToPubKey(address string) ([]byte, error)
	SendTransaction(tx *data.Transaction) (string, error)
	ComputeTransactionHash(tx *data.Transaction) (string, error)
	ComputeReceiptHash(apiReceipt *transaction.ApiReceipt) (string, error)
	ComputeTransactionFeeForMoveBalance(tx *transaction.ApiTransactionResult) *big.Int
	GetMempoolTransactionByHash(hash string) (*transaction.ApiTransactionResult, error)
	LogDescription()
}
