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
	opESDTTransfer           = "ESDTTransfer"
)

var (
	// SupportedOperationTypes is a list of the supported operations.
	SupportedOperationTypes = []string{
		opTransfer,
		opFee,
		opReward,
		opScResult,
		opFeeOfInvalidTx,
		opGenesisBalanceMovement,
		opFeeRefund,
	}

	opStatusSuccess = "Success"

	supportedOperationStatuses = []*types.OperationStatus{
		{
			Status:     opStatusSuccess,
			Successful: true,
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

func populateStatusOfOperations(operations []*types.Operation) {
	for _, operation := range operations {
		// TODO: Improve this, perhaps use a clone?
		operation.Status = &opStatusSuccess
	}
}
