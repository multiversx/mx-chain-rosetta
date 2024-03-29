package resources

import (
	"github.com/multiversx/mx-chain-core-go/core"
)

// AccountQueryOptions defines an (internal) account resource
type AccountQueryOptions struct {
	OnFinalBlock bool
	BlockNonce   core.OptionalUint64
	BlockHash    []byte
}

// NewAccountQueryOptionsOnFinalBlock creates an AccountQueryOptions (for the latest final block)
func NewAccountQueryOptionsOnFinalBlock() AccountQueryOptions {
	return AccountQueryOptions{
		OnFinalBlock: true,
	}
}

// NewAccountQueryOptionsWithBlockNonce creates an AccountQueryOptions (for a given block nonce)
func NewAccountQueryOptionsWithBlockNonce(blockNonce uint64) AccountQueryOptions {
	return AccountQueryOptions{
		BlockNonce: core.OptionalUint64{
			Value:    blockNonce,
			HasValue: true,
		},
	}
}

// NewAccountQueryOptionsWithBlockHash creates an AccountQueryOptions (for a given block hash)
func NewAccountQueryOptionsWithBlockHash(blockHash []byte) AccountQueryOptions {
	return AccountQueryOptions{
		BlockHash: blockHash,
	}
}
