package services

import (
	"encoding/hex"
	"encoding/json"
	"math/big"
	"strings"

	"github.com/multiversx/mx-chain-core-go/data/transaction"
)

func isRelayedV1Transaction(tx *transaction.ApiTransactionResult) bool {
	return (tx.Type == string(transaction.TxTypeNormal)) &&
		(tx.ProcessingTypeOnSource == transactionProcessingTypeRelayed) &&
		(tx.ProcessingTypeOnDestination == transactionProcessingTypeRelayed)
}

