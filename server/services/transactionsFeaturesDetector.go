package services

import (
	"github.com/multiversx/mx-chain-core-go/data/transaction"
)

type transactionsFeaturesDetector struct {
	networkProvider          NetworkProvider
	networkProviderExtension *networkProviderExtension
	eventsController         *transactionEventsController
}

func newTransactionsFeaturesDetector(provider NetworkProvider) *transactionsFeaturesDetector {
	return &transactionsFeaturesDetector{
		networkProvider:          provider,
		networkProviderExtension: newNetworkProviderExtension(provider),
		eventsController:         newTransactionEventsController(provider),
	}
}

// Example SCRs can be found here: https://api.multiversx.com/transactions?function=ClaimDeveloperRewards
// Unfortunately, the network does not provide a way to easily and properly detect whether a SCR
// is the result of claiming developer rewards. Here, we apply a best-effort (and suboptimal) strategy:
// we scan through all the other items in the block, in order to find a parent transaction that matches the operation in question.
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
// See: MX-16423. Analyze whether we can simplify the conditions below, or possibly discard them completely / replace them with simpler ones.
func (detector *transactionsFeaturesDetector) isInvalidTransactionOfTypeMoveBalanceThatOnlyConsumesDataMovementGas(tx *transaction.ApiTransactionResult) bool {
	isInvalid := tx.Type == string(transaction.TxTypeInvalid)
	isMoveBalance := tx.ProcessingTypeOnSource == transactionProcessingTypeMoveBalance && tx.ProcessingTypeOnDestination == transactionProcessingTypeMoveBalance

	if !isInvalid || !isMoveBalance {
		return false
	}

	withSendingValueToNonPayableContract := detector.eventsController.hasSignalErrorOfSendingValueToNonPayableContract(tx)
	withMetaTransactionIsInvalid := detector.eventsController.hasSignalErrorOfMetaTransactionIsInvalid(tx)
	return withSendingValueToNonPayableContract || withMetaTransactionIsInvalid
}

func (detector *transactionsFeaturesDetector) isContractDeploymentWithSignalErrorOrIntrashardContractCallWithSignalError(tx *transaction.ApiTransactionResult) bool {
	return detector.isContractDeploymentWithSignalError(tx) || (detector.isIntrashard(tx) && detector.isContractCallWithSignalError(tx))
}

func (detector *transactionsFeaturesDetector) isContractDeploymentWithSignalError(tx *transaction.ApiTransactionResult) bool {
	return tx.ProcessingTypeOnSource == transactionProcessingTypeContractDeployment &&
		tx.ProcessingTypeOnDestination == transactionProcessingTypeContractDeployment &&
		detector.eventsController.hasAnySignalError(tx)
}

func (detector *transactionsFeaturesDetector) isContractCallWithSignalError(tx *transaction.ApiTransactionResult) bool {
	return tx.ProcessingTypeOnSource == transactionProcessingTypeContractInvoking &&
		tx.ProcessingTypeOnDestination == transactionProcessingTypeContractInvoking &&
		detector.eventsController.hasAnySignalError(tx)
}

func (detector *transactionsFeaturesDetector) isIntrashard(tx *transaction.ApiTransactionResult) bool {
	return tx.SourceShard == tx.DestinationShard
}

// isSmartContractResultIneffectiveRefund detects smart contract results that are ineffective refunds.
// Also see: https://console.cloud.google.com/bigquery?sq=667383445384:de7a5f3f172f4b50a9aaed353ef79839
func (detector *transactionsFeaturesDetector) isSmartContractResultIneffectiveRefund(scr *transaction.ApiTransactionResult) bool {
	return scr.IsRefund && scr.Sender == scr.Receiver && detector.networkProviderExtension.isContractAddress(scr.Sender)
}
