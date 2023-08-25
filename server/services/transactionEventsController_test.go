package services

import (
	"testing"

	"github.com/multiversx/mx-chain-core-go/data/transaction"
	"github.com/multiversx/mx-chain-rosetta/testscommon"
	"github.com/stretchr/testify/require"
)

func TestTransactionEventsController_HasAnySignalError(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	controller := newTransactionEventsController(networkProvider)

	t.Run("arbitrary tx", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{}
		txMatches := controller.hasAnySignalError(tx)
		require.False(t, txMatches)
	})

	t.Run("tx with event 'signalError'", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			Logs: &transaction.ApiLogs{

				Events: []*transaction.Events{
					{
						Identifier: transactionEventSignalError,
					},
				},
			},
		}

		txMatches := controller.hasAnySignalError(tx)
		require.True(t, txMatches)
	})
}

func TestTransactionEventsController_HasSignalErrorOfSendingValueToNonPayableContract(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	controller := newTransactionEventsController(networkProvider)

	t.Run("arbitrary tx", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{}
		txMatches := controller.hasSignalErrorOfSendingValueToNonPayableContract(tx)
		require.False(t, txMatches)
	})

	t.Run("invalid tx with event 'sending value to non-payable contract'", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			Logs: &transaction.ApiLogs{

				Events: []*transaction.Events{
					{
						Identifier: transactionEventSignalError,
						Data:       []byte(sendingValueToNonPayableContractDataPrefix + "aaaabbbbccccdddd"),
					},
				},
			},
		}

		txMatches := controller.hasSignalErrorOfSendingValueToNonPayableContract(tx)
		require.True(t, txMatches)
	})
}

func TestTransactionEventsController_HasSignalErrorOfMetaTransactionIsInvalid(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	controller := newTransactionEventsController(networkProvider)

	t.Run("arbitrary tx", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{}
		txMatches := controller.hasSignalErrorOfMetaTransactionIsInvalid(tx)
		require.False(t, txMatches)
	})

	t.Run("invalid tx with event 'meta transaction is invalid'", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			Logs: &transaction.ApiLogs{

				Events: []*transaction.Events{
					{
						Identifier: transactionEventSignalError,
						Topics: [][]byte{
							[]byte(transactionEventTopicInvalidMetaTransaction),
						},
					},
				},
			},
		}

		txMatches := controller.hasSignalErrorOfMetaTransactionIsInvalid(tx)
		require.True(t, txMatches)
	})

	t.Run("invalid tx with event 'meta transaction is invalid: not enough gas'", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			Logs: &transaction.ApiLogs{

				Events: []*transaction.Events{
					{
						Identifier: transactionEventSignalError,
						Topics: [][]byte{
							[]byte(transactionEventTopicInvalidMetaTransactionNotEnoughGas),
						},
					},
				},
			},
		}

		txMatches := controller.hasSignalErrorOfMetaTransactionIsInvalid(tx)
		require.True(t, txMatches)
	})
}

func Test(t *testing.T) {
	event := transaction.Events{
		Identifier: transactionEventSignalError,
		Topics: [][]byte{
			[]byte("foo"),
		},
	}

	require.True(t, eventHasTopic(&event, "foo"))
	require.False(t, eventHasTopic(&event, "bar"))
}

func TestEventHasTopic(t *testing.T) {
	event := transaction.Events{
		Identifier: transactionEventSignalError,
		Topics: [][]byte{
			[]byte("foo"),
		},
	}

	require.True(t, eventHasTopic(&event, "foo"))
	require.False(t, eventHasTopic(&event, "bar"))
}
