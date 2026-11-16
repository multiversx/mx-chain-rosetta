// Package index holds the notarization index: for each metablock, the shard blocks it notarizes and the "frontier" of each shard
// (the last block of that shard notarized up to that metablock). The index is derived data: it can be deleted and rebuilt from the metachain.
package index

import "errors"

// ErrNotFound signals that a record is not in the index
var ErrNotFound = errors.New("record not found in the notarization index")

// ErrInconsistent signals that the metachain data does not continue the index (e.g. broken hash link, gap in the notarized shard blocks)
var ErrInconsistent = errors.New("inconsistent notarization data")

// BlockRef identifies a shard block
type BlockRef struct {
	Nonce uint64 `json:"nonce"`
	// Hash is empty when not known (e.g. for the genesis block of a shard)
	Hash string `json:"hash,omitempty"`
}

// ShardSpan describes what a metablock notarizes from a given shard
type ShardSpan struct {
	Shard uint32 `json:"shard"`
	// Frontier is the last block of the shard notarized up to (and including) this metablock
	Frontier BlockRef `json:"frontier"`
	// New holds the blocks of the shard first notarized by this metablock, in ascending order (possibly none)
	New []BlockRef `json:"new,omitempty"`
}

// Record is the index entry of one metablock
type Record struct {
	MetaNonce   uint64      `json:"metaNonce"`
	MetaHash    string      `json:"metaHash"`
	PrevHash    string      `json:"prevHash"`
	TimestampMs int64       `json:"timestampMs"`
	Epoch       uint32      `json:"epoch"`
	Round       uint64      `json:"round"`
	Shards      []ShardSpan `json:"shards"`
}

// Span returns the span of a given shard
func (record *Record) Span(shard uint32) (ShardSpan, bool) {
	for _, span := range record.Shards {
		if span.Shard == shard {
			return span, true
		}
	}

	return ShardSpan{}, false
}

// Frontier returns the frontier of a given shard
func (record *Record) Frontier(shard uint32) (BlockRef, bool) {
	span, ok := record.Span(shard)
	if !ok {
		return BlockRef{}, false
	}

	return span.Frontier, true
}

// Store persists the records of the index
type Store interface {
	// Put appends a record. Records are added in ascending order of the nonce, without gaps.
	Put(record *Record) error
	// Get returns the record of a metablock, or ErrNotFound
	Get(metaNonce uint64) (*Record, error)
	// GetNonceByHash returns the nonce of the metablock with the given hash, if indexed
	GetNonceByHash(metaHash string) (uint64, bool, error)
	// Bounds returns the nonces of the first and of the last record. "ok" is false if the store is empty.
	Bounds() (first uint64, last uint64, ok bool, err error)
	// Close releases the resources held by the store
	Close() error
}
