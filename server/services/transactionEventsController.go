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

func (controller *transactionEventsController) extractEventSCDeploy(tx *transaction.ApiTransactionResult) ([]*eventSCDeploy, error) {
	rawEvents := controller.findManyEventsByIdentifier(tx, transactionEventSCDeploy)

	typedEvents := make([]*eventSCDeploy, 0, len(rawEvents))

	for _, event := range rawEvents {
		numTopics := len(event.Topics)
		if numTopics != 2 {
			return nil, fmt.Errorf("%w: bad number of topics for %s event = %d", errCannotRecognizeEvent, transactionEventSCDeploy, numTopics)
		}

		contractAddress := event.Address
		deployerPubKey := event.Topics[1]
		deployerAddress := controller.provider.ConvertPubKeyToAddress(deployerPubKey)

		typedEvents = append(typedEvents, &eventSCDeploy{
			contractAddress: contractAddress,
			deployerAddress: deployerAddress,
		})
	}

	return typedEvents, nil
}

func (controller *transactionEventsController) extractEventTransferValueOnly(tx *transaction.ApiTransactionResult) ([]*eventTransferValueOnly, error) {
	rawEvents := controller.findManyEventsByIdentifier(tx, transactionEventTransferValueOnly)

	typedEvents := make([]*eventTransferValueOnly, 0)

	for _, event := range rawEvents {
		numTopics := len(event.Topics)
		isBeforeSirius := numTopics == 3
		isAfterSirius := numTopics == 2

		var senderPubKey []byte
		var receiverPubKey []byte
		var valueBytes []byte

		if isBeforeSirius {
			senderPubKey = event.Topics[0]
			receiverPubKey = event.Topics[1]
			valueBytes = event.Topics[2]
		} else if isAfterSirius {
			// https://github.com/multiversx/mx-specs/blob/main/releases/protocol/release-specs-v1.6.0-Sirius.md#17-logs--events-changes-5490
			senderPubKey = []byte(event.Address)
			receiverPubKey = event.Topics[0]
			valueBytes = event.Topics[1]
		} else {
			return nil, fmt.Errorf("%w: bad number of topics for 'transferValueOnly' = %d", errCannotRecognizeEvent, numTopics)
		}

		sender := controller.provider.ConvertPubKeyToAddress(senderPubKey)
		receiver := controller.provider.ConvertPubKeyToAddress(receiverPubKey)
		value := big.NewInt(0).SetBytes(valueBytes)

		typedEvents = append(typedEvents, &eventTransferValueOnly{
			sender:         sender,
			senderPubKey:   senderPubKey,
			receiver:       receiver,
			receiverPubKey: receiverPubKey,
			value:          value.String(),
		})
	}

	return typedEvents, nil
}

func filterOutTransferValueOnlyEventsThatAreAlreadyCapturedAsContractResults(
	events []*eventTransferValueOnly,
	txsInBlock []*transaction.ApiTransactionResult,
) []*eventTransferValueOnly {
	// First, we find all contract results in this block, and we "summarize" them (in a map).
	contractResultsSummaries := make(map[string]struct{})

	for _, tx := range txsInBlock {
		isContractResult := tx.Type == string(transaction.TxTypeUnsigned)
		if !isContractResult {
			continue
		}

		summary := fmt.Sprintf("%s-%s-%s", tx.Sender, tx.Receiver, tx.Value)
		contractResultsSummaries[summary] = struct{}{}
	}

	eventsToKeep := make([]*eventTransferValueOnly, 0, len(events))

	for _, event := range events {
		summary := fmt.Sprintf("%s-%s-%s", event.sender, event.receiver, event.value)

		_, isAlreadyCaptured := contractResultsSummaries[summary]
		if isAlreadyCaptured {
			continue
		}

		// Event not captured as contract result, so we should keep it.
		eventsToKeep = append(eventsToKeep, event)
	}

	return eventsToKeep
}

func (controller *transactionEventsController) hasAnySignalError(tx *transaction.ApiTransactionResult) bool {
	if !controller.hasEvents(tx) {
		return false
	}

	for _, event := range tx.Logs.Events {
		isSignalError := event.Identifier == transactionEventSignalError
		if isSignalError {
			return true
		}
	}

	return false
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

func (controller *transactionEventsController) extractEventsESDTOrESDTNFTTransfers(tx *transaction.ApiTransactionResult) ([]*eventESDT, error) {
	rawEventsESDTTransfer := controller.findManyEventsByIdentifier(tx, transactionEventESDTTransfer)
	rawEventsESDTNFTTransfer := controller.findManyEventsByIdentifier(tx, transactionEventESDTNFTTransfer)
	rawEventsMultiESDTNFTTransfer := controller.findManyEventsByIdentifier(tx, transactionEventMultiESDTNFTTransfer)

	typedEvents := make([]*eventESDT, 0)

	// First, handle single transfers
	for _, event := range append(rawEventsESDTTransfer, rawEventsESDTNFTTransfer...) {
		numTopics := len(event.Topics)
		if numTopics != 4 {
			return nil, fmt.Errorf("%w: bad number of topics for (ESDT|ESDTNFT)Transfer event = %d", errCannotRecognizeEvent, numTopics)
		}

		identifier := event.Topics[0]
		nonceAsBytes := event.Topics[1]
		valueBytes := event.Topics[2]
		receiverPubkey := event.Topics[3]

		value := big.NewInt(0).SetBytes(valueBytes)
		receiver := controller.provider.ConvertPubKeyToAddress(receiverPubkey)

		typedEvents = append(typedEvents, &eventESDT{
			senderAddress:   event.Address,
			receiverAddress: receiver,
			identifier:      string(identifier),
			nonceAsBytes:    nonceAsBytes,
			value:           value.String(),
		})
	}

	// Then, handle multi transfers
	for _, event := range rawEventsMultiESDTNFTTransfer {
		numTopics := len(event.Topics)
		numTopicsExceptLast := numTopics - 1
		numTopicsPerTransfer := 3

		if numTopicsExceptLast%numTopicsPerTransfer != 0 {
			return nil, fmt.Errorf("%w: bad number of topics for MultiESDTNFTTransfer event = %d", errCannotRecognizeEvent, numTopics)
		}

		numTransfers := numTopicsExceptLast / numTopicsPerTransfer
		receiverPubkey := event.Topics[numTopics-1]
		receiver := controller.provider.ConvertPubKeyToAddress(receiverPubkey)

		for i := 0; i < numTransfers; i++ {
			identifier := event.Topics[i*numTopicsPerTransfer+0]
			nonceAsBytes := event.Topics[i*numTopicsPerTransfer+1]
			valueBytes := event.Topics[i*numTopicsPerTransfer+2]
			value := big.NewInt(0).SetBytes(valueBytes)

			typedEvents = append(typedEvents, &eventESDT{
				senderAddress:   event.Address,
				receiverAddress: receiver,
				identifier:      string(identifier),
				nonceAsBytes:    nonceAsBytes,
				value:           value.String(),
			})
		}
	}

	return typedEvents, nil
}

func (controller *transactionEventsController) extractEventsESDTLocalBurn(tx *transaction.ApiTransactionResult) ([]*eventESDT, error) {
	rawEvents := controller.findManyEventsByIdentifier(tx, transactionEventESDTLocalBurn)
	typedEvents := make([]*eventESDT, 0, len(rawEvents))

	for _, event := range rawEvents {
		numTopics := len(event.Topics)
		if numTopics != 3 {
			return nil, fmt.Errorf("%w: bad number of topics for ESDTLocalBurn event = %d", errCannotRecognizeEvent, numTopics)
		}

		identifider := event.Topics[0]
		nonceAsBytes := event.Topics[1]
		valueBytes := event.Topics[2]
		value := big.NewInt(0).SetBytes(valueBytes)

		typedEvents = append(typedEvents, &eventESDT{
			otherAddress: event.Address,
			identifier:   string(identifider),
			nonceAsBytes: nonceAsBytes,
			value:        value.String(),
		})
	}

	return typedEvents, nil
}

func (controller *transactionEventsController) extractEventsESDTLocalMint(tx *transaction.ApiTransactionResult) ([]*eventESDT, error) {
	rawEvents := controller.findManyEventsByIdentifier(tx, transactionEventESDTLocalMint)
	typedEvents := make([]*eventESDT, 0, len(rawEvents))

	for _, event := range rawEvents {
		numTopics := len(event.Topics)
		if numTopics != 3 {
			return nil, fmt.Errorf("%w: bad number of topics for ESDTLocalMint event = %d", errCannotRecognizeEvent, numTopics)
		}

		identifider := event.Topics[0]
		nonceAsBytes := event.Topics[1]
		valueBytes := event.Topics[2]
		value := big.NewInt(0).SetBytes(valueBytes)

		typedEvents = append(typedEvents, &eventESDT{
			otherAddress: event.Address,
			identifier:   string(identifider),
			nonceAsBytes: nonceAsBytes,
			value:        value.String(),
		})
	}

	return typedEvents, nil
}

func (controller *transactionEventsController) extractEventsESDTWipe(tx *transaction.ApiTransactionResult) ([]*eventESDT, error) {
	rawEvents := controller.findManyEventsByIdentifier(tx, transactionEventESDTWipe)
	typedEvents := make([]*eventESDT, 0, len(rawEvents))

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

		typedEvents = append(typedEvents, &eventESDT{
			otherAddress: accountAddress,
			identifier:   string(identifider),
			nonceAsBytes: nonceAsBytes,
			value:        value.String(),
		})
	}

	return typedEvents, nil
}

func (controller *transactionEventsController) extractEventsESDTNFTCreate(tx *transaction.ApiTransactionResult) ([]*eventESDT, error) {
	rawEvents := controller.findManyEventsByIdentifier(tx, transactionEventESDTNFTCreate)
	typedEvents := make([]*eventESDT, 0, len(rawEvents))

	for _, event := range rawEvents {
		numTopics := len(event.Topics)
		if numTopics != 4 {
			return nil, fmt.Errorf("%w: bad number of topics for %s event = %d", errCannotRecognizeEvent, transactionEventESDTNFTCreate, numTopics)
		}

		identifider := event.Topics[0]
		nonceAsBytes := event.Topics[1]
		valueBytes := event.Topics[2]
		// We ignore the 4th topic.

		value := big.NewInt(0).SetBytes(valueBytes)

		typedEvents = append(typedEvents, &eventESDT{
			otherAddress: event.Address,
			identifier:   string(identifider),
			nonceAsBytes: nonceAsBytes,
			value:        value.String(),
		})
	}

	return typedEvents, nil
}

func (controller *transactionEventsController) extractEventsESDTNFTBurn(tx *transaction.ApiTransactionResult) ([]*eventESDT, error) {
	rawEvents := controller.findManyEventsByIdentifier(tx, transactionEventESDTNFTBurn)
	typedEvents := make([]*eventESDT, 0, len(rawEvents))

	for _, event := range rawEvents {
		numTopics := len(event.Topics)
		if numTopics != 3 {
			return nil, fmt.Errorf("%w: bad number of topics for %s event = %d", errCannotRecognizeEvent, transactionEventESDTNFTBurn, numTopics)
		}

		identifider := event.Topics[0]
		nonceAsBytes := event.Topics[1]
		valueBytes := event.Topics[2]

		value := big.NewInt(0).SetBytes(valueBytes)

		typedEvents = append(typedEvents, &eventESDT{
			otherAddress: event.Address,
			identifier:   string(identifider),
			nonceAsBytes: nonceAsBytes,
			value:        value.String(),
		})
	}

	return typedEvents, nil
}

func (controller *transactionEventsController) extractEventsESDTNFTAddQuantity(tx *transaction.ApiTransactionResult) ([]*eventESDT, error) {
	rawEvents := controller.findManyEventsByIdentifier(tx, transactionEventESDTNFTAddQuantity)
	typedEvents := make([]*eventESDT, 0, len(rawEvents))

	for _, event := range rawEvents {
		numTopics := len(event.Topics)
		if numTopics != 3 {
			return nil, fmt.Errorf("%w: bad number of topics for %s event = %d", errCannotRecognizeEvent, transactionEventESDTNFTAddQuantity, numTopics)
		}

		identifider := event.Topics[0]
		nonceAsBytes := event.Topics[1]
		valueBytes := event.Topics[2]

		value := big.NewInt(0).SetBytes(valueBytes)

		typedEvents = append(typedEvents, &eventESDT{
			otherAddress: event.Address,
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
