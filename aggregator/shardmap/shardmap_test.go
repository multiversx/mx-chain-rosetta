package shardmap

import (
	"math/rand"
	"testing"

	"github.com/multiversx/mx-chain-core-go/core"
	"github.com/multiversx/mx-chain-core-go/core/pubkeyConverter"
	"github.com/multiversx/mx-chain-go/sharding"
	"github.com/stretchr/testify/require"
)

const (
	// Addresses of the test wallets "alice" and "bob", and of a contract (see "testscommon/addresses.go")
	addressAlice    = "erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th"
	addressBob      = "erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx"
	addressContract = "erd1qqqqqqqqqqqqqpgqfejaxfh4ktp8mh8s77pl90dq0uzvh2vk396qlcwepw"
)

// newAddressOfMetachainContract creates the address of a (system) smart contract that lives on the metachain
func newAddressOfMetachainContract(t *testing.T) string {
	converter, err := pubkeyConverter.NewBech32PubkeyConverter(32, "erd")
	require.NoError(t, err)

	pubKey := make([]byte, 32)
	pubKey[9] = 1
	pubKey[30] = 0xff
	pubKey[31] = 0xff

	address, err := converter.Encode(pubKey)
	require.NoError(t, err)
	return address
}

func TestShardOfPubKey_MatchesProtocol(t *testing.T) {
	rng := rand.New(rand.NewSource(42))

	for numShards := uint32(1); numShards <= 9; numShards++ {
		shardMap, err := New(numShards, []uint32{0}, false, "erd")
		require.NoError(t, err)

		coordinator, err := sharding.NewMultiShardCoordinator(numShards, 0)
		require.NoError(t, err)

		for i := 0; i < 2000; i++ {
			pubKey := make([]byte, 32)
			_, _ = rng.Read(pubKey)

			// Also cover smart contract addresses (which start with zeros), including the ones on the metachain.
			if i%4 == 1 {
				copy(pubKey, make([]byte, 10))
			}
			if i%4 == 2 {
				copy(pubKey, make([]byte, 30))
				pubKey[30] = 0xff
				pubKey[31] = 0xff
			}

			require.Equal(t, coordinator.ComputeId(pubKey), shardMap.ShardOfPubKey(pubKey), "numShards = %d, pubKey = %x", numShards, pubKey)
		}
	}
}

func TestShardOfAddress(t *testing.T) {
	shardMap, err := New(3, []uint32{0, 1, 2}, false, "erd")
	require.NoError(t, err)

	coordinator, err := sharding.NewMultiShardCoordinator(3, 0)
	require.NoError(t, err)

	for _, address := range []string{addressAlice, addressBob, addressContract} {
		pubKey, err := shardMap.DecodeAddress(address)
		require.NoError(t, err)

		shard, err := shardMap.ShardOfAddress(address)
		require.NoError(t, err)
		require.Equal(t, coordinator.ComputeId(pubKey), shard)
		require.Less(t, shard, uint32(3))
	}

	shard, err := shardMap.ShardOfAddress(newAddressOfMetachainContract(t))
	require.NoError(t, err)
	require.Equal(t, core.MetachainShardId, shard)

	// The empty address is attributed to the metachain (the aggregator has no "self" shard).
	require.Equal(t, core.MetachainShardId, shardMap.ShardOfPubKey(make([]byte, 32)))

	_, err = shardMap.ShardOfAddress("")
	require.Error(t, err)
	_, err = shardMap.ShardOfAddress("erd1foobar")
	require.Error(t, err)
	_, err = shardMap.ShardOfAddress("../node/status")
	require.Error(t, err)
}

func TestIsObserved(t *testing.T) {
	withoutContracts, err := New(3, []uint32{0, 1, 2}, false, "erd")
	require.NoError(t, err)

	withContracts, err := New(3, []uint32{0, 1, 2}, true, "erd")
	require.NoError(t, err)

	require.True(t, withoutContracts.IsObserved(addressAlice))
	require.False(t, withoutContracts.IsObserved(addressContract))
	require.True(t, withContracts.IsObserved(addressContract))

	// The metachain is not served.
	require.False(t, withContracts.IsObserved(newAddressOfMetachainContract(t)))
	require.False(t, withContracts.IsObserved("not-an-address"))

	// An address in a shard that is not served is not observed.
	shardOfAlice, err := withContracts.ShardOfAddress(addressAlice)
	require.NoError(t, err)

	otherShards := make([]uint32, 0)
	for shard := uint32(0); shard < 3; shard++ {
		if shard != shardOfAlice {
			otherShards = append(otherShards, shard)
		}
	}

	partial, err := New(3, otherShards, false, "erd")
	require.NoError(t, err)
	require.False(t, partial.IsObserved(addressAlice))
	require.False(t, partial.IsServed(shardOfAlice))
	require.Equal(t, otherShards, partial.ServedShards())
}

func TestNew_Errors(t *testing.T) {
	_, err := New(0, []uint32{0}, false, "erd")
	require.Error(t, err)

	_, err = New(3, []uint32{3}, false, "erd")
	require.ErrorContains(t, err, "out of range")

	_, err = New(3, []uint32{1, 1}, false, "erd")
	require.ErrorContains(t, err, "more than once")
}
