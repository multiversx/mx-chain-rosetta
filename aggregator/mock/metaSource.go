// Package mock holds test doubles for the aggregator: a fake metachain source, fake shard backends,
// and a simulator that generates a multi-shard network (shard chains that reconcile, cross-shard traffic, metachain notarization).
// Nothing in this package talks to a real network.
package mock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/multiversx/mx-chain-core-go/core"
	"github.com/multiversx/mx-chain-rosetta/aggregator/metachain"
)

// MetaSource is an in-memory metachain source
type MetaSource struct {
	mutex sync.RWMutex

	blocks            []*metachain.Block
	highestFinalNonce uint64
	numShards         uint32
	epochStartNonces  map[uint32]uint64

	// NextError, if set, is returned by all calls
	NextError error
	// NumBlockRequests counts the calls to GetBlockByNonce
	NumBlockRequests int
}

// NewMetaSource creates an in-memory metachain source
func NewMetaSource(numShards uint32) *MetaSource {
	return &MetaSource{
		blocks:           make([]*metachain.Block, 0),
		numShards:        numShards,
		epochStartNonces: make(map[uint32]uint64),
	}
}

// AddBlock appends a metablock (nonces must be consecutive, starting with 0)
func (source *MetaSource) AddBlock(block *metachain.Block) {
	source.mutex.Lock()
	defer source.mutex.Unlock()

	if block.Nonce != uint64(len(source.blocks)) {
		panic(fmt.Sprintf("mock.MetaSource: expected metablock %d, got %d", len(source.blocks), block.Nonce))
	}

	source.blocks = append(source.blocks, block)
}

// SetHighestFinalNonce sets the highest final nonce reported by the source
func (source *MetaSource) SetHighestFinalNonce(nonce uint64) {
	source.mutex.Lock()
	defer source.mutex.Unlock()

	source.highestFinalNonce = nonce
}

// SetEpochStartNonce sets the nonce of the first metablock of an epoch
func (source *MetaSource) SetEpochStartNonce(epoch uint32, nonce uint64) {
	source.mutex.Lock()
	defer source.mutex.Unlock()

	source.epochStartNonces[epoch] = nonce
}

// SetNextError makes all subsequent calls fail (pass nil to recover)
func (source *MetaSource) SetNextError(err error) {
	source.mutex.Lock()
	defer source.mutex.Unlock()

	source.NextError = err
}

// GetBlockByNonce returns a metablock
func (source *MetaSource) GetBlockByNonce(_ context.Context, nonce uint64) (*metachain.Block, error) {
	source.mutex.Lock()
	defer source.mutex.Unlock()

	source.NumBlockRequests++

	if source.NextError != nil {
		return nil, source.NextError
	}
	if nonce >= uint64(len(source.blocks)) {
		return nil, fmt.Errorf("metablock %d not found", nonce)
	}

	return source.blocks[nonce], nil
}

// GetStatus returns the status of the metachain
func (source *MetaSource) GetStatus(_ context.Context) (*metachain.Status, error) {
	source.mutex.RLock()
	defer source.mutex.RUnlock()

	if source.NextError != nil {
		return nil, source.NextError
	}

	return &metachain.Status{
		HighestFinalNonce: source.highestFinalNonce,
		IsSynced:          true,
		Version:           "mock",
	}, nil
}

// GetEpochStartNonce returns the nonce of the first metablock of an epoch
func (source *MetaSource) GetEpochStartNonce(_ context.Context, epoch uint32) (uint64, error) {
	source.mutex.RLock()
	defer source.mutex.RUnlock()

	if source.NextError != nil {
		return 0, source.NextError
	}

	nonce, ok := source.epochStartNonces[epoch]
	if !ok {
		return 0, fmt.Errorf("epoch %d not found", epoch)
	}

	return nonce, nil
}

// GetNumShards returns the number of shards
func (source *MetaSource) GetNumShards(_ context.Context) (uint32, error) {
	source.mutex.RLock()
	defer source.mutex.RUnlock()

	if source.NextError != nil {
		return 0, source.NextError
	}

	return source.numShards, nil
}

// NewObserverHandler creates an HTTP handler that mimics the REST API of a metachain observer (only the routes used by the aggregator),
// serving the data of the given source. It allows testing the real HTTP client of the aggregator against local data.
func NewObserverHandler(source *MetaSource) http.Handler {
	mux := http.NewServeMux()

	writeResponse := func(w http.ResponseWriter, statusCode int, data interface{}, err error) {
		envelope := map[string]interface{}{
			"data":  data,
			"error": "",
			"code":  "successful",
		}
		if err != nil {
			envelope["data"] = nil
			envelope["error"] = err.Error()
			envelope["code"] = "internal_issue"
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_ = json.NewEncoder(w).Encode(envelope)
	}

	mux.HandleFunc("/block/by-nonce/", func(w http.ResponseWriter, r *http.Request) {
		nonce, err := strconv.ParseUint(strings.TrimPrefix(r.URL.Path, "/block/by-nonce/"), 10, 64)
		if err != nil {
			writeResponse(w, http.StatusBadRequest, nil, errors.New("bad nonce"))
			return
		}

		block, err := source.GetBlockByNonce(r.Context(), nonce)
		if err != nil {
			writeResponse(w, http.StatusInternalServerError, nil, err)
			return
		}

		notarizedBlocks := make([]map[string]interface{}, 0, len(block.Notarized))
		for _, item := range block.Notarized {
			notarizedBlocks = append(notarizedBlocks, map[string]interface{}{
				"hash":  item.Hash,
				"nonce": item.Nonce,
				"shard": item.Shard,
			})
		}

		writeResponse(w, http.StatusOK, map[string]interface{}{
			"block": map[string]interface{}{
				"nonce":           block.Nonce,
				"round":           block.Round,
				"epoch":           block.Epoch,
				"shard":           core.MetachainShardId,
				"hash":            block.Hash,
				"prevBlockHash":   block.PrevHash,
				"timestamp":       block.TimestampMs / 1000,
				"timestampMs":     block.TimestampMs,
				"notarizedBlocks": notarizedBlocks,
			},
		}, nil)
	})

	mux.HandleFunc("/node/status", func(w http.ResponseWriter, r *http.Request) {
		status, err := source.GetStatus(r.Context())
		if err != nil {
			writeResponse(w, http.StatusInternalServerError, nil, err)
			return
		}

		writeResponse(w, http.StatusOK, map[string]interface{}{
			"metrics": map[string]interface{}{
				"erd_app_version":         status.Version,
				"erd_is_syncing":          0,
				"erd_highest_final_nonce": status.HighestFinalNonce,
			},
		}, nil)
	})

	mux.HandleFunc("/node/epoch-start/", func(w http.ResponseWriter, r *http.Request) {
		epoch, err := strconv.ParseUint(strings.TrimPrefix(r.URL.Path, "/node/epoch-start/"), 10, 32)
		if err != nil {
			writeResponse(w, http.StatusBadRequest, nil, errors.New("bad epoch"))
			return
		}

		nonce, err := source.GetEpochStartNonce(r.Context(), uint32(epoch))
		if err != nil {
			writeResponse(w, http.StatusInternalServerError, nil, err)
			return
		}

		writeResponse(w, http.StatusOK, map[string]interface{}{
			"epochStart": map[string]interface{}{"nonce": nonce},
		}, nil)
	})

	mux.HandleFunc("/network/config", func(w http.ResponseWriter, r *http.Request) {
		numShards, err := source.GetNumShards(r.Context())
		if err != nil {
			writeResponse(w, http.StatusInternalServerError, nil, err)
			return
		}

		writeResponse(w, http.StatusOK, map[string]interface{}{
			"config": map[string]interface{}{"erd_num_shards_without_meta": numShards},
		}, nil)
	})

	return mux
}
