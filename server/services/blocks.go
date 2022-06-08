package services

import (
	"github.com/ElrondNetwork/elrond-proxy-go/data"
	"github.com/ElrondNetwork/rosetta/server/resources"
	"github.com/coinbase/rosetta-sdk-go/types"
)

func blockToIdentifier(block *data.Block) *types.BlockIdentifier {
	return &types.BlockIdentifier{
		Index: int64(block.Nonce),
		Hash:  block.Hash,
	}
}

func blockSummaryToIdentifier(blockSummary *resources.BlockSummary) *types.BlockIdentifier {
	return &types.BlockIdentifier{
		Index: int64(blockSummary.Nonce),
		Hash:  blockSummary.Hash,
	}
}
