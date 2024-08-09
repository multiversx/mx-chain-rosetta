package services

import (
	"os"
	"strconv"
)

func areClaimDeveloperRewardsEventsEnabled(blockNonce uint64) bool {
	nonceAsString := os.Getenv("CLAIM_DEVELOPER_REWARDS_EVENTS_ENABLED_NONCE")
	if nonceAsString == "" {
		return false
	}

	nonce, err := strconv.Atoi(nonceAsString)
	if err != nil {
		return false
	}

	enabled := blockNonce >= uint64(nonce)
	return enabled
}
