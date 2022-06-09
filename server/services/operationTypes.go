package services

const (
	opGenesisBalanceMovement = "GenesisBalanceMovement"
	opTransfer               = "Transfer"
	opFee                    = "Fee"
	opReward                 = "Reward"
	opScResult               = "SmartContractResult"
	opFeeOfInvalidTx         = "FeeOfInvalidTransaction"
)

// SupportedOperationTypes is a list of the supported operations.
var SupportedOperationTypes = []string{
	opTransfer, opFee, opReward, opScResult, opFeeOfInvalidTx, opGenesisBalanceMovement,
}
