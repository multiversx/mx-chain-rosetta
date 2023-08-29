package services

import (
	"bytes"

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
		matchesData := bytes.Equal(tx.Data, []byte(builtInFunctionClaimDeveloperRewards))

		if matchesTypeOnSource && matchesTypeOnDestination && matchesData {
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

func (detector *transactionsFeaturesDetector) isRelayedTransactionCompletelyIntrashardWithSignalError(tx *transaction.ApiTransactionResult, innerTx *innerTransactionOfRelayedV1) bool {
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
