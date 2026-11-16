package index

import (
	"fmt"
	"strings"
	"sync"
)

type memoryStore struct {
	mutex        sync.RWMutex
	records      []*Record
	first        uint64
	noncesByHash map[string]uint64
}

// NewMemoryStore creates a store that keeps the records in memory (not persisted)
func NewMemoryStore() *memoryStore {
	return &memoryStore{
		records:      make([]*Record, 0),
		noncesByHash: make(map[string]uint64),
	}
}

// Put appends a record
func (store *memoryStore) Put(record *Record) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	if len(store.records) == 0 {
		store.first = record.MetaNonce
	} else {
		expectedNonce := store.first + uint64(len(store.records))
		if record.MetaNonce != expectedNonce {
			return fmt.Errorf("%w: cannot store record %d, expected %d", ErrInconsistent, record.MetaNonce, expectedNonce)
		}
	}

	store.records = append(store.records, record)
	store.noncesByHash[normalizeHash(record.MetaHash)] = record.MetaNonce
	return nil
}

// Get returns the record of a metablock, or ErrNotFound
func (store *memoryStore) Get(metaNonce uint64) (*Record, error) {
	store.mutex.RLock()
	defer store.mutex.RUnlock()

	if len(store.records) == 0 || metaNonce < store.first {
		return nil, ErrNotFound
	}

	position := metaNonce - store.first
	if position >= uint64(len(store.records)) {
		return nil, ErrNotFound
	}

	return store.records[position], nil
}

// GetNonceByHash returns the nonce of the metablock with the given hash, if indexed
func (store *memoryStore) GetNonceByHash(metaHash string) (uint64, bool, error) {
	store.mutex.RLock()
	defer store.mutex.RUnlock()

	nonce, ok := store.noncesByHash[normalizeHash(metaHash)]
	return nonce, ok, nil
}

// Bounds returns the nonces of the first and of the last record
func (store *memoryStore) Bounds() (uint64, uint64, bool, error) {
	store.mutex.RLock()
	defer store.mutex.RUnlock()

	if len(store.records) == 0 {
		return 0, 0, false, nil
	}

	return store.first, store.first + uint64(len(store.records)) - 1, true, nil
}

// Close does nothing
func (store *memoryStore) Close() error {
	return nil
}

func normalizeHash(hash string) string {
	return strings.ToLower(hash)
}
