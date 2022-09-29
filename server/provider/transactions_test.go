package provider

import (
	"testing"

	"github.com/ElrondNetwork/elrond-go-core/data/transaction"
	"github.com/stretchr/testify/require"
)

func TestDiscardTransactions(t *testing.T) {
	transactions := []*transaction.ApiTransactionResult{
		{Hash: "aaaa"},
		{Hash: "bbbb"},
		{Hash: "aabb"},
		{Hash: "abba"},
	}

	txsHashesToDiscard := map[string]struct{}{"aabb": {}, "abba": {}}
	result := discardTransactions(transactions, txsHashesToDiscard)
	require.Len(t, result, 2)
	require.Equal(t, "aaaa", result[0].Hash)
	require.Equal(t, "bbbb", result[1].Hash)
}

func TestFilterTransactions(t *testing.T) {
	transactions := []*transaction.ApiTransactionResult{
		{Hash: "aaaa"},
		{Hash: "bbbb"},
		{Hash: "aabb"},
		{Hash: "abba"},
	}

	txsHashesToKeep := map[string]struct{}{"aabb": {}, "abba": {}}
	result := filterTransactions(transactions, txsHashesToKeep)
	require.Len(t, result, 2)
	require.Equal(t, "aabb", result[0].Hash)
	require.Equal(t, "abba", result[1].Hash)
}

func TestDeduplicateTransactions(t *testing.T) {
	transactions := []*transaction.ApiTransactionResult{
		{Hash: "aaaa"},
		{Hash: "bbbb"},
		{Hash: "aabb"},
		{Hash: "bbbb"},
		{Hash: "aaaa"},
	}

	result := deduplicateTransactions(transactions)
	require.Len(t, result, 3)
	require.Equal(t, "aaaa", result[0].Hash)
	require.Equal(t, "bbbb", result[1].Hash)
	require.Equal(t, "aabb", result[2].Hash)
}
