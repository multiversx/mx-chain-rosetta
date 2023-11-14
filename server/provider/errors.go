package provider

import (
	"encoding/json"
	"errors"
	"fmt"
)

var errIsOffline = errors.New("server is in offline mode")
var errCannotGetBlock = errors.New("cannot get block")
var errCannotGetAccount = errors.New("cannot get account")
var errCannotGetTransaction = errors.New("cannot get transaction")
var errCannotGetLatestBlockNonce = errors.New("cannot get latest block nonce, maybe the node didn't start syncing")
var errCannotParseTokenIdentifier = errors.New("cannot parse token identifier")

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

func newErrCannotParseTokenIdentifier(tokenIdentifier string, innerError error) error {
	return fmt.Errorf("%w: %v, tokenIdentifier = %s", errCannotParseTokenIdentifier, innerError, tokenIdentifier)
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
