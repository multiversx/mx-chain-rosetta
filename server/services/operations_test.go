package services

import (
	"testing"

	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/stretchr/testify/require"
)

func TestFilterOperationsByAddress(t *testing.T) {
	operations := []*types.Operation{
		{
			Account: &types.AccountIdentifier{
				Address: "erd1alice",
			},
		},
		{
			Account: &types.AccountIdentifier{
				Address: "erd1bob",
			},
		},
		{
			Account: &types.AccountIdentifier{
				Address: "erd1carol",
			},
		},
	}

	predicate := func(address string) (bool, error) {
		return address == "erd1alice" || address == "erd1bob", nil
	}

	filtered, err := filterOperationsByAddress(operations, predicate)

	require.NoError(t, err)
	require.Len(t, filtered, 2)
	require.Equal(t, "erd1alice", filtered[0].Account.Address)
	require.Equal(t, "erd1bob", filtered[1].Account.Address)
}

func TestFilterOutOperationsWithZeroAmount(t *testing.T) {
	operations := []*types.Operation{
		{
			Amount: &types.Amount{
				Value: "0",
			},
		},
		{
			Amount: &types.Amount{
				Value: "1",
			},
		},
		{
			Amount: &types.Amount{
				Value: "42",
			},
		},
	}

	filtered := filterOutOperationsWithZeroAmount(operations)

	require.Len(t, filtered, 2)
	require.Equal(t, "1", filtered[0].Amount.Value)
	require.Equal(t, "42", filtered[1].Amount.Value)
}
