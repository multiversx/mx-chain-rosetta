package services

import (
	"encoding/hex"
	"strings"

	"github.com/multiversx/mx-chain-core-go/core"
)

var (
	transactionVersion                           = 1
	transactionProcessingTypeRelayed             = "RelayedTx"
	transactionProcessingTypeBuiltInFunctionCall = "BuiltInFunctionCall"
	transactionProcessingTypeMoveBalance         = "MoveBalance"
	transactionProcessingTypeContractInvoking    = "SCInvoking"
	amountZero                                   = "0"
	builtInFunctionClaimDeveloperRewards         = core.BuiltInFunctionClaimDeveloperRewards
	builtInFunctionESDTTransfer                  = core.BuiltInFunctionESDTTransfer
	refundGasMessage                             = "refundedGas"
	argumentsSeparator                           = "@"
	sendingValueToNonPayableContractDataPrefix   = argumentsSeparator + hex.EncodeToString([]byte("sending value to non payable contract"))
	emptyHash                                    = strings.Repeat("0", 64)
	nodeVersionForOfflineRosetta                 = "N / A"
)

var (
	transactionEventSignalError                             = core.SignalErrorOperation
	transactionEventTransferValueOnly                       = "transferValueOnly"
	transactionEventESDTTransfer                            = "ESDTTransfer"
	transactionEventESDTNFTTransfer                         = "ESDTNFTTransfer"
	transactionEventMultiESDTNFTTransfer                    = "MultiESDTNFTTransfer"
	transactionEventESDTLocalBurn                           = "ESDTLocalBurn"
	transactionEventESDTLocalMint                           = "ESDTLocalMint"
	transactionEventESDTWipe                                = "ESDTWipe"
	transactionEventTopicInvalidMetaTransaction             = "meta transaction is invalid"
	transactionEventTopicInvalidMetaTransactionNotEnoughGas = "meta transaction is invalid: not enough gas"
)
