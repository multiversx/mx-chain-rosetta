// Package unified builds the unified chain: one linear chain whose blocks correspond, one to one, to the metachain blocks.
// A unified block holds the transactions of all shard blocks notarized by its metablock.
package unified

import (
	"errors"
	"fmt"

	"github.com/coinbase/rosetta-sdk-go/types"
)

// ErrNotReady signals that the unified chain is not ready yet (e.g. the first synchronization step did not complete)
var ErrNotReady = errors.New("the unified chain is not ready")

// ErrBlockNotYetAvailable signals that the requested block is above the tip of the unified chain (or not indexed yet)
var ErrBlockNotYetAvailable = errors.New("block not yet available on the unified chain")

// ErrBlockTooOld signals that the requested block is older than the oldest available block
var ErrBlockTooOld = errors.New("block is older than the oldest available block")

// ErrBackendUnavailable signals that a shard instance could not be reached
var ErrBackendUnavailable = errors.New("shard backend unavailable")

// ErrBackendInconsistent signals that a shard instance returned data that does not match the metachain
var ErrBackendInconsistent = errors.New("shard backend returned a block that does not match the metachain")

// ErrMetachainUnavailable signals that the metachain source could not be reached
var ErrMetachainUnavailable = errors.New("metachain source unavailable")

// BackendRosettaError wraps a Rosetta error returned by a shard instance (to be passed through to the client)
type BackendRosettaError struct {
	Shard        uint32
	RosettaError *types.Error
}

// Error implements the error interface
func (err *BackendRosettaError) Error() string {
	return fmt.Sprintf("shard %d returned error %d: %s", err.Shard, err.RosettaError.Code, err.RosettaError.Message)
}

func newErrBackendUnavailable(shard uint32, err error) error {
	return fmt.Errorf("%w: shard %d: %v", ErrBackendUnavailable, shard, err)
}

func newErrBackendInconsistent(shard uint32, message string) error {
	return fmt.Errorf("%w: shard %d: %s", ErrBackendInconsistent, shard, message)
}
