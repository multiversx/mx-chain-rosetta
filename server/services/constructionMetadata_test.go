package services

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConstructionMetadata_ToTransactionJson(t *testing.T) {
	t.Parallel()

	options := &constructionMetadata{
		Sender:         "alice",
		Receiver:       "bob",
		Nonce:          42,
		Amount:         "1234",
		CurrencySymbol: "XeGLD",
		GasLimit:       80000,
		GasPrice:       1000000000,
		Data:           []byte("hello"),
		ChainID:        "T",
		Version:        1,
	}

	expectedJson := `{"nonce":42,"value":"1234","receiver":"bob","sender":"alice","gasPrice":1000000000,"gasLimit":80000,"data":"aGVsbG8=","chainID":"T","version":1}`
	actualJson, err := options.toTransactionJson("XeGLD")
	require.Nil(t, err)
	require.Equal(t, expectedJson, string(actualJson))
}

func TestConstructionMetadata_Validate(t *testing.T) {
	t.Parallel()

	require.ErrorContains(t, (&constructionMetadata{}).validate(), "missing metadata: 'sender'")

	require.ErrorContains(t, (&constructionMetadata{
		Sender: "alice",
	}).validate(), "missing metadata: 'receiver'")

	require.ErrorContains(t, (&constructionMetadata{
		Sender:   "alice",
		Receiver: "bob",
	}).validate(), "missing metadata: 'gasLimit'")

	require.ErrorContains(t, (&constructionMetadata{
		Sender:   "alice",
		Receiver: "bob",
		GasLimit: 50000,
	}).validate(), "missing metadata: 'gasPrice'")

	require.ErrorContains(t, (&constructionMetadata{
		Sender:   "alice",
		Receiver: "bob",
		GasLimit: 50000,
		GasPrice: 1000000000,
	}).validate(), "bad metadata: unexpected 'version' 0")

	require.ErrorContains(t, (&constructionMetadata{
		Sender:   "alice",
		Receiver: "bob",
		GasLimit: 50000,
		GasPrice: 1000000000,
		Version:  42,
	}).validate(), "bad metadata: unexpected 'version' 42")

	require.ErrorContains(t, (&constructionMetadata{
		Sender:   "alice",
		Receiver: "bob",
		GasLimit: 50000,
		GasPrice: 1000000000,
		Version:  1,
	}).validate(), "missing metadata: 'chainID'")

	require.Nil(t, (&constructionMetadata{
		Sender:   "alice",
		Receiver: "bob",
		GasLimit: 50000,
		GasPrice: 1000000000,
		Version:  1,
		ChainID:  "T",
	}).validate())
}
