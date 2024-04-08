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
	claimDeveloperRewardsTxs := make(map[string]struct{})

	for _, tx := range allTransactionsInBlock {
		matchesTypeOnSource := tx.ProcessingTypeOnSource == transactionProcessingTypeBuiltInFunctionCall
		matchesTypeOnDestination := tx.ProcessingTypeOnDestination == transactionProcessingTypeBuiltInFunctionCall
		matchesOperation := tx.Operation == builtInFunctionClaimDeveloperRewards

		if matchesTypeOnSource && matchesTypeOnDestination && matchesOperation {
			claimDeveloperRewardsTxs[tx.Hash] = struct{}{}
		}
	}

	_, isResultOfClaimDeveloperRewards := claimDeveloperRewardsTxs[contractResult.OriginalTransactionHash]
	hasNoData := len(contractResult.Data) == 0
	hasZeroNonce := contractResult.Nonce == 0

	return isResultOfClaimDeveloperRewards && hasNoData && hasZeroNonce
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

func (detector *transactionsFeaturesDetector) isIntrashardContractCallWithSignalErrorButWithoutContractResultBearingRefundValue(
	txInQuestion *transaction.ApiTransactionResult,
	allTransactionsInBlock []*transaction.ApiTransactionResult,
) bool {
	if !detector.isContractCallWithSignalError(txInQuestion) {
		return false
	}

	if !detector.isIntrashard(txInQuestion) {
		return false
	}

	for _, tx := range allTransactionsInBlock {
		matchesTypeOnSource := tx.ProcessingTypeOnSource == transactionProcessingTypeMoveBalance
		matchesTypeOnDestination := tx.ProcessingTypeOnDestination == transactionProcessingTypeMoveBalance
		matchesOriginalTransactionHash := tx.OriginalTransactionHash == txInQuestion.Hash
		matchesRefundValue := tx.Value == txInQuestion.Value

		if matchesTypeOnSource && matchesTypeOnDestination && matchesOriginalTransactionHash && matchesRefundValue {
			return false
		}
	}

	return true
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
