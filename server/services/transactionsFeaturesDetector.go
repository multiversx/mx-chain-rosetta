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

// isInvalidTransactionOfSendingValueToNonPayableContract detects intra-shard transactions
// bearing the error "sending value to non-payable contract", which are included in invalid mini-block.
func (extractor *transactionsFeaturesDetector) isInvalidTransactionOfSendingValueToNonPayableContract(tx *data.FullTransaction) bool {
	if tx.Type != string(transaction.TxTypeInvalid) {
		return false
	}

	return extractor.eventsController.hasSignalErrorOfSendingValueToNonPayableContract(tx)
}
