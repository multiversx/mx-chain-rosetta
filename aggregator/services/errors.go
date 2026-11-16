// Package services implements the Rosetta endpoints of the aggregator, on top of the unified chain and of the shard backends.
package services

import (
	"errors"
	"sort"

	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/multiversx/mx-chain-rosetta/aggregator/unified"
)

type errCode int32

// Error codes of the aggregator. They do not overlap with the error codes of the shard instances (which are passed through).
const (
	ErrBlockNotYetAvailable errCode = iota + 120
	ErrBackendUnavailable
	ErrAddressNotServed
	ErrBackendInconsistent
	ErrMetachainUnavailable
	ErrBlockTooOld
	ErrInvalidAddress
	ErrOfflineMode
	ErrNotImplemented
	ErrInvalidRequest
	ErrInternal
)

type errPrototype struct {
	code      errCode
	message   string
	retriable bool
}

var errPrototypes = []errPrototype{
	{code: ErrBlockNotYetAvailable, message: "block not yet available on the unified chain", retriable: true},
	{code: ErrBackendUnavailable, message: "shard backend unavailable", retriable: true},
	{code: ErrAddressNotServed, message: "address belongs to a shard that is not served", retriable: false},
	{code: ErrBackendInconsistent, message: "shard backend returned a block that does not match the metachain", retriable: true},
	{code: ErrMetachainUnavailable, message: "metachain source unavailable", retriable: true},
	{code: ErrBlockTooOld, message: "block is older than the oldest available block", retriable: false},
	{code: ErrInvalidAddress, message: "invalid address", retriable: false},
	{code: ErrOfflineMode, message: "aggregator is in offline mode", retriable: false},
	{code: ErrNotImplemented, message: "operation not implemented", retriable: false},
	{code: ErrInvalidRequest, message: "invalid request", retriable: false},
	{code: ErrInternal, message: "internal error", retriable: false},
}

type errFactory struct {
	prototypesByCode map[errCode]errPrototype
}

func newErrFactory() *errFactory {
	prototypesByCode := make(map[errCode]errPrototype, len(errPrototypes))
	for _, prototype := range errPrototypes {
		prototypesByCode[prototype.code] = prototype
	}

	return &errFactory{prototypesByCode: prototypesByCode}
}

func (factory *errFactory) newErr(code errCode) *types.Error {
	prototype, ok := factory.prototypesByCode[code]
	if !ok {
		prototype = factory.prototypesByCode[ErrInternal]
	}

	return &types.Error{
		Code:      int32(prototype.code),
		Message:   prototype.message,
		Retriable: prototype.retriable,
	}
}

func (factory *errFactory) newErrWithDetails(code errCode, details string) *types.Error {
	err := factory.newErr(code)
	err.Details = map[string]interface{}{
		"details": details,
	}

	return err
}

// fromError converts an error of the unified chain into a Rosetta error.
// Rosetta errors returned by shard instances are passed through (unchanged).
func (factory *errFactory) fromError(err error) *types.Error {
	var backendRosettaErr *unified.BackendRosettaError
	if errors.As(err, &backendRosettaErr) {
		return backendRosettaErr.RosettaError
	}

	switch {
	case errors.Is(err, unified.ErrBlockNotYetAvailable), errors.Is(err, unified.ErrNotReady):
		return factory.newErrWithDetails(ErrBlockNotYetAvailable, err.Error())
	case errors.Is(err, unified.ErrBlockTooOld):
		return factory.newErrWithDetails(ErrBlockTooOld, err.Error())
	case errors.Is(err, unified.ErrBackendUnavailable):
		return factory.newErrWithDetails(ErrBackendUnavailable, err.Error())
	case errors.Is(err, unified.ErrBackendInconsistent):
		return factory.newErrWithDetails(ErrBackendInconsistent, err.Error())
	case errors.Is(err, unified.ErrMetachainUnavailable):
		return factory.newErrWithDetails(ErrMetachainUnavailable, err.Error())
	default:
		return factory.newErrWithDetails(ErrInternal, err.Error())
	}
}

func (factory *errFactory) getPossibleErrors() []*types.Error {
	possibleErrors := make([]*types.Error, 0, len(errPrototypes))
	for _, prototype := range errPrototypes {
		possibleErrors = append(possibleErrors, factory.newErr(prototype.code))
	}

	return possibleErrors
}

// mergeErrors returns the errors of the shard instances together with the errors of the aggregator (sorted by code, no duplicated codes)
func (factory *errFactory) mergeErrors(shardErrors []*types.Error) []*types.Error {
	byCode := make(map[int32]*types.Error)

	for _, err := range shardErrors {
		if err != nil {
			byCode[err.Code] = err
		}
	}
	for _, err := range factory.getPossibleErrors() {
		byCode[err.Code] = err
	}

	merged := make([]*types.Error, 0, len(byCode))
	for _, err := range byCode {
		merged = append(merged, err)
	}

	sort.Slice(merged, func(i, j int) bool { return merged[i].Code < merged[j].Code })
	return merged
}
