package services

import (
	"errors"

	"github.com/coinbase/rosetta-sdk-go/types"
)

type errCode int32

const (
	ErrUnknown errCode = iota + 1
	ErrUnableToGetAccount
	ErrInvalidAccountAddress
	ErrUnableToGetBlock
	ErrNotImplemented
	ErrUnableToSubmitTransaction
	ErrMalformedValue
	ErrUnableToGetNodeStatus
	ErrMustQueryByIndexOrByHash
	ErrConstruction
	ErrUnableToGetNetworkConfig
	ErrUnsupportedCurveType
	ErrInsufficientGasLimit
	ErrGasPriceTooLow
	ErrTransactionIsNotInPool
	ErrCannotParsePoolTransaction
	ErrInvalidInputParam
	ErrOfflineMode
	ErrUnableToGetGenesisBlock
)

type errPrototype struct {
	code      errCode
	message   string
	retriable bool
}

type errFactory struct {
	prototypes    []errPrototype
	prototypesMap map[errCode]errPrototype
}

func newErrFactory() *errFactory {
	prototypes, prototypesMap := createErrPrototypes()
	return &errFactory{
		prototypes:    prototypes,
		prototypesMap: prototypesMap,
	}
}

func createErrPrototypes() ([]errPrototype, map[errCode]errPrototype) {
	prototypes := []errPrototype{
		{
			code:      ErrUnknown,
			message:   "unknown error",
			retriable: false,
		},
		{
			code:      ErrUnableToGetAccount,
			message:   "unable to get account",
			retriable: true,
		},
		{
			code:      ErrInvalidAccountAddress,
			message:   "invalid account address",
			retriable: false,
		},
		{
			code:      ErrUnableToGetBlock,
			message:   "unable to get block",
			retriable: true,
		},
		{
			code:      ErrNotImplemented,
			message:   "operation not implemented",
			retriable: false,
		},
		{
			code:      ErrUnableToSubmitTransaction,
			message:   "unable to submit transaction",
			retriable: true,
		},
		{
			code:      ErrMalformedValue,
			message:   "malformed value",
			retriable: false,
		},
		{
			code:      ErrUnableToGetNodeStatus,
			message:   "unable to get node status",
			retriable: true,
		},
		{
			code:      ErrMustQueryByIndexOrByHash,
			message:   "must query block by index or by hash",
			retriable: false,
		},
		{
			code:      ErrConstruction,
			message:   "construction error",
			retriable: false,
		},
		{
			code:      ErrUnableToGetNetworkConfig,
			message:   "unable to get network config",
			retriable: true,
		},
		{
			code:      ErrInvalidInputParam,
			message:   "Invalid input param: ",
			retriable: false,
		},
		{
			code:      ErrUnsupportedCurveType,
			message:   "unsupported curve type",
			retriable: false,
		},
		{
			code:      ErrInsufficientGasLimit,
			message:   "insufficient gas limit",
			retriable: false,
		},
		{
			code:      ErrGasPriceTooLow,
			message:   "gas price is to low",
			retriable: false,
		},
		{
			code:      ErrTransactionIsNotInPool,
			message:   "transaction is not in pool",
			retriable: true,
		},
		{
			code:      ErrCannotParsePoolTransaction,
			message:   "cannot parse pool transaction",
			retriable: false,
		},
		{
			code:      ErrOfflineMode,
			message:   "rosetta server is in offline mode",
			retriable: false,
		},
		{
			code:      ErrUnableToGetGenesisBlock,
			message:   "unable to get genesis block",
			retriable: true,
		},
	}

	prototypesMap := make(map[errCode]errPrototype)

	for _, prototype := range prototypes {
		prototypesMap[prototype.code] = prototype
	}

	return prototypes, prototypesMap
}

func (factory *errFactory) getPossibleErrors() []*types.Error {
	possibleErrors := make([]*types.Error, 0, len(factory.prototypes))

	for _, prototype := range factory.prototypes {
		possibleErrors = append(possibleErrors, &types.Error{
			Code:      int32(prototype.code),
			Message:   prototype.message,
			Retriable: prototype.retriable,
		})
	}

	return possibleErrors
}

func (factory *errFactory) newErrWithOriginal(code errCode, originalError error) *types.Error {
	err := factory.newErr(code)
	err.Details = map[string]interface{}{
		"originalError": originalError.Error(),
	}

	return err
}

func (factory *errFactory) newErr(code errCode) *types.Error {
	prototype := factory.getPrototypeByCode(code)

	return &types.Error{
		Code:      int32(code),
		Message:   prototype.message,
		Retriable: prototype.retriable,
	}
}

func (factory *errFactory) getPrototypeByCode(code errCode) errPrototype {
	prototype, ok := factory.prototypesMap[code]
	if ok {
		return prototype
	}

	return factory.prototypesMap[ErrUnknown]
}

var errEventNotFound = errors.New("transaction event not found")
var errCannotRecognizeEvent = errors.New("cannot recognize transaction event")
