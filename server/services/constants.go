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

const (
	TransactionProcessingTypeRelayed = "RelayedTx"
)

const emptyHash = "0000000000000000000000000000000000000000000000000000000000000000"
