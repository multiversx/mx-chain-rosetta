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

func (controller *transactionEventsController) findManyEventsByIdentifier(tx *data.FullTransaction, identifier string) []*transaction.Events {
	events := make([]*transaction.Events, 0)

	if !controller.hasEvents(tx) {
		return events
	}

	for _, event := range tx.Logs.Events {
		if event.Identifier == identifier {
			events = append(events, event)
		}
	}

	return events
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

func (controller *transactionEventsController) extractEventsESDTTransfers(tx *data.FullTransaction) ([]*eventTransferESDT, error) {
	rawEvents := controller.findManyEventsByIdentifier(tx, transactionEventESDTTransfer)
	typedEvents := make([]*eventTransferESDT, 0, len(rawEvents))

	for _, event := range rawEvents {
		numTopics := len(event.Topics)
		if numTopics != 4 {
			return nil, fmt.Errorf("%w: bad number of topics for 'ESDTTransfer' = %d", errCannotRecognizeEvent, numTopics)
		}

		tokenIdentifier := event.Topics[0]
		valueBytes := event.Topics[2]
		receiverPubkey := event.Topics[3]

		receiver := controller.provider.ConvertPubKeyToAddress(receiverPubkey)
		value := big.NewInt(0).SetBytes(valueBytes)

		typedEvents = append(typedEvents, &eventTransferESDT{
			sender:          event.Address,
			tokenIdentifier: string(tokenIdentifier),
			receiver:        receiver,
			value:           value.String(),
		})
	}

	return typedEvents, nil
}

func (controller *transactionEventsController) hasEvents(tx *data.FullTransaction) bool {
	return tx.Logs != nil && tx.Logs.Events != nil && len(tx.Logs.Events) > 0
}
