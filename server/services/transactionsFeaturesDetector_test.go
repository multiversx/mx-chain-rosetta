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

func TestTransactionsFeatureDetector_isContractDeploymentWithSignalErrorOrIntrashardContractCallWithSignalError(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	detector := newTransactionsFeaturesDetector(networkProvider)

	t.Run("contract deployment, with signal error", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			ProcessingTypeOnSource:      transactionProcessingTypeContractDeployment,
			ProcessingTypeOnDestination: transactionProcessingTypeContractDeployment,
			Logs: &transaction.ApiLogs{
				Events: []*transaction.Events{
					{
						Identifier: transactionEventSignalError,
					},
				},
			},
		}

		require.True(t, detector.isContractDeploymentWithSignalErrorOrIntrashardContractCallWithSignalError(tx))
	})

	t.Run("contract deployment, without signal error", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			ProcessingTypeOnSource:      transactionProcessingTypeContractDeployment,
			ProcessingTypeOnDestination: transactionProcessingTypeContractDeployment,
		}

		require.False(t, detector.isContractDeploymentWithSignalErrorOrIntrashardContractCallWithSignalError(tx))
	})

	t.Run("contract call, with signal error, intra-shard", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			ProcessingTypeOnSource:      transactionProcessingTypeContractInvoking,
			ProcessingTypeOnDestination: transactionProcessingTypeContractInvoking,
			SourceShard:                 2,
			DestinationShard:            2,
			Logs: &transaction.ApiLogs{
				Events: []*transaction.Events{
					{
						Identifier: transactionEventSignalError,
					},
				},
			},
		}

		require.True(t, detector.isContractDeploymentWithSignalErrorOrIntrashardContractCallWithSignalError(tx))
	})

	t.Run("contract call, with signal error, cross-shard", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			ProcessingTypeOnSource:      transactionProcessingTypeContractInvoking,
			ProcessingTypeOnDestination: transactionProcessingTypeContractInvoking,
			SourceShard:                 0,
			DestinationShard:            1,
			Logs: &transaction.ApiLogs{
				Events: []*transaction.Events{
					{
						Identifier: transactionEventSignalError,
					},
				},
			},
		}

		require.False(t, detector.isContractDeploymentWithSignalErrorOrIntrashardContractCallWithSignalError(tx))
	})

	t.Run("contract call, without signal error", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			ProcessingTypeOnSource:      transactionProcessingTypeContractInvoking,
			ProcessingTypeOnDestination: transactionProcessingTypeContractInvoking,
		}

		require.False(t, detector.isContractDeploymentWithSignalErrorOrIntrashardContractCallWithSignalError(tx))
	})

	t.Run("arbitrary transaction", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			ProcessingTypeOnSource:      transactionProcessingTypeMoveBalance,
			ProcessingTypeOnDestination: transactionProcessingTypeMoveBalance,
		}

		require.False(t, detector.isContractDeploymentWithSignalErrorOrIntrashardContractCallWithSignalError(tx))
	})
}

func TestTransactionsFeaturesDetector_isIntrashard(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	detector := newTransactionsFeaturesDetector(networkProvider)

	require.True(t, detector.isIntrashard(&transaction.ApiTransactionResult{
		SourceShard:      0,
		DestinationShard: 0,
	}))

	require.True(t, detector.isIntrashard(&transaction.ApiTransactionResult{
		SourceShard:      1,
		DestinationShard: 1,
	}))

	require.False(t, detector.isIntrashard(&transaction.ApiTransactionResult{
		SourceShard:      0,
		DestinationShard: 1,
	}))
}

func TestTransactionsFeaturesDetector_isSmartContractResultIneffectiveRefund(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	detector := newTransactionsFeaturesDetector(networkProvider)

	require.True(t, detector.isSmartContractResultIneffectiveRefund(&transaction.ApiTransactionResult{
		Sender:   testscommon.TestAddressOfContract,
		Receiver: testscommon.TestAddressOfContract,
		IsRefund: true,
	}))

	require.False(t, detector.isSmartContractResultIneffectiveRefund(&transaction.ApiTransactionResult{
		Sender:   testscommon.TestAddressOfContract,
		Receiver: testscommon.TestAddressOfContract,
		IsRefund: false,
	}))

	require.False(t, detector.isSmartContractResultIneffectiveRefund(&transaction.ApiTransactionResult{
		Sender:   testscommon.TestAddressOfContract,
		Receiver: testscommon.TestAddressAlice,
		IsRefund: false,
	}))
}
