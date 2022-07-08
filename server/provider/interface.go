package provider

import (
	"github.com/ElrondNetwork/elrond-proxy-go/facade"
	"github.com/ElrondNetwork/elrond-proxy-go/process"
)

type baseProcessor interface {
	process.Processor
}

type accountProcessor interface {
	facade.AccountProcessor
}

type transactionProcessor interface {
	facade.TransactionProcessor
}

type blockProcessor interface {
	facade.BlockProcessor
}
