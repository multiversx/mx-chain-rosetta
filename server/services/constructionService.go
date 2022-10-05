package services

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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
	if err := service.checkOperationsAndMeta(request.Operations, request.Metadata); err != nil {
		return nil, err
	}

	options, errOptions := service.prepareConstructionOptions(request.Operations, request.Metadata)
	if errOptions != nil {
		return nil, service.errFactory.newErrWithOriginal(ErrConstructionCheck, errOptions)
	}

	if len(request.MaxFee) > 0 {
		maxFee := request.MaxFee[0]
		if !service.extension.isNativeCurrency(maxFee.Currency) {
			return nil, service.errFactory.newErrWithOriginal(ErrConstructionCheck, errors.New("invalid currency"))
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

func (service *constructionService) checkOperationsAndMeta(ops []*types.Operation, meta map[string]interface{}) *types.Error {
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

	if meta["gasLimit"] != nil {
		if !checkValueIsOk(meta["gasLimit"]) {
			return service.errFactory.newErrWithOriginal(ErrConstructionCheck, errors.New("invalid metadata gas limit"))
		}
	}
	if meta["gasPrice"] != nil {
		if !checkValueIsOk(meta["gasPrice"]) {
			return service.errFactory.newErrWithOriginal(ErrConstructionCheck, errors.New("invalid metadata gas price"))
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

func (service *constructionService) prepareConstructionOptions(operations []*types.Operation, metadata objectsMap) (objectsMap, error) {
	options := make(objectsMap)
	options["type"] = operations[0].Type
	options["sender"] = operations[0].Account.Address

	if metadata["receiver"] != nil {
		options["receiver"] = metadata["receiver"]
	} else {
		if len(operations) > 1 {
			options["receiver"] = operations[1].Account.Address
		} else {
			return nil, errors.New("cannot prepare transaction receiver")
		}
	}

	if metadata["value"] != nil {
		options["value"] = metadata["value"]
	} else {
		if len(operations) > 1 {
			options["value"] = operations[1].Amount.Value
		} else {
			options["value"] = strings.Trim(operations[0].Amount.Value, "-")
		}
	}

	return options, nil
}

// ConstructionMetadata gets any information required to construct a transaction for a specific network (e.g. the account nonce)
func (service *constructionService) ConstructionMetadata(
	_ context.Context,
	request *types.ConstructionMetadataRequest,
) (*types.ConstructionMetadataResponse, *types.Error) {
	if service.provider.IsOffline() {
		return nil, service.errFactory.newErr(ErrOfflineMode)
	}

	txType, ok := request.Options["type"].(string)
	if !ok {
		return nil, service.errFactory.newErrWithOriginal(ErrInvalidInputParam, errors.New("invalid operation type"))
	}

	metadata, err := service.computeMetadata(request.Options)
	if err != nil {
		return nil, err
	}

	networkConfig := service.provider.GetNetworkConfig()
	suggestedFee, gasPrice, gasLimit, err := service.computeSuggestedFeeAndGas(txType, request.Options, networkConfig)
	if err != nil {
		return nil, err
	}

	metadata["gasLimit"] = gasLimit
	metadata["gasPrice"] = gasPrice

	return &types.ConstructionMetadataResponse{
		Metadata: metadata,
		SuggestedFee: []*types.Amount{
			service.extension.valueToNativeAmount(suggestedFee.String()),
		},
	}, nil
}

func (service *constructionService) computeMetadata(options objectsMap) (objectsMap, *types.Error) {
	metadata := make(objectsMap)
	if dataField, ok := options["data"]; ok {
		// convert string to byte array
		metadata["data"] = []byte(fmt.Sprintf("%v", dataField))
	}

	var ok bool
	if metadata["sender"], ok = options["sender"]; !ok {
		return nil, service.errFactory.newErrWithOriginal(ErrMalformedValue, errors.New("sender address missing"))
	}
	if metadata["receiver"], ok = options["receiver"]; !ok {
		return nil, service.errFactory.newErrWithOriginal(ErrMalformedValue, errors.New("receiver address missing"))
	}
	if metadata["value"], ok = options["value"]; !ok {
		return nil, service.errFactory.newErrWithOriginal(ErrMalformedValue, errors.New("value missing"))
	}

	metadata["chainID"] = service.provider.GetNetworkConfig().NetworkID
	metadata["version"] = transactionVersion

	senderAddressI, ok := options["sender"]
	if !ok {
		return nil, service.errFactory.newErrWithOriginal(ErrInvalidInputParam, errors.New("cannot find sender address"))
	}
	senderAddress, ok := senderAddressI.(string)
	if !ok {
		return nil, service.errFactory.newErrWithOriginal(ErrMalformedValue, errors.New("sender address is invalid"))
	}

	account, err := service.provider.GetAccount(senderAddress)
	if err != nil {
		return nil, service.errFactory.newErrWithOriginal(ErrUnableToGetAccount, err)
	}

	metadata["nonce"] = account.Account.Nonce

	return metadata, nil
}

// ConstructionPayloads returns an unsigned transaction blob and a collection of payloads that must be signed
func (service *constructionService) ConstructionPayloads(
	_ context.Context,
	request *types.ConstructionPayloadsRequest,
) (*types.ConstructionPayloadsResponse, *types.Error) {
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
