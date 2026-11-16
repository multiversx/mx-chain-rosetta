package metachain_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/multiversx/mx-chain-rosetta/aggregator/metachain"
	"github.com/multiversx/mx-chain-rosetta/aggregator/mock"
	"github.com/stretchr/testify/require"
)

func newTestMetaSource() *mock.MetaSource {
	source := mock.NewMetaSource(3)
	source.AddBlock(&metachain.Block{Nonce: 0, Hash: "aa00", PrevHash: "ff", TimestampMs: 1_600_000_000_000})
	source.AddBlock(&metachain.Block{
		Nonce:       1,
		Hash:        "aa01",
		PrevHash:    "aa00",
		TimestampMs: 1_600_000_006_000,
		Epoch:       4,
		Round:       17,
		Notarized: []metachain.NotarizedBlock{
			{Shard: 1, Nonce: 2, Hash: "b102"},
			{Shard: 0, Nonce: 1, Hash: "b001"},
			{Shard: 1, Nonce: 1, Hash: "b101"},
			// Duplicated entry:
			{Shard: 0, Nonce: 1, Hash: "b001"},
		},
	})
	source.SetHighestFinalNonce(1)
	source.SetEpochStartNonce(4, 1)
	return source
}

func TestObserverSource(t *testing.T) {
	ctx := context.Background()
	data := newTestMetaSource()

	observer := httptest.NewServer(mock.NewObserverHandler(data))
	defer observer.Close()

	// The trailing slash and the blank entry are tolerated.
	source, err := metachain.NewObserverSource([]string{" ", observer.URL + "/"}, observer.Client())
	require.NoError(t, err)

	t.Run("block by nonce", func(t *testing.T) {
		block, err := source.GetBlockByNonce(ctx, 1)
		require.NoError(t, err)

		expected, _ := data.GetBlockByNonce(ctx, 1)
		require.Equal(t, expected, block)

		// Notarized blocks of a shard: sorted, without duplicates.
		require.Equal(t, []metachain.NotarizedBlock{{Shard: 1, Nonce: 1, Hash: "b101"}, {Shard: 1, Nonce: 2, Hash: "b102"}}, block.NotarizedOfShard(1))
		require.Equal(t, []metachain.NotarizedBlock{{Shard: 0, Nonce: 1, Hash: "b001"}}, block.NotarizedOfShard(0))
		require.Empty(t, block.NotarizedOfShard(2))
	})

	t.Run("block not found", func(t *testing.T) {
		_, err := source.GetBlockByNonce(ctx, 7)
		require.ErrorContains(t, err, "cannot get metablock 7")
		require.ErrorContains(t, err, "not found")
	})

	t.Run("status", func(t *testing.T) {
		status, err := source.GetStatus(ctx)
		require.NoError(t, err)
		require.Equal(t, &metachain.Status{HighestFinalNonce: 1, IsSynced: true, Version: "mock"}, status)
	})

	t.Run("epoch start", func(t *testing.T) {
		nonce, err := source.GetEpochStartNonce(ctx, 4)
		require.NoError(t, err)
		require.Equal(t, uint64(1), nonce)

		_, err = source.GetEpochStartNonce(ctx, 5)
		require.ErrorContains(t, err, "cannot get start of epoch 5")
	})

	t.Run("number of shards", func(t *testing.T) {
		numShards, err := source.GetNumShards(ctx)
		require.NoError(t, err)
		require.Equal(t, uint32(3), numShards)
	})

	t.Run("errors of the observer are reported", func(t *testing.T) {
		data.SetNextError(errors.New("observer is syncing"))
		defer data.SetNextError(nil)

		_, err := source.GetStatus(ctx)
		require.ErrorContains(t, err, "observer is syncing")
		_, err = source.GetNumShards(ctx)
		require.ErrorContains(t, err, "observer is syncing")
	})

	t.Run("canceled context", func(t *testing.T) {
		canceled, cancel := context.WithCancel(ctx)
		cancel()

		_, err := source.GetStatus(canceled)
		require.ErrorIs(t, err, context.Canceled)
	})
}

func TestObserverSource_FailOver(t *testing.T) {
	ctx := context.Background()
	data := newTestMetaSource()

	observer := httptest.NewServer(mock.NewObserverHandler(data))
	defer observer.Close()

	dead := httptest.NewServer(http.NotFoundHandler())
	deadUrl := dead.URL
	dead.Close()

	// An observer that answers, but with garbage.
	garbage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<html>bad gateway</html>"))
	}))
	defer garbage.Close()

	source, err := metachain.NewObserverSource([]string{deadUrl, garbage.URL, observer.URL}, nil)
	require.NoError(t, err)

	status, err := source.GetStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(1), status.HighestFinalNonce)

	// When no observer can answer, the last error is returned.
	source, err = metachain.NewObserverSource([]string{deadUrl, garbage.URL}, nil)
	require.NoError(t, err)

	_, err = source.GetStatus(ctx)
	require.ErrorContains(t, err, "cannot decode response")
}

func TestObserverSource_RejectsUnexpectedData(t *testing.T) {
	ctx := context.Background()

	newSourceReturning := func(t *testing.T, statusCode int, body string) metachain.Source {
		observer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(statusCode)
			_, _ = fmt.Fprint(w, body)
		}))
		t.Cleanup(observer.Close)

		source, err := metachain.NewObserverSource([]string{observer.URL}, nil)
		require.NoError(t, err)
		return source
	}

	t.Run("block of a regular shard (not a metachain observer)", func(t *testing.T) {
		source := newSourceReturning(t, http.StatusOK, `{"data":{"block":{"nonce":5,"shard":1,"hash":"aa"}},"error":"","code":"successful"}`)
		_, err := source.GetBlockByNonce(ctx, 5)
		require.ErrorContains(t, err, "is it a metachain observer?")
	})

	t.Run("another block than the requested one", func(t *testing.T) {
		source := newSourceReturning(t, http.StatusOK, `{"data":{"block":{"nonce":6,"shard":4294967295,"hash":"aa"}},"error":"","code":"successful"}`)
		_, err := source.GetBlockByNonce(ctx, 5)
		require.ErrorContains(t, err, "returned block 6")
	})

	t.Run("block without hash", func(t *testing.T) {
		source := newSourceReturning(t, http.StatusOK, `{"data":{"block":{"nonce":5,"shard":4294967295}},"error":"","code":"successful"}`)
		_, err := source.GetBlockByNonce(ctx, 5)
		require.ErrorContains(t, err, "without hash")
	})

	t.Run("timestamp in seconds only (older nodes)", func(t *testing.T) {
		source := newSourceReturning(t, http.StatusOK, `{"data":{"block":{"nonce":5,"shard":4294967295,"hash":"aa","timestamp":1600000000}},"error":"","code":"successful"}`)
		block, err := source.GetBlockByNonce(ctx, 5)
		require.NoError(t, err)
		require.Equal(t, int64(1_600_000_000_000), block.TimestampMs)
	})

	t.Run("error status without error message", func(t *testing.T) {
		source := newSourceReturning(t, http.StatusServiceUnavailable, `{}`)
		_, err := source.GetStatus(ctx)
		require.ErrorContains(t, err, "unexpected status code 503")
	})

	t.Run("network config without the number of shards", func(t *testing.T) {
		source := newSourceReturning(t, http.StatusOK, `{"data":{"config":{}},"error":"","code":"successful"}`)
		_, err := source.GetNumShards(ctx)
		require.ErrorContains(t, err, "number of shards is missing")
	})
}

func TestNewObserverSource_Errors(t *testing.T) {
	_, err := metachain.NewObserverSource(nil, nil)
	require.Error(t, err)

	_, err = metachain.NewObserverSource([]string{"", "  "}, nil)
	require.Error(t, err)
}
