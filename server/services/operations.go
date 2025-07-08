package services

import (
	"github.com/coinbase/rosetta-sdk-go/types"
)

const (
	opGenesisBalanceMovement = "GenesisBalanceMovement"
	opTransfer               = "Transfer"
	opFee                    = "Fee"
	opReward                 = "Reward"
	opScResult               = "SmartContractResult"
	opFeeRefundAsScResult    = "FeeRefundAsSmartContractResult"
	opDeveloperRewards       = "DeveloperRewards"
	opFeeOfInvalidTx         = "FeeOfInvalidTransaction"
	opFeeRefund              = "FeeRefund"
	opCustomTransfer         = "CustomTransfer"
)

var (
	// SupportedOperationTypes is a list of the supported operations
	SupportedOperationTypes = []string{
		opGenesisBalanceMovement,
		opTransfer,
		opFee,
		opReward,
		opScResult,
		opFeeRefundAsScResult,
		opDeveloperRewards,
		opFeeOfInvalidTx,
		opFeeRefund,
		opCustomTransfer,
	}

	opStatusSuccess = "Success"
	opStatusFailure = "Failure"

	supportedOperationStatuses = []*types.OperationStatus{
		{
			Status:     opStatusSuccess,
			Successful: true,
		},
		{
			Status:     opStatusFailure,
			Successful: false,
		},
	}
)

func filterOperationsByAddress(operations []*types.Operation, predicate func(address string) (bool, error)) ([]*types.Operation, error) {
	filtered := make([]*types.Operation, 0, len(operations))

	for _, operation := range operations {
		address := operation.Account.Address

		shouldInclude, err := predicate(address)
		if err != nil {
			return nil, err
		}
		if shouldInclude {
			filtered = append(filtered, operation)
		}
	}

	indexOperations(filtered)
	return filtered, nil
}

func filterOutOperationsWithZeroAmount(operations []*types.Operation) []*types.Operation {
	filtered := make([]*types.Operation, 0, len(operations))

	for _, operation := range operations {
		shouldInclude := isNonZeroAmount(operation.Amount.Value)
		if shouldInclude {
			filtered = append(filtered, operation)
		}
	}

	indexOperations(filtered)
	return filtered
}

func indexOperations(operations []*types.Operation) {
	for index, operation := range operations {
		operation.OperationIdentifier = indexToOperationIdentifier(index)
	}
}

func applyDefaultStatusOnOperations(operations []*types.Operation) {
	for _, operation := range operations {
		if operation.Status == nil {
			operation.Status = &opStatusSuccess
		}
	}
}
