package services

const (
	transactionVersion = 1

	opTransfer = "Transfer"
	opFee      = "Fee"
	opReward   = "Reward"
	opScResult = "SmartContractResult"
	opInvalid  = "Invalid"
)

var (
	// OpStatusSuccess is the operation status for successful operations.
	OpStatusSuccess = "Success"
	// OpStatusFailed is the operation status for failed operations.
	OpStatusFailed = "Failed"
)

const (
	TransactionProcessingTypeRelayed = "RelayedTx"
)