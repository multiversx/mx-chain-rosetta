// Package metachain reads block headers from the metachain. The aggregator only needs, for each metablock,
// its identity and the list of shard blocks that it notarizes.
package metachain

import (
	"context"
	"sort"
)

// NotarizedBlock is a shard block notarized by a metablock
type NotarizedBlock struct {
	Shard uint32
	Nonce uint64
	Hash  string
}

// Block is a metablock, reduced to what the aggregator needs
type Block struct {
	Nonce       uint64
	Hash        string
	PrevHash    string
	TimestampMs int64
	Epoch       uint32
	Round       uint64
	Notarized   []NotarizedBlock
}

// NotarizedOfShard returns the blocks of a given shard notarized by this metablock, sorted by nonce (duplicates removed)
func (block *Block) NotarizedOfShard(shard uint32) []NotarizedBlock {
	result := make([]NotarizedBlock, 0)

	for _, item := range block.Notarized {
		if item.Shard == shard {
			result = append(result, item)
		}
	}

	sort.SliceStable(result, func(i, j int) bool { return result[i].Nonce < result[j].Nonce })

	deduplicated := result[:0]
	for i, item := range result {
		if i > 0 && result[i-1].Nonce == item.Nonce && result[i-1].Hash == item.Hash {
			continue
		}

		deduplicated = append(deduplicated, item)
	}

	return deduplicated
}

// Status is the status of the metachain, as seen by the source
type Status struct {
	HighestFinalNonce uint64
	IsSynced          bool
	Version           string
}

// Source provides metachain data
type Source interface {
	// GetBlockByNonce returns the metablock with the given nonce
	GetBlockByNonce(ctx context.Context, nonce uint64) (*Block, error)
	// GetStatus returns the current status of the metachain
	GetStatus(ctx context.Context) (*Status, error)
	// GetEpochStartNonce returns the nonce of the first metablock of the given epoch
	GetEpochStartNonce(ctx context.Context, epoch uint32) (uint64, error)
	// GetNumShards returns the number of regular shards of the network
	GetNumShards(ctx context.Context) (uint32, error)
}
