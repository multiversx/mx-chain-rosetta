package services

const (
	transactionVersion = 1

	opGenesisBalanceMovement = "GenesisBalanceMovement"
	opTransfer               = "Transfer"
	opFee                    = "Fee"
	opReward                 = "Reward"
	opScResult               = "SmartContractResult"
	opInvalid                = "Invalid"
)

var (
	// OpStatusSuccess is the operation status for successful operations.
	OpStatusSuccess = "Success"
	// OpStatusFailed is the operation status for failed operations.
	// TODO: remove this or use it for invalid transactions
	OpStatusFailed = "Failed"
)

const (
	TransactionProcessingTypeRelayed = "RelayedTx"
)

const emptyHash = "0000000000000000000000000000000000000000000000000000000000000000"
