package services

import (
	"encoding/hex"
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

func TestTransactionsFeaturesDetector_IsRelayedTransactionCompletelyIntrashardWithSignalError(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	detector := newTransactionsFeaturesDetector(networkProvider)

	pubkeyAShard0, _ := hex.DecodeString("8049d639e5a6980d1cd2392abcce41029cda74a1563523a202f09641cc2618f8")
	pubkeyBShard0, _ := hex.DecodeString("e32afedc904fe1939746ad973beb383563cf63642ba669b3040f9b9428a5ed60")
	pubkeyShard1, _ := hex.DecodeString("0139472eff6886771a982f3083da5d421f24c29181e63888228dc81ca60d69e1")
	pubkeyShard2, _ := hex.DecodeString("b2a11555ce521e4944e09ab17549d85b487dcd26c84b5017a39e31a3670889ba")

	t.Run("arbitrary relayed tx", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			SourceShard:      0,
			DestinationShard: 1,
		}

		innerTx := &innerTransactionOfRelayedV1{
			SenderPubKey:   pubkeyShard1,
			ReceiverPubKey: pubkeyShard2,
		}

		featureDetected := detector.isRelayedTransactionCompletelyIntrashardWithSignalError(tx, innerTx)
		require.False(t, featureDetected)
	})

	t.Run("completely intrashard relayed tx, but no signal error", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			SourceShard:      0,
			DestinationShard: 0,
		}

		innerTx := &innerTransactionOfRelayedV1{
			SenderPubKey:   pubkeyAShard0,
			ReceiverPubKey: pubkeyBShard0,
		}

		featureDetected := detector.isRelayedTransactionCompletelyIntrashardWithSignalError(tx, innerTx)
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
			SenderPubKey:   pubkeyAShard0,
			ReceiverPubKey: pubkeyBShard0,
		}

		featureDetected := detector.isRelayedTransactionCompletelyIntrashardWithSignalError(tx, innerTx)
		require.True(t, featureDetected)
	})
}
