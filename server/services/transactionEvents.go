package services

import (
	"fmt"
)

type eventTransferValueOnly struct {
	sender   string
	receiver string
	value    string
}

func (event *eventTransferValueOnly) String() string {
	return fmt.Sprintf("%s -> %s (%s)", event.sender, event.receiver, event.value)
}
