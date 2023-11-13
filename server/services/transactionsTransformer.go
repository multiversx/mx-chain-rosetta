package services

import (
	"fmt"

	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/multiversx/mx-chain-core-go/data/api"
	"github.com/multiversx/mx-chain-core-go/data/transaction"
)

// TODO: newTransactionsTransformer(provider, block)
// .transform() -> calls extractFeatures() / classifies (retains invalid txs, built in calls etc.)
//				-> calls doTransform() on each tx

type transactionsTransformer struct {
	provider         NetworkProvider
	extension        *networkProviderExtension
	featuresDetector *transactionsFeaturesDetector
	eventsController *transactionEventsController
}

func newTransactionsTransformer(provider NetworkProvider) *transactionsTransformer {
	return &transactionsTransformer{
		provider:         provider,
		extension:        newNetworkProviderExtension(provider),
		featuresDetector: newTransactionsFeaturesDetector(provider),
		eventsController: newTransactionEventsController(provider),
	}
}

func (transformer *transactionsTransformer) transformBlockTxs(block *api.Block) ([]*types.Transaction, error) {
	txs := make([]*transaction.ApiTransactionResult, 0)
	receipts := make([]*transaction.ApiReceipt, 0)

	for _, miniblock := range block.MiniBlocks {
		for _, tx := range miniblock.Transactions {
			txs = append(txs, tx)
		}
		for _, receipt := range miniblock.Receipts {
			receipts = append(receipts, receipt)
		}
	}

	txs = filterOutIntrashardContractResultsWhoseOriginalTransactionIsInInvalidMiniblock(txs)
	txs = filterOutIntrashardRelayedTransactionAlreadyHeldInInvalidMiniblock(txs)

	rosettaTxs := make([]*types.Transaction, 0)
	for _, tx := range txs {
		rosettaTx, err := transformer.txToRosettaTx(tx, txs)
		if err != nil {
			return nil, err
		}

		rosettaTxs = append(rosettaTxs, rosettaTx)
	}

	for _, receipt := range receipts {
		if receipt.Data == refundGasMessage {
			rosettaTx, err := transformer.refundReceiptToRosettaTx(receipt)
			if err != nil {
				return nil, err
			}

			rosettaTxs = append(rosettaTxs, rosettaTx)
		}
	}

	for _, rosettaTx := range rosettaTxs {
		filteredOperations, err := filterOperationsByAddress(rosettaTx.Operations, transformer.extension.isAddressObserved)
		if err != nil {
			return nil, err
		}

		filteredOperations = filterOutOperationsWithZeroAmount(filteredOperations)

		applyDefaultStatusOnOperations(filteredOperations)
		rosettaTx.Operations = filteredOperations
	}

	rosettaTxs = filterOutRosettaTransactionsWithNoOperations(rosettaTxs)

	return rosettaTxs, nil
}

func (transformer *transactionsTransformer) txToRosettaTx(tx *transaction.ApiTransactionResult, txsInBlock []*transaction.ApiTransactionResult) (*types.Transaction, error) {
	var rosettaTx *types.Transaction
	var err error

	switch tx.Type {
	case string(transaction.TxTypeNormal):
		rosettaTx, err = transformer.normalTxToRosetta(tx, txsInBlock)
		if err != nil {
			return nil, err
		}
	case string(transaction.TxTypeReward):
		rosettaTx = transformer.rewardTxToRosettaTx(tx)
	case string(transaction.TxTypeUnsigned):
		rosettaTx = transformer.unsignedTxToRosettaTx(tx, txsInBlock)
	case string(transaction.TxTypeInvalid):
		rosettaTx = transformer.invalidTxToRosettaTx(tx)
	default:
		return nil, fmt.Errorf("unknown transaction type: %s", tx.Type)
	}

	err = transformer.addOperationsGivenTransactionEvents(tx, txsInBlock, rosettaTx)
	if err != nil {
		return nil, err
	}

	return rosettaTx, nil
}

func (transformer *transactionsTransformer) unsignedTxToRosettaTx(
	scr *transaction.ApiTransactionResult,
	txsInBlock []*transaction.ApiTransactionResult,
) *types.Transaction {
	if scr.IsRefund {
		if scr.Sender == scr.Receiver && !transformer.extension.isUserAddress(scr.Sender) {
			log.Info("unsignedTxToRosettaTx: dismissed refund", "hash", scr.Hash, "originalTxHash", scr.OriginalTransactionHash)

			return &types.Transaction{
				TransactionIdentifier: hashToTransactionIdentifier(scr.Hash),
				Operations:            []*types.Operation{},
			}
		}

		return &types.Transaction{
			TransactionIdentifier: hashToTransactionIdentifier(scr.Hash),
			Operations: []*types.Operation{
				{
					Type:    opFeeRefundAsScResult,
					Account: addressToAccountIdentifier(scr.Receiver),
					Amount:  transformer.extension.valueToNativeAmount(scr.Value),
				},
			},
		}
	}

	if transformer.featuresDetector.doesContractResultHoldRewardsOfClaimDeveloperRewards(scr, txsInBlock) {
		return &types.Transaction{
			TransactionIdentifier: hashToTransactionIdentifier(scr.Hash),
			Operations: []*types.Operation{
				{
					Type:    opDeveloperRewardsAsScResult,
					Account: addressToAccountIdentifier(scr.Receiver),
					Amount:  transformer.extension.valueToNativeAmount(scr.Value),
				},
			},
		}
	}

	return &types.Transaction{
		TransactionIdentifier: hashToTransactionIdentifier(scr.Hash),
		Operations: []*types.Operation{
			{
				Type:    opScResult,
				Account: addressToAccountIdentifier(scr.Sender),
				Amount:  transformer.extension.valueToNativeAmount("-" + scr.Value),
			},
			{
				Type:    opScResult,
				Account: addressToAccountIdentifier(scr.Receiver),
				Amount:  transformer.extension.valueToNativeAmount(scr.Value),
			},
		},
		Metadata: extractTransactionMetadata(scr),
	}
}

func (transformer *transactionsTransformer) rewardTxToRosettaTx(tx *transaction.ApiTransactionResult) *types.Transaction {
	return &types.Transaction{
		TransactionIdentifier: hashToTransactionIdentifier(tx.Hash),
		Operations: []*types.Operation{
			{
				Type:    opReward,
				Account: addressToAccountIdentifier(tx.Receiver),
				Amount:  transformer.extension.valueToNativeAmount(tx.Value),
			},
		},
	}
}

func (transformer *transactionsTransformer) normalTxToRosetta(
	tx *transaction.ApiTransactionResult,
	allTransactionsInBlock []*transaction.ApiTransactionResult,
) (*types.Transaction, error) {
	hasValue := !isZeroAmount(tx.Value)
	operations := make([]*types.Operation, 0)

	if hasValue {
		operations = append(operations, &types.Operation{
			Type:    opTransfer,
			Account: addressToAccountIdentifier(tx.Sender),
			Amount:  transformer.extension.valueToNativeAmount("-" + tx.Value),
		})

		operations = append(operations, &types.Operation{
			Type:    opTransfer,
			Account: addressToAccountIdentifier(tx.Receiver),
			Amount:  transformer.extension.valueToNativeAmount(tx.Value),
		})
	}

	operations = append(operations, &types.Operation{
		Type:    opFee,
		Account: addressToAccountIdentifier(tx.Sender),
		Amount:  transformer.extension.valueToNativeAmount("-" + tx.InitiallyPaidFee),
	})

	innerTxOperationsIfRelayedCompletelyIntrashardWithSignalError, err := transformer.extractInnerTxOperationsIfRelayedCompletelyIntrashardWithSignalError(tx)
	if err != nil {
		return nil, err
	}

	valueRefundOperationIfContractCallWithSignalError, err := transformer.createValueReturnOperationsIfIntrashardContractCallWithSignalError(tx, allTransactionsInBlock)
	if err != nil {
		return nil, err
	}

	operations = append(operations, innerTxOperationsIfRelayedCompletelyIntrashardWithSignalError...)
	operations = append(operations, valueRefundOperationIfContractCallWithSignalError...)

	return &types.Transaction{
		TransactionIdentifier: hashToTransactionIdentifier(tx.Hash),
		Operations:            operations,
		Metadata:              extractTransactionMetadata(tx),
	}, nil
}

func (transformer *transactionsTransformer) extractInnerTxOperationsIfRelayedCompletelyIntrashardWithSignalError(tx *transaction.ApiTransactionResult) ([]*types.Operation, error) {
	isRelayedTransaction := isRelayedV1Transaction(tx)
	if !isRelayedTransaction {
		return []*types.Operation{}, nil
	}

	innerTx, err := parseInnerTxOfRelayedV1(tx)
	if err != nil {
		return []*types.Operation{}, err
	}

	if isZeroBigIntOrNil(&innerTx.Value) {
		return []*types.Operation{}, nil
	}

	if !transformer.featuresDetector.isRelayedTransactionCompletelyIntrashardWithSignalError(tx, innerTx) {
		return []*types.Operation{}, nil
	}

	senderAddress := transformer.provider.ConvertPubKeyToAddress(innerTx.SenderPubKey)
	receiverAddress := transformer.provider.ConvertPubKeyToAddress(innerTx.ReceiverPubKey)

	return []*types.Operation{
		{
			Type:    opTransfer,
			Account: addressToAccountIdentifier(senderAddress),
			Amount:  transformer.extension.valueToNativeAmount("-" + innerTx.Value.String()),
		},
		{
			Type:    opTransfer,
			Account: addressToAccountIdentifier(receiverAddress),
			Amount:  transformer.extension.valueToNativeAmount(innerTx.Value.String()),
		},
	}, nil
}

func (transformer *transactionsTransformer) createValueReturnOperationsIfIntrashardContractCallWithSignalError(
	tx *transaction.ApiTransactionResult,
	allTransactionsInBlock []*transaction.ApiTransactionResult,
) ([]*types.Operation, error) {
	if !transformer.featuresDetector.isIntrashardContractCallWithSignalErrorButWithoutContractResultBearingRefundValue(tx, allTransactionsInBlock) {
		return []*types.Operation{}, nil
	}

	return []*types.Operation{
		{
			Type:    opTransfer,
			Account: addressToAccountIdentifier(tx.Sender),
			Amount:  transformer.extension.valueToNativeAmount(tx.Value),
		},
		{
			Type:    opTransfer,
			Account: addressToAccountIdentifier(tx.Receiver),
			Amount:  transformer.extension.valueToNativeAmount("-" + tx.Value),
		},
	}, nil
}

func (transformer *transactionsTransformer) refundReceiptToRosettaTx(receipt *transaction.ApiReceipt) (*types.Transaction, error) {
	receiptHash, err := transformer.provider.ComputeReceiptHash(receipt)
	if err != nil {
		return nil, err
	}

	return &types.Transaction{
		TransactionIdentifier: hashToTransactionIdentifier(receiptHash),
		Operations: []*types.Operation{
			{
				Type:    opFeeRefund,
				Account: addressToAccountIdentifier(receipt.SndAddr),
				Amount:  transformer.extension.valueToNativeAmount(receipt.Value.String()),
			},
		},
	}, nil
}

func (transformer *transactionsTransformer) invalidTxToRosettaTx(tx *transaction.ApiTransactionResult) *types.Transaction {
	fee := tx.InitiallyPaidFee

	if transformer.featuresDetector.isInvalidTransactionOfTypeMoveBalanceThatOnlyConsumesDataMovementGas(tx) {
		// For this type of transactions, the fee only has the "data movement" component
		// (we ignore tx.InitiallyPaidFee, which is not correctly provided in this case).
		// Though, note that for built-in function calls (e.g. sending tokens using a transfer function) etc.,
		// the fee has the "execution" component, as well.
		fee = transformer.provider.ComputeTransactionFeeForMoveBalance(tx).String()
	}

	return &types.Transaction{
		TransactionIdentifier: hashToTransactionIdentifier(tx.Hash),
		Operations: []*types.Operation{
			{
				Status:  &opStatusFailure,
				Type:    opTransfer,
				Account: addressToAccountIdentifier(tx.Sender),
				Amount:  transformer.extension.valueToNativeAmount("-" + tx.Value),
			},
			{
				Status:  &opStatusFailure,
				Type:    opTransfer,
				Account: addressToAccountIdentifier(tx.Receiver),
				Amount:  transformer.extension.valueToNativeAmount(tx.Value),
			},
			{
				Type:    opFeeOfInvalidTx,
				Account: addressToAccountIdentifier(tx.Sender),
				Amount:  transformer.extension.valueToNativeAmount("-" + fee),
			},
		},
		Metadata: extractTransactionMetadata(tx),
	}
}

func (transformer *transactionsTransformer) mempoolMoveBalanceTxToRosettaTx(tx *transaction.ApiTransactionResult) *types.Transaction {
	hasValue := isNonZeroAmount(tx.Value)
	operations := make([]*types.Operation, 0)

	if hasValue {
		operations = append(operations, &types.Operation{
			Type:    opTransfer,
			Account: addressToAccountIdentifier(tx.Sender),
			Amount:  transformer.extension.valueToNativeAmount("-" + tx.Value),
		})

		operations = append(operations, &types.Operation{
			Type:    opTransfer,
			Account: addressToAccountIdentifier(tx.Receiver),
			Amount:  transformer.extension.valueToNativeAmount(tx.Value),
		})
	}

	indexOperations(operations)

	return &types.Transaction{
		TransactionIdentifier: hashToTransactionIdentifier(tx.Hash),
		Operations:            operations,
		Metadata:              extractTransactionMetadata(tx),
	}
}

func (transformer *transactionsTransformer) addOperationsGivenTransactionEvents(
	tx *transaction.ApiTransactionResult,
	txsInBlock []*transaction.ApiTransactionResult,
	rosettaTx *types.Transaction,
) error {
	eventsTransferValueOnly, err := transformer.eventsController.extractEventTransferValueOnly(tx)
	if err != nil {
		return err
	}

	eventsESDTTransfer, err := transformer.eventsController.extractEventsESDTOrESDTNFTTransfers(tx)
	if err != nil {
		return err
	}

	eventsESDTLocalBurn, err := transformer.eventsController.extractEventsESDTLocalBurn(tx)
	if err != nil {
		return err
	}

	eventsESDTLocalMint, err := transformer.eventsController.extractEventsESDTLocalMint(tx)
	if err != nil {
		return err
	}

	eventsESDTWipe, err := transformer.eventsController.extractEventsESDTWipe(tx)
	if err != nil {
		return err
	}

	eventsTransferValueOnly = filterOutTransferValueOnlyEventsThatAreAlreadyCapturedAsContractResults(eventsTransferValueOnly, txsInBlock)

	for _, event := range eventsTransferValueOnly {
		log.Info("eventTransferValueOnly (effective)", "tx", tx.Hash)

		operations := []*types.Operation{
			{
				Type:    opTransfer,
				Account: addressToAccountIdentifier(event.sender),
				Amount:  transformer.extension.valueToNativeAmount("-" + event.value),
			},
			{
				Type:    opTransfer,
				Account: addressToAccountIdentifier(event.receiver),
				Amount:  transformer.extension.valueToNativeAmount(event.value),
			},
		}

		rosettaTx.Operations = append(rosettaTx.Operations, operations...)
	}

	for _, event := range eventsESDTTransfer {
		if !transformer.provider.HasCustomCurrency(event.identifier) {
			// We are only emitting balance-changing operations for supported currencies.
			continue
		}

		operations := []*types.Operation{
			{
				Type:    opCustomTransfer,
				Account: addressToAccountIdentifier(event.senderAddress),
				Amount:  transformer.extension.valueToCustomAmount("-"+event.value, event.getComposedIdentifier()),
			},
			{
				Type:    opCustomTransfer,
				Account: addressToAccountIdentifier(event.receiverAddress),
				Amount:  transformer.extension.valueToCustomAmount(event.value, event.getComposedIdentifier()),
			},
		}

		rosettaTx.Operations = append(rosettaTx.Operations, operations...)
	}

	for _, event := range eventsESDTLocalBurn {
		if !transformer.provider.HasCustomCurrency(event.identifier) {
			// We are only emitting balance-changing operations for supported currencies.
			continue
		}

		operations := []*types.Operation{
			{
				Type:    opCustomTransfer,
				Account: addressToAccountIdentifier(event.otherAddress),
				Amount:  transformer.extension.valueToCustomAmount("-"+event.value, event.getComposedIdentifier()),
			},
		}

		rosettaTx.Operations = append(rosettaTx.Operations, operations...)
	}

	for _, event := range eventsESDTLocalMint {
		if !transformer.provider.HasCustomCurrency(event.identifier) {
			// We are only emitting balance-changing operations for supported currencies.
			continue
		}

		operations := []*types.Operation{
			{
				Type:    opCustomTransfer,
				Account: addressToAccountIdentifier(event.otherAddress),
				Amount:  transformer.extension.valueToCustomAmount(event.value, event.getComposedIdentifier()),
			},
		}

		rosettaTx.Operations = append(rosettaTx.Operations, operations...)
	}

	for _, event := range eventsESDTWipe {
		if !transformer.provider.HasCustomCurrency(event.identifier) {
			// We are only emitting balance-changing operations for supported currencies.
			continue
		}

		operations := []*types.Operation{
			{
				Type:    opCustomTransfer,
				Account: addressToAccountIdentifier(event.otherAddress),
				Amount:  transformer.extension.valueToCustomAmount("-"+event.value, event.getComposedIdentifier()),
			},
		}

		rosettaTx.Operations = append(rosettaTx.Operations, operations...)
	}

	return nil
}
