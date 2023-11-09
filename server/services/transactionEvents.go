package services

import (
	"fmt"
)

type eventTransferValueOnly struct {
	sender         string
	senderPubKey   []byte
	receiver       string
	receiverPubKey []byte
	value          string
}

type eventESDT struct {
	senderAddress   string
	receiverAddress string
	otherAddress    string
	identifier      string
	nonceAsBytes    []byte
	value           string
}

// getBaseIdentifier returns the token identifier for fungible tokens, and the collection identifier for SFTs, NFTs and MetaESDTs
func (event *eventESDT) getBaseIdentifier() string {
	return event.identifier
}

// getComposedIdentifier returns the "full" token identifier for all types of ESDTs
func (event *eventESDT) getComposedIdentifier() string {
	if len(event.nonceAsBytes) > 0 {
		return fmt.Sprintf("%s-%x", event.identifier, event.nonceAsBytes)
	}

	return event.identifier
}
