package services

import (
	"encoding/json"
	"io"
	"testing"

	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/multiversx/mx-chain-core-go/core"
	"github.com/multiversx/mx-chain-core-go/data/api"
	"github.com/multiversx/mx-chain-core-go/data/transaction"
	"github.com/multiversx/mx-chain-rosetta/server/resources"
	"github.com/multiversx/mx-chain-rosetta/testscommon"
	"github.com/stretchr/testify/require"
)

func TestTransactionsTransformer_NormalTxToRosettaTx(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	extension := newNetworkProviderExtension(networkProvider)
	transformer := newTransactionsTransformer(networkProvider)

	networkProvider.MockActivationEpochSirius = 42

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

	t.Run("contract deployment with signal error (after Sirius)", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			Epoch:                       43,
			ProcessingTypeOnSource:      "SCDeployment",
			ProcessingTypeOnDestination: "SCDeployment",
			Hash:                        "aaaa",
			Sender:                      testscommon.TestAddressAlice,
			Receiver:                    testscommon.TestAddressBob,
			Value:                       "1234",
			InitiallyPaidFee:            "5000000000000000",
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
					Account: addressToAccountIdentifier(testscommon.TestAddressAlice),
					Amount:  extension.valueToNativeAmount("-5000000000000000"),
				},
			},
			Metadata: extractTransactionMetadata(tx),
		}

		rosettaTx, err := transformer.normalTxToRosetta(tx)
		require.NoError(t, err)
		require.Equal(t, expectedRosettaTx, rosettaTx)
	})

	t.Run("intra-shard contract call with signal error (after Sirius)", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			Epoch:                       43,
			ProcessingTypeOnSource:      "SCInvoking",
			ProcessingTypeOnDestination: "SCInvoking",
			Hash:                        "aaaa",
			Sender:                      testscommon.TestUserAShard0.Address,
			Receiver:                    testscommon.TestContractBarShard0.Address,
			SourceShard:                 0,
			DestinationShard:            0,
			Value:                       "1234",
			InitiallyPaidFee:            "5000000000000000",
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
					Amount:  extension.valueToNativeAmount("-5000000000000000"),
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
			Epoch:                       43,
			Type:                        string(transaction.TxTypeNormal),
			ProcessingTypeOnSource:      transactionProcessingTypeRelayedV1,
			ProcessingTypeOnDestination: transactionProcessingTypeRelayedV1,
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
			},
			Metadata: extractTransactionMetadata(tx),
		}

		rosettaTx, err := transformer.normalTxToRosetta(tx)
		require.NoError(t, err)
		require.Equal(t, expectedRosettaTx, rosettaTx)
	})

	t.Run("relayed tx V3", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			Hash:             "aaaa",
			Sender:           testscommon.TestAddressAlice,
			Receiver:         testscommon.TestAddressBob,
			RelayerAddress:   testscommon.TestAddressCarol,
			Value:            "1234",
			InitiallyPaidFee: "50000000000000",
			Signature:        "signature",
			RelayerSignature: "signature",
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
					Account: addressToAccountIdentifier(testscommon.TestAddressCarol),
					Amount:  extension.valueToNativeAmount("-50000000000000"),
				},
			},
			Metadata: extractTransactionMetadata(tx),
		}

		rosettaTx, err := transformer.normalTxToRosetta(tx)
		require.NoError(t, err)
		require.Equal(t, expectedRosettaTx, rosettaTx)
	})
}

func TestTransactionsTransformer_decideFeePayer(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	transformer := newTransactionsTransformer(networkProvider)

	t.Run("when fee payer is sender", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			Sender:   testscommon.TestAddressAlice,
			Receiver: testscommon.TestAddressBob,
		}

		feePayer := transformer.decideFeePayer(tx)
		require.Equal(t, testscommon.TestAddressAlice, feePayer)
	})

	t.Run("when fee payer is relayer", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			Sender:           testscommon.TestAddressAlice,
			Receiver:         testscommon.TestAddressBob,
			RelayerAddress:   testscommon.TestAddressCarol,
			Signature:        "signature",
			RelayerSignature: "signature",
		}

		feePayer := transformer.decideFeePayer(tx)
		require.Equal(t, testscommon.TestAddressCarol, feePayer)
	})
}

func TestTransactionsTransformer_UnsignedTxToRosettaTx(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	extension := newNetworkProviderExtension(networkProvider)
	transformer := newTransactionsTransformer(networkProvider)

	t.Run("refund SCR", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			Hash:     "aaaa",
			Sender:   testscommon.TestAddressOfContract,
			Receiver: testscommon.TestAddressAlice,
			Value:    "1234",
			IsRefund: true,
		}

		expectedTx := &types.Transaction{
			TransactionIdentifier: hashToTransactionIdentifier("aaaa"),
			Operations: []*types.Operation{
				{
					Type:    opFeeRefundAsScResult,
					Account: addressToAccountIdentifier(testscommon.TestAddressAlice),
					Amount:  extension.valueToNativeAmount("1234"),
				},
			},
		}

		rosettaTx := transformer.unsignedTxToRosettaTx(tx, nil)
		require.Equal(t, expectedTx, rosettaTx)
	})

	t.Run("move balance SCR", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			Hash:     "aaaa",
			Sender:   testscommon.TestAddressOfContract,
			Receiver: testscommon.TestAddressAlice,
			Value:    "1234",
		}

		expectedTx := &types.Transaction{
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
			Metadata: extractTransactionMetadata(tx),
		}

		rosettaTx := transformer.unsignedTxToRosettaTx(tx, []*transaction.ApiTransactionResult{tx})
		require.Equal(t, expectedTx, rosettaTx)
	})

	t.Run("ineffective refund SCR", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			Hash:     "aaaa",
			Sender:   testscommon.TestAddressOfContract,
			Receiver: testscommon.TestAddressOfContract,
			Value:    "1234",
			IsRefund: true,
		}

		expectedTx := &types.Transaction{
			TransactionIdentifier: hashToTransactionIdentifier("aaaa"),
			Operations:            []*types.Operation{},
			Metadata:              nil,
		}

		rosettaTx := transformer.unsignedTxToRosettaTx(tx, []*transaction.ApiTransactionResult{tx})
		require.Equal(t, expectedTx, rosettaTx)
	})
}

func TestTransactionsTransformer_InvalidTxToRosettaTx(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	extension := newNetworkProviderExtension(networkProvider)
	transformer := newTransactionsTransformer(networkProvider)

	t.Run("when fee payer is sender", func(t *testing.T) {
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
	})

	t.Run("when fee payer is relayer", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			Hash:             "aaaa",
			Sender:           testscommon.TestAddressAlice,
			Receiver:         testscommon.TestAddressBob,
			RelayerAddress:   testscommon.TestAddressCarol,
			Value:            "1234",
			Type:             string(transaction.TxTypeInvalid),
			InitiallyPaidFee: "50000000000000",
			Signature:        "signature",
			RelayerSignature: "signature",
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
					Account: addressToAccountIdentifier(testscommon.TestAddressCarol),
					Amount:  extension.valueToNativeAmount("-50000000000000"),
				},
			},
			Metadata: extractTransactionMetadata(tx),
		}

		rosettaTx := transformer.invalidTxToRosettaTx(tx)
		require.Equal(t, expectedTx, rosettaTx)
	})
}

func TestTransactionsTransformer_TransformBlockTxsHavingContractDeployments(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	networkProvider.MockObservedActualShard = 0

	extension := newNetworkProviderExtension(networkProvider)
	transformer := newTransactionsTransformer(networkProvider)

	blocks, err := readTestBlocks("testdata/blocks_with_contract_deployments.json")
	require.Nil(t, err)

	t.Run("direct contract deployment, with value", func(t *testing.T) {
		txs, err := transformer.transformBlockTxs(blocks[0])
		require.Nil(t, err)
		require.Len(t, txs, 1)
		require.Len(t, txs[0].Operations, 5)

		expectedTx := &types.Transaction{
			TransactionIdentifier: hashToTransactionIdentifier("2459bb2b9a64c1c920777ecbdaf0fa33d7fe8bcd24d7164562f341b2e4f702da"),
			Operations: []*types.Operation{
				{
					Type:                opTransfer,
					OperationIdentifier: indexToOperationIdentifier(0),
					Account:             addressToAccountIdentifier("erd1tn62hjp72rznp8vq0lplva5csav6rccpqqdungpxtqz0g2hcq6uq9k4cc6"),
					Amount:              extension.valueToNativeAmount("-10000000000000000"),
					Status:              &opStatusSuccess,
				},
				{
					Type:                opTransfer,
					OperationIdentifier: indexToOperationIdentifier(1),
					Account:             addressToAccountIdentifier("erd1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq6gq4hu"),
					Amount:              extension.valueToNativeAmount("10000000000000000"),
					Status:              &opStatusSuccess,
				},
				{
					Type:                opFee,
					OperationIdentifier: indexToOperationIdentifier(2),
					Account:             addressToAccountIdentifier("erd1tn62hjp72rznp8vq0lplva5csav6rccpqqdungpxtqz0g2hcq6uq9k4cc6"),
					Amount:              extension.valueToNativeAmount("-457385000000000"),
					Status:              &opStatusSuccess,
				},
				{
					Type:                opTransfer,
					OperationIdentifier: indexToOperationIdentifier(3),
					Account:             addressToAccountIdentifier("erd1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq6gq4hu"),
					Amount:              extension.valueToNativeAmount("-10000000000000000"),
					Status:              &opStatusSuccess,
				},
				{
					Type:                opTransfer,
					OperationIdentifier: indexToOperationIdentifier(4),
					Account:             addressToAccountIdentifier("erd1qqqqqqqqqqqqqpgqc4vdnqgc48ww26ljxqe2flgl86jewg0nq6uqna2ymj"),
					Amount:              extension.valueToNativeAmount("10000000000000000"),
					Status:              &opStatusSuccess,
				},
			},
			Metadata: extractTransactionMetadata(blocks[0].MiniBlocks[0].Transactions[0]),
		}

		require.Equal(t, expectedTx, txs[0])
	})

	t.Run("direct contract deployment, with value, with signal error", func(t *testing.T) {
		txs, err := transformer.transformBlockTxs(blocks[1])
		require.Nil(t, err)
		require.Len(t, txs, 1)
		require.Len(t, txs[0].Operations, 1)

		expectedTx := &types.Transaction{
			TransactionIdentifier: hashToTransactionIdentifier("52b74f025158f18512353da5f23c2e31c78c49d7b73da7d24f048f86da48b5c5"),
			Operations: []*types.Operation{
				{
					Type:                opFee,
					OperationIdentifier: indexToOperationIdentifier(0),
					Account:             addressToAccountIdentifier("erd1tn62hjp72rznp8vq0lplva5csav6rccpqqdungpxtqz0g2hcq6uq9k4cc6"),
					Amount:              extension.valueToNativeAmount("-2206715000000000"),
					Status:              &opStatusSuccess,
				},
			},
			Metadata: extractTransactionMetadata(blocks[1].MiniBlocks[0].Transactions[0]),
		}

		require.Equal(t, expectedTx, txs[0])
	})
}

func TestTransactionsTransformer_TransformBlockTxsHavingContractCalls(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	networkProvider.MockObservedActualShard = 0

	extension := newNetworkProviderExtension(networkProvider)
	transformer := newTransactionsTransformer(networkProvider)

	blocks, err := readTestBlocks("testdata/blocks_with_contract_calls.json")
	require.Nil(t, err)

	t.Run("direct contract call, with value, with signal error", func(t *testing.T) {
		txs, err := transformer.transformBlockTxs(blocks[0])
		require.Nil(t, err)
		require.Len(t, txs, 1)
		require.Len(t, txs[0].Operations, 1)

		expectedTx := &types.Transaction{
			TransactionIdentifier: hashToTransactionIdentifier("9851ab6cb221bce5800030f9db2a5fbd8ed4a9db4c6f9d190c16113f2b080e57"),
			Operations: []*types.Operation{
				{
					Type:                opFee,
					OperationIdentifier: indexToOperationIdentifier(0),
					Account:             addressToAccountIdentifier("erd1tn62hjp72rznp8vq0lplva5csav6rccpqqdungpxtqz0g2hcq6uq9k4cc6"),
					Amount:              extension.valueToNativeAmount("-144050000000000"),
					Status:              &opStatusSuccess,
				},
			},
			Metadata: extractTransactionMetadata(blocks[0].MiniBlocks[0].Transactions[0]),
		}

		require.Equal(t, expectedTx, txs[0])
	})
}

func TestTransactionsTransformer_TransformBlockTxsHavingESDTIssue(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	networkProvider.MockCustomCurrencies = []resources.Currency{{Symbol: "FOO-6d28db"}}

	extension := newNetworkProviderExtension(networkProvider)
	transformer := newTransactionsTransformer(networkProvider)

	blocks, err := readTestBlocks("testdata/blocks_with_esdt_issue.json")
	require.Nil(t, err)

	// Block 0 (issue ESDT)
	txs, err := transformer.transformBlockTxs(blocks[0])
	require.Nil(t, err)
	require.Len(t, txs, 1)

	expectedIssueTx := &types.Transaction{
		TransactionIdentifier: hashToTransactionIdentifier("851d90b7b0770c648de5945ca76d2ded62a540856467911db5e550ce6a959807"),
		Operations: []*types.Operation{
			{
				Type:                opTransfer,
				OperationIdentifier: indexToOperationIdentifier(0),
				Account:             addressToAccountIdentifier("erd1testnlersh4z0wsv8kjx39me4rmnvjkwu8dsaea7ukdvvc9z396qykv7z7"),
				Amount:              extension.valueToNativeAmount("-50000000000000000"),
				Status:              &opStatusSuccess,
			},
			{
				Type:                opFee,
				OperationIdentifier: indexToOperationIdentifier(1),
				Account:             addressToAccountIdentifier("erd1testnlersh4z0wsv8kjx39me4rmnvjkwu8dsaea7ukdvvc9z396qykv7z7"),
				Amount:              extension.valueToNativeAmount("-1220275000000000"),
				Status:              &opStatusSuccess,
			},
		},
		Metadata: extractTransactionMetadata(blocks[0].MiniBlocks[0].Transactions[0]),
	}

	require.Equal(t, expectedIssueTx, txs[0])

	// Block 1 (results of issue ESDT)
	txs, err = transformer.transformBlockTxs(blocks[1])
	require.Nil(t, err)
	require.Len(t, txs, 2)

	expectedRefundSCR := &types.Transaction{
		TransactionIdentifier: hashToTransactionIdentifier("8fa82004d9eb82e34b39bbe22521a7b85a190950cd6876d2e97950de906622d7"),
		Operations: []*types.Operation{
			{
				Type:                opFeeRefundAsScResult,
				OperationIdentifier: indexToOperationIdentifier(0),
				Account:             addressToAccountIdentifier("erd1testnlersh4z0wsv8kjx39me4rmnvjkwu8dsaea7ukdvvc9z396qykv7z7"),
				Amount:              extension.valueToNativeAmount("497775000000000"),
				Status:              &opStatusSuccess,
			},
		},
	}

	expectedTransferSCR := &types.Transaction{
		TransactionIdentifier: hashToTransactionIdentifier("e462e1b73b720015315d0f508d165817ba1989cb1d2c63903bd01c3b8450afb8"),
		Operations: []*types.Operation{
			{
				Type:                opCustomTransfer,
				OperationIdentifier: indexToOperationIdentifier(0),
				Account:             addressToAccountIdentifier("erd1testnlersh4z0wsv8kjx39me4rmnvjkwu8dsaea7ukdvvc9z396qykv7z7"),
				Amount:              extension.valueToCustomAmount("1000000000000", "FOO-6d28db"),
				Status:              &opStatusSuccess,
			},
		},
		Metadata: extractTransactionMetadata(blocks[1].MiniBlocks[0].Transactions[1]),
	}

	require.Equal(t, expectedRefundSCR, txs[0])
	require.Equal(t, expectedTransferSCR, txs[1])
}

func TestTransactionsTransformer_ExtractOperationsFromEventESDT(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	networkProvider.MockCustomCurrencies = []resources.Currency{{Symbol: "ROSETTA-3a2edf"}}

	extension := newNetworkProviderExtension(networkProvider)
	transformer := newTransactionsTransformer(networkProvider)

	t.Run("with custom currency (known)", func(t *testing.T) {
		event := &eventESDT{
			identifier:      "ROSETTA-3a2edf",
			senderAddress:   testscommon.TestAddressAlice,
			receiverAddress: testscommon.TestAddressBob,
			value:           "1234",
		}

		expectedOperations := []*types.Operation{
			{
				Type:    opCustomTransfer,
				Account: addressToAccountIdentifier(testscommon.TestAddressAlice),
				Amount:  extension.valueToCustomAmount("-1234", "ROSETTA-3a2edf"),
			},
			{
				Type:    opCustomTransfer,
				Account: addressToAccountIdentifier(testscommon.TestAddressBob),
				Amount:  extension.valueToCustomAmount("1234", "ROSETTA-3a2edf"),
			},
		}

		operations := transformer.extractOperationsFromEventESDT(event)
		require.Equal(t, expectedOperations, operations)
	})

	t.Run("with custom currency (unknown)", func(t *testing.T) {
		event := &eventESDT{
			identifier:      "UNKNOWN-3a2edf",
			senderAddress:   testscommon.TestAddressAlice,
			receiverAddress: testscommon.TestAddressBob,
			value:           "1234",
		}

		operations := transformer.extractOperationsFromEventESDT(event)
		require.Len(t, operations, 0)
	})

	t.Run("with native currency", func(t *testing.T) {
		event := &eventESDT{
			identifier:      nativeAsESDTIdentifier,
			senderAddress:   testscommon.TestAddressAlice,
			receiverAddress: testscommon.TestAddressBob,
			value:           "1234",
		}

		expectedOperations := []*types.Operation{
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
		}

		operations := transformer.extractOperationsFromEventESDT(event)
		require.Equal(t, expectedOperations, operations)
	})
}

func TestTransactionsTransformer_TransformBlockTxsHavingESDTTransfer(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	networkProvider.MockCustomCurrencies = []resources.Currency{{Symbol: "ROSETTA-3a2edf"}}

	extension := newNetworkProviderExtension(networkProvider)
	transformer := newTransactionsTransformer(networkProvider)

	blocks, err := readTestBlocks("testdata/blocks_with_esdt_transfer.json")
	require.Nil(t, err)

	// Block 0 contains the transfer and the fee refund
	txs, err := transformer.transformBlockTxs(blocks[0])
	require.Nil(t, err)
	require.Len(t, txs, 2)

	expectedTransferTx := &types.Transaction{
		TransactionIdentifier: hashToTransactionIdentifier("b35680324380e8fb4c954a26190159bfc7b55463497443163b1123a6407040a7"),
		Operations: []*types.Operation{
			{
				Type:                opFee,
				OperationIdentifier: indexToOperationIdentifier(0),
				Account:             addressToAccountIdentifier("erd1testnlersh4z0wsv8kjx39me4rmnvjkwu8dsaea7ukdvvc9z396qykv7z7"),
				Amount:              extension.valueToNativeAmount("-119840000000000"),
				Status:              &opStatusSuccess,
			},
			{
				Type:                opCustomTransfer,
				OperationIdentifier: indexToOperationIdentifier(1),
				Account:             addressToAccountIdentifier("erd1testnlersh4z0wsv8kjx39me4rmnvjkwu8dsaea7ukdvvc9z396qykv7z7"),
				Amount:              extension.valueToCustomAmount("-50", "ROSETTA-3a2edf"),
				Status:              &opStatusSuccess,
			},
			{
				Type:                opCustomTransfer,
				OperationIdentifier: indexToOperationIdentifier(2),
				Account:             addressToAccountIdentifier("erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx"),
				Amount:              extension.valueToCustomAmount("50", "ROSETTA-3a2edf"),
				Status:              &opStatusSuccess,
			},
		},
		Metadata: extractTransactionMetadata(blocks[0].MiniBlocks[0].Transactions[0]),
	}

	expectedRefundTx := &types.Transaction{
		TransactionIdentifier: hashToTransactionIdentifier("1928a22522845ca82bdfebea4fd37b067d72a3219a4ccef9b523491ae8eb102b"),
		Operations: []*types.Operation{
			{
				Type:                opFeeRefundAsScResult,
				OperationIdentifier: indexToOperationIdentifier(0),
				Account:             addressToAccountIdentifier("erd1testnlersh4z0wsv8kjx39me4rmnvjkwu8dsaea7ukdvvc9z396qykv7z7"),
				Amount:              extension.valueToNativeAmount("1840000000000"),
				Status:              &opStatusSuccess,
			},
		},
	}

	require.Equal(t, expectedTransferTx, txs[0])
	require.Equal(t, expectedRefundTx, txs[1])
}

func TestTransactionsTransformer_TransformBlockTxsHavingMultiESDTTransfer(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	networkProvider.MockCustomCurrencies = []resources.Currency{
		{Symbol: "TEST-dbc5c0"},
		{Symbol: "TEST-d65229"},
	}

	extension := newNetworkProviderExtension(networkProvider)
	transformer := newTransactionsTransformer(networkProvider)

	blocks, err := readTestBlocks("testdata/blocks_with_multi_esdt_transfer.json")
	require.Nil(t, err)

	// Block 0 contains the transfer(s) and the refund
	txs, err := transformer.transformBlockTxs(blocks[0])
	require.Nil(t, err)
	require.Len(t, txs, 2)

	expectedTransferTx := &types.Transaction{
		TransactionIdentifier: hashToTransactionIdentifier("996487dc3fdcc648c3989a74b32fd8b33339f788a6cfd757e1f80be933b441a9"),
		Operations: []*types.Operation{
			{
				Type:                opFee,
				OperationIdentifier: indexToOperationIdentifier(0),
				Account:             addressToAccountIdentifier("erd1testnlersh4z0wsv8kjx39me4rmnvjkwu8dsaea7ukdvvc9z396qykv7z7"),
				Amount:              extension.valueToNativeAmount("-283000000000000"),
				Status:              &opStatusSuccess,
			},
			{
				Type:                opCustomTransfer,
				OperationIdentifier: indexToOperationIdentifier(1),
				Account:             addressToAccountIdentifier("erd1testnlersh4z0wsv8kjx39me4rmnvjkwu8dsaea7ukdvvc9z396qykv7z7"),
				Amount:              extension.valueToCustomAmount("-100", "TEST-dbc5c0"),
				Status:              &opStatusSuccess,
			},
			{
				Type:                opCustomTransfer,
				OperationIdentifier: indexToOperationIdentifier(2),
				Account:             addressToAccountIdentifier("erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx"),
				Amount:              extension.valueToCustomAmount("100", "TEST-dbc5c0"),
				Status:              &opStatusSuccess,
			},
			{
				Type:                opCustomTransfer,
				OperationIdentifier: indexToOperationIdentifier(3),
				Account:             addressToAccountIdentifier("erd1testnlersh4z0wsv8kjx39me4rmnvjkwu8dsaea7ukdvvc9z396qykv7z7"),
				Amount:              extension.valueToCustomAmount("-50", "TEST-d65229"),
				Status:              &opStatusSuccess,
			},
			{
				Type:                opCustomTransfer,
				OperationIdentifier: indexToOperationIdentifier(4),
				Account:             addressToAccountIdentifier("erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx"),
				Amount:              extension.valueToCustomAmount("50", "TEST-d65229"),
				Status:              &opStatusSuccess,
			},
		},
		Metadata: extractTransactionMetadata(blocks[0].MiniBlocks[0].Transactions[0]),
	}

	expectedRefundTx := &types.Transaction{
		TransactionIdentifier: hashToTransactionIdentifier("a18187de36c120cfc3e01203145490fab722f8fe52bcdc6688e435e2ccd1f934"),
		Operations: []*types.Operation{
			{
				Type:                opFeeRefundAsScResult,
				OperationIdentifier: indexToOperationIdentifier(0),
				Account:             addressToAccountIdentifier("erd1testnlersh4z0wsv8kjx39me4rmnvjkwu8dsaea7ukdvvc9z396qykv7z7"),
				Amount:              extension.valueToNativeAmount("16000000000000"),
				Status:              &opStatusSuccess,
			},
		},
	}

	require.Equal(t, expectedTransferTx, txs[0])
	require.Equal(t, expectedRefundTx, txs[1])
}

func TestTransactionsTransformer_TransformBlockTxsHavingESDTMint(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	networkProvider.MockObservedActualShard = 1
	networkProvider.MockCustomCurrencies = []resources.Currency{{Symbol: "TEST-484fa1"}}

	extension := newNetworkProviderExtension(networkProvider)
	transformer := newTransactionsTransformer(networkProvider)

	blocks, err := readTestBlocks("testdata/blocks_with_esdt_mint.json")
	require.Nil(t, err)

	// Block 0 contains the mint operation and the fee refund
	txs, err := transformer.transformBlockTxs(blocks[0])
	require.Nil(t, err)
	require.Len(t, txs, 2)

	expectedMintTx := &types.Transaction{
		TransactionIdentifier: hashToTransactionIdentifier("ff2c048f3df91f9dd72a7c7472d1b72e9497814b71ae62f68cdf759b67da5796"),
		Operations: []*types.Operation{
			{
				Type:                opFee,
				OperationIdentifier: indexToOperationIdentifier(0),
				Account:             addressToAccountIdentifier("erd1r69gk66fmedhhcg24g2c5kn2f2a5k4kvpr6jfw67dn2lyydd8cfswy6ede"),
				Amount:              extension.valueToNativeAmount("-111500000000000"),
				Status:              &opStatusSuccess,
			},
			{
				Type:                opCustomTransfer,
				OperationIdentifier: indexToOperationIdentifier(1),
				Account:             addressToAccountIdentifier("erd1r69gk66fmedhhcg24g2c5kn2f2a5k4kvpr6jfw67dn2lyydd8cfswy6ede"),
				Amount:              extension.valueToCustomAmount("200", "TEST-484fa1"),
				Status:              &opStatusSuccess,
			},
		},
		Metadata: extractTransactionMetadata(blocks[0].MiniBlocks[0].Transactions[0]),
	}

	expectedRefundTx := &types.Transaction{
		TransactionIdentifier: hashToTransactionIdentifier("d8bf57701afcbf3a474b9bfd274f504ba786061d6157ca886cc9d29551b492d9"),
		Operations: []*types.Operation{
			{
				Type:                opFeeRefundAsScResult,
				OperationIdentifier: indexToOperationIdentifier(0),
				Account:             addressToAccountIdentifier("erd1r69gk66fmedhhcg24g2c5kn2f2a5k4kvpr6jfw67dn2lyydd8cfswy6ede"),
				Amount:              extension.valueToNativeAmount("2500000000000"),
				Status:              &opStatusSuccess,
			},
		},
	}

	require.Equal(t, expectedMintTx, txs[0])
	require.Equal(t, expectedRefundTx, txs[1])
}

func TestTransactionsTransformer_TransformBlockTxsHavingESDTBurn(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	networkProvider.MockObservedActualShard = 1
	networkProvider.MockCustomCurrencies = []resources.Currency{{Symbol: "TEST-484fa1"}}

	extension := newNetworkProviderExtension(networkProvider)
	transformer := newTransactionsTransformer(networkProvider)

	blocks, err := readTestBlocks("testdata/blocks_with_esdt_burn.json")
	require.Nil(t, err)

	// Block 0 contains the burn operation and the fee refund
	txs, err := transformer.transformBlockTxs(blocks[0])
	require.Nil(t, err)
	require.Len(t, txs, 2)

	expectedBurnTx := &types.Transaction{
		TransactionIdentifier: hashToTransactionIdentifier("29d734c1210d16ab8a796005156581a7b522701173bd8900ba5c5c9078cea4dd"),
		Operations: []*types.Operation{
			{
				Type:                opFee,
				OperationIdentifier: indexToOperationIdentifier(0),
				Account:             addressToAccountIdentifier("erd1r69gk66fmedhhcg24g2c5kn2f2a5k4kvpr6jfw67dn2lyydd8cfswy6ede"),
				Amount:              extension.valueToNativeAmount("-111500000000000"),
				Status:              &opStatusSuccess,
			},
			{
				Type:                opCustomTransfer,
				OperationIdentifier: indexToOperationIdentifier(1),
				Account:             addressToAccountIdentifier("erd1r69gk66fmedhhcg24g2c5kn2f2a5k4kvpr6jfw67dn2lyydd8cfswy6ede"),
				Amount:              extension.valueToCustomAmount("-50", "TEST-484fa1"),
				Status:              &opStatusSuccess,
			},
		},
		Metadata: extractTransactionMetadata(blocks[0].MiniBlocks[0].Transactions[0]),
	}

	expectedRefundTx := &types.Transaction{
		TransactionIdentifier: hashToTransactionIdentifier("64ad128f0982e363c94f5dcbc8328dfde07f44da3e3c40e7782c8dccccac3be9"),
		Operations: []*types.Operation{
			{
				Type:                opFeeRefundAsScResult,
				OperationIdentifier: indexToOperationIdentifier(0),
				Account:             addressToAccountIdentifier("erd1r69gk66fmedhhcg24g2c5kn2f2a5k4kvpr6jfw67dn2lyydd8cfswy6ede"),
				Amount:              extension.valueToNativeAmount("2500000000000"),
				Status:              &opStatusSuccess,
			},
		},
	}

	require.Equal(t, expectedBurnTx, txs[0])
	require.Equal(t, expectedRefundTx, txs[1])
}

func TestTransactionsTransformer_TransformBlockTxsHavingESDTWipe(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	networkProvider.MockObservedActualShard = 1
	networkProvider.MockCustomCurrencies = []resources.Currency{{Symbol: "TEST-484fa1"}}

	extension := newNetworkProviderExtension(networkProvider)
	transformer := newTransactionsTransformer(networkProvider)

	blocks, err := readTestBlocks("testdata/blocks_with_esdt_wipe.json")
	require.Nil(t, err)

	// Block 0 (wipe ESDT)
	txs, err := transformer.transformBlockTxs(blocks[0])
	require.Nil(t, err)
	require.Len(t, txs, 1)

	expectedWipeTx := &types.Transaction{
		TransactionIdentifier: hashToTransactionIdentifier("8fb3dc1ddc2b09de3a1e1aa477832f50f4c504c9d4ca010d6af02ddb04eef387"),
		Operations: []*types.Operation{
			{
				Type:                opFee,
				OperationIdentifier: indexToOperationIdentifier(0),
				Account:             addressToAccountIdentifier("erd1kdl46yctawygtwg2k462307dmz2v55c605737dp3zkxh04sct7asqylhyv"),
				Amount:              extension.valueToNativeAmount("-791000000000000"),
				Status:              &opStatusSuccess,
			},
		},
		Metadata: extractTransactionMetadata(blocks[0].MiniBlocks[0].Transactions[0]),
	}

	require.Equal(t, expectedWipeTx, txs[0])

	// Block 1 (results of wipe ESDT, refund)
	txs, err = transformer.transformBlockTxs(blocks[1])
	require.Nil(t, err)
	require.Len(t, txs, 2)

	expectedWipeSCR := &types.Transaction{
		TransactionIdentifier: hashToTransactionIdentifier("8f5217a38e261e220733366ec4af471c8623042cf5a74cd0629b7a93f0ffe39c"),
		Operations: []*types.Operation{
			{
				Type:                opCustomTransfer,
				OperationIdentifier: indexToOperationIdentifier(0),
				Account:             addressToAccountIdentifier("erd1r69gk66fmedhhcg24g2c5kn2f2a5k4kvpr6jfw67dn2lyydd8cfswy6ede"),
				Amount:              extension.valueToCustomAmount("-10", "TEST-484fa1"),
				Status:              &opStatusSuccess,
			},
		},
		Metadata: extractTransactionMetadata(blocks[1].MiniBlocks[0].Transactions[0]),
	}

	expectedRefundSCR := &types.Transaction{
		TransactionIdentifier: hashToTransactionIdentifier("c91694107f68a5f1b338036ab495f9f206c5a29d45ea68719f3c255a1788f374"),
		Operations: []*types.Operation{
			{
				Type:                opFeeRefundAsScResult,
				OperationIdentifier: indexToOperationIdentifier(0),
				Account:             addressToAccountIdentifier("erd1kdl46yctawygtwg2k462307dmz2v55c605737dp3zkxh04sct7asqylhyv"),
				Amount:              extension.valueToNativeAmount("100000000000000"),
				Status:              &opStatusSuccess,
			},
		},
	}

	require.Equal(t, expectedWipeSCR, txs[0])
	require.Equal(t, expectedRefundSCR, txs[1])
}

func TestTransactionsTransformer_TransformBlockTxsHavingNFTCreate(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	networkProvider.MockObservedActualShard = 1
	networkProvider.MockCustomCurrencies = []resources.Currency{{Symbol: "FRANK-73523d"}}

	extension := newNetworkProviderExtension(networkProvider)
	transformer := newTransactionsTransformer(networkProvider)

	blocks, err := readTestBlocks("testdata/blocks_with_nft_create.json")
	require.Nil(t, err)

	// Block 0 (NFT create and refund)
	txs, err := transformer.transformBlockTxs(blocks[0])
	require.Nil(t, err)
	require.Len(t, txs, 2)

	expectedNftCreateTx := &types.Transaction{
		TransactionIdentifier: hashToTransactionIdentifier("8f404c670dc857e0158b86ba02c50426fff0eefa708b9e161455ece0e604030c"),
		Operations: []*types.Operation{
			{
				Type:                opFee,
				OperationIdentifier: indexToOperationIdentifier(0),
				Account:             addressToAccountIdentifier("erd1r69gk66fmedhhcg24g2c5kn2f2a5k4kvpr6jfw67dn2lyydd8cfswy6ede"),
				Amount:              extension.valueToNativeAmount("-212500000000000"),
				Status:              &opStatusSuccess,
			},
			{
				Type:                opCustomTransfer,
				OperationIdentifier: indexToOperationIdentifier(1),
				Account:             addressToAccountIdentifier("erd1r69gk66fmedhhcg24g2c5kn2f2a5k4kvpr6jfw67dn2lyydd8cfswy6ede"),
				Amount:              extension.valueToCustomAmount("30", "FRANK-73523d-03"),
				Status:              &opStatusSuccess,
			},
		},
		Metadata: extractTransactionMetadata(blocks[0].MiniBlocks[0].Transactions[0]),
	}

	require.Equal(t, expectedNftCreateTx, txs[0])

	expectedRefundSCR := &types.Transaction{
		TransactionIdentifier: hashToTransactionIdentifier("d2eca235c07837b05838ebeb18cc71d6eb99ce5292e8db889f3b6b41e81ef8ae"),
		Operations: []*types.Operation{
			{
				Type:                opFeeRefundAsScResult,
				OperationIdentifier: indexToOperationIdentifier(0),
				Account:             addressToAccountIdentifier("erd1r69gk66fmedhhcg24g2c5kn2f2a5k4kvpr6jfw67dn2lyydd8cfswy6ede"),
				Amount:              extension.valueToNativeAmount("33400000000000"),
				Status:              &opStatusSuccess,
			},
		},
	}

	require.Equal(t, expectedRefundSCR, txs[1])
}

func TestTransactionsTransformer_TransformBlockTxsHavingNFTAddQuantity(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	networkProvider.MockObservedActualShard = 1
	networkProvider.MockCustomCurrencies = []resources.Currency{{Symbol: "FRANK-ad3529"}}

	extension := newNetworkProviderExtension(networkProvider)
	transformer := newTransactionsTransformer(networkProvider)

	blocks, err := readTestBlocks("testdata/blocks_with_nft_add_quantity.json")
	require.Nil(t, err)

	// Block 0 (NFT add quantity, all gas taken)
	txs, err := transformer.transformBlockTxs(blocks[0])
	require.Nil(t, err)
	require.Len(t, txs, 1)

	expectedNftAddQuantityTx := &types.Transaction{
		TransactionIdentifier: hashToTransactionIdentifier("01fd564ee984e601fba3e683ea8d7fc4b58089a28931059da7f7f2210f798263"),
		Operations: []*types.Operation{
			{
				Type:                opFee,
				OperationIdentifier: indexToOperationIdentifier(0),
				Account:             addressToAccountIdentifier("erd1r69gk66fmedhhcg24g2c5kn2f2a5k4kvpr6jfw67dn2lyydd8cfswy6ede"),
				Amount:              extension.valueToNativeAmount("-133500000000000"),
				Status:              &opStatusSuccess,
			},
			{
				Type:                opCustomTransfer,
				OperationIdentifier: indexToOperationIdentifier(1),
				Account:             addressToAccountIdentifier("erd1r69gk66fmedhhcg24g2c5kn2f2a5k4kvpr6jfw67dn2lyydd8cfswy6ede"),
				Amount:              extension.valueToCustomAmount("3", "FRANK-ad3529-01"),
				Status:              &opStatusSuccess,
			},
		},
		Metadata: extractTransactionMetadata(blocks[0].MiniBlocks[0].Transactions[0]),
	}

	require.Equal(t, expectedNftAddQuantityTx, txs[0])
}

func TestTransactionsTransformer_TransformBlockTxsHavingNFTBurn(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	networkProvider.MockObservedActualShard = 1
	networkProvider.MockCustomCurrencies = []resources.Currency{{Symbol: "FRANK-ad3529"}}

	extension := newNetworkProviderExtension(networkProvider)
	transformer := newTransactionsTransformer(networkProvider)

	blocks, err := readTestBlocks("testdata/blocks_with_nft_burn.json")
	require.Nil(t, err)

	// Block 0 (NFT burn, all gas taken)
	txs, err := transformer.transformBlockTxs(blocks[0])
	require.Nil(t, err)
	require.Len(t, txs, 1)

	expectedNftBurnTx := &types.Transaction{
		TransactionIdentifier: hashToTransactionIdentifier("06f38e2608aed1b811111e535b9f9d8ebc875d6e8330e9dddacb90cd7ab61853"),
		Operations: []*types.Operation{
			{
				Type:                opFee,
				OperationIdentifier: indexToOperationIdentifier(0),
				Account:             addressToAccountIdentifier("erd1r69gk66fmedhhcg24g2c5kn2f2a5k4kvpr6jfw67dn2lyydd8cfswy6ede"),
				Amount:              extension.valueToNativeAmount("-123000000000000"),
				Status:              &opStatusSuccess,
			},
			{
				Type:                opCustomTransfer,
				OperationIdentifier: indexToOperationIdentifier(1),
				Account:             addressToAccountIdentifier("erd1r69gk66fmedhhcg24g2c5kn2f2a5k4kvpr6jfw67dn2lyydd8cfswy6ede"),
				Amount:              extension.valueToCustomAmount("-2", "FRANK-ad3529-02"),
				Status:              &opStatusSuccess,
			},
		},
		Metadata: extractTransactionMetadata(blocks[0].MiniBlocks[0].Transactions[0]),
	}

	require.Equal(t, expectedNftBurnTx, txs[0])
}

func TestTransactionsTransformer_TransformBlockTxsHavingClaimDeveloperRewards(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	networkProvider.MockObservedActualShard = 0
	networkProvider.MockActivationEpochSpica = 42

	extension := newNetworkProviderExtension(networkProvider)
	transformer := newTransactionsTransformer(networkProvider)

	blocks, err := readTestBlocks("testdata/blocks_with_claim_developer_rewards.json")
	require.Nil(t, err)

	t.Run("recover operations using ClaimDeveloperRewards events", func(t *testing.T) {
		txs, err := transformer.transformBlockTxs(blocks[0])
		require.Nil(t, err)
		require.Len(t, txs, 2)
		require.Len(t, txs[0].Operations, 2)
		require.Len(t, txs[1].Operations, 1)

		expectedTx0 := &types.Transaction{
			TransactionIdentifier: hashToTransactionIdentifier("e0cb3e108c4e04a6d2406d718c20f976b365de3d1dd0067fd2fae996f7c9cbd1"),
			Operations: []*types.Operation{
				{
					Type:                opFee,
					OperationIdentifier: indexToOperationIdentifier(0),
					Account:             addressToAccountIdentifier("erd1tn62hjp72rznp8vq0lplva5csav6rccpqqdungpxtqz0g2hcq6uq9k4cc6"),
					Amount:              extension.valueToNativeAmount("-160685000000000"),
					Status:              &opStatusSuccess,
				},
				{
					Type:                opDeveloperRewards,
					OperationIdentifier: indexToOperationIdentifier(1),
					Account:             addressToAccountIdentifier("erd1tn62hjp72rznp8vq0lplva5csav6rccpqqdungpxtqz0g2hcq6uq9k4cc6"),
					Amount:              extension.valueToNativeAmount("1774725000000"),
					Status:              &opStatusSuccess,
				},
			},
			Metadata: extractTransactionMetadata(blocks[0].MiniBlocks[0].Transactions[0]),
		}

		require.Equal(t, expectedTx0, txs[0])

		// Fee refund
		expectedTx1 := &types.Transaction{
			TransactionIdentifier: hashToTransactionIdentifier("06f81486996225b597bc7ed89b4062cd895491b33b1e81f34205ef48272bd644"),
			Operations: []*types.Operation{
				{
					Type:                opFeeRefundAsScResult,
					OperationIdentifier: indexToOperationIdentifier(0),
					Account:             addressToAccountIdentifier("erd1tn62hjp72rznp8vq0lplva5csav6rccpqqdungpxtqz0g2hcq6uq9k4cc6"),
					Amount:              extension.valueToNativeAmount("29185000000000"),
					Status:              &opStatusSuccess,
				},
			},
		}

		require.Equal(t, expectedTx1, txs[1])
	})
}

func readTestBlocks(filePath string) ([]*api.Block, error) {
	var blocks []*api.Block

	err := readJson(filePath, &blocks)
	if err != nil {
		return nil, err
	}

	return blocks, nil
}

func readJson(filePath string, value interface{}) error {
	file, err := core.OpenFile(filePath)
	if err != nil {
		return err
	}

	content, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	err = json.Unmarshal(content, value)
	if err != nil {
		return err
	}

	return nil
}
