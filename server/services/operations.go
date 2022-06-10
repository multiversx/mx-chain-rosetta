package services

import "github.com/coinbase/rosetta-sdk-go/types"

const (
	opGenesisBalanceMovement = "GenesisBalanceMovement"
	opTransfer               = "Transfer"
	opFee                    = "Fee"
	opReward                 = "Reward"
	opScResult               = "SmartContractResult"
	opFeeOfInvalidTx         = "FeeOfInvalidTransaction"
	opFeeRefund              = "FeeRefund"
)

// SupportedOperationTypes is a list of the supported operations.
var SupportedOperationTypes = []string{
	opTransfer, opFee, opReward, opScResult, opFeeOfInvalidTx, opGenesisBalanceMovement, opFeeRefund,
}

var (
	// opStatusSuccess is the operation status for successful operations.
	opStatusSuccess = "Success"
)

var supportedOperationStatuses = []*types.OperationStatus{
	{
		Status:     opStatusSuccess,
		Successful: true,
	},
}

func indexOperations(operations []*types.Operation) {
	for index, operation := range operations {
		operation.OperationIdentifier = indexToOperationIdentifier(index)
	}
}

func populateStatusOfOperations(operations []*types.Operation) {
	for _, operation := range operations {
		operation.Status = &opStatusSuccess
	}
}
