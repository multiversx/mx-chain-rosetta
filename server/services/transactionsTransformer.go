package services

import (
	"fmt"

	"github.com/ElrondNetwork/elrond-go-core/data/transaction"
	"github.com/ElrondNetwork/elrond-proxy-go/data"
	"github.com/coinbase/rosetta-sdk-go/types"
)

type transactionsTransformer struct {
	provider  NetworkProvider
	extension networkProviderExtension
}

func newTransactionsTransformer(provider NetworkProvider) *transactionsTransformer {
	return &transactionsTransformer{
		provider:  provider,
		extension: *newNetworkProviderExtension(provider),
	}
}

func (transformer *transactionsTransformer) transformTxsFromBlock(block *data.Block) ([]*types.Transaction, error) {
	txs := make([]*data.FullTransaction, 0)

	for _, miniblock := range block.MiniBlocks {
		for _, tx := range miniblock.Transactions {
			txs = append(txs, tx)
		}
	}

	txs = filterOutIntrashardContractResultsWhoseOriginalTransactionIsInInvalidMiniblock(txs)
	txs = filterOutIntrashardRelayedTransactionAlreadyHeldInInvalidMiniblock(txs)
	txs = filterOutContractResultsWithNoValue(txs)

	rosettaTxs := make([]*types.Transaction, 0)
	for _, tx := range txs {
		rosettaTx, err := transformer.txToRosettaTx(tx)
		if err != nil {
			return nil, err
		}

		rosettaTx.Operations, err = transformer.extension.filterObservedOperations(rosettaTx.Operations)
		if err != nil {
			return nil, err
		}

		populateStatusOfOperations(rosettaTx.Operations)
		rosettaTxs = append(rosettaTxs, rosettaTx)
	}

	return rosettaTxs, nil
}

func (transformer *transactionsTransformer) txToRosettaTx(tx *data.FullTransaction) (*types.Transaction, error) {
	switch tx.Type {
	case string(transaction.TxTypeNormal):
		return transformer.moveBalanceTxToRosetta(tx), nil
	case string(transaction.TxTypeReward):
		return transformer.rewardTxToRosettaTx(tx), nil
	case string(transaction.TxTypeUnsigned):
		return transformer.unsignedTxToRosettaTx(tx), nil
	case string(transaction.TxTypeInvalid):
		return transformer.invalidTxToRosettaTx(tx), nil
	default:
		return nil, fmt.Errorf("unknown transaction type: %s", tx.Type)
	}
}

func (transformer *transactionsTransformer) unsignedTxToRosettaTx(tx *data.FullTransaction) *types.Transaction {
	if tx.IsRefund {
		return &types.Transaction{
			TransactionIdentifier: hashToTransactionIdentifier(tx.Hash),
			Operations: []*types.Operation{
				{
					Type:    opScResult,
					Account: addressToAccountIdentifier(tx.Receiver),
					Amount:  transformer.extension.valueToNativeAmount(tx.Value),
				},
			},
		}
	} else {
		return &types.Transaction{
			TransactionIdentifier: hashToTransactionIdentifier(tx.Hash),
			Operations: []*types.Operation{
				{
					Type:    opScResult,
					Account: addressToAccountIdentifier(tx.Sender),
					Amount:  transformer.extension.valueToNativeAmount("-" + tx.Value),
				},
				{
					Type:    opScResult,
					Account: addressToAccountIdentifier(tx.Receiver),
					Amount:  transformer.extension.valueToNativeAmount(tx.Value),
				},
			},
		}
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
	}
}

func (transformer *transactionsTransformer) invalidTxToRosettaTx(tx *data.FullTransaction) *types.Transaction {
	return &types.Transaction{
		TransactionIdentifier: hashToTransactionIdentifier(tx.Hash),
		Operations: []*types.Operation{
			{
				Type:    opFeeOfInvalidTx,
				Account: addressToAccountIdentifier(tx.Sender),
				Amount:  transformer.extension.valueToNativeAmount("-" + tx.InitiallyPaidFee),
			},
		},
	}
}

func (transformer *transactionsTransformer) mempoolMoveBalanceTxToRosettaTx(tx *data.FullTransaction) *types.Transaction {
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
	}
}
