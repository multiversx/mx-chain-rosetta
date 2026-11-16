package backend

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/coinbase/rosetta-sdk-go/client"
	"github.com/coinbase/rosetta-sdk-go/types"
)

const userAgent = "mx-chain-rosetta-aggregator"

var errNoBackendUrls = errors.New("no URL provided for shard backend")

// httpBackend talks to a shard instance over HTTP. When several URLs are provided (replicas of the same shard instance),
// they are tried in order, until one of them can be reached.
type httpBackend struct {
	clients []*client.APIClient
}

// NewHttpBackend creates a backend for a shard instance reachable at the given URL(s)
func NewHttpBackend(urls []string, httpClient *http.Client) (*httpBackend, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	clients := make([]*client.APIClient, 0, len(urls))

	for _, url := range urls {
		url = strings.TrimRight(strings.TrimSpace(url), "/")
		if len(url) == 0 {
			continue
		}

		clients = append(clients, client.NewAPIClient(client.NewConfiguration(url, userAgent, httpClient)))
	}

	if len(clients) == 0 {
		return nil, errNoBackendUrls
	}

	return &httpBackend{clients: clients}, nil
}

// call invokes "fn" against the replicas, in order. A replica that answers (with a response or with a Rosetta error) ends the iteration.
// The next replica is only tried upon transport errors.
func call[T any](ctx context.Context, backend *httpBackend, fn func(apiClient *client.APIClient) (T, *types.Error, error)) (T, *types.Error, error) {
	var zero T
	var lastErr error

	for _, apiClient := range backend.clients {
		response, rosettaErr, err := fn(apiClient)
		if rosettaErr != nil {
			return zero, rosettaErr, err
		}
		if err == nil {
			return response, nil, nil
		}

		lastErr = err

		if ctx.Err() != nil {
			break
		}
	}

	return zero, nil, lastErr
}

// NetworkList calls /network/list
func (backend *httpBackend) NetworkList(ctx context.Context, request *types.MetadataRequest) (*types.NetworkListResponse, *types.Error, error) {
	return call(ctx, backend, func(apiClient *client.APIClient) (*types.NetworkListResponse, *types.Error, error) {
		return apiClient.NetworkAPI.NetworkList(ctx, request)
	})
}

// NetworkOptions calls /network/options
func (backend *httpBackend) NetworkOptions(ctx context.Context, request *types.NetworkRequest) (*types.NetworkOptionsResponse, *types.Error, error) {
	return call(ctx, backend, func(apiClient *client.APIClient) (*types.NetworkOptionsResponse, *types.Error, error) {
		return apiClient.NetworkAPI.NetworkOptions(ctx, request)
	})
}

// NetworkStatus calls /network/status
func (backend *httpBackend) NetworkStatus(ctx context.Context, request *types.NetworkRequest) (*types.NetworkStatusResponse, *types.Error, error) {
	return call(ctx, backend, func(apiClient *client.APIClient) (*types.NetworkStatusResponse, *types.Error, error) {
		return apiClient.NetworkAPI.NetworkStatus(ctx, request)
	})
}

// Block calls /block
func (backend *httpBackend) Block(ctx context.Context, request *types.BlockRequest) (*types.BlockResponse, *types.Error, error) {
	return call(ctx, backend, func(apiClient *client.APIClient) (*types.BlockResponse, *types.Error, error) {
		return apiClient.BlockAPI.Block(ctx, request)
	})
}

// AccountBalance calls /account/balance
func (backend *httpBackend) AccountBalance(ctx context.Context, request *types.AccountBalanceRequest) (*types.AccountBalanceResponse, *types.Error, error) {
	return call(ctx, backend, func(apiClient *client.APIClient) (*types.AccountBalanceResponse, *types.Error, error) {
		return apiClient.AccountAPI.AccountBalance(ctx, request)
	})
}

// MempoolTransaction calls /mempool/transaction
func (backend *httpBackend) MempoolTransaction(ctx context.Context, request *types.MempoolTransactionRequest) (*types.MempoolTransactionResponse, *types.Error, error) {
	return call(ctx, backend, func(apiClient *client.APIClient) (*types.MempoolTransactionResponse, *types.Error, error) {
		return apiClient.MempoolAPI.MempoolTransaction(ctx, request)
	})
}

// ConstructionCombine calls /construction/combine
func (backend *httpBackend) ConstructionCombine(ctx context.Context, request *types.ConstructionCombineRequest) (*types.ConstructionCombineResponse, *types.Error, error) {
	return call(ctx, backend, func(apiClient *client.APIClient) (*types.ConstructionCombineResponse, *types.Error, error) {
		return apiClient.ConstructionAPI.ConstructionCombine(ctx, request)
	})
}

// ConstructionDerive calls /construction/derive
func (backend *httpBackend) ConstructionDerive(ctx context.Context, request *types.ConstructionDeriveRequest) (*types.ConstructionDeriveResponse, *types.Error, error) {
	return call(ctx, backend, func(apiClient *client.APIClient) (*types.ConstructionDeriveResponse, *types.Error, error) {
		return apiClient.ConstructionAPI.ConstructionDerive(ctx, request)
	})
}

// ConstructionHash calls /construction/hash
func (backend *httpBackend) ConstructionHash(ctx context.Context, request *types.ConstructionHashRequest) (*types.TransactionIdentifierResponse, *types.Error, error) {
	return call(ctx, backend, func(apiClient *client.APIClient) (*types.TransactionIdentifierResponse, *types.Error, error) {
		return apiClient.ConstructionAPI.ConstructionHash(ctx, request)
	})
}

// ConstructionMetadata calls /construction/metadata
func (backend *httpBackend) ConstructionMetadata(ctx context.Context, request *types.ConstructionMetadataRequest) (*types.ConstructionMetadataResponse, *types.Error, error) {
	return call(ctx, backend, func(apiClient *client.APIClient) (*types.ConstructionMetadataResponse, *types.Error, error) {
		return apiClient.ConstructionAPI.ConstructionMetadata(ctx, request)
	})
}

// ConstructionParse calls /construction/parse
func (backend *httpBackend) ConstructionParse(ctx context.Context, request *types.ConstructionParseRequest) (*types.ConstructionParseResponse, *types.Error, error) {
	return call(ctx, backend, func(apiClient *client.APIClient) (*types.ConstructionParseResponse, *types.Error, error) {
		return apiClient.ConstructionAPI.ConstructionParse(ctx, request)
	})
}

// ConstructionPayloads calls /construction/payloads
func (backend *httpBackend) ConstructionPayloads(ctx context.Context, request *types.ConstructionPayloadsRequest) (*types.ConstructionPayloadsResponse, *types.Error, error) {
	return call(ctx, backend, func(apiClient *client.APIClient) (*types.ConstructionPayloadsResponse, *types.Error, error) {
		return apiClient.ConstructionAPI.ConstructionPayloads(ctx, request)
	})
}

// ConstructionPreprocess calls /construction/preprocess
func (backend *httpBackend) ConstructionPreprocess(ctx context.Context, request *types.ConstructionPreprocessRequest) (*types.ConstructionPreprocessResponse, *types.Error, error) {
	return call(ctx, backend, func(apiClient *client.APIClient) (*types.ConstructionPreprocessResponse, *types.Error, error) {
		return apiClient.ConstructionAPI.ConstructionPreprocess(ctx, request)
	})
}

// ConstructionPreprocessOperations calls /construction/preprocess_operations
func (backend *httpBackend) ConstructionPreprocessOperations(ctx context.Context, request *types.ConstructionPreprocessOperationsRequest) (*types.ConstructionPreprocessOperationsResponse, *types.Error, error) {
	return call(ctx, backend, func(apiClient *client.APIClient) (*types.ConstructionPreprocessOperationsResponse, *types.Error, error) {
		return apiClient.ConstructionAPI.ConstructionPreprocessOperations(ctx, request)
	})
}

// ConstructionSubmit calls /construction/submit
func (backend *httpBackend) ConstructionSubmit(ctx context.Context, request *types.ConstructionSubmitRequest) (*types.TransactionIdentifierResponse, *types.Error, error) {
	return call(ctx, backend, func(apiClient *client.APIClient) (*types.TransactionIdentifierResponse, *types.Error, error) {
		return apiClient.ConstructionAPI.ConstructionSubmit(ctx, request)
	})
}
