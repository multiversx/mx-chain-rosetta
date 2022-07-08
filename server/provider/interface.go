package provider

import (
	"github.com/ElrondNetwork/elrond-proxy-go/common"
	"github.com/ElrondNetwork/elrond-proxy-go/data"
)

type observerFacade interface {
	CallGetRestEndPoint(baseUrl string, path string, value interface{}) (int, error)
	ComputeShardId(pubKey []byte) (uint32, error)
	GetAccount(address string, options common.AccountQueryOptions) (*data.AccountModel, error)
	SendTransaction(tx *data.Transaction) (int, string, error)
	ComputeTransactionHash(tx *data.Transaction) (string, error)
	GetTransactionByHashAndSenderAddress(hash string, sender string, withEvents bool) (*data.FullTransaction, int, error)
	GetBlockByHash(shardID uint32, hash string, options common.BlockQueryOptions) (*data.BlockApiResponse, error)
	GetBlockByNonce(shardID uint32, nonce uint64, options common.BlockQueryOptions) (*data.BlockApiResponse, error)
}
