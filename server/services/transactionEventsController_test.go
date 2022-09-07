package services

import (
	"testing"

	"github.com/ElrondNetwork/elrond-go-core/data/transaction"
	"github.com/ElrondNetwork/elrond-proxy-go/data"
	"github.com/ElrondNetwork/rosetta/testscommon"
	"github.com/stretchr/testify/require"
)

func TestTransactionEventsController_HasSignalErrorOfSendingValueToNonPayableContract(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	controller := newTransactionEventsController(networkProvider)

	t.Run("arbitrary tx", func(t *testing.T) {
		tx := &data.FullTransaction{}
		txMatches := controller.hasSignalErrorOfSendingValueToNonPayableContract(tx)
		require.False(t, txMatches)
	})

	t.Run("invalid tx with event 'sending value to non-payable contract'", func(t *testing.T) {
		tx := &data.FullTransaction{
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
		tx := &data.FullTransaction{}
		txMatches := controller.hasSignalErrorOfMetaTransactionIsInvalid(tx)
		require.False(t, txMatches)
	})

	t.Run("invalid tx with event 'invalid meta transaction'", func(t *testing.T) {
		tx := &data.FullTransaction{
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
