package services

import (
	"github.com/multiversx/mx-chain-core-go/data/transaction"
)

func isContractResultOfOpaquelyClaimingDeveloperRewards(tx *transaction.ApiTransactionResult) bool {
	if tx.BlockNonce == 15693863 && tx.SourceShard == 2 && tx.Hash == "8ddecc831c70ecf0ca312a46e04bbdc7508fabada494714ec41fd49c8ec13915" {
		log.Warn("Exception: 8ddecc831c70ecf0ca312a46e04bbdc7508fabada494714ec41fd49c8ec13915")
		return true
	}

	return false
}
