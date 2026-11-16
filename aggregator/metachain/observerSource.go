package metachain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/multiversx/mx-chain-core-go/core"
	"github.com/multiversx/mx-chain-core-go/data/api"
)

const (
	pathBlockByNonce  = "/block/by-nonce/%d"
	pathNodeStatus    = "/node/status"
	pathEpochStart    = "/node/epoch-start/%d"
	pathNetworkConfig = "/network/config"

	// Responses that hold block headers only are small. This is a generous upper bound.
	maxResponseBodySize = 64 << 20
)

var errNoObserverUrls = errors.New("no metachain observer URL provided")

type observerSource struct {
	urls       []string
	httpClient *http.Client
}

// NewObserverSource creates a metachain source that reads from the REST API of one (or more, for fail-over) metachain observers
func NewObserverSource(urls []string, httpClient *http.Client) (*observerSource, error) {
	cleanedUrls := make([]string, 0, len(urls))
	for _, url := range urls {
		url = strings.TrimRight(strings.TrimSpace(url), "/")
		if len(url) > 0 {
			cleanedUrls = append(cleanedUrls, url)
		}
	}

	if len(cleanedUrls) == 0 {
		return nil, errNoObserverUrls
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &observerSource{
		urls:       cleanedUrls,
		httpClient: httpClient,
	}, nil
}

type envelope struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

type blockResponse struct {
	envelope
	Data struct {
		Block api.Block `json:"block"`
	} `json:"data"`
}

type nodeStatusResponse struct {
	envelope
	Data struct {
		Metrics struct {
			Version           string `json:"erd_app_version"`
			IsSyncing         int    `json:"erd_is_syncing"`
			HighestFinalNonce uint64 `json:"erd_highest_final_nonce"`
		} `json:"metrics"`
	} `json:"data"`
}

type epochStartResponse struct {
	envelope
	Data struct {
		EpochStart struct {
			Nonce uint64 `json:"nonce"`
		} `json:"epochStart"`
	} `json:"data"`
}

type networkConfigResponse struct {
	envelope
	Data struct {
		Config struct {
			NumShards *uint32 `json:"erd_num_shards_without_meta"`
		} `json:"config"`
	} `json:"data"`
}

// GetBlockByNonce returns the metablock with the given nonce
func (source *observerSource) GetBlockByNonce(ctx context.Context, nonce uint64) (*Block, error) {
	response := &blockResponse{}
	err := source.get(ctx, fmt.Sprintf(pathBlockByNonce, nonce), response, &response.envelope)
	if err != nil {
		return nil, fmt.Errorf("cannot get metablock %d: %w", nonce, err)
	}

	block := &response.Data.Block
	if block.Shard != core.MetachainShardId {
		return nil, fmt.Errorf("cannot get metablock %d: the observer returned a block of shard %d (is it a metachain observer?)", nonce, block.Shard)
	}
	if block.Nonce != nonce {
		return nil, fmt.Errorf("cannot get metablock %d: the observer returned block %d", nonce, block.Nonce)
	}
	if len(block.Hash) == 0 {
		return nil, fmt.Errorf("cannot get metablock %d: the observer returned a block without hash", nonce)
	}

	return convertBlock(block), nil
}

func convertBlock(block *api.Block) *Block {
	notarized := make([]NotarizedBlock, 0, len(block.NotarizedBlocks))
	for _, item := range block.NotarizedBlocks {
		if item == nil {
			continue
		}

		notarized = append(notarized, NotarizedBlock{
			Shard: item.Shard,
			Nonce: item.Nonce,
			Hash:  item.Hash,
		})
	}

	timestampMs := block.TimestampMs
	if timestampMs <= 0 {
		timestampMs = block.Timestamp * 1000
	}

	return &Block{
		Nonce:       block.Nonce,
		Hash:        block.Hash,
		PrevHash:    block.PrevBlockHash,
		TimestampMs: timestampMs,
		Epoch:       block.Epoch,
		Round:       block.Round,
		Notarized:   notarized,
	}
}

// GetStatus returns the current status of the metachain
func (source *observerSource) GetStatus(ctx context.Context) (*Status, error) {
	response := &nodeStatusResponse{}
	err := source.get(ctx, pathNodeStatus, response, &response.envelope)
	if err != nil {
		return nil, fmt.Errorf("cannot get metachain status: %w", err)
	}

	return &Status{
		HighestFinalNonce: response.Data.Metrics.HighestFinalNonce,
		IsSynced:          response.Data.Metrics.IsSyncing == 0,
		Version:           response.Data.Metrics.Version,
	}, nil
}

// GetEpochStartNonce returns the nonce of the first metablock of the given epoch
func (source *observerSource) GetEpochStartNonce(ctx context.Context, epoch uint32) (uint64, error) {
	response := &epochStartResponse{}
	err := source.get(ctx, fmt.Sprintf(pathEpochStart, epoch), response, &response.envelope)
	if err != nil {
		return 0, fmt.Errorf("cannot get start of epoch %d: %w", epoch, err)
	}

	return response.Data.EpochStart.Nonce, nil
}

// GetNumShards returns the number of regular shards of the network
func (source *observerSource) GetNumShards(ctx context.Context) (uint32, error) {
	response := &networkConfigResponse{}
	err := source.get(ctx, pathNetworkConfig, response, &response.envelope)
	if err != nil {
		return 0, fmt.Errorf("cannot get network config: %w", err)
	}
	if response.Data.Config.NumShards == nil {
		return 0, errors.New("cannot get network config: number of shards is missing")
	}

	return *response.Data.Config.NumShards, nil
}

// get performs a GET request against the observers, in order, until one of them answers.
func (source *observerSource) get(ctx context.Context, path string, response interface{}, responseEnvelope *envelope) error {
	var lastErr error

	for _, baseUrl := range source.urls {
		// Reset the envelope, since "response" is reused between attempts.
		*responseEnvelope = envelope{}

		statusCode, err := source.getFromOne(ctx, baseUrl+path, response)
		if err == nil && len(responseEnvelope.Error) > 0 {
			err = errors.New(responseEnvelope.Error)
		}
		if err == nil && statusCode != http.StatusOK {
			err = fmt.Errorf("unexpected status code %d", statusCode)
		}
		if err == nil {
			return nil
		}

		lastErr = err

		if ctx.Err() != nil {
			break
		}
	}

	return lastErr
}

func (source *observerSource) getFromOne(ctx context.Context, url string, response interface{}) (int, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}

	httpResponse, err := source.httpClient.Do(request)
	if err != nil {
		return 0, err
	}

	defer func() {
		_ = httpResponse.Body.Close()
	}()

	body, err := io.ReadAll(io.LimitReader(httpResponse.Body, maxResponseBodySize))
	if err != nil {
		return httpResponse.StatusCode, err
	}

	// The observer returns a JSON envelope (holding an error message) on failures, as well.
	err = json.Unmarshal(body, response)
	if err != nil {
		return httpResponse.StatusCode, fmt.Errorf("status code %d, cannot decode response: %w", httpResponse.StatusCode, err)
	}

	return httpResponse.StatusCode, nil
}
