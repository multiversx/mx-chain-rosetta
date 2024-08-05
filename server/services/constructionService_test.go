package services

import (
	"context"
	"testing"

	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/multiversx/mx-chain-proxy-go/data"
	"github.com/multiversx/mx-chain-rosetta/server/resources"
	"github.com/multiversx/mx-chain-rosetta/testscommon"
	"github.com/stretchr/testify/require"
)

func TestConstructionService_ConstructionPreprocess(t *testing.T) {
	t.Parallel()

	networkProvider := testscommon.NewNetworkProviderMock()
	extension := newNetworkProviderExtension(networkProvider)
	service := NewConstructionService(networkProvider)

	t.Run("with minimal (empty) 'metadata', 'options' being inferred from 'operations'", func(t *testing.T) {
		t.Parallel()

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

		response, err := service.ConstructionPreprocess(context.Background(),
			&types.ConstructionPreprocessRequest{
				Operations: operations,
				Metadata:   objectsMap{},
			},
		)

		expectedOptions := &constructionOptions{
			Sender:         testscommon.TestAddressAlice,
			Receiver:       testscommon.TestAddressBob,
			Amount:         "1234",
			CurrencySymbol: "XeGLD",
		}

		actualOptions := &constructionOptions{}
		_ = fromObjectsMap(response.Options, actualOptions)

		require.Nil(t, err)
		require.Equal(t, expectedOptions, actualOptions)
	})

	t.Run("with one operation, with metadata having: 'receiver'", func(t *testing.T) {
		t.Parallel()

		operations := []*types.Operation{
			{
				OperationIdentifier: indexToOperationIdentifier(0),
				Type:                opTransfer,
				Account:             addressToAccountIdentifier(testscommon.TestAddressAlice),
				Amount:              extension.valueToNativeAmount("-1234"),
			},
		}

		response, errTyped := service.ConstructionPreprocess(context.Background(),
			&types.ConstructionPreprocessRequest{
				Operations: operations,
				Metadata: objectsMap{
					"receiver": testscommon.TestAddressBob,
				},
			},
		)

		require.Nil(t, errTyped)

		expectedOptions := &constructionOptions{
			Sender:         testscommon.TestAddressAlice,
			Receiver:       testscommon.TestAddressBob,
			Amount:         "1234",
			CurrencySymbol: "XeGLD",
		}

		actualOptions := &constructionOptions{}
		err := fromObjectsMap(response.Options, actualOptions)
		require.NoError(t, err)
		require.Equal(t, expectedOptions, actualOptions)
	})

	t.Run("with one operation, and metadata having: 'receiver', 'amount'", func(t *testing.T) {
		t.Parallel()

		operations := []*types.Operation{
			{
				OperationIdentifier: indexToOperationIdentifier(0),
				Type:                opTransfer,
				Account:             addressToAccountIdentifier(testscommon.TestAddressAlice),
				Amount:              extension.valueToNativeAmount("ignored"),
			},
		}

		response, errTyped := service.ConstructionPreprocess(context.Background(),
			&types.ConstructionPreprocessRequest{
				Operations: operations,
				Metadata: objectsMap{
					"receiver": testscommon.TestAddressBob,
					"amount":   "1234",
				},
			},
		)

		require.Nil(t, errTyped)

		expectedOptions := &constructionOptions{
			Sender:         testscommon.TestAddressAlice,
			Receiver:       testscommon.TestAddressBob,
			Amount:         "1234",
			CurrencySymbol: "XeGLD",
		}

		actualOptions := &constructionOptions{}
		err := fromObjectsMap(response.Options, actualOptions)
		require.NoError(t, err)
		require.Equal(t, expectedOptions, actualOptions)
	})

	t.Run("with one operation, and missing metadata: 'receiver'", func(t *testing.T) {
		t.Parallel()

		operations := []*types.Operation{
			{
				OperationIdentifier: indexToOperationIdentifier(0),
				Type:                opTransfer,
				Account:             addressToAccountIdentifier(testscommon.TestAddressAlice),
				Amount:              extension.valueToNativeAmount("1234"),
			},
		}

		response, err := service.ConstructionPreprocess(context.Background(),
			&types.ConstructionPreprocessRequest{
				Operations: operations,
				Metadata:   objectsMap{},
			},
		)

		require.Equal(t, int32(ErrConstruction), err.Code)
		require.Nil(t, response)
	})

	t.Run("with maximal 'metadata', without 'operations'", func(t *testing.T) {
		t.Parallel()

		response, errTyped := service.ConstructionPreprocess(context.Background(),
			&types.ConstructionPreprocessRequest{
				Metadata: objectsMap{
					"sender":         testscommon.TestAddressAlice,
					"receiver":       testscommon.TestAddressBob,
					"amount":         "1234",
					"currencySymbol": "XeGLD",
					"gasLimit":       70000,
					"gasPrice":       1000000000,
					"data":           []byte("hello"),
				},
			},
		)

		require.Nil(t, errTyped)

		expectedOptions := &constructionOptions{
			Sender:         testscommon.TestAddressAlice,
			Receiver:       testscommon.TestAddressBob,
			Amount:         "1234",
			CurrencySymbol: "XeGLD",
			GasLimit:       70000,
			GasPrice:       1000000000,
			Data:           []byte("hello"),
		}

		actualOptions := &constructionOptions{}
		err := fromObjectsMap(response.Options, actualOptions)
		require.NoError(t, err)
		require.Equal(t, expectedOptions, actualOptions)
	})

	t.Run("with incomplete 'metadata', without 'operations'", func(t *testing.T) {
		t.Parallel()

		response, errTyped := service.ConstructionPreprocess(context.Background(),
			&types.ConstructionPreprocessRequest{
				Metadata: objectsMap{
					"sender":   testscommon.TestAddressAlice,
					"receiver": testscommon.TestAddressBob,
				},
			},
		)

		require.Equal(t, int32(ErrConstruction), errTyped.Code)
		require.Nil(t, response)
	})
}

func TestConstructionService_ConstructionMetadata(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	networkProvider.MockAccountsByAddress[testscommon.TestAddressAlice] = &resources.Account{
		Address: testscommon.TestAddressAlice,
		Nonce:   42,
	}

	service := NewConstructionService(networkProvider)

	t.Run("with native currency, with explicitly providing gas limit and price", func(t *testing.T) {
		t.Parallel()

		response, errTyped := service.ConstructionMetadata(context.Background(),
			&types.ConstructionMetadataRequest{
				Options: objectsMap{
					"receiver":       testscommon.TestAddressBob,
					"sender":         testscommon.TestAddressAlice,
					"amount":         "1234",
					"currencySymbol": "XeGLD",
					"gasLimit":       100000,
					"gasPrice":       1500000000,
				},
			},
		)

		require.Nil(t, errTyped)

		expectedMetadata := &constructionMetadata{
			Sender:         testscommon.TestAddressAlice,
			Receiver:       testscommon.TestAddressBob,
			Nonce:          42,
			Amount:         "1234",
			CurrencySymbol: "XeGLD",
			GasLimit:       100000,
			GasPrice:       1500000000,
			ChainID:        "T",
			Version:        1,
		}

		actualMetadata := &constructionMetadata{}
		err := fromObjectsMap(response.Metadata, actualMetadata)
		require.NoError(t, err)

		// We are suggesting the fee by considering the refund
		require.Equal(t, "75000000000000", response.SuggestedFee[0].Value)
		require.Equal(t, expectedMetadata, actualMetadata)
	})

	t.Run("with native currency, with explicitly providing gas limit and price (with data)", func(t *testing.T) {
		t.Parallel()

		response, errTyped := service.ConstructionMetadata(context.Background(),
			&types.ConstructionMetadataRequest{
				Options: objectsMap{
					"receiver":       testscommon.TestAddressBob,
					"sender":         testscommon.TestAddressAlice,
					"amount":         "1234",
					"currencySymbol": "XeGLD",
					"gasLimit":       70000,
					"gasPrice":       1500000000,
					"data":           []byte("hello"),
				},
			},
		)

		require.Nil(t, errTyped)

		expectedMetadata := &constructionMetadata{
			Sender:         testscommon.TestAddressAlice,
			Receiver:       testscommon.TestAddressBob,
			Nonce:          42,
			Amount:         "1234",
			CurrencySymbol: "XeGLD",
			GasLimit:       70000,
			GasPrice:       1500000000,
			Data:           []byte("hello"),
			ChainID:        "T",
			Version:        1,
		}

		actualMetadata := &constructionMetadata{}
		err := fromObjectsMap(response.Metadata, actualMetadata)
		require.NoError(t, err)

		// We are suggesting the fee by considering the refund
		require.Equal(t, "86250000000000", response.SuggestedFee[0].Value)
		require.Equal(t, expectedMetadata, actualMetadata)
	})

	t.Run("with native currency, with explicitly providing gas limit and price (but too little gas)", func(t *testing.T) {
		t.Parallel()

		_, errTyped := service.ConstructionMetadata(context.Background(),
			&types.ConstructionMetadataRequest{
				Options: objectsMap{
					"receiver":       testscommon.TestAddressBob,
					"sender":         testscommon.TestAddressAlice,
					"amount":         "1234",
					"currencySymbol": "XeGLD",
					"gasLimit":       50000,
					"gasPrice":       1000000000,
					"data":           []byte("hello"),
				},
			},
		)

		require.Equal(t, ErrInsufficientGasLimit, errCode(errTyped.Code))
	})

	t.Run("with native currency, without providing gas limit and price", func(t *testing.T) {
		t.Parallel()

		response, errTyped := service.ConstructionMetadata(context.Background(),
			&types.ConstructionMetadataRequest{
				Options: objectsMap{
					"receiver":       testscommon.TestAddressBob,
					"sender":         testscommon.TestAddressAlice,
					"amount":         "1234",
					"currencySymbol": "XeGLD",
					"data":           []byte("hello"),
				},
			},
		)

		require.Nil(t, errTyped)

		expectedMetadata := &constructionMetadata{
			Sender:         testscommon.TestAddressAlice,
			Receiver:       testscommon.TestAddressBob,
			Nonce:          42,
			Amount:         "1234",
			CurrencySymbol: "XeGLD",
			GasLimit:       57500,
			GasPrice:       1000000000,
			Data:           []byte("hello"),
			ChainID:        "T",
			Version:        1,
		}

		actualMetadata := &constructionMetadata{}
		err := fromObjectsMap(response.Metadata, actualMetadata)
		require.NoError(t, err)

		require.Equal(t, "57500000000000", response.SuggestedFee[0].Value)
		require.Equal(t, expectedMetadata, actualMetadata)
	})

	t.Run("with custom currency, with explicitly providing gas limit and price (but too little gas)", func(t *testing.T) {
		t.Parallel()

		_, errTyped := service.ConstructionMetadata(context.Background(),
			&types.ConstructionMetadataRequest{
				Options: objectsMap{
					"receiver":       testscommon.TestAddressBob,
					"sender":         testscommon.TestAddressAlice,
					"amount":         "1234",
					"currencySymbol": "TEST-abcdef",
					"gasLimit":       50000,
					"gasPrice":       1000000000,
				},
			},
		)

		require.Equal(t, ErrInsufficientGasLimit, errCode(errTyped.Code))
	})

	t.Run("with custom currency, with explicitly providing gas limit and price", func(t *testing.T) {
		t.Parallel()

		response, errTyped := service.ConstructionMetadata(context.Background(),
			&types.ConstructionMetadataRequest{
				Options: objectsMap{
					"receiver":       testscommon.TestAddressBob,
					"sender":         testscommon.TestAddressAlice,
					"amount":         "1234",
					"currencySymbol": "TEST-abcdef",
					"gasLimit":       500000,
					"gasPrice":       1000000000,
				},
			},
		)

		require.Nil(t, errTyped)

		expectedMetadata := &constructionMetadata{
			Sender:         testscommon.TestAddressAlice,
			Receiver:       testscommon.TestAddressBob,
			Nonce:          42,
			Amount:         "0",
			CurrencySymbol: "TEST-abcdef",
			GasLimit:       500000,
			GasPrice:       1000000000,
			Data:           []byte("ESDTTransfer@544553542d616263646566@04d2"),
			ChainID:        "T",
			Version:        1,
		}

		actualMetadata := &constructionMetadata{}
		err := fromObjectsMap(response.Metadata, actualMetadata)
		require.NoError(t, err)

		// We are suggesting the fee by considering the refund
		require.Equal(t, "112000000000000", response.SuggestedFee[0].Value)
		require.Equal(t, expectedMetadata, actualMetadata)
	})

	t.Run("with custom currency, without providing gas limit and price", func(t *testing.T) {
		t.Parallel()

		response, errTyped := service.ConstructionMetadata(context.Background(),
			&types.ConstructionMetadataRequest{
				Options: objectsMap{
					"receiver":       testscommon.TestAddressBob,
					"sender":         testscommon.TestAddressAlice,
					"amount":         "1234",
					"currencySymbol": "TEST-abcdef",
				},
			},
		)

		require.Nil(t, errTyped)

		expectedMetadata := &constructionMetadata{
			Sender:         testscommon.TestAddressAlice,
			Receiver:       testscommon.TestAddressBob,
			Nonce:          42,
			Amount:         "0",
			CurrencySymbol: "TEST-abcdef",
			GasLimit:       310000,
			GasPrice:       1000000000,
			Data:           []byte("ESDTTransfer@544553542d616263646566@04d2"),
			ChainID:        "T",
			Version:        1,
		}

		actualMetadata := &constructionMetadata{}
		err := fromObjectsMap(response.Metadata, actualMetadata)
		require.NoError(t, err)

		require.Equal(t, "112000000000000", response.SuggestedFee[0].Value)
		require.Equal(t, expectedMetadata, actualMetadata)
	})
}

func TestConstructionService_ConstructionPayloads(t *testing.T) {
	t.Parallel()

	networkProvider := testscommon.NewNetworkProviderMock()
	networkProvider.MockAccountsByAddress[testscommon.TestAddressAlice] = &resources.Account{
		Address: testscommon.TestAddressAlice,
		Nonce:   42,
	}

	service := NewConstructionService(networkProvider)

	response, errTyped := service.ConstructionPayloads(context.Background(),
		&types.ConstructionPayloadsRequest{
			Metadata: objectsMap{
				"sender":         testscommon.TestAddressAlice,
				"receiver":       testscommon.TestAddressBob,
				"nonce":          42,
				"amount":         "1234",
				"currencySymbol": "XeGLD",
				"gasLimit":       57500,
				"gasPrice":       1000000000,
				"data":           []byte("hello"),
				"chainID":        "T",
				"version":        1,
			},
		},
	)

	expectedTxJson := `{"nonce":42,"value":"1234","receiver":"erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx","sender":"erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th","gasPrice":1000000000,"gasLimit":57500,"data":"aGVsbG8=","chainID":"T","version":1}`

	require.Nil(t, errTyped)
	require.Len(t, response.Payloads, 1)
	require.Equal(t, expectedTxJson, response.UnsignedTransaction)
	require.Equal(t, []byte(expectedTxJson), response.Payloads[0].Bytes)
	require.Equal(t, testscommon.TestAddressAlice, response.Payloads[0].AccountIdentifier.Address)
	require.Equal(t, types.Ed25519, response.Payloads[0].SignatureType)
}

func TestConstructionService_ConstructionParse(t *testing.T) {
	t.Parallel()

	networkProvider := testscommon.NewNetworkProviderMock()
	extension := newNetworkProviderExtension(networkProvider)
	service := NewConstructionService(networkProvider)

	notSignedTx := `{"nonce":42,"value":"1234","receiver":"erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx","sender":"erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th","gasPrice":1100000000,"gasLimit":57500,"data":"aGVsbG8=","chainID":"T","version":1}`

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

	response, errTyped := service.ConstructionParse(context.Background(),
		&types.ConstructionParseRequest{
			Signed:      false,
			Transaction: notSignedTx,
		},
	)

	require.Nil(t, errTyped)
	require.Equal(t, operations, response.Operations)
	require.Nil(t, response.AccountIdentifierSigners)
}

func TestConstructionService_ConstructionCombine(t *testing.T) {
	t.Parallel()

	networkProvider := testscommon.NewNetworkProviderMock()
	service := NewConstructionService(networkProvider)

	notSignedTx := `{"nonce":42,"value":"1234","receiver":"erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx","sender":"erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th","gasPrice":1100000000,"gasLimit":57500,"data":"aGVsbG8=","chainID":"T","version":1}`

	response, errTyped := service.ConstructionCombine(context.Background(),
		&types.ConstructionCombineRequest{
			UnsignedTransaction: notSignedTx,
			Signatures: []*types.Signature{
				{
					Bytes: []byte{0xaa, 0xbb},
				},
			},
		},
	)

	signedTx := `{"nonce":42,"value":"1234","receiver":"erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx","sender":"erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th","gasPrice":1100000000,"gasLimit":57500,"data":"aGVsbG8=","signature":"aabb","chainID":"T","version":1}`

	require.Nil(t, errTyped)
	require.Equal(t, signedTx, response.SignedTransaction)
}

func TestConstructionService_ConstructionDerive(t *testing.T) {
	t.Parallel()

	networkProvider := testscommon.NewNetworkProviderMock()
	service := NewConstructionService(networkProvider)

	response, errTyped := service.ConstructionDerive(context.Background(),
		&types.ConstructionDeriveRequest{
			PublicKey: &types.PublicKey{
				Bytes:     testscommon.TestPubKeyAlice,
				CurveType: types.Edwards25519,
			},
		},
	)

	require.Nil(t, errTyped)
	require.Equal(t, testscommon.TestAddressAlice, response.AccountIdentifier.Address)
}

func TestConstructionService_ConstructionHash(t *testing.T) {
	t.Parallel()

	networkProvider := testscommon.NewNetworkProviderMock()
	networkProvider.MockComputedTransactionHash = "aaaa"
	service := NewConstructionService(networkProvider)

	signedTx := `{"nonce":42,"value":"1234","receiver":"erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx","sender":"erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th","gasPrice":1100000000,"gasLimit":57500,"data":"aGVsbG8=","signature":"aabb","chainID":"T","version":1}`

	response, errTyped := service.ConstructionHash(context.Background(),
		&types.ConstructionHashRequest{
			SignedTransaction: signedTx,
		},
	)
	require.Nil(t, errTyped)
	require.Equal(t, "aaaa", response.TransactionIdentifier.Hash)
}

func TestConstructionService_ConstructionSubmit(t *testing.T) {
	t.Parallel()

	networkProvider := testscommon.NewNetworkProviderMock()

	var calledWithTransaction *data.Transaction
	networkProvider.SendTransactionCalled = func(tx *data.Transaction) (string, error) {
		calledWithTransaction = tx
		return "aaaa", nil
	}

	service := NewConstructionService(networkProvider)

	signedTx := `{"nonce":42,"value":"1234","receiver":"erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx","sender":"erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th","gasPrice":1100000000,"gasLimit":57500,"data":"aGVsbG8=","signature":"aabb","chainID":"T","version":1}`

	response, errTyped := service.ConstructionSubmit(context.Background(),
		&types.ConstructionSubmitRequest{
			SignedTransaction: signedTx,
		},
	)
	require.Nil(t, errTyped)
	require.Equal(t, "aaaa", response.TransactionIdentifier.Hash)
	require.Equal(t, "T", calledWithTransaction.ChainID)
	require.Equal(t, uint64(42), calledWithTransaction.Nonce)
}

func TestConstructionService_CreateOperationsFromPreparedTx(t *testing.T) {
	t.Parallel()

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

	operations, err := service.createOperationsFromPreparedTx(preparedTx)
	require.Nil(t, err)
	require.Equal(t, expectedOperations, operations)
}
