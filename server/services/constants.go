package services

import (
	"encoding/hex"
	"strings"
)

var (
	transactionVersion                           = 1
	transactionProcessingTypeRelayed             = "RelayedTx"
	transactionProcessingTypeBuiltInFunctionCall = "BuiltInFunctionCall"
	transactionProcessingTypeMoveBalance         = "MoveBalance"
	builtInFunctionClaimDeveloperRewards         = "ClaimDeveloperRewards"
	refundGasMessage                             = "refundedGas"
	argumentsSeparator                           = "@"
	sendingValueToNonPayableContractDataPrefix   = argumentsSeparator + hex.EncodeToString([]byte("sending value to non payable contract"))
	emptyHash                                    = strings.Repeat("0", 64)
	nodeVersionForOfflineRosetta                 = "N / A"
)

var (
	transactionEventSignalError                             = "signalError"
	transactionEventTransferValueOnly                       = "transferValueOnly"
	transactionEventTopicInvalidMetaTransaction             = "meta transaction is invalid"
	transactionEventTopicInvalidMetaTransactionNotEnoughGas = "meta transaction is invalid: not enough gas"
)
