package services

import (
	"fmt"
	"strings"

	"github.com/coinbase/rosetta-sdk-go/types"
	logger "github.com/multiversx/mx-chain-logger-go"
)

var log = logger.GetOrCreate("server/services")

func traceBlockResponse(response *types.BlockResponse) {
	if log.GetLevel() > logger.LogTrace {
		return
	}

	log.Trace("Block",
		"nonce", response.Block.BlockIdentifier.Index,
		"hash", response.Block.BlockIdentifier.Hash,
		"numTxs", len(response.Block.Transactions),
	)

	for txIndex, tx := range response.Block.Transactions {
		fmt.Printf("(TX %d) %s\n", txIndex, tx.TransactionIdentifier.Hash)

		for opIndex, op := range tx.Operations {
			formattedAmount := formatAmount(op.Amount)

			fmt.Printf("\t(OP %d) %s\t%25s\n", opIndex, op.Account.Address, formattedAmount)
		}
	}
}

func formatAmount(amount *types.Amount) string {
	value := amount.Value
	hasSign := strings.HasPrefix(value, "-") || strings.HasPrefix(value, "+")
	if !hasSign {
		value = "+" + value
	}

	return value
}
