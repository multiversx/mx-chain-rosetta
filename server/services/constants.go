package services

import "encoding/hex"

var (
	transactionVersion                           = 1
	transactionProcessingTypeRelayed             = "RelayedTx"
	transactionProcessingTypeBuiltInFunctionCall = "BuiltInFunctionCall"
	builtInFunctionClaimDeveloperRewards         = "ClaimDeveloperRewards"
	refundGasMessage                             = "refundedGas"
	sendingValueToNonPayableContractDataPrefix   = "@" + hex.EncodeToString([]byte("sending value to non payable contract"))
	emptyHash                                    = "0000000000000000000000000000000000000000000000000000000000000000"
)
