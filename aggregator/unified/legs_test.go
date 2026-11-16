package unified

import (
	"encoding/json"
	"testing"

	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/multiversx/mx-chain-core-go/core"
	"github.com/multiversx/mx-chain-rosetta/aggregator/shardmap"
	"github.com/stretchr/testify/require"
)

const (
	testAddressAlice    = "erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th"
	testAddressBob      = "erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx"
	testAddressContract = "erd1qqqqqqqqqqqqqpgqfejaxfh4ktp8mh8s77pl90dq0uzvh2vk396qlcwepw"
)

type legsFixture struct {
	shardMap    *shardmap.ShardMap
	classifier  *legsClassifier
	shardAlice  uint32
	shardBob    uint32
	otherShard  uint32
	shardOfCode uint32
}

func newLegsFixture(t *testing.T, forwardLinks bool, handleContracts bool) *legsFixture {
	shardMap, err := shardmap.New(3, []uint32{0, 1, 2}, handleContracts, "erd")
	require.NoError(t, err)

	shardAlice, err := shardMap.ShardOfAddress(testAddressAlice)
	require.NoError(t, err)
	shardBob, err := shardMap.ShardOfAddress(testAddressBob)
	require.NoError(t, err)
	shardOfCode, err := shardMap.ShardOfAddress(testAddressContract)
	require.NoError(t, err)

	// The tests below assume that Alice and Bob are in different shards.
	require.NotEqual(t, shardAlice, shardBob)

	return &legsFixture{
		shardMap:    shardMap,
		classifier:  newLegsClassifier(shardMap, forwardLinks),
		shardAlice:  shardAlice,
		shardBob:    shardBob,
		otherShard:  3 - shardAlice - shardBob,
		shardOfCode: shardOfCode,
	}
}

func newTx(hash string, metadata map[string]interface{}) *types.Transaction {
	return &types.Transaction{
		TransactionIdentifier: &types.TransactionIdentifier{Hash: hash},
		Operations: []*types.Operation{
			{OperationIdentifier: &types.OperationIdentifier{Index: 0}, Type: "Transfer"},
		},
		Metadata: metadata,
	}
}

func transferMetadata(sourceShard uint32, destinationShard uint32, value string, receiver string) map[string]interface{} {
	return map[string]interface{}{
		"sourceShard":      sourceShard,
		"destinationShard": destinationShard,
		"miniblockType":    "TxBlock",
		"value":            value,
		"receiver":         receiver,
	}
}

func related(hash string, direction types.Direction) *types.RelatedTransaction {
	return &types.RelatedTransaction{
		TransactionIdentifier: &types.TransactionIdentifier{Hash: hash},
		Direction:             direction,
	}
}

func TestLegsClassifier_LocalTransactionsAreUnchanged(t *testing.T) {
	fixture := newLegsFixture(t, true, false)

	t.Run("intra-shard", func(t *testing.T) {
		tx := newTx("aa", transferMetadata(fixture.shardAlice, fixture.shardAlice, "10", testAddressAlice))
		result := fixture.classifier.apply(fixture.shardAlice, tx)

		require.Equal(t, tx, result)
		require.NotSame(t, tx, result)
	})

	t.Run("no metadata (rewards, refunds)", func(t *testing.T) {
		tx := newTx("aa", nil)
		result := fixture.classifier.apply(fixture.shardAlice, tx)

		require.Equal(t, tx, result)
		require.Nil(t, result.Metadata)
		require.Nil(t, result.RelatedTransactions)
	})

	t.Run("metadata without shards", func(t *testing.T) {
		tx := newTx("aa", map[string]interface{}{"value": "10"})
		require.Equal(t, tx, fixture.classifier.apply(fixture.shardAlice, tx))
	})
}

func TestLegsClassifier_SourceLeg(t *testing.T) {
	fixture := newLegsFixture(t, true, false)

	tx := newTx("aa", transferMetadata(fixture.shardAlice, fixture.shardBob, "10", testAddressBob))
	result := fixture.classifier.apply(fixture.shardAlice, tx)

	// The identifier is unchanged, and the destination leg is linked (forward).
	require.Equal(t, "aa", result.TransactionIdentifier.Hash)
	require.Equal(t, []*types.RelatedTransaction{related("aa-dst", types.Forward)}, result.RelatedTransactions)
	require.Equal(t, map[string]interface{}{
		"leg":              "source",
		"sourceShard":      fixture.shardAlice,
		"destinationShard": fixture.shardBob,
	}, result.Metadata["crossShard"])
	require.Equal(t, tx.Operations, result.Operations)

	// The input is not modified.
	require.Nil(t, tx.RelatedTransactions)
	require.NotContains(t, tx.Metadata, "crossShard")
}

func TestLegsClassifier_DestinationLeg(t *testing.T) {
	fixture := newLegsFixture(t, true, false)

	tx := newTx("aa", transferMetadata(fixture.shardAlice, fixture.shardBob, "10", testAddressBob))
	result := fixture.classifier.apply(fixture.shardBob, tx)

	require.Equal(t, "aa-dst", result.TransactionIdentifier.Hash)
	require.Equal(t, []*types.RelatedTransaction{related("aa", types.Backward)}, result.RelatedTransactions)
	require.Equal(t, map[string]interface{}{
		"leg":              "destination",
		"hash":             "aa",
		"sourceShard":      fixture.shardAlice,
		"destinationShard": fixture.shardBob,
	}, result.Metadata["crossShard"])
	require.Equal(t, tx.Operations, result.Operations)

	// The input is not modified.
	require.Equal(t, "aa", tx.TransactionIdentifier.Hash)
}

func TestLegsClassifier_ForwardLinkOnlyWhenDestinationLegIsCertain(t *testing.T) {
	hasForwardLink := func(fixture *legsFixture, metadata map[string]interface{}) bool {
		result := fixture.classifier.apply(fixture.shardAlice, newTx("aa", metadata))
		require.Equal(t, "source", result.Metadata["crossShard"].(map[string]interface{})["leg"])

		for _, item := range result.RelatedTransactions {
			if item.Direction == types.Forward {
				require.Equal(t, "aa-dst", item.TransactionIdentifier.Hash)
				return true
			}
		}

		return false
	}

	fixture := newLegsFixture(t, true, false)

	t.Run("transfer of value to an observed address", func(t *testing.T) {
		require.True(t, hasForwardLink(fixture, transferMetadata(fixture.shardAlice, fixture.shardBob, "1", testAddressBob)))
	})

	t.Run("no value", func(t *testing.T) {
		require.False(t, hasForwardLink(fixture, transferMetadata(fixture.shardAlice, fixture.shardBob, "0", testAddressBob)))
		require.False(t, hasForwardLink(fixture, transferMetadata(fixture.shardAlice, fixture.shardBob, "", testAddressBob)))
		require.False(t, hasForwardLink(fixture, transferMetadata(fixture.shardAlice, fixture.shardBob, "not-a-number", testAddressBob)))
	})

	t.Run("not a regular transaction (e.g. invalid, smart contract result)", func(t *testing.T) {
		for _, miniblockType := range []string{"InvalidBlock", "SmartContractResultBlock", ""} {
			metadata := transferMetadata(fixture.shardAlice, fixture.shardBob, "1", testAddressBob)
			metadata["miniblockType"] = miniblockType
			require.False(t, hasForwardLink(fixture, metadata), miniblockType)
		}
	})

	t.Run("destination is the metachain (not served)", func(t *testing.T) {
		require.False(t, hasForwardLink(fixture, transferMetadata(fixture.shardAlice, core.MetachainShardId, "1", testAddressBob)))
	})

	t.Run("receiver does not match the destination shard", func(t *testing.T) {
		require.False(t, hasForwardLink(fixture, transferMetadata(fixture.shardAlice, fixture.otherShard, "1", testAddressBob)))
		require.False(t, hasForwardLink(fixture, transferMetadata(fixture.shardAlice, fixture.shardBob, "1", "bad-address")))
	})

	t.Run("receiver is a contract", func(t *testing.T) {
		if fixture.shardOfCode == fixture.shardAlice {
			t.Skip("the test contract is in the shard of the sender")
		}

		metadata := transferMetadata(fixture.shardAlice, fixture.shardOfCode, "1", testAddressContract)

		// Contracts are not handled: the destination shard does not report operations on the contract.
		require.False(t, hasForwardLink(fixture, metadata))
		// Contracts are handled.
		require.True(t, hasForwardLink(newLegsFixture(t, true, true), metadata))
	})

	t.Run("forward links disabled", func(t *testing.T) {
		fixtureWithoutLinks := newLegsFixture(t, false, false)
		require.False(t, hasForwardLink(fixtureWithoutLinks, transferMetadata(fixtureWithoutLinks.shardAlice, fixtureWithoutLinks.shardBob, "1", testAddressBob)))
	})
}

func TestLegsClassifier_IncomingFromShardThatIsNotServed(t *testing.T) {
	fixture := newLegsFixture(t, true, false)

	// E.g. a smart contract result sent by a system contract (metachain): no source leg can exist, thus the identifier is not changed.
	tx := newTx("aa", transferMetadata(core.MetachainShardId, fixture.shardBob, "10", testAddressBob))
	result := fixture.classifier.apply(fixture.shardBob, tx)

	require.Equal(t, "aa", result.TransactionIdentifier.Hash)
	require.Nil(t, result.RelatedTransactions)
	require.NotContains(t, result.Metadata, "crossShard")
}

func TestLegsClassifier_LinksToOriginalTransaction(t *testing.T) {
	fixture := newLegsFixture(t, true, false)

	t.Run("intra-shard result", func(t *testing.T) {
		metadata := transferMetadata(fixture.shardAlice, fixture.shardAlice, "10", testAddressAlice)
		metadata["originalTransaction"] = "root"

		result := fixture.classifier.apply(fixture.shardAlice, newTx("aa", metadata))
		require.Equal(t, "aa", result.TransactionIdentifier.Hash)
		require.Equal(t, []*types.RelatedTransaction{related("root", types.Backward)}, result.RelatedTransactions)
	})

	t.Run("destination leg of a result: linked to its source leg and to the original transaction", func(t *testing.T) {
		metadata := transferMetadata(fixture.shardBob, fixture.shardAlice, "10", testAddressAlice)
		metadata["miniblockType"] = "SmartContractResultBlock"
		metadata["originalTransaction"] = "root"

		result := fixture.classifier.apply(fixture.shardAlice, newTx("aa", metadata))
		require.Equal(t, "aa-dst", result.TransactionIdentifier.Hash)
		require.Equal(t, []*types.RelatedTransaction{
			related("aa", types.Backward),
			related("root", types.Backward),
		}, result.RelatedTransactions)
	})

	t.Run("original transaction equal to own hash is not linked", func(t *testing.T) {
		metadata := transferMetadata(fixture.shardAlice, fixture.shardAlice, "10", testAddressAlice)
		metadata["originalTransaction"] = "aa"

		result := fixture.classifier.apply(fixture.shardAlice, newTx("aa", metadata))
		require.Nil(t, result.RelatedTransactions)
	})

	t.Run("existing links are kept, duplicates are not added", func(t *testing.T) {
		metadata := transferMetadata(fixture.shardAlice, fixture.shardAlice, "10", testAddressAlice)
		metadata["originalTransaction"] = "root"

		tx := newTx("aa", metadata)
		tx.RelatedTransactions = []*types.RelatedTransaction{related("root", types.Backward)}

		result := fixture.classifier.apply(fixture.shardAlice, tx)
		require.Len(t, result.RelatedTransactions, 1)
	})
}

func TestReadShardMetadata_AfterJsonRoundTrip(t *testing.T) {
	// Metadata, as received from a shard instance over HTTP: numbers are decoded as float64.
	tx := newTx("aa", transferMetadata(2, core.MetachainShardId, "10", testAddressBob))
	encoded, err := json.Marshal(tx)
	require.NoError(t, err)

	decoded := &types.Transaction{}
	require.NoError(t, json.Unmarshal(encoded, decoded))

	metadata := readShardMetadata(decoded)
	require.True(t, metadata.hasShards)
	require.Equal(t, uint32(2), metadata.sourceShard)
	require.Equal(t, core.MetachainShardId, metadata.destinationShard)
	require.Equal(t, "TxBlock", metadata.miniblockType)
	require.Equal(t, "10", metadata.value)
	require.Equal(t, testAddressBob, metadata.receiver)
}

func TestReadUint32(t *testing.T) {
	for _, value := range []interface{}{float64(7), json.Number("7"), uint32(7), uint64(7), uint(7), int(7), int64(7), int32(7)} {
		result, ok := readUint32(value)
		require.True(t, ok, "%T", value)
		require.Equal(t, uint32(7), result)
	}

	for _, value := range []interface{}{nil, "7", float64(-1), float64(1.5), float64(1 << 33), json.Number("x"), int(-1), int64(-1), int32(-1), uint64(1 << 33)} {
		_, ok := readUint32(value)
		require.False(t, ok, "%T %v", value, value)
	}
}

func TestMergeDuplicates(t *testing.T) {
	status := "Success"
	newOperation := func(index int64, address string) *types.Operation {
		return &types.Operation{
			OperationIdentifier: &types.OperationIdentifier{Index: index},
			Type:                "Transfer",
			Status:              &status,
			Account:             &types.AccountIdentifier{Address: address},
		}
	}

	first := &types.Transaction{
		TransactionIdentifier: &types.TransactionIdentifier{Hash: "aa"},
		Operations:            []*types.Operation{newOperation(0, "alice"), newOperation(1, "bob")},
		RelatedTransactions:   []*types.RelatedTransaction{related("root", types.Backward)},
	}
	other := &types.Transaction{
		TransactionIdentifier: &types.TransactionIdentifier{Hash: "bb"},
		Operations:            []*types.Operation{newOperation(0, "carol")},
	}
	second := &types.Transaction{
		TransactionIdentifier: &types.TransactionIdentifier{Hash: "aa"},
		Operations: []*types.Operation{
			newOperation(0, "dan"),
			{
				OperationIdentifier: &types.OperationIdentifier{Index: 1},
				RelatedOperations:   []*types.OperationIdentifier{{Index: 0}},
				Type:                "Transfer",
				Account:             &types.AccountIdentifier{Address: "eve"},
			},
		},
		RelatedTransactions: []*types.RelatedTransaction{related("root", types.Backward), related("zz", types.Forward)},
		Metadata:            map[string]interface{}{"foo": "bar"},
	}

	merged, numDuplicates := mergeDuplicates([]*types.Transaction{first, other, second})
	require.Equal(t, 1, numDuplicates)
	require.Len(t, merged, 2)

	// The order of the first occurrences is preserved.
	require.Equal(t, "aa", merged[0].TransactionIdentifier.Hash)
	require.Equal(t, "bb", merged[1].TransactionIdentifier.Hash)

	// Operations are concatenated and re-indexed (including the references among them).
	require.Len(t, merged[0].Operations, 4)
	for i, operation := range merged[0].Operations {
		require.Equal(t, int64(i), operation.OperationIdentifier.Index)
	}
	require.Equal(t, "dan", merged[0].Operations[2].Account.Address)
	require.Equal(t, int64(2), merged[0].Operations[3].RelatedOperations[0].Index)

	require.Equal(t, []*types.RelatedTransaction{related("root", types.Backward), related("zz", types.Forward)}, merged[0].RelatedTransactions)
	require.Equal(t, map[string]interface{}{"foo": "bar"}, merged[0].Metadata)

	// The inputs are not modified.
	require.Len(t, first.Operations, 2)
	require.Equal(t, int64(0), second.Operations[0].OperationIdentifier.Index)
	require.Equal(t, int64(0), second.Operations[1].RelatedOperations[0].Index)

	// No duplicates: nothing changes.
	merged, numDuplicates = mergeDuplicates([]*types.Transaction{first, other})
	require.Equal(t, 0, numDuplicates)
	require.Same(t, first, merged[0])
	require.Same(t, other, merged[1])
}
