package components

import (
	"github.com/ElrondNetwork/elrond-proxy-go/facade"
	"github.com/ElrondNetwork/elrond-proxy-go/process"
)

// ObserverFacade holds (embeds) several components implemented in elrond-proxy-go
type ObserverFacade struct {
	process.Processor
	facade.TransactionProcessor
	facade.BlockProcessor
}
