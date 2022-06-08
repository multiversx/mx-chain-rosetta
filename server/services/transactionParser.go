package services

import (
	"fmt"

	"github.com/ElrondNetwork/elrond-go/core"
	"github.com/ElrondNetwork/elrond-go/data/transaction"
	"github.com/ElrondNetwork/elrond-proxy-go/data"
	"github.com/coinbase/rosetta-sdk-go/types"
)

type transactionsParser struct {
	provider  NetworkProvider
	extension networkProviderExtension
}

func newTransactionParser(provider NetworkProvider) *transactionsParser {
	return &transactionsParser{
		provider:  provider,
		extension: *newNetworkProviderExtension(provider),
	}
}

func (parser *transactionsParser) parseTxsFromBlock(block *data.Block) ([]*types.Transaction, error) {
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
		rosettaTx, err := parser.parseTx(tx, false)
		if err != nil {
			return nil, err
		}

		// TODO: Should we populate related transactions?
		// populateRelatedTransactions(tx, eTx)
		rosettaTxs = append(rosettaTxs, rosettaTx)
	}

	return rosettaTxs, nil
}

func (tp *transactionsParser) parseTx(eTx *data.FullTransaction, isInPool bool) (*types.Transaction, error) {
	switch eTx.Type {
	case string(transaction.TxTypeNormal):
		return tp.createRosettaTxFromMoveBalance(eTx, isInPool), nil
	case string(transaction.TxTypeReward):
		return tp.createRosettaTxFromReward(eTx), nil
	case string(transaction.TxTypeUnsigned):
		return tp.createRosettaTxFromUnsignedTx(eTx), nil
	case string(transaction.TxTypeInvalid):
		return tp.createRosettaTxFromInvalidTx(eTx), nil
	default:
		return nil, fmt.Errorf("unknown transaction type: %s", eTx.Type)
	}
}

func (parser *transactionsParser) createRosettaTxFromUnsignedTx(eTx *data.FullTransaction) *types.Transaction {
	if eTx.IsRefund {
		return parser.createRosettaTxWithGasRefund(eTx)
	} else {
		return parser.createRosettaTxUnsignedTxSendFunds(eTx)
	}
}

func (parser *transactionsParser) createRosettaTxWithGasRefund(eTx *data.FullTransaction) *types.Transaction {
	return &types.Transaction{
		TransactionIdentifier: &types.TransactionIdentifier{
			Hash: eTx.Hash,
		},
		Operations: []*types.Operation{
			{
				OperationIdentifier: &types.OperationIdentifier{
					Index: 0,
				},
				Type:   opScResult,
				Status: &OpStatusSuccess,
				Account: &types.AccountIdentifier{
					Address: eTx.Receiver,
				},
				Amount: &types.Amount{
					Value:    eTx.Value,
					Currency: parser.extension.getNativeCurrency(),
				},
			},
		},
	}
}

func (parser *transactionsParser) createRosettaTxUnsignedTxSendFunds(eTx *data.FullTransaction) *types.Transaction {
	isFromMetachain := eTx.SourceShard == core.MetachainShardId
	isToMetachain := eTx.DestinationShard == core.MetachainShardId

	operations := make([]*types.Operation, 0)
	operationIndex := int64(0)

	if !isFromMetachain {
		operations = append(operations, &types.Operation{
			OperationIdentifier: &types.OperationIdentifier{
				Index: operationIndex,
			},
			Type:   opScResult,
			Status: &OpStatusSuccess,
			Account: &types.AccountIdentifier{
				Address: eTx.Sender,
			},
			Amount: &types.Amount{
				Value:    "-" + eTx.Value,
				Currency: parser.extension.getNativeCurrency(),
			},
		})

		operationIndex++
	}

	if !isToMetachain {
		operations = append(operations, &types.Operation{
			OperationIdentifier: &types.OperationIdentifier{
				Index: operationIndex,
			},
			Type:   opScResult,
			Status: &OpStatusSuccess,
			Account: &types.AccountIdentifier{
				Address: eTx.Receiver,
			},
			Amount: &types.Amount{
				Value:    eTx.Value,
				Currency: parser.extension.getNativeCurrency(),
			},
		})

		operationIndex++
	}

	tx := &types.Transaction{
		TransactionIdentifier: &types.TransactionIdentifier{
			Hash: eTx.Hash,
		},
		Operations: operations,
	}

	return tx
}

func (parser *transactionsParser) createRosettaTxFromReward(eTx *data.FullTransaction) *types.Transaction {
	return &types.Transaction{
		TransactionIdentifier: &types.TransactionIdentifier{
			Hash: eTx.Hash,
		},
		Operations: []*types.Operation{
			{
				OperationIdentifier: &types.OperationIdentifier{
					Index: 0,
				},
				Type:   opReward,
				Status: &OpStatusSuccess,
				Account: &types.AccountIdentifier{
					Address: eTx.Receiver,
				},
				Amount: &types.Amount{
					Value:    eTx.Value,
					Currency: parser.extension.getNativeCurrency(),
				},
			},
		},
	}
}

func (parser *transactionsParser) createRosettaTxFromMoveBalance(eTx *data.FullTransaction, isInPool bool) *types.Transaction {
	hasValue := eTx.Value != "0"
	isFromMetachain := eTx.SourceShard == core.MetachainShardId
	isToMetachain := eTx.DestinationShard == core.MetachainShardId

	operations := make([]*types.Operation, 0)
	operationIndex := int64(0)

	if hasValue {
		if !isFromMetachain {
			operations = append(operations, &types.Operation{
				OperationIdentifier: &types.OperationIdentifier{
					Index: 0,
				},
				Type:   opTransfer,
				Status: &OpStatusSuccess,
				Account: &types.AccountIdentifier{
					Address: eTx.Sender,
				},
				Amount: &types.Amount{
					Value:    "-" + eTx.Value,
					Currency: parser.extension.getNativeCurrency(),
				},
			})

			operationIndex++
		}

		if !isToMetachain {
			operations = append(operations, &types.Operation{
				OperationIdentifier: &types.OperationIdentifier{
					Index: operationIndex,
				},
				RelatedOperations: []*types.OperationIdentifier{
					{Index: 0},
				},
				Type:   opTransfer,
				Status: &OpStatusSuccess,
				Account: &types.AccountIdentifier{
					Address: eTx.Receiver,
				},
				Amount: &types.Amount{
					Value:    eTx.Value,
					Currency: parser.extension.getNativeCurrency(),
				},
			})

			operationIndex++
		}
	}

	// check if transaction has fee and transaction is not in pool
	// TODO / QUESTION for review: can it <not have fee>? can gas limit be 0?
	// TODO: also, why not declare fee as well if it's in pool?
	if eTx.GasLimit != 0 && !isInPool {
		operations = append(operations, &types.Operation{
			OperationIdentifier: &types.OperationIdentifier{
				Index: operationIndex,
			},
			Type:   opFee,
			Status: &OpStatusSuccess,
			Account: &types.AccountIdentifier{
				Address: eTx.Sender,
			},
			Amount: &types.Amount{
				Value:    "-" + eTx.InitiallyPaidFee,
				Currency: parser.extension.getNativeCurrency(),
			},
		})
	}

	tx := &types.Transaction{
		TransactionIdentifier: &types.TransactionIdentifier{
			Hash: eTx.Hash,
		},
		Operations: operations,
	}

	return tx
}

func (parser *transactionsParser) createOperationsFromPreparedTx(tx *data.Transaction) []*types.Operation {
	operations := make([]*types.Operation, 0)

	operations = append(operations, &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{
			Index: 0,
		},
		Type: opTransfer,
		Account: &types.AccountIdentifier{
			Address: tx.Sender,
		},
		Amount: &types.Amount{
			Value:    "-" + tx.Value,
			Currency: parser.extension.getNativeCurrency(),
		},
	})

	operations = append(operations, &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{
			Index: 1,
		},
		RelatedOperations: []*types.OperationIdentifier{
			{Index: 0},
		},
		Type: opTransfer,
		Account: &types.AccountIdentifier{
			Address: tx.Receiver,
		},
		Amount: &types.Amount{
			Value:    tx.Value,
			Currency: parser.extension.getNativeCurrency(),
		},
	})

	return operations
}

func (parser *transactionsParser) createRosettaTxFromInvalidTx(eTx *data.FullTransaction) *types.Transaction {
	return &types.Transaction{
		TransactionIdentifier: &types.TransactionIdentifier{
			Hash: eTx.Hash,
		},
		Operations: []*types.Operation{
			{
				OperationIdentifier: &types.OperationIdentifier{
					Index: 0,
				},
				// TODO: how to handle this? Also specify types in NetworkOptionsResponse.
				Type:   opInvalid,
				Status: &OpStatusSuccess,
				Account: &types.AccountIdentifier{
					Address: eTx.Sender,
				},
				Amount: &types.Amount{
					Value:    "-" + eTx.InitiallyPaidFee,
					Currency: parser.extension.getNativeCurrency(),
				},
			},
		},
	}
}

func populateRelatedTransactions(rosettaTx *types.Transaction, nodeTx *data.FullTransaction) {
	if nodeTx.OriginalTransactionHash != "" {
		rosettaTx.RelatedTransactions = append(rosettaTx.RelatedTransactions, &types.RelatedTransaction{
			TransactionIdentifier: &types.TransactionIdentifier{
				Hash: nodeTx.OriginalTransactionHash,
			},
			Direction: types.Backward,
		})
	}

	if nodeTx.PreviousTransactionHash != "" && nodeTx.PreviousTransactionHash != nodeTx.OriginalTransactionHash {
		rosettaTx.RelatedTransactions = append(rosettaTx.RelatedTransactions, &types.RelatedTransaction{
			TransactionIdentifier: &types.TransactionIdentifier{
				Hash: nodeTx.PreviousTransactionHash,
			},
			Direction: types.Backward,
		})
	}
}
