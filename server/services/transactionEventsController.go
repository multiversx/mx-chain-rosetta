package services

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/multiversx/mx-chain-core-go/data/transaction"
)

type transactionEventsController struct {
	provider NetworkProvider
}

func newTransactionEventsController(provider NetworkProvider) *transactionEventsController {
	return &transactionEventsController{
		provider: provider,
	}
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

func (controller *transactionEventsController) extractEventsESDTOrESDTNFTTransfers(tx *transaction.ApiTransactionResult) ([]*eventESDTOrESDTNFTTransfer, error) {
	rawEventsESDTTransfer := controller.findManyEventsByIdentifier(tx, transactionEventESDTTransfer)
	rawEventsESDTNFTTransfer := controller.findManyEventsByIdentifier(tx, transactionEventESDTNFTTransfer)
	rawEventsMultiESDTNFTTransfer := controller.findManyEventsByIdentifier(tx, transactionEventMultiESDTNFTTransfer)

	rawEvents := make([]*transaction.Events, 0, len(rawEventsESDTTransfer)+len(rawEventsESDTNFTTransfer)+len(rawEventsMultiESDTNFTTransfer))
	rawEvents = append(rawEvents, rawEventsESDTTransfer...)
	rawEvents = append(rawEvents, rawEventsESDTNFTTransfer...)
	rawEvents = append(rawEvents, rawEventsMultiESDTNFTTransfer...)

	typedEvents := make([]*eventESDTOrESDTNFTTransfer, 0, len(rawEvents))

	for _, event := range rawEvents {
		numTopics := len(event.Topics)
		if numTopics != 4 {
			return nil, fmt.Errorf("%w: bad number of topics for (ESDT|ESDTNFT|MultiESDTNFT)Transfer event = %d", errCannotRecognizeEvent, numTopics)
		}

		identifider := event.Topics[0]
		nonceAsBytes := event.Topics[1]
		valueBytes := event.Topics[2]
		receiverPubkey := event.Topics[3]

		value := big.NewInt(0).SetBytes(valueBytes)
		receiver := controller.provider.ConvertPubKeyToAddress(receiverPubkey)

		typedEvents = append(typedEvents, &eventESDTOrESDTNFTTransfer{
			sender:       event.Address,
			identifier:   string(identifider),
			nonceAsBytes: nonceAsBytes,
			value:        value.String(),
			receiver:     receiver,
		})
	}

	return typedEvents, nil
}

func (controller *transactionEventsController) extractEventsESDTLocalBurn(tx *transaction.ApiTransactionResult) ([]*eventESDTLocalBurn, error) {
	rawEvents := controller.findManyEventsByIdentifier(tx, transactionEventESDTLocalBurn)
	typedEvents := make([]*eventESDTLocalBurn, 0, len(rawEvents))

	for _, event := range rawEvents {
		numTopics := len(event.Topics)
		if numTopics != 3 {
			return nil, fmt.Errorf("%w: bad number of topics for ESDTLocalBurn event = %d", errCannotRecognizeEvent, numTopics)
		}

		identifider := event.Topics[0]
		nonceAsBytes := event.Topics[1]
		valueBytes := event.Topics[2]
		value := big.NewInt(0).SetBytes(valueBytes)

		typedEvents = append(typedEvents, &eventESDTLocalBurn{
			address:      event.Address,
			identifier:   string(identifider),
			nonceAsBytes: nonceAsBytes,
			value:        value.String(),
		})
	}

	return typedEvents, nil
}

func (controller *transactionEventsController) extractEventsESDTLocalMint(tx *transaction.ApiTransactionResult) ([]*eventESDTLocalMint, error) {
	rawEvents := controller.findManyEventsByIdentifier(tx, transactionEventESDTLocalMint)
	typedEvents := make([]*eventESDTLocalMint, 0, len(rawEvents))

	for _, event := range rawEvents {
		numTopics := len(event.Topics)
		if numTopics != 3 {
			return nil, fmt.Errorf("%w: bad number of topics for ESDTLocalMint event = %d", errCannotRecognizeEvent, numTopics)
		}

		identifider := event.Topics[0]
		nonceAsBytes := event.Topics[1]
		valueBytes := event.Topics[2]
		value := big.NewInt(0).SetBytes(valueBytes)

		typedEvents = append(typedEvents, &eventESDTLocalMint{
			address:      event.Address,
			identifier:   string(identifider),
			nonceAsBytes: nonceAsBytes,
			value:        value.String(),
		})
	}

	return typedEvents, nil
}

func (controller *transactionEventsController) extractEventsESDTWipe(tx *transaction.ApiTransactionResult) ([]*eventESDTWipe, error) {
	rawEvents := controller.findManyEventsByIdentifier(tx, transactionEventESDTWipe)
	typedEvents := make([]*eventESDTWipe, 0, len(rawEvents))

	for _, event := range rawEvents {
		numTopics := len(event.Topics)
		if numTopics != 4 {
			return nil, fmt.Errorf("%w: bad number of topics for ESDTWipe event = %d", errCannotRecognizeEvent, numTopics)
		}

		identifider := event.Topics[0]
		nonceAsBytes := event.Topics[1]
		valueBytes := event.Topics[2]
		accountPubkey := event.Topics[3]

		value := big.NewInt(0).SetBytes(valueBytes)
		accountAddress := controller.provider.ConvertPubKeyToAddress(accountPubkey)

		typedEvents = append(typedEvents, &eventESDTWipe{
			address:      accountAddress,
			identifier:   string(identifider),
			nonceAsBytes: nonceAsBytes,
			value:        value.String(),
		})
	}

	return typedEvents, nil
}

func (controller *transactionEventsController) findManyEventsByIdentifier(tx *transaction.ApiTransactionResult, identifier string) []*transaction.Events {
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
