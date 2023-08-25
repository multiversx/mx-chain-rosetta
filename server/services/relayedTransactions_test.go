package services

import (
	"testing"

	"github.com/multiversx/mx-chain-core-go/data/transaction"
	"github.com/stretchr/testify/require"
)

func Test_IsRelayedV1Transaction(t *testing.T) {
	t.Run("arbitrary tx", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{}
		require.False(t, isRelayedV1Transaction(tx))
	})

	t.Run("relayed v1 tx", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			Type:                        string(transaction.TxTypeNormal),
			ProcessingTypeOnSource:      transactionProcessingTypeRelayed,
			ProcessingTypeOnDestination: transactionProcessingTypeRelayed,
		}

		require.True(t, isRelayedV1Transaction(tx))
	})
}
