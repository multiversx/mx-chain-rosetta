package services

import (
	"testing"

	"github.com/multiversx/mx-chain-core-go/data/transaction"
	"github.com/multiversx/mx-chain-rosetta/testscommon"
	"github.com/stretchr/testify/require"
)

func TestTransactionsFeaturesDetector_IsInvalidTransactionOfTypeMoveBalanceThatOnlyConsumesDataMovementGas(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	detector := newTransactionsFeaturesDetector(networkProvider)

	t.Run("arbitrary tx", func(t *testing.T) {
		arbitraryTx := &transaction.ApiTransactionResult{
			Hash:     "aaaa",
			Sender:   testscommon.TestAddressAlice,
			Receiver: testscommon.TestAddressBob,
			Value:    "1234",
		}

		featureDetected := detector.isInvalidTransactionOfTypeMoveBalanceThatOnlyConsumesDataMovementGas(arbitraryTx)
		require.False(t, featureDetected)
	})

	t.Run("invalid tx with event 'sending value to non-payable contract'", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			ProcessingTypeOnSource:      transactionProcessingTypeMoveBalance,
			ProcessingTypeOnDestination: transactionProcessingTypeMoveBalance,
			Hash:                        "bbbb",
			Sender:                      testscommon.TestAddressAlice,
			Receiver:                    testscommon.TestAddressOfContract,
			Value:                       "1234",
			Type:                        string(transaction.TxTypeInvalid),
			Logs: &transaction.ApiLogs{

				Events: []*transaction.Events{
					{
						Identifier: transactionEventSignalError,
						Data:       []byte(sendingValueToNonPayableContractDataPrefix + "aaaabbbbccccdddd"),
					},
				},
			},
		}

		featureDetected := detector.isInvalidTransactionOfTypeMoveBalanceThatOnlyConsumesDataMovementGas(tx)
		require.True(t, featureDetected)
	})

	t.Run("invalid tx with event 'invalid meta transaction'", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			ProcessingTypeOnSource:      transactionProcessingTypeMoveBalance,
			ProcessingTypeOnDestination: transactionProcessingTypeMoveBalance,
			Hash:                        "cccc",
			Sender:                      testscommon.TestAddressAlice,
			Receiver:                    testscommon.TestAddressOfContract,
			Value:                       "1234",
			Type:                        string(transaction.TxTypeInvalid),
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

		featureDetected := detector.isInvalidTransactionOfTypeMoveBalanceThatOnlyConsumesDataMovementGas(tx)
		require.True(t, featureDetected)
	})
}

func TestTransactionsFeaturesDetector_isRelayedV1TransactionCompletelyIntrashardWithSignalError(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	detector := newTransactionsFeaturesDetector(networkProvider)

	t.Run("arbitrary relayed tx", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			SourceShard:      0,
			DestinationShard: 1,
		}

		innerTx := &innerTransactionOfRelayedV1{
			SenderPubKey:   testscommon.TestUserShard1.PubKey,
			ReceiverPubKey: testscommon.TestUserShard2.PubKey,
		}

		featureDetected := detector.isRelayedV1TransactionCompletelyIntrashardWithSignalError(tx, innerTx)
		require.False(t, featureDetected)
	})

	t.Run("completely intrashard relayed tx, but no signal error", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			SourceShard:      0,
			DestinationShard: 0,
		}

		innerTx := &innerTransactionOfRelayedV1{
			SenderPubKey:   testscommon.TestUserAShard0.PubKey,
			ReceiverPubKey: testscommon.TestUserBShard0.PubKey,
		}

		featureDetected := detector.isRelayedV1TransactionCompletelyIntrashardWithSignalError(tx, innerTx)
		require.False(t, featureDetected)
	})

	t.Run("completely intrashard relayed tx, with signal error", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			SourceShard:      0,
			DestinationShard: 0,
			Logs: &transaction.ApiLogs{
				Events: []*transaction.Events{
					{
						Identifier: transactionEventSignalError,
					},
				},
			},
		}

		innerTx := &innerTransactionOfRelayedV1{
			SenderPubKey:   testscommon.TestUserAShard0.PubKey,
			ReceiverPubKey: testscommon.TestUserBShard0.PubKey,
		}

		featureDetected := detector.isRelayedV1TransactionCompletelyIntrashardWithSignalError(tx, innerTx)
		require.True(t, featureDetected)
	})
}
