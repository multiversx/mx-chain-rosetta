package services

import (
	"errors"
	"fmt"

	"github.com/ElrondNetwork/elrond-go-core/data/api"
	"github.com/ElrondNetwork/elrond-go-core/data/transaction"
	"github.com/coinbase/rosetta-sdk-go/types"
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

func (transformer *transactionsTransformer) transformTxsFromBlock(block *api.Block) ([]*types.Transaction, error) {
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
	txs = filterOutContractResultsWithNoValue(txs)

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
		filteredOperations, err := transformer.extension.filterObservedOperations(rosettaTx.Operations)
		if err != nil {
			return nil, err
		}

		applyDefaultStatusOnOperations(filteredOperations)
		rosettaTx.Operations = filteredOperations
	}

	rosettaTxs = filterOutRosettaTransactionsWithNoOperations(rosettaTxs)

	return rosettaTxs, nil
}

func (transformer *transactionsTransformer) txToRosettaTx(tx *transaction.ApiTransactionResult, txsInBlock []*transaction.ApiTransactionResult) (*types.Transaction, error) {
	var rosettaTx *types.Transaction

	switch tx.Type {
	case string(transaction.TxTypeNormal):
		rosettaTx = transformer.moveBalanceTxToRosetta(tx)
	case string(transaction.TxTypeReward):
		rosettaTx = transformer.rewardTxToRosettaTx(tx)
	case string(transaction.TxTypeUnsigned):
		rosettaTx = transformer.unsignedTxToRosettaTx(tx, txsInBlock)
	case string(transaction.TxTypeInvalid):
		rosettaTx = transformer.invalidTxToRosettaTx(tx)
	default:
		return nil, fmt.Errorf("unknown transaction type: %s", tx.Type)
	}

	err := transformer.addOperationsGivenTransactionEvents(tx, rosettaTx)
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

func (transformer *transactionsTransformer) moveBalanceTxToRosetta(tx *transaction.ApiTransactionResult) *types.Transaction {
	hasValue := tx.Value != "0"
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

	return &types.Transaction{
		TransactionIdentifier: hashToTransactionIdentifier(tx.Hash),
		Operations:            operations,
		Metadata:              extractTransactionMetadata(tx),
	}
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
	hasValue := tx.Value != "0"
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

func (transformer *transactionsTransformer) addOperationsGivenTransactionEvents(tx *transaction.ApiTransactionResult, rosettaTx *types.Transaction) error {
	// TBD: uncomment when applicable ("transferValueOnly" events duplicate the information of SCRs in most contexts)
	// err := transformer.addOperationsGivenEventTransferValueOnly(tx, rosettaTx)
	// if err != nil {
	// 	return err
	// }

	return nil
}

func (transformer *transactionsTransformer) addOperationsGivenEventTransferValueOnly(tx *transaction.ApiTransactionResult, rosettaTx *types.Transaction) error {
	event, err := transformer.eventsController.extractEventTransferValueOnly(tx)
	if err != nil {
		if errors.Is(err, errEventNotFound) {
			return nil
		}
		return err
	}

	log.Debug("addOperationsGivenEventTransferValueOnly(), event found", "tx", tx.Hash, "event", event.String())

	operations := transformer.eventTransferValueOnlyToOperations(event)
	rosettaTx.Operations = append(rosettaTx.Operations, operations...)
	return nil
}

func (transformer *transactionsTransformer) eventTransferValueOnlyToOperations(event *eventTransferValueOnly) []*types.Operation {
	return []*types.Operation{
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
}
