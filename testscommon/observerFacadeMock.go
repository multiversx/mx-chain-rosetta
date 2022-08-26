package testscommon

import (
	"encoding/json"
	"fmt"

	"github.com/ElrondNetwork/elrond-go/sharding"
	"github.com/ElrondNetwork/elrond-proxy-go/common"
	"github.com/ElrondNetwork/elrond-proxy-go/data"
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
	MockTransactionsByHash map[string]*data.FullTransaction
	MockBlocks             []*data.Block

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
		MockTransactionsByHash: make(map[string]*data.FullTransaction),
		MockBlocks: []*data.Block{{
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

	data, err := json.Marshal(mock.MockGetResponse)
	if err != nil {
		return 500, err
	}

	err = json.Unmarshal(data, value)
	if err != nil {
		return 500, err
	}

	return 200, nil
}

// ComputeShardId -
func (mock *observerFacadeMock) ComputeShardId(pubKey []byte) (uint32, error) {
	if mock.MockNextError != nil {
		return 0, mock.MockNextError
	}

	shardCoordinator, err := sharding.NewMultiShardCoordinator(mock.MockNumShards, mock.MockSelfShard)
	if err != nil {
		return 0, err
	}

	shard := shardCoordinator.ComputeId(pubKey)
	return shard, nil
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
func (mock *observerFacadeMock) ComputeTransactionHash(tx *data.Transaction) (string, error) {
	if mock.MockNextError != nil {
		return "", mock.MockNextError
	}

	return mock.MockComputedTransactionHash, nil
}

// GetTransactionByHashAndSenderAddress -
func (mock *observerFacadeMock) GetTransactionByHashAndSenderAddress(hash string, sender string, withEvents bool) (*data.FullTransaction, int, error) {
	if mock.MockNextError != nil {
		return nil, 0, mock.MockNextError
	}

	transaction, ok := mock.MockTransactionsByHash[hash]
	if ok {
		return transaction, 0, mock.MockNextError
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
