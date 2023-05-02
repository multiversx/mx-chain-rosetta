package services

import (
	"encoding/hex"

	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/multiversx/mx-chain-core-go/data/api"
	"github.com/multiversx/mx-chain-rosetta/server/resources"
)

func blockToIdentifier(block *api.Block) *types.BlockIdentifier {
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
		return resources.NewAccountQueryOptionsOnFinalBlock(), nil
	}

	if identifier.Index != nil {
		blockNonce := uint64(*identifier.Index)
		return resources.NewAccountQueryOptionsWithBlockNonce(blockNonce), nil
	}

	if identifier.Hash != nil {
		decodedHash, err := hex.DecodeString(*identifier.Hash)
		if err != nil {
			return resources.AccountQueryOptions{}, err
		}

		return resources.NewAccountQueryOptionsWithBlockHash(decodedHash), nil
	}

	return resources.NewAccountQueryOptionsOnFinalBlock(), nil
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
