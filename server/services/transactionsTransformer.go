package services

import (
	"fmt"

	"github.com/ElrondNetwork/elrond-go-core/data/transaction"
	"github.com/ElrondNetwork/elrond-proxy-go/data"
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

func (transformer *transactionsTransformer) transformTxsOfBlock(block *data.Block) ([]*types.Transaction, error) {
	txs := make([]*data.FullTransaction, 0)
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

		populateStatusOfOperations(filteredOperations)
		rosettaTx.Operations = filteredOperations
	}

	rosettaTxs = filterOutRosettaTransactionsWithNoOperations(rosettaTxs)

	return rosettaTxs, nil
}

func (transformer *transactionsTransformer) txToRosettaTx(tx *data.FullTransaction, txsInBlock []*data.FullTransaction) (*types.Transaction, error) {
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
	scr *data.FullTransaction,
	txsInBlock []*data.FullTransaction,
) *types.Transaction {
	if scr.IsRefund {
		return &types.Transaction{
			TransactionIdentifier: hashToTransactionIdentifier(scr.Hash),
			Operations: []*types.Operation{
				{
					Type:    opScResult,
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
					Type:    opScResult,
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
	}
}

func (transformer *transactionsTransformer) rewardTxToRosettaTx(tx *data.FullTransaction) *types.Transaction {
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

func (transformer *transactionsTransformer) moveBalanceTxToRosetta(tx *data.FullTransaction) *types.Transaction {
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

	operations = append(operations, &types.Operation{
		Type:    opFee,
		Account: addressToAccountIdentifier(tx.Sender),
		Amount:  transformer.extension.valueToNativeAmount("-" + tx.InitiallyPaidFee),
	})

	return &types.Transaction{
		TransactionIdentifier: hashToTransactionIdentifier(tx.Hash),
		Operations:            operations,
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

func (transformer *transactionsTransformer) invalidTxToRosettaTx(tx *data.FullTransaction) *types.Transaction {
	fee := tx.InitiallyPaidFee

	if transformer.featuresDetector.isInvalidTransactionOfSendingValueToNonPayableContractOfTypeMoveBalance(tx) {
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
				Type:    opFeeOfInvalidTx,
				Account: addressToAccountIdentifier(tx.Sender),
				Amount:  transformer.extension.valueToNativeAmount("-" + fee),
			},
		},
	}
}

func (transformer *transactionsTransformer) mempoolMoveBalanceTxToRosettaTx(tx *data.FullTransaction) *types.Transaction {
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
	}
}

func (transformer *transactionsTransformer) addOperationsGivenTransactionEvents(tx *data.FullTransaction, rosettaTx *types.Transaction) error {
	eventsESDTTransfer, err := transformer.eventsController.extractEventsESDTTransfers(tx)
	if err != nil {
		return err
	}

	for _, event := range eventsESDTTransfer {
		// TODO: Check if identifier is enabled
		operations := []*types.Operation{
			{
				Type:    opESDTTransfer,
				Account: addressToAccountIdentifier(event.sender),
				Amount:  transformer.extension.valueToCustomAmount("-"+event.value, event.tokenIdentifier),
			},
			{
				Type:    opESDTTransfer,
				Account: addressToAccountIdentifier(event.receiver),
				Amount:  transformer.extension.valueToCustomAmount(event.value, event.tokenIdentifier),
			},
		}

		rosettaTx.Operations = append(rosettaTx.Operations, operations...)
	}

	return nil
}
