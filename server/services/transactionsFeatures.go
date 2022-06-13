package services

import (
	"bytes"

	"github.com/ElrondNetwork/elrond-proxy-go/data"
)

// Example SCRs can be found here: https://api.elrond.com/transactions?function=ClaimDeveloperRewards
func doesContractResultHoldRewardsOfClaimDeveloperRewards(
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
