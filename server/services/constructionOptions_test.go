package services

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConstructionOptions_CoalesceGasLimit(t *testing.T) {
	t.Parallel()

	options := &constructionOptions{
		GasLimit: 80000,
	}
	require.Equal(t, uint64(80000), options.coalesceGasLimit(50000))

	options = &constructionOptions{}
	require.Equal(t, uint64(50000), options.coalesceGasLimit(50000))
}

func TestConstructionOptions_CoalesceGasPrice(t *testing.T) {
	t.Parallel()

	options := &constructionOptions{
		GasPrice: 1000000001,
	}
	require.Equal(t, uint64(1000000001), options.coalesceGasPrice(1000000000))

	options = &constructionOptions{}
	require.Equal(t, uint64(1000000000), options.coalesceGasLimit(1000000000))
}

func TestConstructionOptions_Validate(t *testing.T) {
	t.Parallel()

	require.ErrorContains(t, (&constructionOptions{}).validate("XeGLD"), "missing option 'sender'")

	require.ErrorContains(t, (&constructionOptions{
		Sender: "alice",
	}).validate("XeGLD"), "missing option 'receiver'")

	require.ErrorContains(t, (&constructionOptions{
		Sender:   "alice",
		Receiver: "bob",
	}).validate("XeGLD"), "missing option 'amount'")

	require.ErrorContains(t, (&constructionOptions{
		Sender:   "alice",
		Receiver: "bob",
		Amount:   "1234",
	}).validate("XeGLD"), "missing option 'currencySymbol'")

	require.ErrorContains(t, (&constructionOptions{
		Sender:         "alice",
		Receiver:       "bob",
		Amount:         "1234",
		CurrencySymbol: "FOO",
		Data:           []byte("hello"),
	}).validate("XeGLD"), "for custom currencies, option 'data' must be empty")

	require.Nil(t, (&constructionOptions{
		Sender:         "alice",
		Receiver:       "bob",
		Amount:         "1234",
		CurrencySymbol: "XeGLD",
	}).validate("XeGLD"))
}
