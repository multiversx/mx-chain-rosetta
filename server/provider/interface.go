package provider

import (
	"github.com/multiversx/mx-chain-core-go/data/transaction"
	"github.com/multiversx/mx-chain-proxy-go/common"
	"github.com/multiversx/mx-chain-proxy-go/data"
)

type observerFacade interface {
	CallGetRestEndPoint(baseUrl string, path string, value interface{}) (int, error)
	ComputeShardId(pubKey []byte) (uint32, error)
	SendTransaction(tx *data.Transaction) (int, string, error)
	ComputeTransactionHash(tx *data.Transaction) (string, error)
	GetTransactionByHashAndSenderAddress(hash string, sender string, withEvents bool) (*transaction.ApiTransactionResult, int, error)
	GetBlockByHash(shardID uint32, hash string, options common.BlockQueryOptions) (*data.BlockApiResponse, error)
	GetBlockByNonce(shardID uint32, nonce uint64, options common.BlockQueryOptions) (*data.BlockApiResponse, error)
}

type resourceApiResponseHandler interface {
	GetErrorMessage() string
}

type blocksCache interface {
	Get(key []byte) (value interface{}, ok bool)
	Put(key []byte, value interface{}, size int) (evicted bool)
	Len() int
	Keys() [][]byte
	Clear()
}
