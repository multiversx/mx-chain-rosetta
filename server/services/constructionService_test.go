package services

import (
	"context"
	"testing"

	"github.com/ElrondNetwork/elrond-proxy-go/data"
	"github.com/ElrondNetwork/rosetta/server/resources"
	"github.com/ElrondNetwork/rosetta/testscommon"
	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/stretchr/testify/require"
)

func TestConstructionService_ConstructionPreprocess(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	extension := newNetworkProviderExtension(networkProvider)
	service := NewConstructionService(networkProvider)

	operations := []*types.Operation{
		{
			OperationIdentifier: indexToOperationIdentifier(0),
			Type:                opTransfer,
			Account:             addressToAccountIdentifier(testscommon.TestAddressAlice),
			Amount:              extension.valueToNativeAmount("-1234"),
		},
		{
			OperationIdentifier: indexToOperationIdentifier(1),
			Type:                opTransfer,
			Account:             addressToAccountIdentifier(testscommon.TestAddressBob),
			Amount:              extension.valueToNativeAmount("1234"),
		},
	}

	feeMultiplier := 1.1

	response, err := service.ConstructionPreprocess(context.Background(),
		&types.ConstructionPreprocessRequest{
			Operations: operations,
			MaxFee: []*types.Amount{
				extension.valueToNativeAmount("123"),
			},
			SuggestedFeeMultiplier: &feeMultiplier,
			Metadata: objectsMap{
				"gasPrice": 1000000000,
				"gasLimit": 50000,
				"data":     "hello",
			},
		},
	)
	require.Nil(t, err)
	require.Equal(t, map[string]interface{}{
		"receiver":      testscommon.TestAddressBob,
		"sender":        testscommon.TestAddressAlice,
		"gasPrice":      1000000000,
		"gasLimit":      50000,
		"feeMultiplier": feeMultiplier,
		"data":          "hello",
		"value":         "1234",
		"maxFee":        "123",
		"type":          opTransfer,
	}, response.Options)
}

func TestConstructionService_ConstructionMetadata(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	networkProvider.MockNetworkConfig.NetworkID = "T"
	networkProvider.MockAccountsByAddress[testscommon.TestAddressAlice] = &resources.Account{
		Address: testscommon.TestAddressAlice,
		Nonce:   42,
	}

	service := NewConstructionService(networkProvider)

	options := map[string]interface{}{
		"receiver":      testscommon.TestAddressBob,
		"sender":        testscommon.TestAddressAlice,
		"gasPrice":      uint64(1000000000),
		"gasLimit":      uint64(57500),
		"feeMultiplier": 1.1,
		"data":          "hello",
		"value":         "1234",
		"maxFee":        "1",
		"type":          opTransfer,
	}
	response, err := service.ConstructionMetadata(context.Background(),
		&types.ConstructionMetadataRequest{
			Options: options,
		},
	)

	require.Nil(t, err)
	require.Equal(t, "63250000000000", response.SuggestedFee[0].Value)

	expectedMetadata := map[string]interface{}{
		"receiver": testscommon.TestAddressBob,
		"sender":   testscommon.TestAddressAlice,
		"chainID":  "T",
		"version":  1,
		"data":     []byte("hello"),
		"value":    "1234",
		"nonce":    uint64(42),
		"gasPrice": uint64(1100000000),
		"gasLimit": uint64(57500),
	}

	require.Equal(t, expectedMetadata, response.Metadata)
}

func TestConstructionService_ConstructionPayloads(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	networkProvider.MockNetworkConfig.NetworkID = "T"
	networkProvider.MockAccountsByAddress[testscommon.TestAddressAlice] = &resources.Account{
		Address: testscommon.TestAddressAlice,
		Nonce:   42,
	}

	extension := newNetworkProviderExtension(networkProvider)
	service := NewConstructionService(networkProvider)

	metadata := map[string]interface{}{
		"receiver": testscommon.TestAddressBob,
		"sender":   testscommon.TestAddressAlice,
		"chainID":  "T",
		"version":  1,
		"data":     []byte("hello"),
		"value":    "1234",
		"nonce":    uint64(42),
		"gasPrice": uint64(1100000000),
		"gasLimit": uint64(57500),
	}

	operations := []*types.Operation{
		{
			OperationIdentifier: indexToOperationIdentifier(0),
			Type:                opTransfer,
			Account:             addressToAccountIdentifier(testscommon.TestAddressAlice),
			Amount:              extension.valueToNativeAmount("-1234"),
		},
		{
			OperationIdentifier: indexToOperationIdentifier(1),
			Type:                opTransfer,
			Account:             addressToAccountIdentifier(testscommon.TestAddressBob),
			Amount:              extension.valueToNativeAmount("1234"),
		},
	}

	response, err := service.ConstructionPayloads(context.Background(),
		&types.ConstructionPayloadsRequest{
			Operations: operations,
			Metadata:   metadata,
		},
	)

	unsignedTx := `{"nonce":42,"value":"1234","receiver":"erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx","sender":"erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th","gasPrice":1100000000,"gasLimit":57500,"data":"aGVsbG8=","chainID":"T","version":1}`
	unsignedTxBytes := []byte(unsignedTx)

	require.Nil(t, err)
	firstPayload := response.Payloads[0]
	require.Equal(t, unsignedTx, response.UnsignedTransaction)
	require.Equal(t, unsignedTxBytes, firstPayload.Bytes)
	require.Equal(t, testscommon.TestAddressAlice, firstPayload.AccountIdentifier.Address)
	require.Equal(t, types.Ed25519, firstPayload.SignatureType)
}

func TestConstructionService_ConstructionParse(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	networkProvider.MockNetworkConfig.NetworkID = "T"

	extension := newNetworkProviderExtension(networkProvider)
	service := NewConstructionService(networkProvider)

	unsignedTx := `{"nonce":42,"value":"1234","receiver":"erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx","sender":"erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th","gasPrice":1100000000,"gasLimit":57500,"data":"aGVsbG8=","chainID":"T","version":1}`

	operations := []*types.Operation{
		{
			OperationIdentifier: indexToOperationIdentifier(0),
			Type:                opTransfer,
			Account:             addressToAccountIdentifier(testscommon.TestAddressAlice),
			Amount:              extension.valueToNativeAmount("-1234"),
		},
		{
			OperationIdentifier: indexToOperationIdentifier(1),
			Type:                opTransfer,
			Account:             addressToAccountIdentifier(testscommon.TestAddressBob),
			Amount:              extension.valueToNativeAmount("1234"),
		},
	}

	response, err := service.ConstructionParse(context.Background(),
		&types.ConstructionParseRequest{
			Signed:      false,
			Transaction: unsignedTx,
		},
	)
	require.Nil(t, err)
	require.Equal(t, operations, response.Operations)
	require.Nil(t, response.AccountIdentifierSigners)
}

func TestConstructionService_ConstructionCombine(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	networkProvider.MockNetworkConfig.NetworkID = "T"

	service := NewConstructionService(networkProvider)

	unsignedTx := `{"nonce":42,"value":"1234","receiver":"erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx","sender":"erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th","gasPrice":1100000000,"gasLimit":57500,"data":"aGVsbG8=","chainID":"T","version":1}`

	response, err := service.ConstructionCombine(context.Background(),
		&types.ConstructionCombineRequest{
			UnsignedTransaction: unsignedTx,
			Signatures: []*types.Signature{
				{
					Bytes: []byte{0xaa, 0xbb},
				},
			},
		},
	)

	signedTx := `{"nonce":42,"value":"1234","receiver":"erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx","sender":"erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th","gasPrice":1100000000,"gasLimit":57500,"data":"aGVsbG8=","signature":"aabb","chainID":"T","version":1}`
	require.Nil(t, err)
	require.Equal(t, signedTx, response.SignedTransaction)
}

func TestConstructionService_ConstructionDerive(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	service := NewConstructionService(networkProvider)

	response, err := service.ConstructionDerive(context.Background(),
		&types.ConstructionDeriveRequest{
			PublicKey: &types.PublicKey{
				Bytes:     testscommon.TestPubKeyAlice,
				CurveType: types.Edwards25519,
			},
		},
	)

	require.Nil(t, err)
	require.Equal(t, testscommon.TestAddressAlice, response.AccountIdentifier.Address)
}

func TestConstructionService_ConstructionHash(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	networkProvider.MockComputedTransactionHash = "aaaa"
	service := NewConstructionService(networkProvider)

	signedTx := `{"nonce":42,"value":"1234","receiver":"erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx","sender":"erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th","gasPrice":1100000000,"gasLimit":57500,"data":"aGVsbG8=","signature":"aabb","chainID":"T","version":1}`

	response, err := service.ConstructionHash(context.Background(),
		&types.ConstructionHashRequest{
			SignedTransaction: signedTx,
		},
	)
	require.Nil(t, err)
	require.Equal(t, "aaaa", response.TransactionIdentifier.Hash)
}

func TestConstructionService_ConstructionSubmit(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()

	var calledWithTransaction *data.Transaction
	networkProvider.SendTransactionCalled = func(tx *data.Transaction) (string, error) {
		calledWithTransaction = tx
		return "aaaa", nil
	}

	service := NewConstructionService(networkProvider)

	signedTx := `{"nonce":42,"value":"1234","receiver":"erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx","sender":"erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th","gasPrice":1100000000,"gasLimit":57500,"data":"aGVsbG8=","signature":"aabb","chainID":"T","version":1}`

	response, err := service.ConstructionSubmit(context.Background(),
		&types.ConstructionSubmitRequest{
			SignedTransaction: signedTx,
		},
	)
	require.Nil(t, err)
	require.Equal(t, "aaaa", response.TransactionIdentifier.Hash)
	require.Equal(t, "T", calledWithTransaction.ChainID)
	require.Equal(t, uint64(42), calledWithTransaction.Nonce)
}

func TestConstructionService_CreateOperationsFromPreparedTx(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	extension := newNetworkProviderExtension(networkProvider)
	service := NewConstructionService(networkProvider).(*constructionService)

	preparedTx := &data.Transaction{
		Value:    "12345",
		Receiver: testscommon.TestAddressBob,
		Sender:   testscommon.TestAddressAlice,
	}

	expectedOperations := []*types.Operation{
		{
			OperationIdentifier: indexToOperationIdentifier(0),
			Type:                opTransfer,
			Account:             addressToAccountIdentifier(testscommon.TestAddressAlice),
			Amount:              extension.valueToNativeAmount("-12345"),
		},
		{
			OperationIdentifier: indexToOperationIdentifier(1),
			Type:                opTransfer,
			Account:             addressToAccountIdentifier(testscommon.TestAddressBob),
			Amount:              extension.valueToNativeAmount("12345"),
		},
	}

	operations := service.createOperationsFromPreparedTx(preparedTx)
	require.Equal(t, expectedOperations, operations)
}

func TestConstructionService_PrepareConstructionOptions(t *testing.T) {
	t.Parallel()

	networkProvider := testscommon.NewNetworkProviderMock()
	extension := newNetworkProviderExtension(networkProvider)
	service := NewConstructionService(networkProvider).(*constructionService)

	t.Run("two operations, no metadata", func(t *testing.T) {
		t.Parallel()

		operations := []*types.Operation{
			{
				OperationIdentifier: indexToOperationIdentifier(0),
				Type:                opTransfer,
				Account:             addressToAccountIdentifier(testscommon.TestAddressAlice),
				Amount:              extension.valueToNativeAmount("-12345"),
			},
			{
				OperationIdentifier: indexToOperationIdentifier(1),
				Type:                opTransfer,
				Account:             addressToAccountIdentifier(testscommon.TestAddressBob),
				Amount:              extension.valueToNativeAmount("12345"),
			},
		}

		metadata := make(objectsMap)

		options, err := service.prepareConstructionOptions(operations, metadata)
		require.Nil(t, err)
		require.Equal(t, opTransfer, options["type"])
		require.Equal(t, testscommon.TestAddressAlice, options["sender"])
		require.Equal(t, testscommon.TestAddressBob, options["receiver"])
		require.Equal(t, "12345", options["value"])
	})

	t.Run("one operation, with metadata having: receiver", func(t *testing.T) {
		t.Parallel()

		operations := []*types.Operation{
			{
				OperationIdentifier: indexToOperationIdentifier(0),
				Type:                opTransfer,
				Account:             addressToAccountIdentifier(testscommon.TestAddressAlice),
				Amount:              extension.valueToNativeAmount("-12345"),
			},
		}

		metadata := make(objectsMap)
		metadata["receiver"] = testscommon.TestAddressBob

		options, err := service.prepareConstructionOptions(operations, metadata)
		require.Nil(t, err)
		require.Equal(t, opTransfer, options["type"])
		require.Equal(t, testscommon.TestAddressAlice, options["sender"])
		require.Equal(t, testscommon.TestAddressBob, options["receiver"])
		require.Equal(t, "12345", options["value"])
	})

	t.Run("one operation, with metadata having: receiver, value", func(t *testing.T) {
		t.Parallel()

		operations := []*types.Operation{
			{
				OperationIdentifier: indexToOperationIdentifier(0),
				Type:                opTransfer,
				Account:             addressToAccountIdentifier(testscommon.TestAddressAlice),
				Amount:              extension.valueToNativeAmount("ignored"),
			},
		}

		metadata := make(objectsMap)
		metadata["receiver"] = testscommon.TestAddressBob
		metadata["value"] = "12345"

		options, err := service.prepareConstructionOptions(operations, metadata)
		require.Nil(t, err)
		require.Equal(t, opTransfer, options["type"])
		require.Equal(t, testscommon.TestAddressAlice, options["sender"])
		require.Equal(t, testscommon.TestAddressBob, options["receiver"])
		require.Equal(t, "12345", options["value"])
	})

	t.Run("one operation, with missing metadata: receiver", func(t *testing.T) {
		t.Parallel()

		operations := []*types.Operation{
			{
				OperationIdentifier: indexToOperationIdentifier(0),
				Type:                opTransfer,
				Account:             addressToAccountIdentifier(testscommon.TestAddressAlice),
				Amount:              extension.valueToNativeAmount("ignored"),
			},
		}

		metadata := make(objectsMap)

		options, err := service.prepareConstructionOptions(operations, metadata)
		require.ErrorContains(t, err, "cannot prepare transaction receiver")
		require.Nil(t, options)
	})
}
