package provider

import (
	"errors"
	"testing"

	"github.com/multiversx/mx-chain-core-go/data/api"
	"github.com/multiversx/mx-chain-proxy-go/common"
	"github.com/multiversx/mx-chain-proxy-go/data"
	"github.com/multiversx/mx-chain-rosetta/server/resources"
	"github.com/multiversx/mx-chain-rosetta/testscommon"
	"github.com/stretchr/testify/require"
)

func TestSupernovaBlockWithoutExecutedSuccessor(t *testing.T) {
	facade := testscommon.NewObserverFacadeMock()
	args := createDefaultArgsNewNetworkProvider()
	args.ObserverFacade = facade
	provider, err := NewNetworkProvider(args)
	require.NoError(t, err)
	executedNonce := uint64(11)
	facade.CallGetRestEndPointCalled = func(_, _ string, value interface{}) (int, error) {
		value.(*resources.NodeStatusApiResponse).Data.Status = resources.NodeStatus{
			HighestFinalNonce: 20, LastExecutedNonce: executedNonce,
		}
		return 200, nil
	}
	block := api.Block{Nonce: 10, Hash: "ten", MiniBlocks: []*api.MiniBlock{}, LastExecutionResult: &api.ExecutionResult{HeaderNonce: 7},
		ExecutionResults: []*api.ExecutionResult{{HeaderNonce: 8}}}
	reads := 0
	facade.GetBlockByNonceCalled = func(_ uint32, nonce uint64, _ common.BlockQueryOptions) (*data.BlockApiResponse, error) {
		reads++
		require.Equal(t, uint64(10), nonce, "V3 must not fetch neighbouring blocks")
		return &data.BlockApiResponse{Data: data.BlockApiResponsePayload{Block: block}}, nil
	}
	facade.GetBlockByHashCalled = func(_ uint32, hash string, _ common.BlockQueryOptions) (*data.BlockApiResponse, error) {
		require.Equal(t, "ten", hash)
		return &data.BlockApiResponse{Data: data.BlockApiResponsePayload{Block: block}}, nil
	}
	for i := 0; i < 2; i++ {
		actual, getErr := provider.GetBlockByNonce(10)
		require.NoError(t, getErr)
		require.Equal(t, &block, actual)
	}
	require.Equal(t, 1, reads)
	actual, err := provider.GetBlockByHash("ten")
	require.NoError(t, err)
	require.Equal(t, &block, actual)
	executedNonce = 9
	_, err = provider.GetBlockByHash("ten")
	require.Error(t, err)
	_, err = provider.GetBlockByNonce(10)
	require.Error(t, err)
	_, err = provider.GetBlockByNonce(11)
	require.Error(t, err)
}

func TestMissingExecutionResponseIsNotCached(t *testing.T) {
	facade := testscommon.NewObserverFacadeMock()
	args := createDefaultArgsNewNetworkProvider()
	args.ObserverFacade = facade
	provider, err := NewNetworkProvider(args)
	require.NoError(t, err)
	facade.GetBlockByNonceCalled = func(uint32, uint64, common.BlockQueryOptions) (*data.BlockApiResponse, error) {
		return nil, errors.New("execution result unavailable")
	}
	_, err = provider.doGetBlockByNonce(10)
	require.Error(t, err)
	require.Zero(t, provider.blocksCache.Len())
}

func TestBlockReadinessByHashAndNonce(t *testing.T) {
	for _, test := range []struct {
		name            string
		final, executed uint64
		status          string
		statusErr       error
		wantError       bool
	}{
		{name: "execution boundary", final: 20, executed: 11},
		{name: "execution pending", final: 20, executed: 9, wantError: true},
		{name: "finality boundary", final: 15, executed: 15},
		{name: "finality pending", final: 14, executed: 15, wantError: true},
		{name: "reverted", final: 20, executed: 15, status: "reverted", wantError: true},
		{name: "status unavailable", statusErr: errors.New("offline"), wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			facade := testscommon.NewObserverFacadeMock()
			args := createDefaultArgsNewNetworkProvider()
			args.ObserverFacade = facade
			provider, err := NewNetworkProvider(args)
			require.NoError(t, err)
			facade.CallGetRestEndPointCalled = func(_, _ string, value interface{}) (int, error) {
				value.(*resources.NodeStatusApiResponse).Data.Status = resources.NodeStatus{
					HighestFinalNonce: test.final, LastExecutedNonce: test.executed,
				}
				return 200, test.statusErr
			}
			response := &data.BlockApiResponse{Data: data.BlockApiResponsePayload{Block: api.Block{
				Nonce: 10, Hash: "ten", Status: test.status, LastExecutionResult: &api.ExecutionResult{},
			}}}
			facade.GetBlockByNonceCalled = func(_ uint32, nonce uint64, _ common.BlockQueryOptions) (*data.BlockApiResponse, error) {
				require.Equal(t, uint64(10), nonce)
				return response, nil
			}
			facade.GetBlockByHashCalled = func(_ uint32, hash string, _ common.BlockQueryOptions) (*data.BlockApiResponse, error) {
				require.Equal(t, "ten", hash)
				return response, nil
			}
			for _, get := range []func() (*api.Block, error){
				func() (*api.Block, error) { return provider.GetBlockByNonce(10) },
				func() (*api.Block, error) { return provider.GetBlockByHash("ten") },
			} {
				block, getErr := get()
				if test.wantError {
					require.Error(t, getErr)
					require.Nil(t, block)
				} else {
					require.NoError(t, getErr)
					require.Equal(t, uint64(10), block.Nonce)
				}
			}
		})
	}
}

func TestExecutionUnavailableResponseCanBeRetried(t *testing.T) {
	facade := testscommon.NewObserverFacadeMock()
	args := createDefaultArgsNewNetworkProvider()
	args.ObserverFacade = facade
	provider, err := NewNetworkProvider(args)
	require.NoError(t, err)
	reads := 0
	facade.GetBlockByNonceCalled = func(uint32, uint64, common.BlockQueryOptions) (*data.BlockApiResponse, error) {
		reads++
		if reads == 1 {
			return &data.BlockApiResponse{Error: "execution result unavailable"}, nil
		}
		return &data.BlockApiResponse{Data: data.BlockApiResponsePayload{Block: api.Block{
			Nonce: 10, LastExecutionResult: &api.ExecutionResult{},
		}}}, nil
	}
	_, err = provider.doGetBlockByNonce(10)
	require.Error(t, err)
	require.Zero(t, provider.blocksCache.Len())
	for i := 0; i < 2; i++ {
		block, getErr := provider.doGetBlockByNonce(10)
		require.NoError(t, getErr)
		require.NotNil(t, block.LastExecutionResult)
	}
	require.Equal(t, 2, reads)
}

func TestScheduledNormalizationAtSupernovaBoundary(t *testing.T) {
	facade := testscommon.NewObserverFacadeMock()
	args := createDefaultArgsNewNetworkProvider()
	args.ObserverFacade = facade
	provider, err := NewNetworkProvider(args)
	require.NoError(t, err)
	var requested []uint64
	facade.GetBlockByNonceCalled = func(_ uint32, nonce uint64, _ common.BlockQueryOptions) (*data.BlockApiResponse, error) {
		requested = append(requested, nonce)
		block := api.Block{Nonce: nonce}
		if nonce == 11 {
			block.LastExecutionResult = &api.ExecutionResult{}
		}
		return &data.BlockApiResponse{Data: data.BlockApiResponsePayload{Block: block}}, nil
	}
	legacy := &api.Block{Nonce: 10}
	require.NoError(t, provider.simplifyBlockWithScheduledTransactions(legacy))
	require.Equal(t, []uint64{9, 11}, requested)
	require.Len(t, legacy.MiniBlocks, 2)
	firstV3 := &api.Block{Nonce: 11, LastExecutionResult: &api.ExecutionResult{}}
	require.NoError(t, provider.simplifyBlockWithScheduledTransactions(firstV3))
	require.Equal(t, []uint64{9, 11}, requested)
	require.Empty(t, firstV3.MiniBlocks)
}
