package services

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/ElrondNetwork/elrond-proxy-go/data"
	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
)

type constructionService struct {
	provider   NetworkProvider
	extension  *networkProviderExtension
	errFactory *errFactory
}

// NewConstructionService creates a new instance of an constructionService
func NewConstructionService(
	networkProvider NetworkProvider,
) server.ConstructionAPIServicer {
	return &constructionService{
		provider:   networkProvider,
		extension:  newNetworkProviderExtension(networkProvider),
		errFactory: newErrFactory(),
	}
}

// ConstructionPreprocess determines which metadata is needed for construction
func (service *constructionService) ConstructionPreprocess(
	_ context.Context,
	request *types.ConstructionPreprocessRequest,
) (*types.ConstructionPreprocessResponse, *types.Error) {
	log.Debug("constructionService.ConstructionPreprocess()", "metadata", request.Metadata)

	noOperationProvided := len(request.Operations) == 0
	lessThanTwoOperationsProvided := len(request.Operations) < 2

	requestMetadata, err := newConstructionPreprocessMetadata(request.Metadata)
	if err != nil {
		return nil, service.errFactory.newErrWithOriginal(ErrConstruction, err)
	}

	responseOptions := &constructionOptions{}

	if len(requestMetadata.Sender) > 0 {
		responseOptions.Sender = requestMetadata.Sender
	} else {
		// Fallback: get "sender" from the first operation
		if noOperationProvided {
			return nil, service.errFactory.newErrWithOriginal(ErrConstruction, errors.New("cannot prepare sender"))
		}
		responseOptions.Sender = request.Operations[0].Account.Address
	}

	if len(requestMetadata.Receiver) > 0 {
		responseOptions.Receiver = requestMetadata.Receiver
	} else {
		// Fallback: get "receiver" from the second operation
		if lessThanTwoOperationsProvided {
			return nil, service.errFactory.newErrWithOriginal(ErrConstruction, errors.New("cannot prepare receiver"))
		}
		responseOptions.Receiver = request.Operations[1].Account.Address
	}

	if len(requestMetadata.Amount) > 0 {
		responseOptions.Amount = requestMetadata.Amount
	} else {
		// Fallback: get "amount" from the first operation
		if noOperationProvided {
			return nil, service.errFactory.newErrWithOriginal(ErrConstruction, errors.New("cannot prepare amount"))
		}
		responseOptions.Amount = getMagnitudeOfAmount(request.Operations[0].Amount.Value)
	}

	if len(requestMetadata.CurrencySymbol) > 0 {
		responseOptions.CurrencySymbol = requestMetadata.CurrencySymbol
	} else {
		// Fallback: get "currencySymbol" from the first operation
		if noOperationProvided {
			return nil, service.errFactory.newErrWithOriginal(ErrConstruction, errors.New("cannot prepare currency"))
		}
		responseOptions.CurrencySymbol = request.Operations[0].Amount.Currency.Symbol
	}

	if requestMetadata.GasLimit > 0 {
		responseOptions.GasLimit = requestMetadata.GasLimit
	}
	if requestMetadata.GasPrice > 0 {
		responseOptions.GasPrice = requestMetadata.GasPrice
	}
	if len(requestMetadata.Data) > 0 {
		responseOptions.Data = requestMetadata.Data
	}

	err = responseOptions.validate(
		service.extension.getNativeCurrencySymbol(),
	)
	if err != nil {
		return nil, service.errFactory.newErrWithOriginal(ErrConstruction, err)
	}

	optionsAsObjectsMap, err := toObjectsMap(responseOptions)
	if err != nil {
		return nil, service.errFactory.newErrWithOriginal(ErrConstruction, err)
	}

	return &types.ConstructionPreprocessResponse{
		Options: optionsAsObjectsMap,
	}, nil
}

// ConstructionMetadata gets any information required to construct a transaction for a specific network (e.g. the account nonce)
func (service *constructionService) ConstructionMetadata(
	_ context.Context,
	request *types.ConstructionMetadataRequest,
) (*types.ConstructionMetadataResponse, *types.Error) {
	log.Debug("constructionService.ConstructionMetadata()", "options", request.Options)

	if service.provider.IsOffline() {
		return nil, service.errFactory.newErr(ErrOfflineMode)
	}

	requestOptions, err := newConstructionOptions(request.Options)
	if err != nil {
		return nil, service.errFactory.newErrWithOriginal(ErrConstruction, err)
	}

	account, err := service.provider.GetAccount(requestOptions.Sender)
	if err != nil {
		return nil, service.errFactory.newErrWithOriginal(ErrUnableToGetAccount, err)
	}

	computedData := service.computeData(requestOptions)

	fee, gasLimit, gasPrice, errTyped := service.computeFeeComponents(requestOptions, computedData)
	if err != nil {
		return nil, errTyped
	}

	metadata := &constructionMetadata{}
	metadata.Nonce = account.Account.Nonce
	metadata.Sender = requestOptions.Sender
	metadata.Receiver = requestOptions.Receiver
	metadata.Amount = requestOptions.Amount
	metadata.CurrencySymbol = requestOptions.CurrencySymbol
	metadata.GasLimit = gasLimit
	metadata.GasPrice = gasPrice
	metadata.Data = computedData
	metadata.ChainID = service.provider.GetNetworkConfig().NetworkID
	metadata.Version = transactionVersion

	metadataAsObjectsMap, err := toObjectsMap(metadata)
	if err != nil {
		return nil, service.errFactory.newErrWithOriginal(ErrConstruction, err)
	}

	return &types.ConstructionMetadataResponse{
		Metadata: metadataAsObjectsMap,
		SuggestedFee: []*types.Amount{
			service.extension.valueToNativeAmount(fee.String()),
		},
	}, nil
}

func (service *constructionService) computeData(options *constructionOptions) []byte {
	if service.extension.isNativeCurrencySymbol(options.CurrencySymbol) {
		return options.Data
	}

	// TODO: Handle in a future PR
	return make([]byte, 0)
}

// ConstructionPayloads returns an unsigned transaction blob and a collection of payloads that must be signed
func (service *constructionService) ConstructionPayloads(
	_ context.Context,
	request *types.ConstructionPayloadsRequest,
) (*types.ConstructionPayloadsResponse, *types.Error) {
	log.Debug("constructionService.ConstructionPayloads()", "metadata", request.Metadata)

	metadata, err := newConstructionMetadata(request.Metadata)
	if err != nil {
		return nil, service.errFactory.newErrWithOriginal(ErrConstruction, err)
	}

	txJson, err := metadata.toTransactionJson()
	if err != nil {
		return nil, service.errFactory.newErrWithOriginal(ErrConstruction, err)
	}

	return &types.ConstructionPayloadsResponse{
		UnsignedTransaction: string(txJson),
		Payloads: []*types.SigningPayload{
			{
				AccountIdentifier: addressToAccountIdentifier(metadata.Sender),
				SignatureType:     types.Ed25519,
				Bytes:             txJson,
			},
		},
	}, nil
}

// ConstructionParse is called on both unsigned and signed transactions to understand the intent of the formulated transaction.
// This is run as a sanity check before signing (after /payloads) and before broadcast (after /combine).
func (service *constructionService) ConstructionParse(
	_ context.Context,
	request *types.ConstructionParseRequest,
) (*types.ConstructionParseResponse, *types.Error) {
	tx, err := getTxFromRequest(request.Transaction)
	if err != nil {
		return nil, service.errFactory.newErrWithOriginal(ErrMalformedValue, err)
	}

	var signers []*types.AccountIdentifier
	if request.Signed {
		signers = []*types.AccountIdentifier{
			{
				Address: tx.Sender,
			},
		}
	}

	return &types.ConstructionParseResponse{
		Operations:               service.createOperationsFromPreparedTx(tx),
		AccountIdentifierSigners: signers,
	}, nil
}

func (service *constructionService) createOperationsFromPreparedTx(tx *data.Transaction) []*types.Operation {
	operations := []*types.Operation{
		{
			Type:    opTransfer,
			Account: addressToAccountIdentifier(tx.Sender),
			Amount:  service.extension.valueToNativeAmount("-" + tx.Value),
		},
		{
			Type:    opTransfer,
			Account: addressToAccountIdentifier(tx.Receiver),
			Amount:  service.extension.valueToNativeAmount(tx.Value),
		},
	}

	indexOperations(operations)

	return operations
}

func getTxFromRequest(txString string) (*data.Transaction, error) {
	txBytes := []byte(txString)

	var tx data.Transaction
	err := json.Unmarshal(txBytes, &tx)
	if err != nil {
		return nil, err
	}

	return &tx, nil
}

// ConstructionCombine creates a ready-to-broadcast transaction from an unsigned transaction and an array of provided signatures.
func (service *constructionService) ConstructionCombine(
	_ context.Context,
	request *types.ConstructionCombineRequest,
) (*types.ConstructionCombineResponse, *types.Error) {
	tx, err := getTxFromRequest(request.UnsignedTransaction)
	if err != nil {
		return nil, service.errFactory.newErrWithOriginal(ErrMalformedValue, err)
	}

	if len(request.Signatures) != 1 {
		return nil, service.errFactory.newErr(ErrInvalidInputParam)
	}

	tx.Signature = hex.EncodeToString(request.Signatures[0].Bytes)

	signedTxBytes, err := json.Marshal(tx)
	if err != nil {
		return nil, service.errFactory.newErrWithOriginal(ErrMalformedValue, err)
	}

	return &types.ConstructionCombineResponse{
		SignedTransaction: string(signedTxBytes),
	}, nil
}

// ConstructionDerive returns a bech32 address from public key bytes
func (service *constructionService) ConstructionDerive(
	_ context.Context,
	request *types.ConstructionDeriveRequest,
) (*types.ConstructionDeriveResponse, *types.Error) {
	if request.PublicKey.CurveType != types.Edwards25519 {
		return nil, service.errFactory.newErr(ErrUnsupportedCurveType)
	}

	pubKey := request.PublicKey.Bytes
	address := service.provider.ConvertPubKeyToAddress(pubKey)

	return &types.ConstructionDeriveResponse{
		AccountIdentifier: addressToAccountIdentifier(address),
		Metadata:          nil,
	}, nil
}

// ConstructionHash will calculate transaction hash
func (service *constructionService) ConstructionHash(
	_ context.Context,
	request *types.ConstructionHashRequest,
) (*types.TransactionIdentifierResponse, *types.Error) {
	elrondTx, err := getTxFromRequest(request.SignedTransaction)
	if err != nil {
		return nil, service.errFactory.newErrWithOriginal(ErrMalformedValue, err)
	}

	txHash, err := service.provider.ComputeTransactionHash(elrondTx)
	if err != nil {
		return nil, service.errFactory.newErrWithOriginal(ErrMalformedValue, err)
	}

	return &types.TransactionIdentifierResponse{
		TransactionIdentifier: &types.TransactionIdentifier{
			Hash: txHash,
		},
	}, nil
}

// ConstructionSubmit will submit transaction and return hash
func (service *constructionService) ConstructionSubmit(
	_ context.Context,
	request *types.ConstructionSubmitRequest,
) (*types.TransactionIdentifierResponse, *types.Error) {
	log.Debug("constructionService.ConstructionSubmit()", "transaction", request.SignedTransaction)

	if service.provider.IsOffline() {
		return nil, service.errFactory.newErr(ErrOfflineMode)
	}

	elrondTx, err := getTxFromRequest(request.SignedTransaction)
	if err != nil {
		return nil, service.errFactory.newErrWithOriginal(ErrMalformedValue, err)
	}

	txHash, err := service.provider.SendTransaction(elrondTx)
	if err != nil {
		return nil, service.errFactory.newErrWithOriginal(ErrUnableToSubmitTransaction, err)
	}

	return &types.TransactionIdentifierResponse{
		TransactionIdentifier: &types.TransactionIdentifier{
			Hash: txHash,
		},
	}, nil
}
