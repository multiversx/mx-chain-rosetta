package provider

import (
	"github.com/multiversx/mx-chain-core-go/data/transaction"
)

// IsRelayedTxV3 checks whether the provided (API) transaction is relayed (V3).
// See: https://github.com/multiversx/mx-chain-go/blob/v1.8.8/common/common.go.
func IsRelayedTxV3(tx *transaction.ApiTransactionResult) bool {
	hasRelayer := len(tx.RelayerAddress) > 0 && len(tx.RelayerAddress) == len(tx.Sender)
	if !hasRelayer {
		return false
	}

	hasRelayerSignature := len(tx.RelayerSignature) > 0 && len(tx.RelayerSignature) == len(tx.Signature)
	if !hasRelayerSignature {
		return false
	}

	return true
}
