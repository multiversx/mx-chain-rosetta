package components

import (
	"github.com/multiversx/mx-chain-proxy-go/facade"
	"github.com/multiversx/mx-chain-proxy-go/process"
)

// ObserverFacade holds (embeds) several components implemented in proxy-go
type ObserverFacade struct {
	process.Processor
	facade.TransactionProcessor
	facade.BlockProcessor
}

func (facade *ObserverFacade) ComputeShardId(pubKey []byte) uint32 {
	return facade.GetShardCoordinator().ComputeId(pubKey)
}
