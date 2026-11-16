package index

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/syndtr/goleveldb/leveldb"
)

var (
	keyPrefixRecord = []byte("r/")
	keyPrefixHash   = []byte("h/")
	keyFirst        = []byte("m/first")
	keyLast         = []byte("m/last")
)

type levelDBStore struct {
	mutex sync.RWMutex
	db    *leveldb.DB

	hasBounds bool
	first     uint64
	last      uint64
}

// NewLevelDBStore creates (or opens) a store persisted on disk, at the given path
func NewLevelDBStore(path string) (*levelDBStore, error) {
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot open the notarization index at %s: %w", path, err)
	}

	store := &levelDBStore{db: db}

	err = store.loadBounds()
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (store *levelDBStore) loadBounds() error {
	first, hasFirst, err := store.getUint64(keyFirst)
	if err != nil {
		return err
	}

	last, hasLast, err := store.getUint64(keyLast)
	if err != nil {
		return err
	}

	if hasFirst != hasLast {
		return fmt.Errorf("%w: the notarization index is corrupted (bounds)", ErrInconsistent)
	}

	store.hasBounds = hasFirst
	store.first = first
	store.last = last
	return nil
}

func (store *levelDBStore) getUint64(key []byte) (uint64, bool, error) {
	value, err := store.db.Get(key, nil)
	if errors.Is(err, leveldb.ErrNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if len(value) != 8 {
		return 0, false, fmt.Errorf("%w: the notarization index is corrupted (key %s)", ErrInconsistent, string(key))
	}

	return binary.BigEndian.Uint64(value), true, nil
}

// Put appends a record
func (store *levelDBStore) Put(record *Record) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	if store.hasBounds && record.MetaNonce != store.last+1 {
		return fmt.Errorf("%w: cannot store record %d, expected %d", ErrInconsistent, record.MetaNonce, store.last+1)
	}

	encodedRecord, err := json.Marshal(record)
	if err != nil {
		return err
	}

	encodedNonce := encodeUint64(record.MetaNonce)

	// The record, the lookup by hash and the bounds are written atomically.
	batch := new(leveldb.Batch)
	batch.Put(append(append([]byte{}, keyPrefixRecord...), encodedNonce...), encodedRecord)
	batch.Put(append(append([]byte{}, keyPrefixHash...), []byte(normalizeHash(record.MetaHash))...), encodedNonce)
	batch.Put(keyLast, encodedNonce)
	if !store.hasBounds {
		batch.Put(keyFirst, encodedNonce)
	}

	err = store.db.Write(batch, nil)
	if err != nil {
		return err
	}

	if !store.hasBounds {
		store.hasBounds = true
		store.first = record.MetaNonce
	}

	store.last = record.MetaNonce
	return nil
}

// Get returns the record of a metablock, or ErrNotFound
func (store *levelDBStore) Get(metaNonce uint64) (*Record, error) {
	key := append(append([]byte{}, keyPrefixRecord...), encodeUint64(metaNonce)...)

	value, err := store.db.Get(key, nil)
	if errors.Is(err, leveldb.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	record := &Record{}
	err = json.Unmarshal(value, record)
	if err != nil {
		return nil, fmt.Errorf("%w: cannot decode record %d: %v", ErrInconsistent, metaNonce, err)
	}

	return record, nil
}

// GetNonceByHash returns the nonce of the metablock with the given hash, if indexed
func (store *levelDBStore) GetNonceByHash(metaHash string) (uint64, bool, error) {
	key := append(append([]byte{}, keyPrefixHash...), []byte(normalizeHash(metaHash))...)
	return store.getUint64(key)
}

// Bounds returns the nonces of the first and of the last record
func (store *levelDBStore) Bounds() (uint64, uint64, bool, error) {
	store.mutex.RLock()
	defer store.mutex.RUnlock()

	return store.first, store.last, store.hasBounds, nil
}

// Close closes the underlying database
func (store *levelDBStore) Close() error {
	return store.db.Close()
}

func encodeUint64(value uint64) []byte {
	encoded := make([]byte, 8)
	binary.BigEndian.PutUint64(encoded, value)
	return encoded
}
