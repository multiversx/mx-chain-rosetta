package services

import "github.com/multiversx/mx-chain-core-go/data/transaction"

func extractTransactionMetadata(tx *transaction.ApiTransactionResult) objectsMap {
	metadata := objectsMap{
		"timestamp":         tx.Timestamp,
		"value":             tx.Value,
		"nonce":             tx.Nonce,
		"typeOnSource":      tx.ProcessingTypeOnSource,
		"typeOnDestination": tx.ProcessingTypeOnDestination,
		"epoch":             tx.Epoch,
		"sender":            tx.Sender,
		"receiver":          tx.Receiver,
		"sourceShard":       tx.SourceShard,
		"destinationShard":  tx.DestinationShard,
		"miniblock":         tx.MiniBlockHash,
		"miniblockType":     tx.MiniBlockType,
	}

	if len(tx.OriginalSender) > 0 {
		metadata["originalSender"] = tx.OriginalSender
	}
	if len(tx.SenderUsername) > 0 {
		metadata["senderUsername"] = string(tx.SenderUsername)
	}
	if len(tx.ReceiverUsername) > 0 {
		metadata["receiverUsername"] = string(tx.ReceiverUsername)
	}
	if len(tx.OriginalTransactionHash) > 0 {
		metadata["originalTransaction"] = tx.OriginalTransactionHash
	}
	if len(tx.PreviousTransactionHash) > 0 {
		metadata["previousTransaction"] = tx.PreviousTransactionHash
	}
	if len(tx.Data) > 0 {
		metadata["data"] = tx.Data
	}
	if tx.GasPrice > 0 {
		metadata["gasPrice"] = tx.GasPrice
	}
	if tx.GasLimit > 0 {
		metadata["gasLimit"] = tx.GasLimit
	}

	return metadata
}
