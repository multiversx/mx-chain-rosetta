package services

import (
	"encoding/hex"
	"strings"

	"github.com/multiversx/mx-chain-core-go/core"
)

var (
	transactionVersion                           = 1
	transactionProcessingTypeRelayedV1           = "RelayedTx"
	transactionProcessingTypeBuiltInFunctionCall = "BuiltInFunctionCall"
	transactionProcessingTypeMoveBalance         = "MoveBalance"
	transactionProcessingTypeContractInvoking    = "SCInvoking"
	transactionProcessingTypeContractDeployment  = "SCDeployment"
	amountZero                                   = "0"
	builtInFunctionClaimDeveloperRewards         = core.BuiltInFunctionClaimDeveloperRewards
	builtInFunctionESDTTransfer                  = core.BuiltInFunctionESDTTransfer
	refundGasMessage                             = "refundedGas"
	argumentsSeparator                           = "@"
	sendingValueToNonPayableContractDataPrefix   = argumentsSeparator + hex.EncodeToString([]byte("sending value to non payable contract"))
	emptyHash                                    = strings.Repeat("0", 64)
	nodeVersionForOfflineRosetta                 = "N / A"
	systemContractDeployAddress                  = "erd1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq6gq4hu"
	nativeAsESDTIdentifier                       = "EGLD-000000"
)

var (
	transactionEventSignalError                             = core.SignalErrorOperation
	transactionEventInternalVMErrors                        = "internalVMErrors"
	transactionEventSCDeploy                                = core.SCDeployIdentifier
	transactionEventTransferValueOnly                       = "transferValueOnly"
	transactionEventESDTTransfer                            = "ESDTTransfer"
	transactionEventESDTNFTTransfer                         = "ESDTNFTTransfer"
	transactionEventESDTNFTCreate                           = "ESDTNFTCreate"
	transactionEventESDTNFTBurn                             = "ESDTNFTBurn"
	transactionEventESDTNFTAddQuantity                      = "ESDTNFTAddQuantity"
	transactionEventMultiESDTNFTTransfer                    = "MultiESDTNFTTransfer"
	transactionEventESDTLocalBurn                           = core.BuiltInFunctionESDTLocalBurn
	transactionEventESDTLocalMint                           = core.BuiltInFunctionESDTLocalMint
	transactionEventESDTWipe                                = core.BuiltInFunctionESDTWipe
	transactionEventClaimDeveloperRewards                   = core.BuiltInFunctionClaimDeveloperRewards
	transactionEventTopicInvalidMetaTransaction             = "meta transaction is invalid"
	transactionEventTopicInvalidMetaTransactionNotEnoughGas = "meta transaction is invalid: not enough gas"

	transactionEventDataExecuteOnDestContext = "ExecuteOnDestContext"
	transactionEventDataAsyncCall            = "AsyncCall"
)

var (
	numTopicsOfEventESDTTransfer                    = 4
	numTopicsPerTransferOfEventMultiESDTNFTTransfer = 3
	numTopicsOfEventESDTLocalBurn                   = 3
	numTopicsOfEventESDTLocalMint                   = 3
	numTopicsOfEventESDTWipe                        = 4
	numTopicsOfEventESDTNFTCreate                   = 4
	numTopicsOfEventESDTNFTBurn                     = 3
	numTopicsOfEventESDTNFTAddQuantity              = 3
	numTopicsOfEventSCDeployBeforeSirius            = 2
	numTopicsOfEventClaimDeveloperRewards           = 2
	numTopicsOfEventTransferValueOnlyAfterSirius    = 2
)
