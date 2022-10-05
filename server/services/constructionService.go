package services

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

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
	log.Info("constructionService.ConstructionPreprocess()",
		"metadata", request.Metadata,
		"maxFee", request.MaxFee,
		"suggestedFeeMultiplier", request.SuggestedFeeMultiplier,
	)

	requestMetadata, err := newConstructionPreprocessMetadata(request.Metadata)
	if err != nil {
		return nil, service.errFactory.newErrWithOriginal(ErrConstructionCheck, err)
	}

	responseOptions := &constructionOptions{}

	// TODO:
	if err := service.checkOperationsAndMeta(request.Operations, request.Metadata); err != nil {
		return nil, err
	}

	if len(requestMetadata.Sender) > 0 {
		responseOptions.Sender = requestMetadata.Sender
	} else {
		responseOptions.Sender = request.Operations[0].Account.Address
	}

	if len(requestMetadata.Receiver) > 0 {
		responseOptions.Receiver = requestMetadata.Receiver
	} else {
		if len(request.Operations) < 2 {
			return nil, service.errFactory.newErrWithOriginal(ErrConstructionCheck, errors.New("cannot prepare receiver"))
		}
		responseOptions.Receiver = request.Operations[1].Account.Address
	}

	if len(requestMetadata.Value) > 0 {
		responseOptions.Value = requestMetadata.Value
	} else {
		// TODO:
		responseOptions.Value = strings.Trim(request.Operations[0].Amount.Value, "-")
	}

	responseOptions.FirstOperationType = request.Operations[0].Type

	if len(request.MaxFee) > 0 {
		// TODO:
		maxFee := request.MaxFee[0]
		if !service.extension.isNativeCurrency(maxFee.Currency) {
			return nil, service.errFactory.newErrWithOriginal(ErrConstructionCheck, errors.New("invalid currency"))
		}

		responseOptions.MaxFee = maxFee.Value
	}

	if request.SuggestedFeeMultiplier != nil {
		responseOptions.FeeMultiplier = *request.SuggestedFeeMultiplier
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

	optionsAsObjectsMap, err := responseOptions.toObjectsMap()
	if err != nil {
		return nil, service.errFactory.newErrWithOriginal(ErrConstructionCheck, err)
	}

	// TODO: Ensure sender & receiver & value & FirstOperationType are known!
	// if metadata["sender"], ok = options["sender"]; !ok {
	// 	return nil, service.errFactory.newErrWithOriginal(ErrMalformedValue, errors.New("sender address missing"))
	// }
	// if metadata["receiver"], ok = options["receiver"]; !ok {
	// 	return nil, service.errFactory.newErrWithOriginal(ErrMalformedValue, errors.New("receiver address missing"))
	// }
	// if metadata["value"], ok = options["value"]; !ok {
	// 	return nil, service.errFactory.newErrWithOriginal(ErrMalformedValue, errors.New("value missing"))
	// }

	return &types.ConstructionPreprocessResponse{
		Options: optionsAsObjectsMap,
	}, nil
}

func (service *constructionService) checkOperationsAndMeta(ops []*types.Operation, meta map[string]interface{}) *types.Error {
	// TODO: remove this constraint; metadata should be sufficient, as well
	if len(ops) == 0 {
		return service.errFactory.newErrWithOriginal(ErrConstructionCheck, errors.New("invalid number of operations"))
	}

	for _, op := range ops {
		if !checkOperationsType(op) {
			return service.errFactory.newErrWithOriginal(ErrConstructionCheck, errors.New("unsupported operation type"))
		}
		if op.Amount.Currency.Symbol != service.extension.getNativeCurrency().Symbol {
			return service.errFactory.newErrWithOriginal(ErrConstructionCheck, errors.New("unsupported currency symbol"))
		}
	}

	return nil
}

func checkOperationsType(op *types.Operation) bool {
	for _, supOp := range SupportedOperationTypesForConstruction {
		if supOp == op.Type {
			return true
		}
	}

	return false
}

// ConstructionMetadata gets any information required to construct a transaction for a specific network (e.g. the account nonce)
func (service *constructionService) ConstructionMetadata(
	_ context.Context,
	request *types.ConstructionMetadataRequest,
) (*types.ConstructionMetadataResponse, *types.Error) {
	log.Info("constructionService.ConstructionMetadata()", "options", request.Options)

	if service.provider.IsOffline() {
		return nil, service.errFactory.newErr(ErrOfflineMode)
	}

	requestOptions, err := newConstructionOptions(request.Options)
	if err != nil {
		return nil, service.errFactory.newErrWithOriginal(ErrConstructionCheck, err)
	}

	txType := requestOptions.FirstOperationType

	metadata := &constructionMetadata{}
	metadata.Data = requestOptions.Data
	metadata.ChainID = service.provider.GetNetworkConfig().NetworkID
	metadata.Version = transactionVersion
	metadata.Sender = requestOptions.Sender
	metadata.Receiver = requestOptions.Receiver
	metadata.Value = requestOptions.Value

	account, err := service.provider.GetAccount(requestOptions.Sender)
	if err != nil {
		return nil, service.errFactory.newErrWithOriginal(ErrUnableToGetAccount, err)
	}

	metadata.Nonce = account.Account.Nonce

	networkConfig := service.provider.GetNetworkConfig()
	suggestedFee, gasPrice, gasLimit, errTyped := service.computeSuggestedFeeAndGas(txType, requestOptions, networkConfig)
	if err != nil {
		return nil, errTyped
	}

	metadata.GasLimit = gasLimit
	metadata.GasPrice = gasPrice

	metadataAsObjectsMap, err := metadata.toObjectsMap()
	if err != nil {
		return nil, service.errFactory.newErrWithOriginal(ErrConstructionCheck, err)
	}

	return &types.ConstructionMetadataResponse{
		Metadata: metadataAsObjectsMap,
		SuggestedFee: []*types.Amount{
			service.extension.valueToNativeAmount(suggestedFee.String()),
		},
	}, nil
}

// ConstructionPayloads returns an unsigned transaction blob and a collection of payloads that must be signed
func (service *constructionService) ConstructionPayloads(
	_ context.Context,
	request *types.ConstructionPayloadsRequest,
) (*types.ConstructionPayloadsResponse, *types.Error) {
	log.Info("constructionService.ConstructionPayloads()", "metadata", request.Metadata)

	if err := service.checkOperationsAndMeta(request.Operations, request.Metadata); err != nil {
		return nil, err
	}

	tx, err := createTransaction(request)
	if err != nil {
		return nil, service.errFactory.newErrWithOriginal(ErrMalformedValue, err)
	}

	txJson, err := json.Marshal(tx)
	if err != nil {
		return nil, service.errFactory.newErrWithOriginal(ErrMalformedValue, err)
	}

	signer := request.Operations[0].Account.Address

	return &types.ConstructionPayloadsResponse{
		UnsignedTransaction: string(txJson),
		Payloads: []*types.SigningPayload{
			{
				AccountIdentifier: addressToAccountIdentifier(signer),
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

func createTransaction(request *types.ConstructionPayloadsRequest) (*data.Transaction, error) {
	tx := &data.Transaction{}

	requestMetadataBytes, err := json.Marshal(request.Metadata)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(requestMetadataBytes, tx)
	if err != nil {
		return nil, err
	}

	return tx, nil
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
	log.Info("constructionService.ConstructionSubmit()", "transaction", request.SignedTransaction)

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
