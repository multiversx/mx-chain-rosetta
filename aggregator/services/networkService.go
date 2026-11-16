package services

import (
	"context"

	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/multiversx/mx-chain-rosetta/aggregator/backend"
)

type networkService struct {
	*base
}

// NewNetworkService creates the service behind the /network endpoints
func NewNetworkService(args ArgsNewServices) (server.NetworkAPIServicer, error) {
	base, err := newBase(args)
	if err != nil {
		return nil, err
	}

	return &networkService{base: base}, nil
}

// NetworkList implements the /network/list endpoint
func (service *networkService) NetworkList(_ context.Context, _ *types.MetadataRequest) (*types.NetworkListResponse, *types.Error) {
	return &types.NetworkListResponse{
		NetworkIdentifiers: []*types.NetworkIdentifier{service.Network},
	}, nil
}

// NetworkStatus implements the /network/status endpoint
func (service *networkService) NetworkStatus(_ context.Context, _ *types.NetworkRequest) (*types.NetworkStatusResponse, *types.Error) {
	if service.IsOffline {
		return nil, service.errFactory.newErr(ErrOfflineMode)
	}

	snapshot, err := service.Chain.Snapshot()
	if err != nil {
		return nil, service.errFactory.fromError(err)
	}

	currentTimestamp := service.GenesisTimestampMs
	if snapshot.Tip > 0 && snapshot.TipRecord != nil {
		currentTimestamp = snapshot.TipRecord.TimestampMs
	}

	currentIndex := int64(snapshot.Tip)
	targetIndex := int64(snapshot.MetaFinalNonce)
	if targetIndex < currentIndex {
		targetIndex = currentIndex
	}

	isSynced := snapshot.IsSynced()

	peers := make([]*types.Peer, 0)
	for _, shard := range snapshot.Shards {
		for _, peer := range shard.Peers {
			if peer == nil || len(peer.PeerID) == 0 {
				continue
			}

			metadata := make(map[string]interface{}, len(peer.Metadata)+1)
			for key, value := range peer.Metadata {
				metadata[key] = value
			}
			metadata["shard"] = shard.Shard

			peers = append(peers, &types.Peer{
				PeerID:   peer.PeerID,
				Metadata: metadata,
			})
		}
	}

	return &types.NetworkStatusResponse{
		CurrentBlockIdentifier: service.getBlockIdentifier(snapshot.Tip, snapshot.TipRecord),
		CurrentBlockTimestamp:  currentTimestamp,
		GenesisBlockIdentifier: service.GenesisBlockIdentifier,
		OldestBlockIdentifier:  service.getBlockIdentifier(snapshot.Oldest, snapshot.OldestRecord),
		SyncStatus: &types.SyncStatus{
			CurrentIndex: &currentIndex,
			TargetIndex:  &targetIndex,
			Synced:       &isSynced,
		},
		Peers: peers,
	}, nil
}

// NetworkOptions implements the /network/options endpoint.
// Operation types and statuses are the ones of the shard instances. Errors are the ones of the shard instances, plus the ones of the aggregator.
func (service *networkService) NetworkOptions(ctx context.Context, request *types.NetworkRequest) (*types.NetworkOptionsResponse, *types.Error) {
	shardOptions, typedErr := callAnyBackend(ctx, service.base, func(shardBackend backend.ShardBackend) (*types.NetworkOptionsResponse, *types.Error, error) {
		return shardBackend.NetworkOptions(ctx, &types.NetworkRequest{NetworkIdentifier: request.NetworkIdentifier})
	})
	if typedErr != nil {
		return nil, typedErr
	}
	if shardOptions == nil || shardOptions.Version == nil || shardOptions.Allow == nil {
		return nil, service.errFactory.newErrWithDetails(ErrBackendInconsistent, "incomplete network options")
	}

	aggregatorVersion := service.AggregatorVersion
	versionMetadata := map[string]interface{}{}
	if shardOptions.Version.MiddlewareVersion != nil {
		versionMetadata["shardMiddlewareVersion"] = *shardOptions.Version.MiddlewareVersion
	}

	allow := *shardOptions.Allow
	allow.Errors = service.errFactory.mergeErrors(shardOptions.Allow.Errors)
	allow.HistoricalBalanceLookup = true

	return &types.NetworkOptionsResponse{
		Version: &types.Version{
			RosettaVersion:    shardOptions.Version.RosettaVersion,
			NodeVersion:       shardOptions.Version.NodeVersion,
			MiddlewareVersion: &aggregatorVersion,
			Metadata:          versionMetadata,
		},
		Allow: &allow,
	}, nil
}
