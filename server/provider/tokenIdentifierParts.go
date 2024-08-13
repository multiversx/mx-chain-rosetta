package provider

import (
	"fmt"
	"strconv"
	"strings"
)

type tokenIdentifierParts struct {
	ticker                   string
	randomSequence           string
	tickerWithRandomSequence string
	nonce                    uint64
}

func parseTokenIdentifierIntoParts(tokenIdentifier string) (*tokenIdentifierParts, error) {
	parts := strings.Split(tokenIdentifier, "-")

	// Fungible tokens
	if len(parts) == 2 {
		return &tokenIdentifierParts{
			ticker:                   parts[0],
			randomSequence:           parts[1],
			tickerWithRandomSequence: tokenIdentifier,
			nonce:                    0,
		}, nil
	}

	// Non-fungible tokens
	if len(parts) == 3 {
		nonceHex := parts[2]
		nonce, err := strconv.ParseUint(nonceHex, 16, 64)
		if err != nil {
			return nil, newErrCannotParseTokenIdentifier(tokenIdentifier, err)
		}

		return &tokenIdentifierParts{
			ticker:                   parts[0],
			randomSequence:           parts[1],
			tickerWithRandomSequence: fmt.Sprintf("%s-%s", parts[0], parts[1]),
			nonce:                    nonce,
		}, nil
	}

	return nil, newErrCannotParseTokenIdentifier(tokenIdentifier, nil)
}
