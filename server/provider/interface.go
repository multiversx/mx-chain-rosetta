package provider

import (
	"github.com/ElrondNetwork/elrond-proxy-go/common"
	"github.com/ElrondNetwork/elrond-proxy-go/data"
)

type baseProcessor interface {
	CallGetRestEndPoint(address string, path string, value interface{}) (int, error)
	ComputeShardId(addressBuff []byte) (uint32, error)
}

type accountProcessor interface {
	GetAccount(address string, options common.AccountQueryOptions) (*data.AccountModel, error)
}

type transactionProcessor interface {
	SendTransaction(tx *data.Transaction) (int, string, error)
	ComputeTransactionHash(tx *data.Transaction) (string, error)
	GetTransactionByHashAndSenderAddress(txHash string, sndAddr string, withEvents bool) (*data.FullTransaction, int, error)
}

type blockProcessor interface {
	GetBlockByHash(shardID uint32, hash string, options common.BlockQueryOptions) (*data.BlockApiResponse, error)
	GetBlockByNonce(shardID uint32, nonce uint64, options common.BlockQueryOptions) (*data.BlockApiResponse, error)
}
