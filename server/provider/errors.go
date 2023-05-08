package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/multiversx/mx-chain-rosetta/server/resources"
)

var errIsOffline = errors.New("server is in offline mode")
var errCannotGetBlock = errors.New("cannot get block")
var errCannotGetAccount = errors.New("cannot get account")
var errCannotGetTransaction = errors.New("cannot get transaction")
var errCannotGetLatestBlockNonce = errors.New("cannot get latest block nonce, maybe the node didn't start syncing")

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

// In proxy-go, the function CallGetRestEndPoint() returns an error message as the JSON content of the erroneous HTTP response.
// Here, we attempt to decode that JSON and create an error with a "flat" error message.
func convertStructuredApiErrToFlatErr(apiErr error) error {
	structuredApiErr := &structuredApiError{}
	err := json.Unmarshal([]byte(apiErr.Error()), structuredApiErr)
	if err != nil {
		// Not parsable, fallback to original
		return apiErr
	}

	flatErrString := fmt.Sprintf("%s: %s", structuredApiErr.Error, structuredApiErr.Code)
	flatErr := errors.New(flatErrString)
	return flatErr
}

type structuredApiError struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// parseBlockCoordinatesIfErrAccountNotFoundAtBlock parses block coordinates from errors such as:
// "get esdt balance for account error: account was not found at block: nonce = ..., hash = ..."
// Perhap we should handle these situations on Node API and return appropriately structured errors.
func parseBlockCoordinatesIfErrAccountNotFoundAtBlock(inputErr error) (resources.BlockCoordinates, bool) {
	errMessage := inputErr.Error()

	if !strings.Contains(errMessage, "account was not found at block") {
		return resources.BlockCoordinates{}, false
	}

	parts := strings.FieldsFunc(errMessage, func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsNumber(c)
	})

	nonce, err := strconv.ParseUint(parts[13], 10, 64)
	if err != nil {
		log.Warn("cannot parse block coordinates from error", "inputErr", inputErr, "err", err)
		return resources.BlockCoordinates{}, false
	}

	hash := parts[15]

	return resources.BlockCoordinates{
		Nonce: nonce,
		Hash:  hash,
	}, true
}
