package services

import (
	"bytes"

	"github.com/ElrondNetwork/elrond-go-core/data/transaction"
	"github.com/ElrondNetwork/elrond-proxy-go/data"
)

type transactionsFeaturesDetector struct {
	eventsController *transactionEventsController
}

func newTransactionsFeaturesDetector(provider NetworkProvider) *transactionsFeaturesDetector {
	return &transactionsFeaturesDetector{
		eventsController: newTransactionEventsController(provider),
	}
}

// Example SCRs can be found here: https://api.elrond.com/transactions?function=ClaimDeveloperRewards
func (extractor *transactionsFeaturesDetector) doesContractResultHoldRewardsOfClaimDeveloperRewards(
	contractResult *data.FullTransaction,
	allTransactionsInBlock []*data.FullTransaction,
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
func (extractor *transactionsFeaturesDetector) isInvalidTransactionOfTypeMoveBalanceThatOnlyConsumesDataMovementGas(tx *data.FullTransaction) bool {
	isInvalid := tx.Type == string(transaction.TxTypeInvalid)
	isMoveBalance := tx.ProcessingTypeOnSource == transactionProcessingTypeMoveBalance && tx.ProcessingTypeOnDestination == transactionProcessingTypeMoveBalance

	if !isInvalid || !isMoveBalance {
		return false
	}

	// TODO: Analyze whether we can simplify the conditions below, or possibly discard them completely / replace them with simpler ones.
	withSendingValueToNonPayableContract := extractor.eventsController.hasSignalErrorOfSendingValueToNonPayableContract(tx)
	withMetaTransactionIsInvalid := extractor.eventsController.hasSignalErrorOfMetaTransactionIsInvalid(tx)
	return withSendingValueToNonPayableContract || withMetaTransactionIsInvalid
}
