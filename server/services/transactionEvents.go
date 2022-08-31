package services

import (
	"fmt"
)

type eventTransferESDT struct {
	sender          string
	tokenIdentifier string
	receiver        string
	value           string
}

func (event *eventTransferESDT) String() string {
	return fmt.Sprintf("%s - (%s) - > %s (%s)", event.sender, event.tokenIdentifier, event.receiver, event.value)
}
