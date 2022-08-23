package services

import (
	"encoding/hex"

	"github.com/ElrondNetwork/elrond-proxy-go/data"
	"github.com/ElrondNetwork/rosetta/common"
	"github.com/ElrondNetwork/rosetta/server/resources"
	"github.com/coinbase/rosetta-sdk-go/types"
)

func blockToIdentifier(block *data.Block) *types.BlockIdentifier {
	return &types.BlockIdentifier{
		Index: int64(block.Nonce),
		Hash:  block.Hash,
	}
}

func accountBlockCoordinatesToIdentifier(block resources.BlockCoordinates) *types.BlockIdentifier {
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

func blockIdentifierToAccountQueryOptions(identifier *types.PartialBlockIdentifier) (resources.AccountQueryOptions, error) {
	if identifier == nil {
		return resources.AccountQueryOptions{OnFinalBlock: true}, nil
	}

	if identifier.Index != nil {
		blockNonce := common.OptionalUint64{Value: uint64(*identifier.Index), HasValue: true}
		return resources.AccountQueryOptions{BlockNonce: blockNonce}, nil
	}

	if identifier.Hash != nil {
		decodedHash, err := hex.DecodeString(*identifier.Hash)
		if err != nil {
			return resources.AccountQueryOptions{}, err
		}

		return resources.AccountQueryOptions{BlockHash: decodedHash}, nil
	}

	return resources.AccountQueryOptions{OnFinalBlock: true}, nil
}

func addressToAccountIdentifier(address string) *types.AccountIdentifier {
	return &types.AccountIdentifier{
		Address: address,
	}
}

func hashToTransactionIdentifier(hash string) *types.TransactionIdentifier {
	return &types.TransactionIdentifier{
		Hash: hash,
	}
}

func indexToOperationIdentifier(index int) *types.OperationIdentifier {
	return &types.OperationIdentifier{Index: int64(index)}
}

func timestampInMilliseconds(timestamp int64) int64 {
	return timestamp * 1000
}
