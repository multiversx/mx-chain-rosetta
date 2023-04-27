package services

import (
	"testing"

	"github.com/multiversx/mx-chain-core-go/data/transaction"
	"github.com/stretchr/testify/require"
)

func TestExtractTransactionMetadata(t *testing.T) {
	tx := &transaction.ApiTransactionResult{
		Timestamp:                   1000,
		Value:                       "1001",
		Nonce:                       1002,
		ProcessingTypeOnSource:      "a",
		ProcessingTypeOnDestination: "b",
		Epoch:                       1003,
		Sender:                      "alice",
		Receiver:                    "bob",
		SourceShard:                 7,
		DestinationShard:            8,
		MiniBlockHash:               "abba",
		MiniBlockType:               "foo",
	}

	expectedMetadata := objectsMap{
		"timestamp":         int64(1000),
		"value":             "1001",
		"nonce":             uint64(1002),
		"typeOnSource":      "a",
		"typeOnDestination": "b",
		"epoch":             uint32(1003),
		"sender":            "alice",
		"receiver":          "bob",
		"sourceShard":       uint32(7),
		"destinationShard":  uint32(8),
		"miniblock":         "abba",
		"miniblockType":     "foo",
	}

	require.Equal(t, expectedMetadata, extractTransactionMetadata(tx))

	tx.OriginalSender = "alice"
	tx.SenderUsername = []byte("alice")
	tx.ReceiverUsername = []byte("bob")
	tx.OriginalTransactionHash = "aaaa"
	tx.PreviousTransactionHash = "bbbb"
	tx.Data = []byte("test")
	tx.GasPrice = 42
	tx.GasLimit = 43

	expectedMetadata["originalSender"] = "alice"
	expectedMetadata["senderUsername"] = "alice"
	expectedMetadata["receiverUsername"] = "bob"
	expectedMetadata["originalTransaction"] = "aaaa"
	expectedMetadata["previousTransaction"] = "bbbb"
	expectedMetadata["data"] = []byte("test")
	expectedMetadata["gasPrice"] = uint64(42)
	expectedMetadata["gasLimit"] = uint64(43)

	require.Equal(t, expectedMetadata, extractTransactionMetadata(tx))
}
