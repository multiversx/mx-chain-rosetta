package unified

import (
	"encoding/json"
	"math"
	"math/big"

	"github.com/coinbase/rosetta-sdk-go/types"
)

const (
	// DestinationLegSuffix is appended to the hash of a cross-shard transaction, to form the identifier of its destination leg.
	// The suffix does not contain "/", which is used as a separator by the storage of rosetta-sdk-go.
	DestinationLegSuffix = "-dst"

	legSource      = "source"
	legDestination = "destination"

	// Keys of the transaction metadata, as set by the shard instances (see "server/services/transactionMetadata.go")
	metadataKeySourceShard         = "sourceShard"
	metadataKeyDestinationShard    = "destinationShard"
	metadataKeyMiniblockType       = "miniblockType"
	metadataKeyValue               = "value"
	metadataKeyReceiver            = "receiver"
	metadataKeyOriginalTransaction = "originalTransaction"

	// Key of the transaction metadata, set by the aggregator
	metadataKeyCrossShard = "crossShard"

	miniblockTypeRegularTransactions = "TxBlock"
)

type addressesObserver interface {
	IsServed(shard uint32) bool
	IsObserved(address string) bool
	ShardOfAddress(address string) (uint32, error)
}

// shardMetadata holds the fields (of the transaction metadata) that the classification relies on
type shardMetadata struct {
	hasShards           bool
	sourceShard         uint32
	destinationShard    uint32
	miniblockType       string
	value               string
	receiver            string
	originalTransaction string
}

func readShardMetadata(tx *types.Transaction) shardMetadata {
	result := shardMetadata{}
	if tx.Metadata == nil {
		return result
	}

	sourceShard, hasSourceShard := readUint32(tx.Metadata[metadataKeySourceShard])
	destinationShard, hasDestinationShard := readUint32(tx.Metadata[metadataKeyDestinationShard])

	result.hasShards = hasSourceShard && hasDestinationShard
	result.sourceShard = sourceShard
	result.destinationShard = destinationShard
	result.miniblockType, _ = tx.Metadata[metadataKeyMiniblockType].(string)
	result.value, _ = tx.Metadata[metadataKeyValue].(string)
	result.receiver, _ = tx.Metadata[metadataKeyReceiver].(string)
	result.originalTransaction, _ = tx.Metadata[metadataKeyOriginalTransaction].(string)

	return result
}

// readUint32 handles both values decoded from JSON and values held in memory (not yet serialized)
func readUint32(value interface{}) (uint32, bool) {
	switch typed := value.(type) {
	case float64:
		if typed < 0 || typed > math.MaxUint32 || typed != math.Trunc(typed) {
			return 0, false
		}
		return uint32(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil || parsed < 0 || parsed > math.MaxUint32 {
			return 0, false
		}
		return uint32(parsed), true
	case uint32:
		return typed, true
	case uint64:
		if typed > math.MaxUint32 {
			return 0, false
		}
		return uint32(typed), true
	case uint:
		if uint64(typed) > math.MaxUint32 {
			return 0, false
		}
		return uint32(typed), true
	case int:
		if typed < 0 || int64(typed) > math.MaxUint32 {
			return 0, false
		}
		return uint32(typed), true
	case int64:
		if typed < 0 || typed > math.MaxUint32 {
			return 0, false
		}
		return uint32(typed), true
	case int32:
		if typed < 0 {
			return 0, false
		}
		return uint32(typed), true
	default:
		return 0, false
	}
}

// legsClassifier decides, for each transaction returned by a shard instance, whether it is a leg of a cross-shard transaction,
// then relabels and links it accordingly. It never changes operations.
type legsClassifier struct {
	addresses    addressesObserver
	forwardLinks bool
}

func newLegsClassifier(addresses addressesObserver, forwardLinks bool) *legsClassifier {
	return &legsClassifier{
		addresses:    addresses,
		forwardLinks: forwardLinks,
	}
}

// apply returns a (possibly) relabeled and linked copy of the transaction. The input is not modified.
func (classifier *legsClassifier) apply(servingShard uint32, tx *types.Transaction) *types.Transaction {
	metadata := readShardMetadata(tx)
	result := copyTransaction(tx)
	hash := tx.TransactionIdentifier.Hash

	isCrossShard := metadata.hasShards && metadata.sourceShard != metadata.destinationShard
	if isCrossShard {
		isSourceLeg := metadata.sourceShard == servingShard
		isDestinationLeg := metadata.destinationShard == servingShard && classifier.addresses.IsServed(metadata.sourceShard)

		if isSourceLeg {
			setCrossShardMetadata(result, legSource, "", metadata)

			if classifier.isDestinationLegCertain(metadata) {
				addRelatedTransaction(result, hash+DestinationLegSuffix, types.Forward)
			}
		} else if isDestinationLeg {
			result.TransactionIdentifier = &types.TransactionIdentifier{Hash: hash + DestinationLegSuffix}
			setCrossShardMetadata(result, legDestination, hash, metadata)
			addRelatedTransaction(result, hash, types.Backward)
		}

		// Otherwise: the transaction comes from a shard that is not served (e.g. the metachain), thus no source leg can exist.
		// It is passed through (unchanged).
	}

	// Link smart contract results (and so on) to the transaction that originated them.
	if len(metadata.originalTransaction) > 0 && metadata.originalTransaction != hash {
		addRelatedTransaction(result, metadata.originalTransaction, types.Backward)
	}

	return result
}

// isDestinationLegCertain tells whether the destination leg of a (source leg) transaction is certain to be reported by the shard instance of the destination shard.
// A forward link is a promise (that the destination leg will appear), therefore it is only made when the outcome is known in advance:
// regular transactions that transfer (native) value to an observed address. For these, the destination shard reports an operation for the receiver,
// whether the transfer succeeds or fails there.
func (classifier *legsClassifier) isDestinationLegCertain(metadata shardMetadata) bool {
	if !classifier.forwardLinks {
		return false
	}
	if metadata.miniblockType != miniblockTypeRegularTransactions {
		return false
	}
	if !classifier.addresses.IsServed(metadata.destinationShard) {
		return false
	}

	value, ok := big.NewInt(0).SetString(metadata.value, 10)
	if !ok || value.Sign() <= 0 {
		return false
	}

	receiverShard, err := classifier.addresses.ShardOfAddress(metadata.receiver)
	if err != nil || receiverShard != metadata.destinationShard {
		return false
	}

	return classifier.addresses.IsObserved(metadata.receiver)
}

func copyTransaction(tx *types.Transaction) *types.Transaction {
	result := *tx

	if tx.Metadata != nil {
		result.Metadata = make(map[string]interface{}, len(tx.Metadata)+1)
		for key, value := range tx.Metadata {
			result.Metadata[key] = value
		}
	}

	if tx.RelatedTransactions != nil {
		result.RelatedTransactions = make([]*types.RelatedTransaction, len(tx.RelatedTransactions))
		copy(result.RelatedTransactions, tx.RelatedTransactions)
	}

	return &result
}

func setCrossShardMetadata(tx *types.Transaction, leg string, canonicalHash string, metadata shardMetadata) {
	crossShard := map[string]interface{}{
		"leg":                       leg,
		metadataKeySourceShard:      metadata.sourceShard,
		metadataKeyDestinationShard: metadata.destinationShard,
	}

	if len(canonicalHash) > 0 {
		crossShard["hash"] = canonicalHash
	}

	if tx.Metadata == nil {
		tx.Metadata = make(map[string]interface{}, 1)
	}

	tx.Metadata[metadataKeyCrossShard] = crossShard
}

// addRelatedTransaction links a transaction to another one (duplicates and self-references are skipped)
func addRelatedTransaction(tx *types.Transaction, relatedHash string, direction types.Direction) {
	if len(relatedHash) == 0 || relatedHash == tx.TransactionIdentifier.Hash {
		return
	}

	for _, existing := range tx.RelatedTransactions {
		if existing.NetworkIdentifier == nil && existing.TransactionIdentifier != nil && existing.TransactionIdentifier.Hash == relatedHash {
			return
		}
	}

	tx.RelatedTransactions = append(tx.RelatedTransactions, &types.RelatedTransaction{
		TransactionIdentifier: &types.TransactionIdentifier{Hash: relatedHash},
		Direction:             direction,
	})
}

// mergeDuplicates merges transactions that share an identifier into a single transaction (operations are concatenated and re-indexed).
// The order of the first occurrences is preserved. It returns the merged transactions and the number of duplicates found.
func mergeDuplicates(txs []*types.Transaction) ([]*types.Transaction, int) {
	positionByHash := make(map[string]int, len(txs))
	merged := make([]*types.Transaction, 0, len(txs))
	numDuplicates := 0

	for _, tx := range txs {
		hash := tx.TransactionIdentifier.Hash

		position, isDuplicate := positionByHash[hash]
		if !isDuplicate {
			positionByHash[hash] = len(merged)
			merged = append(merged, tx)
			continue
		}

		numDuplicates++
		merged[position] = mergeTwoTransactions(merged[position], tx)
	}

	return merged, numDuplicates
}

func mergeTwoTransactions(first *types.Transaction, second *types.Transaction) *types.Transaction {
	result := copyTransaction(first)
	offset := int64(len(first.Operations))

	operations := make([]*types.Operation, 0, len(first.Operations)+len(second.Operations))
	operations = append(operations, first.Operations...)

	for _, operation := range second.Operations {
		operationCopy := *operation

		if operation.OperationIdentifier != nil {
			identifierCopy := *operation.OperationIdentifier
			identifierCopy.Index += offset
			operationCopy.OperationIdentifier = &identifierCopy
		}

		if len(operation.RelatedOperations) > 0 {
			operationCopy.RelatedOperations = make([]*types.OperationIdentifier, 0, len(operation.RelatedOperations))
			for _, related := range operation.RelatedOperations {
				relatedCopy := *related
				relatedCopy.Index += offset
				operationCopy.RelatedOperations = append(operationCopy.RelatedOperations, &relatedCopy)
			}
		}

		operations = append(operations, &operationCopy)
	}

	result.Operations = operations

	for _, related := range second.RelatedTransactions {
		if related.TransactionIdentifier != nil && related.NetworkIdentifier == nil {
			addRelatedTransaction(result, related.TransactionIdentifier.Hash, related.Direction)
		}
	}

	if result.Metadata == nil && second.Metadata != nil {
		result.Metadata = copyTransaction(second).Metadata
	}

	return result
}
