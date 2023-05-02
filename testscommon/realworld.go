package testscommon

import (
	"github.com/multiversx/mx-chain-core-go/core/pubkeyConverter"
	hasherFactory "github.com/multiversx/mx-chain-core-go/hashing/factory"
	marshalFactory "github.com/multiversx/mx-chain-core-go/marshal/factory"
	logger "github.com/multiversx/mx-chain-logger-go"
)

// RealWorldBech32PubkeyConverter is a bech32 converter, to be used in tests
var RealWorldBech32PubkeyConverter, _ = pubkeyConverter.NewBech32PubkeyConverter(32, logger.GetOrCreate("testscommon"))

// RealWorldBlake2bHasher is a blake2b hasher, to be used in tests
var RealWorldBlake2bHasher, _ = hasherFactory.NewHasher("blake2b")

// MarshalizerForHashing is a gogo protobuf marshalizer, to be used in tests
var MarshalizerForHashing, _ = marshalFactory.NewMarshalizer("gogo protobuf")
