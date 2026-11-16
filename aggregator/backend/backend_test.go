package backend_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/multiversx/mx-chain-rosetta/aggregator/backend"
	"github.com/multiversx/mx-chain-rosetta/aggregator/mock"
	"github.com/multiversx/mx-chain-rosetta/server/factory"
	shardTestscommon "github.com/multiversx/mx-chain-rosetta/testscommon"
	"github.com/stretchr/testify/require"
)

var testNetwork = &types.NetworkIdentifier{Blockchain: "MultiversX", Network: "testnet"}

// newShardInstanceServer starts a real shard instance (the services of this repository), on top of a mocked network provider
func newShardInstanceServer(t *testing.T, numRequests *atomic.Int64) *httptest.Server {
	controllers, err := factory.CreateControllers(shardTestscommon.NewNetworkProviderMock())
	require.NoError(t, err)

	router := server.NewRouter(controllers...)
	shardInstance := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if numRequests != nil {
			numRequests.Add(1)
		}

		router.ServeHTTP(w, r)
	}))

	t.Cleanup(shardInstance.Close)
	return shardInstance
}

func newDeadUrl() string {
	dead := httptest.NewServer(http.NotFoundHandler())
	url := dead.URL
	dead.Close()
	return url
}

func TestHttpBackend_AgainstRealShardInstance(t *testing.T) {
	ctx := context.Background()
	shardInstance := newShardInstanceServer(t, nil)

	shardBackend, err := backend.NewHttpBackend([]string{shardInstance.URL}, shardInstance.Client())
	require.NoError(t, err)

	networks, rosettaErr, err := shardBackend.NetworkList(ctx, &types.MetadataRequest{})
	require.NoError(t, err)
	require.Nil(t, rosettaErr)
	require.Equal(t, []*types.NetworkIdentifier{testNetwork}, networks.NetworkIdentifiers)

	options, rosettaErr, err := shardBackend.NetworkOptions(ctx, &types.NetworkRequest{NetworkIdentifier: testNetwork})
	require.NoError(t, err)
	require.Nil(t, rosettaErr)
	require.NotEmpty(t, options.Allow.OperationTypes)

	status, rosettaErr, err := shardBackend.NetworkStatus(ctx, &types.NetworkRequest{NetworkIdentifier: testNetwork})
	require.NoError(t, err)
	require.Nil(t, rosettaErr)
	require.NotNil(t, status.CurrentBlockIdentifier)

	// A Rosetta error returned by the shard instance is not a transport error: it is handed over, as it is.
	blockIndex := int64(12345)
	_, rosettaErr, _ = shardBackend.Block(ctx, &types.BlockRequest{
		NetworkIdentifier: testNetwork,
		BlockIdentifier:   &types.PartialBlockIdentifier{Index: &blockIndex},
	})
	require.NotNil(t, rosettaErr)
	require.Equal(t, int32(4), rosettaErr.Code)
	require.True(t, rosettaErr.Retriable)

	_, rosettaErr, _ = shardBackend.MempoolTransaction(ctx, &types.MempoolTransactionRequest{
		NetworkIdentifier:     testNetwork,
		TransactionIdentifier: &types.TransactionIdentifier{Hash: mock.FakeHash("unknown")},
	})
	require.NotNil(t, rosettaErr)
	require.Equal(t, int32(15), rosettaErr.Code)

	derived, rosettaErr, err := shardBackend.ConstructionDerive(ctx, &types.ConstructionDeriveRequest{
		NetworkIdentifier: testNetwork,
		PublicKey:         &types.PublicKey{Bytes: shardTestscommon.TestPubKeyAlice, CurveType: types.Edwards25519},
	})
	require.NoError(t, err)
	require.Nil(t, rosettaErr)
	require.Equal(t, shardTestscommon.TestAddressAlice, derived.AccountIdentifier.Address)
}

func TestHttpBackend_FailOver(t *testing.T) {
	ctx := context.Background()

	var numRequestsFirst, numRequestsSecond atomic.Int64
	first := newShardInstanceServer(t, &numRequestsFirst)
	second := newShardInstanceServer(t, &numRequestsSecond)

	call := func(shardBackend backend.ShardBackend) (*types.Error, error) {
		_, rosettaErr, err := shardBackend.NetworkList(ctx, &types.MetadataRequest{})
		return rosettaErr, err
	}

	t.Run("the first replica that can be reached answers", func(t *testing.T) {
		shardBackend, err := backend.NewHttpBackend([]string{newDeadUrl(), first.URL, second.URL}, nil)
		require.NoError(t, err)

		rosettaErr, err := call(shardBackend)
		require.NoError(t, err)
		require.Nil(t, rosettaErr)
		require.Equal(t, int64(1), numRequestsFirst.Load())
		require.Equal(t, int64(0), numRequestsSecond.Load())
	})

	t.Run("a Rosetta error does not trigger the fail-over", func(t *testing.T) {
		shardBackend, err := backend.NewHttpBackend([]string{first.URL, second.URL}, nil)
		require.NoError(t, err)

		before := numRequestsSecond.Load()
		blockIndex := int64(12345)
		_, rosettaErr, _ := shardBackend.Block(ctx, &types.BlockRequest{
			NetworkIdentifier: testNetwork,
			BlockIdentifier:   &types.PartialBlockIdentifier{Index: &blockIndex},
		})
		require.NotNil(t, rosettaErr)
		require.Equal(t, before, numRequestsSecond.Load())
	})

	t.Run("no replica can be reached", func(t *testing.T) {
		shardBackend, err := backend.NewHttpBackend([]string{newDeadUrl(), newDeadUrl()}, nil)
		require.NoError(t, err)

		rosettaErr, err := call(shardBackend)
		require.Nil(t, rosettaErr)
		require.Error(t, err)
	})

	t.Run("a replica that does not speak Rosetta is skipped", func(t *testing.T) {
		notRosetta := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		}))
		defer notRosetta.Close()

		shardBackend, err := backend.NewHttpBackend([]string{notRosetta.URL, second.URL}, nil)
		require.NoError(t, err)

		before := numRequestsSecond.Load()
		rosettaErr, err := call(shardBackend)
		require.NoError(t, err)
		require.Nil(t, rosettaErr)
		require.Equal(t, before+1, numRequestsSecond.Load())
	})

	t.Run("canceled context", func(t *testing.T) {
		shardBackend, err := backend.NewHttpBackend([]string{first.URL, second.URL}, nil)
		require.NoError(t, err)

		canceled, cancel := context.WithCancel(ctx)
		cancel()

		before := numRequestsSecond.Load()
		_, rosettaErr, err := shardBackend.NetworkList(canceled, &types.MetadataRequest{})
		require.Nil(t, rosettaErr)
		require.ErrorIs(t, err, context.Canceled)
		// The other replica is not tried.
		require.Equal(t, before, numRequestsSecond.Load())
	})
}

func TestNewHttpBackend_Errors(t *testing.T) {
	_, err := backend.NewHttpBackend(nil, nil)
	require.Error(t, err)

	_, err = backend.NewHttpBackend([]string{" ", ""}, nil)
	require.Error(t, err)
}

func TestSet(t *testing.T) {
	chain := mock.NewShardChain(0)

	set, err := backend.NewSet(map[uint32]backend.ShardBackend{
		2: mock.NewShardBackend(testNetwork, chain),
		0: mock.NewShardBackend(testNetwork, chain),
	})
	require.NoError(t, err)
	require.Equal(t, []uint32{0, 2}, set.Shards())

	_, ok := set.Get(2)
	require.True(t, ok)
	_, ok = set.Get(1)
	require.False(t, ok)

	_, err = backend.NewSet(nil)
	require.Error(t, err)

	_, err = backend.NewSet(map[uint32]backend.ShardBackend{0: nil})
	require.ErrorContains(t, err, "nil backend for shard 0")
}
