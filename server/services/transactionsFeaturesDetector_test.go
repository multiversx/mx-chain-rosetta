package services

import (
	"testing"

	"github.com/ElrondNetwork/elrond-go-core/data/transaction"
	"github.com/ElrondNetwork/elrond-proxy-go/data"
	"github.com/ElrondNetwork/rosetta/testscommon"
	"github.com/stretchr/testify/require"
)

func TestFeaturesDetector_IsInvalidTransactionOfTypeMoveBalanceThatOnlyConsumesDataMovementGas(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	detector := newTransactionsFeaturesDetector(networkProvider)

	t.Run("arbitrary tx", func(t *testing.T) {
		arbitraryTx := &data.FullTransaction{
			Hash:     "aaaa",
			Sender:   testscommon.TestAddressAlice,
			Receiver: testscommon.TestAddressBob,
			Value:    "1234",
		}

		featureDetected := detector.isInvalidTransactionOfTypeMoveBalanceThatOnlyConsumesDataMovementGas(arbitraryTx)
		require.False(t, featureDetected)
	})

	t.Run("invalid tx with event 'sending value to non-payable contract'", func(t *testing.T) {
		tx := &data.FullTransaction{
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
		tx := &data.FullTransaction{
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