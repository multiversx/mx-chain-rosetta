package services

import (
	"fmt"
	"math/big"
)

type eventTransferValueOnly struct {
	sender   string
	receiver string
	value    string
}

type eventESDT struct {
	senderAddress   string
	receiverAddress string
	otherAddress    string
	identifier      string
	nonceAsBytes    []byte
	value           string
}

// newEventESDTFromBasicTopics creates an eventESDT from the given topics. The following topics are expected:
// - topic 0: the identifier of the token
// - topic 1: the nonce of the token
// - topic 2: the value of the token
func newEventESDTFromBasicTopics(topics [][]byte) (*eventESDT, error) {
	if len(topics) < 3 {
		return nil, fmt.Errorf("newEventESDTFromBasicTopics: bad number of topics: %d", len(topics))
	}

	identifier := topics[0]
	nonceAsBytes := topics[1]
	valueBytes := topics[2]
	value := big.NewInt(0).SetBytes(valueBytes)

	return &eventESDT{
		identifier:   string(identifier),
		nonceAsBytes: nonceAsBytes,
		value:        value.String(),
	}, nil
}

// getBaseIdentifier returns the token identifier for fungible tokens, and the collection identifier for SFTs, NFTs and MetaESDTs
func (event *eventESDT) getBaseIdentifier() string {
	return event.identifier
}

// getExtendedIdentifier returns the "full" token identifier for all types of ESDTs
func (event *eventESDT) getExtendedIdentifier() string {
	if len(event.nonceAsBytes) > 0 {
		return fmt.Sprintf("%s-%x", event.identifier, event.nonceAsBytes)
	}

	return event.identifier
}

type eventSCDeploy struct {
	contractAddress string
	deployerAddress string
}

type eventClaimDeveloperRewards struct {
	value           string
	receiverAddress string
}
