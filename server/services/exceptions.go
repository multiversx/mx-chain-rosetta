package services

import (
	"github.com/multiversx/mx-chain-core-go/data/transaction"
)

// See:
// - https://explorer.multiversx.com/transactions?function=claimRewardsAllContracts
// - https://github.com/multiversx/mx-chain-vm-common-go/pull/247
func isContractResultOfOpaquelyClaimingDeveloperRewards(tx *transaction.ApiTransactionResult) bool {
	if tx.BlockNonce == 15693863 && tx.SourceShard == 2 && tx.Hash == "8ddecc831c70ecf0ca312a46e04bbdc7508fabada494714ec41fd49c8ec13915" {
		log.Warn("Exception: 8ddecc831c70ecf0ca312a46e04bbdc7508fabada494714ec41fd49c8ec13915")
		return true
	}

	if tx.BlockNonce == 18098197 && tx.SourceShard == 2 && tx.Hash == "90357d8f72bb1c749b46e394a4be349d4b2700a8f734470f815a524059f7031b" {
		log.Warn("Exception: 90357d8f72bb1c749b46e394a4be349d4b2700a8f734470f815a524059f7031b")
		return true
	}

	return false
}
