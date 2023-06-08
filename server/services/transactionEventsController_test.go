package services

import (
	"testing"

	"github.com/multiversx/mx-chain-core-go/data/transaction"
	"github.com/multiversx/mx-chain-rosetta/testscommon"
	"github.com/stretchr/testify/require"
)

func TestTransactionEventsController_FindManyEventsByIdentifier(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	controller := newTransactionEventsController(networkProvider)

	t.Run("no matching events", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			Logs: &transaction.ApiLogs{
				Events: []*transaction.Events{
					{
						Identifier: "a",
					},
				},
			},
		}

		events := controller.findManyEventsByIdentifier(tx, "b")
		require.Len(t, events, 0)
	})

	t.Run("more than one matching event", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			Logs: &transaction.ApiLogs{
				Events: []*transaction.Events{
					{
						Identifier: "a",
						Data:       []byte("1"),
					},
					{
						Identifier: "a",
						Data:       []byte("2"),
					},
					{
						Identifier: "b",
						Data:       []byte("3"),
					},
				},
			},
		}

		events := controller.findManyEventsByIdentifier(tx, "a")
		require.Len(t, events, 2)
		require.Equal(t, []byte("1"), events[0].Data)
		require.Equal(t, []byte("2"), events[1].Data)
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
