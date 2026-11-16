// Package shardmap computes the shard of an address, exactly as the protocol does,
// and decides whether an address is observed by the aggregator.
package shardmap

import (
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/multiversx/mx-chain-core-go/core"
	"github.com/multiversx/mx-chain-core-go/core/pubkeyConverter"
)

const pubKeyLength = 32

var errNoShards = errors.New("number of shards must be greater than zero")

// ShardMap maps addresses to shards
type ShardMap struct {
	numShards       uint32
	maskHigh        uint32
	maskLow         uint32
	served          map[uint32]struct{}
	servedSorted    []uint32
	handleContracts bool
	converter       core.PubkeyConverter
}

// New creates a shard map.
// "servedShards" are the shards for which a shard instance (backend) is configured.
// "handleContracts" must have the same value as the "--handle-contracts" flag of the shard instances.
func New(numShards uint32, servedShards []uint32, handleContracts bool, addressPrefix string) (*ShardMap, error) {
	if numShards == 0 {
		return nil, errNoShards
	}

	converter, err := pubkeyConverter.NewBech32PubkeyConverter(pubKeyLength, addressPrefix)
	if err != nil {
		return nil, err
	}

	served := make(map[uint32]struct{}, len(servedShards))
	servedSorted := make([]uint32, 0, len(servedShards))

	for _, shard := range servedShards {
		if shard >= numShards {
			return nil, fmt.Errorf("served shard %d is out of range (number of shards = %d)", shard, numShards)
		}
		if _, exists := served[shard]; exists {
			return nil, fmt.Errorf("served shard %d is listed more than once", shard)
		}

		served[shard] = struct{}{}
		servedSorted = append(servedSorted, shard)
	}

	sort.Slice(servedSorted, func(i, j int) bool { return servedSorted[i] < servedSorted[j] })

	maskHigh, maskLow := calculateMasks(numShards)

	return &ShardMap{
		numShards:       numShards,
		maskHigh:        maskHigh,
		maskLow:         maskLow,
		served:          served,
		servedSorted:    servedSorted,
		handleContracts: handleContracts,
		converter:       converter,
	}, nil
}

// Same computation as in mx-chain-go's "multiShardCoordinator".
func calculateMasks(numShards uint32) (uint32, uint32) {
	n := math.Ceil(math.Log2(float64(numShards)))
	if n < 1 {
		// A single shard: every (non-metachain) address belongs to shard 0.
		return 0, 0
	}

	return (1 << uint(n)) - 1, (1 << uint(n-1)) - 1
}

// ServedShards returns the served shards, in ascending order
func (shardMap *ShardMap) ServedShards() []uint32 {
	result := make([]uint32, len(shardMap.servedSorted))
	copy(result, shardMap.servedSorted)
	return result
}

// IsServed returns whether a backend is configured for the given shard
func (shardMap *ShardMap) IsServed(shard uint32) bool {
	_, ok := shardMap.served[shard]
	return ok
}

// DecodeAddress converts a bech32 address to a public key
func (shardMap *ShardMap) DecodeAddress(address string) ([]byte, error) {
	if len(address) == 0 {
		return nil, errors.New("empty address")
	}

	return shardMap.converter.Decode(address)
}

// ShardOfAddress computes the shard of a bech32 address
func (shardMap *ShardMap) ShardOfAddress(address string) (uint32, error) {
	pubKey, err := shardMap.DecodeAddress(address)
	if err != nil {
		return 0, err
	}

	return shardMap.ShardOfPubKey(pubKey), nil
}

// ShardOfPubKey computes the shard of a public key.
// It mirrors "ComputeIdFromBytes" of mx-chain-go's "multiShardCoordinator", with one difference:
// the empty (all zeros) address is not attributed to a "self" shard (the aggregator has none), but to the metachain.
func (shardMap *ShardMap) ShardOfPubKey(pubKey []byte) uint32 {
	if len(pubKey) == 0 || core.IsEmptyAddress(pubKey) {
		return core.MetachainShardId
	}

	bytesNeeded := 1
	if shardMap.numShards > 16777216 {
		bytesNeeded = 4
	} else if shardMap.numShards > 65536 {
		bytesNeeded = 3
	} else if shardMap.numShards > 256 {
		bytesNeeded = 2
	}

	startingIndex := 0
	if len(pubKey) > bytesNeeded {
		startingIndex = len(pubKey) - bytesNeeded
	}

	suffix := pubKey[startingIndex:]
	if core.IsSmartContractOnMetachain(suffix, pubKey) {
		return core.MetachainShardId
	}

	value := uint32(0)
	for i := 0; i < len(suffix); i++ {
		value = value<<8 + uint32(suffix[i])
	}

	shard := value & shardMap.maskHigh
	if shard > shardMap.numShards-1 {
		shard = value & shardMap.maskLow
	}

	return shard
}

// IsObserved returns whether balance-changing operations of the address are reported by a shard instance:
// the address is in a served shard and, unless contracts are handled, it is not a smart contract.
func (shardMap *ShardMap) IsObserved(address string) bool {
	pubKey, err := shardMap.DecodeAddress(address)
	if err != nil {
		return false
	}

	if !shardMap.IsServed(shardMap.ShardOfPubKey(pubKey)) {
		return false
	}

	return shardMap.handleContracts || !core.IsSmartContractAddress(pubKey)
}
