package services

import (
	"encoding/hex"
	"encoding/json"
	"math/big"
	"strings"

	"github.com/multiversx/mx-chain-core-go/data/transaction"
)

// innerTransactionOfRelayedV1 is used to parse the inner transaction of a relayed V1 transaction, and holds only the fields handled by Rosetta.
type innerTransactionOfRelayedV1 struct {
	Value          big.Int `json:"value"`
	ReceiverPubKey []byte  `json:"receiver"`
	SenderPubKey   []byte  `json:"sender"`
}

// innerTransactionOfRelayedV2 is used to parse the inner transaction of a relayed V2 transaction, and holds only the fields handled by Rosetta.
type innerTransactionOfRelayedV2 struct {
	ReceiverPubKey []byte `json:"receiver"`
}

func isRelayedV1Transaction(tx *transaction.ApiTransactionResult) bool {
	return (tx.Type == string(transaction.TxTypeNormal)) &&
		(tx.ProcessingTypeOnSource == transactionProcessingTypeRelayedV1) &&
		(tx.ProcessingTypeOnDestination == transactionProcessingTypeRelayedV1)
}

func isRelayedV2Transaction(tx *transaction.ApiTransactionResult) bool {
	return (tx.Type == string(transaction.TxTypeNormal)) &&
		(tx.ProcessingTypeOnSource == transactionProcessingTypeRelayedV2) &&
		(tx.ProcessingTypeOnDestination == transactionProcessingTypeRelayedV2)
}

func parseInnerTxOfRelayedV1(tx *transaction.ApiTransactionResult) (*innerTransactionOfRelayedV1, error) {
	subparts := strings.Split(string(tx.Data), argumentsSeparator)
	if len(subparts) != 2 {
		return nil, errCannotParseRelayedV1
	}

	innerTxPayloadDecoded, err := hex.DecodeString(subparts[1])
	if err != nil {
		return nil, err
	}

	var innerTx innerTransactionOfRelayedV1

	err = json.Unmarshal(innerTxPayloadDecoded, &innerTx)
	if err != nil {
		return nil, err
	}

	return &innerTx, nil
}

func parseInnerTxOfRelayedV2(tx *transaction.ApiTransactionResult) (*innerTransactionOfRelayedV2, error) {
	subparts := strings.Split(string(tx.Data), argumentsSeparator)
	if len(subparts) != 5 {
		return nil, errCannotParseRelayedV2
	}

	receiverPubKey, err := hex.DecodeString(subparts[1])
	if err != nil {
		return nil, err
	}

	return &innerTransactionOfRelayedV2{
		ReceiverPubKey: receiverPubKey,
	}, nil
}
