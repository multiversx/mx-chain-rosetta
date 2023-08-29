package services

import (
	"testing"

	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/multiversx/mx-chain-core-go/data/transaction"
	"github.com/multiversx/mx-chain-rosetta/testscommon"
	"github.com/stretchr/testify/require"
)

func TestTransactionsTransformer_NormalTxToRosettaTx(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	extension := newNetworkProviderExtension(networkProvider)
	transformer := newTransactionsTransformer(networkProvider)

	t.Run("move balance tx", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			Hash:             "aaaa",
			Sender:           testscommon.TestAddressAlice,
			Receiver:         testscommon.TestAddressBob,
			Value:            "1234",
			InitiallyPaidFee: "50000000000000",
		}

		expectedRosettaTx := &types.Transaction{
			TransactionIdentifier: hashToTransactionIdentifier("aaaa"),
			Operations: []*types.Operation{
				{
					Type:    opTransfer,
					Account: addressToAccountIdentifier(testscommon.TestAddressAlice),
					Amount:  extension.valueToNativeAmount("-1234"),
				},
				{
					Type:    opTransfer,
					Account: addressToAccountIdentifier(testscommon.TestAddressBob),
					Amount:  extension.valueToNativeAmount("1234"),
				},
				{
					Type:    opFee,
					Account: addressToAccountIdentifier(testscommon.TestAddressAlice),
					Amount:  extension.valueToNativeAmount("-50000000000000"),
				},
			},
			Metadata: extractTransactionMetadata(tx),
		}

		rosettaTx, err := transformer.normalTxToRosetta(tx)
		require.NoError(t, err)
		require.Equal(t, expectedRosettaTx, rosettaTx)
	})

	t.Run("relayed tx, completely intrashard, with signal error", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			Type:                        string(transaction.TxTypeNormal),
			ProcessingTypeOnSource:      transactionProcessingTypeRelayed,
			ProcessingTypeOnDestination: transactionProcessingTypeRelayed,
			Hash:                        "aaaa",
			Sender:                      testscommon.TestUserAShard0.Address,
			Receiver:                    testscommon.TestUserBShard0.Address,
			SourceShard:                 0,
			DestinationShard:            0,
			InitiallyPaidFee:            "50000000000000",
			// Has non-zero value
			Data: []byte("relayedTx@7b226e6f6e6365223a372c2273656e646572223a226e69424758747949504349644a78793373796c6c455a474c78506a503148734a45646e43732b6d726577413d222c227265636569766572223a224141414141414141414141464145356c3079623173734a3933504433672f4b396f48384579366d576958513d222c2276616c7565223a313030303030303030303030303030303030302c226761735072696365223a313030303030303030302c226761734c696d6974223a35303030302c2264617461223a22222c227369676e6174757265223a226e6830743338585a77614b6a725878446969716f6d364d6a5671724d612f6b70767474696a33692b5a6d43492f3778626830596762363548424151445744396f7036575567674541755430756e5253595736455341413d3d222c22636861696e4944223a224d513d3d222c2276657273696f6e223a327d"),
			Logs: &transaction.ApiLogs{
				Events: []*transaction.Events{
					{
						Identifier: transactionEventSignalError,
					},
				},
			},
		}

		expectedRosettaTx := &types.Transaction{
			TransactionIdentifier: hashToTransactionIdentifier("aaaa"),
			Operations: []*types.Operation{
				{
					Type:    opFee,
					Account: addressToAccountIdentifier(testscommon.TestUserAShard0.Address),
					Amount:  extension.valueToNativeAmount("-50000000000000"),
				},
				{
					Type:    opTransfer,
					Account: addressToAccountIdentifier(testscommon.TestUserCShard0.Address),
					Amount:  extension.valueToNativeAmount("-1000000000000000000"),
				},
			},
			Metadata: extractTransactionMetadata(tx),
		}

		rosettaTx, err := transformer.normalTxToRosetta(tx)
		require.NoError(t, err)
		require.Equal(t, expectedRosettaTx, rosettaTx)
	})
}

func TestTransactionsTransformer_ExtractInnerTxOperationsIfRelayedCompletelyIntrashardWithSignalError(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	extension := newNetworkProviderExtension(networkProvider)
	transformer := newTransactionsTransformer(networkProvider)

	t.Run("non-relayed tx", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			Type: string(transaction.TxTypeNormal),
		}

		operations, err := transformer.extractInnerTxOperationsIfRelayedCompletelyIntrashardWithSignalError(tx)
		require.NoError(t, err)
		require.Len(t, operations, 0)
	})

	t.Run("relayed tx (badly formatted)", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			Type:                        string(transaction.TxTypeNormal),
			ProcessingTypeOnSource:      transactionProcessingTypeRelayed,
			ProcessingTypeOnDestination: transactionProcessingTypeRelayed,
			Data:                        []byte("bad"),
		}

		operations, err := transformer.extractInnerTxOperationsIfRelayedCompletelyIntrashardWithSignalError(tx)
		require.ErrorIs(t, err, errCannotParseRelayedV1)
		require.Nil(t, operations)
	})

	t.Run("relayed tx, completely intrashard, with signal error, inner tx has non-zero value", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			Type:                        string(transaction.TxTypeNormal),
			ProcessingTypeOnSource:      transactionProcessingTypeRelayed,
			ProcessingTypeOnDestination: transactionProcessingTypeRelayed,
			SourceShard:                 0,
			DestinationShard:            0,
			Data:                        []byte("relayedTx@7b226e6f6e6365223a372c2273656e646572223a226e69424758747949504349644a78793373796c6c455a474c78506a503148734a45646e43732b6d726577413d222c227265636569766572223a224141414141414141414141464145356c3079623173734a3933504433672f4b396f48384579366d576958513d222c2276616c7565223a313030303030303030303030303030303030302c226761735072696365223a313030303030303030302c226761734c696d6974223a35303030302c2264617461223a22222c227369676e6174757265223a226e6830743338585a77614b6a725878446969716f6d364d6a5671724d612f6b70767474696a33692b5a6d43492f3778626830596762363548424151445744396f7036575567674541755430756e5253595736455341413d3d222c22636861696e4944223a224d513d3d222c2276657273696f6e223a327d"),
			Logs: &transaction.ApiLogs{
				Events: []*transaction.Events{
					{
						Identifier: transactionEventSignalError,
					},
				},
			},
		}

		operations, err := transformer.extractInnerTxOperationsIfRelayedCompletelyIntrashardWithSignalError(tx)
		require.NoError(t, err)
		require.Equal(t, []*types.Operation{
			{
				Type:    opTransfer,
				Account: addressToAccountIdentifier(testscommon.TestUserCShard0.Address),
				Amount:  extension.valueToNativeAmount("-1000000000000000000"),
			},
		}, operations)
	})

	t.Run("relayed tx, completely intrashard, with signal error, inner tx has zero value", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			Type:                        string(transaction.TxTypeNormal),
			ProcessingTypeOnSource:      transactionProcessingTypeRelayed,
			ProcessingTypeOnDestination: transactionProcessingTypeRelayed,
			SourceShard:                 0,
			DestinationShard:            0,
			Data:                        []byte("relayedTx@7b226e6f6e6365223a372c2273656e646572223a226e69424758747949504349644a78793373796c6c455a474c78506a503148734a45646e43732b6d726577413d222c227265636569766572223a224141414141414141414141464145356c3079623173734a3933504433672f4b396f48384579366d576958513d222c2276616c7565223a302c226761735072696365223a313030303030303030302c226761734c696d6974223a35303030302c2264617461223a22222c227369676e6174757265223a22336c644e7a32435734416143675069495863636c466b4654324149586a4a4757316a526a306c542b4f3161736b6241394a744e365a764173396e394f58716d343130574a49665332332b4168666e48793267446c41773d3d222c22636861696e4944223a224d513d3d222c2276657273696f6e223a327d"),
			Logs: &transaction.ApiLogs{
				Events: []*transaction.Events{
					{
						Identifier: transactionEventSignalError,
					},
				},
			},
		}

		operations, err := transformer.extractInnerTxOperationsIfRelayedCompletelyIntrashardWithSignalError(tx)
		require.NoError(t, err)
		require.Len(t, operations, 0)
	})
}

func TestTransactionsTransformer_UnsignedTxToRosettaTx(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	extension := newNetworkProviderExtension(networkProvider)
	transformer := newTransactionsTransformer(networkProvider)

	refundTx := &transaction.ApiTransactionResult{
		Hash:     "aaaa",
		Sender:   testscommon.TestAddressOfContract,
		Receiver: testscommon.TestAddressAlice,
		Value:    "1234",
		IsRefund: true,
	}

	expectedRefundTx := &types.Transaction{
		TransactionIdentifier: hashToTransactionIdentifier("aaaa"),
		Operations: []*types.Operation{
			{
				Type:    opFeeRefundAsScResult,
				Account: addressToAccountIdentifier(testscommon.TestAddressAlice),
				Amount:  extension.valueToNativeAmount("1234"),
			},
		},
	}

	moveBalanceTx := &transaction.ApiTransactionResult{
		Hash:     "aaaa",
		Sender:   testscommon.TestAddressOfContract,
		Receiver: testscommon.TestAddressAlice,
		Value:    "1234",
	}

	expectedMoveBalanceTx := &types.Transaction{
		TransactionIdentifier: hashToTransactionIdentifier("aaaa"),
		Operations: []*types.Operation{
			{
				Type:    opScResult,
				Account: addressToAccountIdentifier(testscommon.TestAddressOfContract),
				Amount:  extension.valueToNativeAmount("-1234"),
			},
			{
				Type:    opScResult,
				Account: addressToAccountIdentifier(testscommon.TestAddressAlice),
				Amount:  extension.valueToNativeAmount("1234"),
			},
		},
		Metadata: extractTransactionMetadata(moveBalanceTx),
	}

	txsInBlock := []*transaction.ApiTransactionResult{refundTx, moveBalanceTx}

	rosettaRefundTx := transformer.unsignedTxToRosettaTx(refundTx, txsInBlock)
	rosettaMoveBalanceTx := transformer.unsignedTxToRosettaTx(moveBalanceTx, txsInBlock)
	require.Equal(t, expectedRefundTx, rosettaRefundTx)
	require.Equal(t, expectedMoveBalanceTx, rosettaMoveBalanceTx)
}

func TestTransactionsTransformer_InvalidTxToRosettaTx(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	extension := newNetworkProviderExtension(networkProvider)
	transformer := newTransactionsTransformer(networkProvider)

	tx := &transaction.ApiTransactionResult{
		Hash:             "aaaa",
		Sender:           testscommon.TestAddressAlice,
		Receiver:         testscommon.TestAddressBob,
		Value:            "1234",
		Type:             string(transaction.TxTypeInvalid),
		InitiallyPaidFee: "50000000000000",
	}

	expectedTx := &types.Transaction{
		TransactionIdentifier: hashToTransactionIdentifier("aaaa"),
		Operations: []*types.Operation{
			{
				Status:  &opStatusFailure,
				Type:    opTransfer,
				Account: addressToAccountIdentifier(testscommon.TestAddressAlice),
				Amount:  extension.valueToNativeAmount("-1234"),
			},
			{
				Status:  &opStatusFailure,
				Type:    opTransfer,
				Account: addressToAccountIdentifier(testscommon.TestAddressBob),
				Amount:  extension.valueToNativeAmount("1234"),
			},
			{
				Type:    opFeeOfInvalidTx,
				Account: addressToAccountIdentifier(testscommon.TestAddressAlice),
				Amount:  extension.valueToNativeAmount("-50000000000000"),
			},
		},
		Metadata: extractTransactionMetadata(tx),
	}

	rosettaTx := transformer.invalidTxToRosettaTx(tx)
	require.Equal(t, expectedTx, rosettaTx)
}
