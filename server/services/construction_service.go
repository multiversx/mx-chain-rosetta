package services

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ElrondNetwork/elrond-proxy-go/data"
	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
)

type constructionAPIService struct {
	provider  NetworkProvider
	txsParser *transactionsParser
}

// NewConstructionAPIService creates a new instance of an constructionAPIService.
func NewConstructionAPIService(
	networkProvider NetworkProvider,
) server.ConstructionAPIServicer {
	return &constructionAPIService{
		provider:  networkProvider,
		txsParser: newTransactionParser(networkProvider),
	}
}

func (service *constructionAPIService) getNativeCurrency() *types.Currency {
	currency := service.provider.GetNativeCurrency()

	return &types.Currency{
		Symbol:   currency.Symbol,
		Decimals: currency.Decimals,
	}
}

// ConstructionPreprocess will preprocess data that in provided in request
func (service *constructionAPIService) ConstructionPreprocess(
	_ context.Context,
	request *types.ConstructionPreprocessRequest,
) (*types.ConstructionPreprocessResponse, *types.Error) {
	if err := service.checkOperationsAndMeta(request.Operations, request.Metadata); err != nil {
		return nil, err
	}

	options, err := getOptionsFromOperations(request.Operations)
	if err != nil {
		return nil, err
	}

	if len(request.MaxFee) > 0 {
		maxFee := request.MaxFee[0]
		if maxFee.Currency != service.getNativeCurrency() {
			return nil, wrapErr(ErrConstructionCheck, errors.New("invalid currency"))
		}

		options["maxFee"] = maxFee.Value
	}

	if request.SuggestedFeeMultiplier != nil {
		options["feeMultiplier"] = *request.SuggestedFeeMultiplier
	}

	if request.Metadata["gasLimit"] != nil {
		options["gasLimit"] = request.Metadata["gasLimit"]
	}
	if request.Metadata["gasPrice"] != nil {
		options["gasPrice"] = request.Metadata["gasPrice"]
	}
	if request.Metadata["data"] != nil {
		options["data"] = request.Metadata["data"]
	}

	return &types.ConstructionPreprocessResponse{
		Options: options,
	}, nil
}

func (service *constructionAPIService) checkOperationsAndMeta(ops []*types.Operation, meta map[string]interface{}) *types.Error {
	if len(ops) == 0 {
		return wrapErr(ErrConstructionCheck, errors.New("invalid number of operations"))
	}

	for _, op := range ops {
		if !checkOperationsType(op) {
			return wrapErr(ErrConstructionCheck, errors.New("unsupported operation type"))
		}
		if op.Amount.Currency.Symbol != service.getNativeCurrency().Symbol {
			return wrapErr(ErrConstructionCheck, errors.New("unsupported currency symbol"))
		}
	}

	if meta["gasLimit"] != nil {
		if !checkValueIsOk(meta["gasLimit"]) {
			return wrapErr(ErrConstructionCheck, errors.New("invalid metadata gas limit"))
		}
	}
	if meta["gasPrice"] != nil {
		if !checkValueIsOk(meta["gasPrice"]) {
			return wrapErr(ErrConstructionCheck, errors.New("invalid metadata gas price"))
		}
	}

	return nil
}

func checkValueIsOk(value interface{}) bool {
	switch value.(type) {
	case uint64, float64, int:
		return true
	default:
		return false
	}
}

func checkOperationsType(op *types.Operation) bool {
	for _, supOp := range SupportedOperationTypes {
		if supOp == op.Type {
			return true
		}
	}

	return false
}

func getOptionsFromOperations(ops []*types.Operation) (objectsMap, *types.Error) {
	if len(ops) < 2 {
		return nil, wrapErr(ErrConstructionCheck, errors.New("invalid number of operations"))
	}
	options := make(objectsMap)
	options["sender"] = ops[0].Account.Address
	options["receiver"] = ops[1].Account.Address
	options["type"] = ops[0].Type
	options["value"] = ops[1].Amount.Value

	return options, nil
}

// ConstructionMetadata construct metadata for a transaction
func (service *constructionAPIService) ConstructionMetadata(
	_ context.Context,
	request *types.ConstructionMetadataRequest,
) (*types.ConstructionMetadataResponse, *types.Error) {
	if service.provider.IsOffline() {
		return nil, ErrOfflineMode
	}

	txType, ok := request.Options["type"].(string)
	if !ok {
		return nil, wrapErr(ErrInvalidInputParam, errors.New("invalid operation type"))
	}

	metadata, errS := service.computeMetadata(request.Options)
	if errS != nil {
		return nil, errS
	}

	suggestedFee, gasPrice, gasLimit, errS := computeSuggestedFeeAndGas(txType, request.Options, service.provider)
	if errS != nil {
		return nil, errS
	}

	metadata["gasLimit"] = gasLimit
	metadata["gasPrice"] = gasPrice

	return &types.ConstructionMetadataResponse{
		Metadata: metadata,
		SuggestedFee: []*types.Amount{
			{
				Value:    suggestedFee.String(),
				Currency: service.getNativeCurrency(),
			},
		},
	}, nil
}

func (service *constructionAPIService) computeMetadata(options objectsMap) (objectsMap, *types.Error) {
	metadata := make(objectsMap)
	if dataField, ok := options["data"]; ok {
		// convert string to byte array
		metadata["data"] = []byte(fmt.Sprintf("%v", dataField))
	}

	var ok bool
	if metadata["sender"], ok = options["sender"]; !ok {
		return nil, wrapErr(ErrMalformedValue, errors.New("sender address missing"))
	}
	if metadata["receiver"], ok = options["receiver"]; !ok {
		return nil, wrapErr(ErrMalformedValue, errors.New("receiver address missing"))
	}
	if metadata["value"], ok = options["value"]; !ok {
		return nil, wrapErr(ErrMalformedValue, errors.New("value missing"))
	}

	chainID, err := service.provider.GetChainID()
	if err != nil {
		return nil, wrapErr(ErrUnableToGetChainID, err)
	}

	metadata["chainID"] = chainID
	metadata["version"] = transactionVersion

	senderAddressI, ok := options["sender"]
	if !ok {
		return nil, wrapErr(ErrInvalidInputParam, errors.New("cannot find sender address"))
	}
	senderAddress, ok := senderAddressI.(string)
	if !ok {
		return nil, wrapErr(ErrMalformedValue, errors.New("sender address is invalid"))
	}

	account, err := service.provider.GetAccount(senderAddress)
	if err != nil {
		return nil, wrapErr(ErrUnableToGetAccount, err)
	}

	metadata["nonce"] = account.Nonce

	return metadata, nil
}

// ConstructionPayloads will prepare a transaction for signing
func (service *constructionAPIService) ConstructionPayloads(
	_ context.Context,
	request *types.ConstructionPayloadsRequest,
) (*types.ConstructionPayloadsResponse, *types.Error) {
	if err := service.checkOperationsAndMeta(request.Operations, request.Metadata); err != nil {
		return nil, err
	}

	erdTx, err := createTransaction(request)
	if err != nil {
		return nil, wrapErr(ErrMalformedValue, err)
	}

	mtx, err := json.Marshal(erdTx)
	if err != nil {
		return nil, wrapErr(ErrMalformedValue, err)
	}

	unsignedTx := hex.EncodeToString(mtx)

	return &types.ConstructionPayloadsResponse{
		UnsignedTransaction: unsignedTx,
		Payloads: []*types.SigningPayload{
			{
				AccountIdentifier: &types.AccountIdentifier{
					Address: request.Operations[0].Account.Address,
				},
				SignatureType: types.Ed25519,
				Bytes:         mtx,
			},
		},
	}, nil
}

// ConstructionParse will check if a transaction is correctly formatted
func (service *constructionAPIService) ConstructionParse(
	_ context.Context,
	request *types.ConstructionParseRequest,
) (*types.ConstructionParseResponse, *types.Error) {
	elrondTx, err := getTxFromRequest(request.Transaction)
	if err != nil {
		return nil, wrapErr(ErrMalformedValue, err)
	}

	var signers []*types.AccountIdentifier
	if request.Signed {
		signers = []*types.AccountIdentifier{
			{
				Address: elrondTx.Sender,
			},
		}
	}

	return &types.ConstructionParseResponse{
		Operations:               service.txsParser.createOperationsFromPreparedTx(elrondTx),
		AccountIdentifierSigners: signers,
	}, nil
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
	txBytes, err := hex.DecodeString(txString)
	if err != nil {
		return nil, err
	}

	var elrondTx data.Transaction
	err = json.Unmarshal(txBytes, &elrondTx)
	if err != nil {
		return nil, err
	}

	return &elrondTx, nil
}

// ConstructionCombine will create a signed transaction for transaction bytes and signature
func (service *constructionAPIService) ConstructionCombine(
	_ context.Context,
	request *types.ConstructionCombineRequest,
) (*types.ConstructionCombineResponse, *types.Error) {
	elrondTx, err := getTxFromRequest(request.UnsignedTransaction)
	if err != nil {
		return nil, wrapErr(ErrMalformedValue, err)
	}

	if len(request.Signatures) != 1 {
		return nil, ErrInvalidInputParam
	}

	txSignature := hex.EncodeToString(request.Signatures[0].Bytes)
	elrondTx.Signature = txSignature

	signedTxBytes, err := json.Marshal(elrondTx)
	if err != nil {
		return nil, wrapErr(ErrMalformedValue, err)
	}

	signedTx := hex.EncodeToString(signedTxBytes)

	return &types.ConstructionCombineResponse{
		SignedTransaction: signedTx,
	}, nil
}

// ConstructionDerive returns a bech32 address from public key bytes
func (service *constructionAPIService) ConstructionDerive(
	_ context.Context,
	request *types.ConstructionDeriveRequest,
) (*types.ConstructionDeriveResponse, *types.Error) {
	if request.PublicKey.CurveType != types.Edwards25519 {
		return nil, ErrUnsupportedCurveType
	}

	pubKey := request.PublicKey.Bytes
	address, err := service.provider.ConvertPubKeyToAddress(pubKey)
	if err != nil {
		return nil, wrapErr(ErrMalformedValue, err)
	}

	return &types.ConstructionDeriveResponse{
		AccountIdentifier: &types.AccountIdentifier{
			Address: address,
		},
		Metadata: nil,
	}, nil
}

// ConstructionHash will calculate transaction hash
func (service *constructionAPIService) ConstructionHash(
	_ context.Context,
	request *types.ConstructionHashRequest,
) (*types.TransactionIdentifierResponse, *types.Error) {
	elrondTx, err := getTxFromRequest(request.SignedTransaction)
	if err != nil {
		return nil, wrapErr(ErrMalformedValue, err)
	}

	txHash, err := service.provider.ComputeTransactionHash(elrondTx)
	if err != nil {
		return nil, wrapErr(ErrMalformedValue, err)
	}

	return &types.TransactionIdentifierResponse{
		TransactionIdentifier: &types.TransactionIdentifier{
			Hash: txHash,
		},
	}, nil
}

// ConstructionSubmit will submit transaction and return hash
func (service *constructionAPIService) ConstructionSubmit(
	_ context.Context,
	request *types.ConstructionSubmitRequest,
) (*types.TransactionIdentifierResponse, *types.Error) {
	if service.provider.IsOffline() {
		return nil, ErrOfflineMode
	}

	elrondTx, err := getTxFromRequest(request.SignedTransaction)
	if err != nil {
		return nil, wrapErr(ErrMalformedValue, err)
	}

	txHash, err := service.provider.SendTransaction(elrondTx)
	if err != nil {
		return nil, wrapErr(ErrUnableToSubmitTransaction, err)
	}

	return &types.TransactionIdentifierResponse{
		TransactionIdentifier: &types.TransactionIdentifier{
			Hash: txHash,
		},
	}, nil
}
