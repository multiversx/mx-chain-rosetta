package services

import (
	"fmt"

	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/multiversx/mx-chain-core-go/data/api"
	"github.com/multiversx/mx-chain-core-go/data/transaction"
	"github.com/multiversx/mx-chain-rosetta/server/provider"
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
			// Make sure SCRs also have the block nonce set.
			tx.BlockNonce = block.Nonce
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
		filteredOperations, err := filterOperationsByAddress(rosettaTx.Operations, transformer.provider.IsAddressObserved)
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
		rosettaTx, err = transformer.normalTxToRosetta(tx)
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

	err = transformer.addOperationsGivenTransactionEvents(tx, rosettaTx)
	if err != nil {
		return nil, err
	}

	return rosettaTx, nil
}

func (transformer *transactionsTransformer) unsignedTxToRosettaTx(
	scr *transaction.ApiTransactionResult,
	txsInBlock []*transaction.ApiTransactionResult,
) *types.Transaction {
	if transformer.featuresDetector.isSmartContractResultIneffectiveRefund(scr) {
		log.Debug("unsignedTxToRosettaTx: ineffective refund", "hash", scr.Hash, "block", scr.BlockNonce)

		return &types.Transaction{
			TransactionIdentifier: hashToTransactionIdentifier(scr.Hash),
			Operations:            []*types.Operation{},
		}
	}

	if scr.IsRefund {
		// Refund is properly given to the fee payer (sender or relayer).
		refundReceiver := scr.Receiver

		return &types.Transaction{
			TransactionIdentifier: hashToTransactionIdentifier(scr.Hash),
			Operations: []*types.Operation{
				{
					Type:    opFeeRefundAsScResult,
					Account: addressToAccountIdentifier(refundReceiver),
					Amount:  transformer.extension.valueToNativeAmount(scr.Value),
				},
			},
		}
	}

	// Handle developer rewards:
	//
	// (a) When the developer rewards are claimed in an intra-shard fashion, the network generates misleading SCRs.
	// In addition to the regular refund SCR, there's a SCR that notarizes the rewards as a misleading balance transfer, from the developer to self:
	//	- https://explorer.multiversx.com/transactions?function=ClaimDeveloperRewards&senderShard=0&receiverShard=0
	//	- and so on...
	//
	// (b) When the developer rewards are claimed in a cross-shard fashion, the network generates misleading SCRs.
	// In addition to the regular refund SCR, there's a SCR that notarizes the rewards as a misleading balance transfer, from the contract to the developer:
	// - https://explorer.multiversx.com/transactions?function=ClaimDeveloperRewards&senderShard=0&receiverShard=1
	// - and so on ...
	//
	// Either way, correct transaction events with identifier "ClaimDeveloperRewards" are generated.
	// Here, we simply ignore all SCRs which **seem to hold a developer reward**,
	// since they are properly handled by "addOperationsGivenTransactionEvents".
	if transformer.featuresDetector.doesContractResultHoldRewardsOfClaimDeveloperRewards(scr, txsInBlock) {
		return &types.Transaction{
			TransactionIdentifier: hashToTransactionIdentifier(scr.Hash),
			Operations:            []*types.Operation{},
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

func (transformer *transactionsTransformer) normalTxToRosetta(tx *transaction.ApiTransactionResult) (*types.Transaction, error) {
	operations := make([]*types.Operation, 0)

	transfersValue := isNonZeroAmount(tx.Value)

	if transformer.provider.IsReleaseSiriusActive(tx.Epoch) {
		// Special handling of:
		// - intra-shard contract calls, bearing value, which fail with signal error
		// - direct contract deployments, bearing value, which fail with signal error
		// For these, the protocol does not generate an explicit SCR with the value refund (before Sirius, in some cases, it did).
		// However, since the value remains at the sender, we don't emit any operations in these circumstances.
		transfersValue = transfersValue && !transformer.featuresDetector.isContractDeploymentWithSignalErrorOrIntrashardContractCallWithSignalError(tx)
	}

	if transfersValue {
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

	feePayer := transformer.decideFeePayer(tx)
	operations = append(operations, &types.Operation{
		Type:    opFee,
		Account: addressToAccountIdentifier(feePayer),
		Amount:  transformer.extension.valueToNativeAmount("-" + tx.InitiallyPaidFee),
	})

	return &types.Transaction{
		TransactionIdentifier: hashToTransactionIdentifier(tx.Hash),
		Operations:            operations,
		Metadata:              extractTransactionMetadata(tx),
	}, nil
}

func (transformer *transactionsTransformer) decideFeePayer(tx *transaction.ApiTransactionResult) string {
	if provider.IsRelayedTxV3(tx) {
		return tx.RelayerAddress
	}

	return tx.Sender
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
				Type: opFeeRefund,
				// Refund is properly given to the fee payer (sender or relayer).
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

	feePayer := transformer.decideFeePayer(tx)

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
				Account: addressToAccountIdentifier(feePayer),
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

func (transformer *transactionsTransformer) addOperationsGivenTransactionEvents(tx *transaction.ApiTransactionResult, rosettaTx *types.Transaction) error {
	hasSignalError := transformer.featuresDetector.eventsController.hasAnySignalError(tx)
	if hasSignalError {
		return nil
	}

	eventsSCDeploy, err := transformer.eventsController.extractEventSCDeploy(tx)
	if err != nil {
		return err
	}

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

	eventsESDTNFTCreate, err := transformer.eventsController.extractEventsESDTNFTCreate(tx)
	if err != nil {
		return err
	}

	eventsESDTNFTBurn, err := transformer.eventsController.extractEventsESDTNFTBurn(tx)
	if err != nil {
		return err
	}

	eventsESDTNFTAddQuantity, err := transformer.eventsController.extractEventsESDTNFTAddQuantity(tx)
	if err != nil {
		return err
	}

	eventsClaimDeveloperRewards, err := transformer.eventsController.extractEventsClaimDeveloperRewards(tx)
	if err != nil {
		return err
	}

	for _, event := range eventsSCDeploy {
		// Handle direct deployments with transfer of value (indirect deployments are currently excluded to prevent any potential misinterpretations).
		if tx.Receiver == systemContractDeployAddress {
			operations := []*types.Operation{
				// Deployer's balance change is already captured in operations recovered not from logs / events, but from the transaction itself.
				// It remains to "simulate" the transfer from the system deployment address to the contract address.
				{
					Type:    opTransfer,
					Account: addressToAccountIdentifier(tx.Receiver),
					Amount:  transformer.extension.valueToNativeAmount("-" + tx.Value),
				},
				{
					Type:    opTransfer,
					Account: addressToAccountIdentifier(event.contractAddress),
					Amount:  transformer.extension.valueToNativeAmount(tx.Value),
				},
			}

			rosettaTx.Operations = append(rosettaTx.Operations, operations...)
		}
	}

	for _, event := range eventsTransferValueOnly {
		log.Debug("eventTransferValueOnly (effective)", "tx", tx.Hash, "block", tx.BlockNonce)

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
		operations := transformer.extractOperationsFromEventESDT(event)
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
				Amount:  transformer.extension.valueToCustomAmount("-"+event.value, event.getExtendedIdentifier()),
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
				Amount:  transformer.extension.valueToCustomAmount(event.value, event.getExtendedIdentifier()),
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
				Amount:  transformer.extension.valueToCustomAmount("-"+event.value, event.getExtendedIdentifier()),
			},
		}

		rosettaTx.Operations = append(rosettaTx.Operations, operations...)
	}

	for _, event := range eventsESDTNFTCreate {
		if !transformer.provider.HasCustomCurrency(event.identifier) {
			// We are only emitting balance-changing operations for supported currencies.
			continue
		}

		operations := []*types.Operation{
			{
				Type:    opCustomTransfer,
				Account: addressToAccountIdentifier(event.otherAddress),
				Amount:  transformer.extension.valueToCustomAmount(event.value, event.getExtendedIdentifier()),
			},
		}

		rosettaTx.Operations = append(rosettaTx.Operations, operations...)
	}

	for _, event := range eventsESDTNFTBurn {
		if !transformer.provider.HasCustomCurrency(event.identifier) {
			// We are only emitting balance-changing operations for supported currencies.
			continue
		}

		operations := []*types.Operation{
			{
				Type:    opCustomTransfer,
				Account: addressToAccountIdentifier(event.otherAddress),
				Amount:  transformer.extension.valueToCustomAmount("-"+event.value, event.getExtendedIdentifier()),
			},
		}

		rosettaTx.Operations = append(rosettaTx.Operations, operations...)
	}

	for _, event := range eventsESDTNFTAddQuantity {
		if !transformer.provider.HasCustomCurrency(event.identifier) {
			// We are only emitting balance-changing operations for supported currencies.
			continue
		}

		operations := []*types.Operation{
			{
				Type:    opCustomTransfer,
				Account: addressToAccountIdentifier(event.otherAddress),
				Amount:  transformer.extension.valueToCustomAmount(event.value, event.getExtendedIdentifier()),
			},
		}

		rosettaTx.Operations = append(rosettaTx.Operations, operations...)
	}

	for _, event := range eventsClaimDeveloperRewards {
		operations := []*types.Operation{
			{
				Type:    opDeveloperRewards,
				Account: addressToAccountIdentifier(event.receiverAddress),
				Amount:  transformer.extension.valueToNativeAmount(event.value),
			},
		}

		rosettaTx.Operations = append(rosettaTx.Operations, operations...)
	}

	return nil
}

func (transformer *transactionsTransformer) extractOperationsFromEventESDT(event *eventESDT) []*types.Operation {
	if event.identifier == nativeAsESDTIdentifier {
		return []*types.Operation{
			{
				Type:    opTransfer,
				Account: addressToAccountIdentifier(event.senderAddress),
				Amount:  transformer.extension.valueToNativeAmount("-" + event.value),
			},
			{
				Type:    opTransfer,
				Account: addressToAccountIdentifier(event.receiverAddress),
				Amount:  transformer.extension.valueToNativeAmount(event.value),
			},
		}
	}

	if transformer.provider.HasCustomCurrency(event.identifier) {
		// We are only emitting balance-changing operations for supported currencies.
		return []*types.Operation{
			{
				Type:    opCustomTransfer,
				Account: addressToAccountIdentifier(event.senderAddress),
				Amount:  transformer.extension.valueToCustomAmount("-"+event.value, event.getExtendedIdentifier()),
			},
			{
				Type:    opCustomTransfer,
				Account: addressToAccountIdentifier(event.receiverAddress),
				Amount:  transformer.extension.valueToCustomAmount(event.value, event.getExtendedIdentifier()),
			},
		}
	}

	return make([]*types.Operation, 0)
}
