package services

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ElrondNetwork/elrond-go-core/data/transaction"
	"github.com/ElrondNetwork/elrond-proxy-go/data"
)

type transactionEventsController struct {
	provider NetworkProvider
}

func newTransactionEventsController(provider NetworkProvider) *transactionEventsController {
	return &transactionEventsController{
		provider: provider,
	}
}

func (controller *transactionEventsController) findEventByIdentifier(tx *data.FullTransaction, identifier string) (*transaction.Events, error) {
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

func (controller *transactionEventsController) extractEventTransferValueOnly(tx *data.FullTransaction) (*eventTransferValueOnly, error) {
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

func (controller *transactionEventsController) hasSignalErrorOfSendingValueToNonPayableContract(tx *data.FullTransaction) bool {
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

func (controller *transactionEventsController) hasEvents(tx *data.FullTransaction) bool {
	return tx.Logs != nil && tx.Logs.Events != nil && len(tx.Logs.Events) > 0
}
