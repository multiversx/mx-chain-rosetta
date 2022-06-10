package provider

import (
	"errors"
	"fmt"
)

var errIsOffline = errors.New("server is in offline mode")
var errCannotGetBlock = errors.New("cannot get block")
var errCannotGetAccount = errors.New("cannot get account")
var errCannotGetTransaction = errors.New("cannot get transaction")

func newErrCannotGetBlockByNonce(nonce uint64, innerError error) error {
	return fmt.Errorf("%w: %v, nonce = %d", errCannotGetBlock, innerError, nonce)
}

func newErrCannotGetBlockByHash(hash string, innerError error) error {
	return fmt.Errorf("%w: %v, hash = %s", errCannotGetBlock, innerError, hash)
}

func newErrCannotGetAccount(address string, innerError error) error {
	return fmt.Errorf("%w: %v, address = %s", errCannotGetAccount, innerError, address)
}

func newErrCannotGetTransaction(hash string, innerError error) error {
	return fmt.Errorf("%w: %v, address = %s", errCannotGetTransaction, innerError, hash)
}
