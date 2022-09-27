package services

import (
	"math/big"

	"github.com/ElrondNetwork/elrond-go-core/data/api"
	"github.com/ElrondNetwork/elrond-go-core/data/transaction"
	"github.com/ElrondNetwork/elrond-proxy-go/data"
	"github.com/ElrondNetwork/rosetta/server/resources"
)

type NetworkProvider interface {
	IsOffline() bool
	GetBlockchainName() string
	GetNativeCurrency() resources.NativeCurrency
	GetObserverPubkey() string
	GetNetworkConfig() *resources.NetworkConfig
	GetGenesisBlockSummary() *resources.BlockSummary
	GetGenesisTimestamp() int64
	GetGenesisBalances() ([]*resources.GenesisBalance, error)
	GetNodeStatus() (*resources.AggregatedNodeStatus, error)
	GetBlockByNonce(nonce uint64) (*api.Block, error)
	GetBlockByHash(hash string) (*api.Block, error)
	GetAccount(address string) (*resources.AccountOnBlock, error)
	GetAccountNativeBalance(address string, options resources.AccountQueryOptions) (*resources.AccountNativeBalance, error)
	GetAccountESDTBalance(address string, tokenIdentifier string, options resources.AccountQueryOptions) (*resources.AccountESDTBalance, error)
	IsAddressObserved(address string) (bool, error)
	ConvertPubKeyToAddress(pubkey []byte) string
	ConvertAddressToPubKey(address string) ([]byte, error)
	SendTransaction(tx *data.Transaction) (string, error)
	ComputeTransactionHash(tx *data.Transaction) (string, error)
	ComputeReceiptHash(apiReceipt *transaction.ApiReceipt) (string, error)
	ComputeTransactionFeeForMoveBalance(tx *transaction.ApiTransactionResult) *big.Int
	GetMempoolTransactionByHash(hash string) (*transaction.ApiTransactionResult, error)
}
