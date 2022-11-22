package services

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ElrondNetwork/elrond-go-core/data/transaction"
)

type transactionEventsController struct {
	provider NetworkProvider
}

func newTransactionEventsController(provider NetworkProvider) *transactionEventsController {
	return &transactionEventsController{
		provider: provider,
	}
}

func (controller *transactionEventsController) findEventByIdentifier(tx *transaction.ApiTransactionResult, identifier string) (*transaction.Events, error) {
	if !controller.hasEvents(tx) {
		return nil, errEventNotFound
	}

	for _, event := range tx.Logs.Events {
		if event.Identifier == identifier {
			return event, nil
		}
	}

	return nil, errEventNotFound
}

func (controller *transactionEventsController) extractEventTransferValueOnly(tx *transaction.ApiTransactionResult) (*eventTransferValueOnly, error) {
	event, err := controller.findEventByIdentifier(tx, transactionEventTransferValueOnly)
	if err != nil {
		return nil, err
	}

	numTopics := len(event.Topics)
	if numTopics != 3 {
		return nil, fmt.Errorf("%w: bad number of topics for 'transferValueOnly' = %d", errCannotRecognizeEvent, numTopics)
	}

	senderPubkey := event.Topics[0]
	receiverPubkey := event.Topics[1]
	valueBytes := event.Topics[2]

	sender := controller.provider.ConvertPubKeyToAddress(senderPubkey)
	receiver := controller.provider.ConvertPubKeyToAddress(receiverPubkey)
	value := big.NewInt(0).SetBytes(valueBytes)

	return &eventTransferValueOnly{
		sender:   sender,
		receiver: receiver,
		value:    value.String(),
	}, nil
}

func (controller *transactionEventsController) hasSignalErrorOfSendingValueToNonPayableContract(tx *transaction.ApiTransactionResult) bool {
	if !controller.hasEvents(tx) {
		return false
	}

	for _, event := range tx.Logs.Events {
		isSignalError := event.Identifier == transactionEventSignalError
		dataAsString := string(event.Data)
		dataMatchesError := strings.HasPrefix(dataAsString, sendingValueToNonPayableContractDataPrefix)

		if isSignalError && dataMatchesError {
			return true
		}
	}

	return false
}

func (controller *transactionEventsController) hasSignalErrorOfMetaTransactionIsInvalid(tx *transaction.ApiTransactionResult) bool {
	if !controller.hasEvents(tx) {
		return false
	}

	for _, event := range tx.Logs.Events {
		isSignalError := event.Identifier == transactionEventSignalError
		if !isSignalError {
			continue
		}

		if eventHasTopic(event, transactionEventTopicInvalidMetaTransaction) {
			return true
		}
		if eventHasTopic(event, transactionEventTopicInvalidMetaTransactionNotEnoughGas) {
			return true
		}
	}

	return false
}

func (controller *transactionEventsController) hasEvents(tx *transaction.ApiTransactionResult) bool {
	return tx.Logs != nil && tx.Logs.Events != nil && len(tx.Logs.Events) > 0
}

func eventHasTopic(event *transaction.Events, topicToFind string) bool {
	for _, topic := range event.Topics {
		if string(topic) == topicToFind {
			return true
		}
	}

	return false
}
