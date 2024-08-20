package services

import (
	"github.com/multiversx/mx-chain-core-go/data/transaction"
)

type transactionsFeaturesDetector struct {
	networkProvider  NetworkProvider
	eventsController *transactionEventsController
}

func newTransactionsFeaturesDetector(provider NetworkProvider) *transactionsFeaturesDetector {
	return &transactionsFeaturesDetector{
		networkProvider:  provider,
		eventsController: newTransactionEventsController(provider),
	}
}

// Example SCRs can be found here: https://api.multiversx.com/transactions?function=ClaimDeveloperRewards
func (detector *transactionsFeaturesDetector) doesContractResultHoldRewardsOfClaimDeveloperRewards(
	contractResult *transaction.ApiTransactionResult,
	allTransactionsInBlock []*transaction.ApiTransactionResult,
) bool {
	hasData := len(contractResult.Data) > 0
	hasNonZeroNonce := contractResult.Nonce > 0
	if hasData || hasNonZeroNonce {
		return false
	}

	for _, tx := range allTransactionsInBlock {
		matchesTypeOnSource := tx.ProcessingTypeOnSource == transactionProcessingTypeBuiltInFunctionCall
		if !matchesTypeOnSource {
			continue
		}

		matchesTypeOnDestination := tx.ProcessingTypeOnDestination == transactionProcessingTypeBuiltInFunctionCall
		if !matchesTypeOnDestination {
			continue
		}

		matchesOperation := tx.Operation == builtInFunctionClaimDeveloperRewards
		if !matchesOperation {
			continue
		}

		matchesOriginalTransactionHash := tx.Hash == contractResult.OriginalTransactionHash
		if !matchesOriginalTransactionHash {
			continue
		}

		return true
	}

	return false
}

// isInvalidTransactionOfTypeMoveBalanceThatOnlyConsumesDataMovementGas detects (intra-shard) invalid transactions
// that only consume the "data movement" component of the gas:
// - "sending value to non-payable contract"
// - "meta transaction is invalid"
func (detector *transactionsFeaturesDetector) isInvalidTransactionOfTypeMoveBalanceThatOnlyConsumesDataMovementGas(tx *transaction.ApiTransactionResult) bool {
	isInvalid := tx.Type == string(transaction.TxTypeInvalid)
	isMoveBalance := tx.ProcessingTypeOnSource == transactionProcessingTypeMoveBalance && tx.ProcessingTypeOnDestination == transactionProcessingTypeMoveBalance

	if !isInvalid || !isMoveBalance {
		return false
	}

	// TODO: Analyze whether we can simplify the conditions below, or possibly discard them completely / replace them with simpler ones.
	withSendingValueToNonPayableContract := detector.eventsController.hasSignalErrorOfSendingValueToNonPayableContract(tx)
	withMetaTransactionIsInvalid := detector.eventsController.hasSignalErrorOfMetaTransactionIsInvalid(tx)
	return withSendingValueToNonPayableContract || withMetaTransactionIsInvalid
}

func (detector *transactionsFeaturesDetector) isRelayedV1TransactionCompletelyIntrashardWithSignalError(tx *transaction.ApiTransactionResult, innerTx *innerTransactionOfRelayedV1) bool {
	innerTxSenderShard := detector.networkProvider.ComputeShardIdOfPubKey(innerTx.SenderPubKey)
	innerTxReceiverShard := detector.networkProvider.ComputeShardIdOfPubKey(innerTx.ReceiverPubKey)

	isCompletelyIntrashard := tx.SourceShard == tx.DestinationShard &&
		innerTxSenderShard == innerTxReceiverShard &&
		innerTxSenderShard == tx.SourceShard
	if !isCompletelyIntrashard {
		return false
	}

	isWithSignalError := detector.eventsController.hasAnySignalError(tx)
	return isWithSignalError
}

func (detector *transactionsFeaturesDetector) isContractDeploymentWithSignalErrorOrIntrashardContractCallWithSignalError(tx *transaction.ApiTransactionResult) bool {
	return detector.isContractDeploymentWithSignalError(tx) || (detector.isIntrashard(tx) && detector.isContractCallWithSignalError(tx))
}

func (detector *transactionsFeaturesDetector) isContractCallWithSignalError(tx *transaction.ApiTransactionResult) bool {
	return tx.ProcessingTypeOnSource == transactionProcessingTypeContractInvoking &&
		tx.ProcessingTypeOnDestination == transactionProcessingTypeContractInvoking &&
		detector.eventsController.hasAnySignalError(tx)
}

func (detector *transactionsFeaturesDetector) isContractDeploymentWithSignalError(tx *transaction.ApiTransactionResult) bool {
	return tx.ProcessingTypeOnSource == transactionProcessingTypeContractDeployment &&
		tx.ProcessingTypeOnDestination == transactionProcessingTypeContractDeployment &&
		detector.eventsController.hasAnySignalError(tx)
}

func (detector *transactionsFeaturesDetector) isIntrashard(tx *transaction.ApiTransactionResult) bool {
	return tx.SourceShard == tx.DestinationShard
}
