package services

import (
	"fmt"
)

type eventESDTOrESDTNFTTransfer struct {
	sender       string
	identifier   string
	nonceAsBytes []byte
	value        string
	receiver     string
}

// getBaseIdentifier returns the token identifier for fungible tokens, and the collection identifier for SFTs, NFTs and MetaESDTs
func (event *eventESDTOrESDTNFTTransfer) getBaseIdentifier() string {
	return event.identifier
}

// getComposedIdentifier returns the "full" token identifier for all types of ESDTs
func (event *eventESDTOrESDTNFTTransfer) getComposedIdentifier() string {
	if len(event.nonceAsBytes) > 0 {
		return fmt.Sprintf("%s-%x", event.identifier, event.nonceAsBytes)
	}

	return event.identifier
}

func (event *eventESDTOrESDTNFTTransfer) String() string {
	return fmt.Sprintf("%s - (%s-%x) - > %s (%s)", event.sender, event.identifier, event.nonceAsBytes, event.receiver, event.value)
}
