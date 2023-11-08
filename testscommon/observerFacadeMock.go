package testscommon

import (
	"encoding/json"
	"fmt"

	"github.com/multiversx/mx-chain-core-go/data/api"
	"github.com/multiversx/mx-chain-core-go/data/transaction"
	"github.com/multiversx/mx-chain-go/sharding"
	"github.com/multiversx/mx-chain-proxy-go/common"
	"github.com/multiversx/mx-chain-proxy-go/data"
)

type observerFacadeMock struct {
	MockNumShards               uint32
	MockSelfShard               uint32
	MockGetResponse             interface{}
	MockAccount                 *data.AccountModel
	MockComputedTransactionHash string
	MockNextError               error
	MockNextApiResponseError    string

	MockAccountsByAddress  map[string]*data.Account
	MockTransactionsByHash map[string]*transaction.ApiTransactionResult
	MockBlocks             []*api.Block

	GetBlockByNonceCalled     func(shardID uint32, nonce uint64, options common.BlockQueryOptions) (*data.BlockApiResponse, error)
	GetBlockByHashCalled      func(shardID uint32, hash string, options common.BlockQueryOptions) (*data.BlockApiResponse, error)
	CallGetRestEndPointCalled func(baseUrl string, path string, value interface{}) (int, error)
	SendTransactionCalled     func(tx *data.Transaction) (int, string, error)

	RecordedBaseUrl string
	RecordedPath    string
}

// NewObserverFacadeMock -
func NewObserverFacadeMock() *observerFacadeMock {
	return &observerFacadeMock{
		MockNumShards: 3,
		MockSelfShard: 0,

		MockComputedTransactionHash: emptyHash,
		MockNextError:               nil,

		MockAccountsByAddress:  make(map[string]*data.Account),
		MockTransactionsByHash: make(map[string]*transaction.ApiTransactionResult),
		MockBlocks: []*api.Block{{
			Nonce: 0,
			Hash:  "0000",
		}},
	}
}

// CallGetRestEndPoint -
func (mock *observerFacadeMock) CallGetRestEndPoint(baseUrl string, path string, value interface{}) (int, error) {
	mock.RecordedBaseUrl = baseUrl
	mock.RecordedPath = path

	if mock.CallGetRestEndPointCalled != nil {
		return mock.CallGetRestEndPointCalled(baseUrl, path, value)
	}

	if mock.MockNextError != nil {
		return 0, mock.MockNextError
	}

	marshalledData, err := json.Marshal(mock.MockGetResponse)
	if err != nil {
		return 500, err
	}

	err = json.Unmarshal(marshalledData, value)
	if err != nil {
		return 500, err
	}

	return 200, nil
}

// ComputeShardId -
func (mock *observerFacadeMock) ComputeShardId(pubKey []byte) uint32 {
	shardCoordinator, err := sharding.NewMultiShardCoordinator(mock.MockNumShards, mock.MockSelfShard)
	if err != nil {
		return 0
	}

	shard := shardCoordinator.ComputeId(pubKey)
	return shard
}

// SendTransaction -
func (mock *observerFacadeMock) SendTransaction(tx *data.Transaction) (int, string, error) {
	if mock.MockNextError != nil {
		return 500, "", mock.MockNextError
	}

	if mock.SendTransactionCalled != nil {
		return mock.SendTransactionCalled(tx)
	}

	return 200, mock.MockComputedTransactionHash, nil
}

// ComputeTransactionHash -
func (mock *observerFacadeMock) ComputeTransactionHash(_ *data.Transaction) (string, error) {
	if mock.MockNextError != nil {
		return "", mock.MockNextError
	}

	return mock.MockComputedTransactionHash, nil
}

// GetTransactionByHashAndSenderAddress -
func (mock *observerFacadeMock) GetTransactionByHashAndSenderAddress(hash string, _ string, _ bool) (*transaction.ApiTransactionResult, int, error) {
	if mock.MockNextError != nil {
		return nil, 0, mock.MockNextError
	}

	transactionObj, ok := mock.MockTransactionsByHash[hash]
	if ok {
		return transactionObj, 0, mock.MockNextError
	}

	return nil, 0, fmt.Errorf("transaction %s not found", hash)
}

// GetBlockByHash -
func (mock *observerFacadeMock) GetBlockByHash(shardID uint32, hash string, options common.BlockQueryOptions) (*data.BlockApiResponse, error) {
	if mock.GetBlockByHashCalled != nil {
		return mock.GetBlockByHashCalled(shardID, hash, options)
	}

	if mock.MockNextError != nil {
		return nil, mock.MockNextError
	}

	if mock.MockNextApiResponseError != "" {
		return &data.BlockApiResponse{
			Code:  data.ReturnCodeInternalError,
			Error: mock.MockNextApiResponseError,
		}, nil
	}

	for _, block := range mock.MockBlocks {
		if block.Hash == hash {
			return &data.BlockApiResponse{
				Code: data.ReturnCodeSuccess,
				Data: data.BlockApiResponsePayload{Block: *block},
			}, nil
		}
	}

	return &data.BlockApiResponse{
		Code:  data.ReturnCodeInternalError,
		Error: "block not found",
	}, nil
}

// GetBlockByNonce -
func (mock *observerFacadeMock) GetBlockByNonce(shardID uint32, nonce uint64, options common.BlockQueryOptions) (*data.BlockApiResponse, error) {
	if mock.GetBlockByNonceCalled != nil {
		return mock.GetBlockByNonceCalled(shardID, nonce, options)
	}

	if mock.MockNextError != nil {
		return nil, mock.MockNextError
	}

	if mock.MockNextApiResponseError != "" {
		return &data.BlockApiResponse{
			Code:  data.ReturnCodeInternalError,
			Error: mock.MockNextApiResponseError,
		}, nil
	}

	for _, block := range mock.MockBlocks {
		if block.Nonce == nonce {
			return &data.BlockApiResponse{
				Code: data.ReturnCodeSuccess,
				Data: data.BlockApiResponsePayload{Block: *block},
			}, nil
		}
	}

	return &data.BlockApiResponse{
		Code:  data.ReturnCodeInternalError,
		Error: "block not found",
	}, nil
}
