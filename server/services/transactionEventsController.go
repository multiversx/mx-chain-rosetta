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
		if numTopics < numTopicsOfEventSCDeployBeforeSirius {
			// Before Sirius, there are 2 topics: contract address, deployer address.
			// After Sirius, there are 3 topics: contract address, deployer address, codehash (not used).
			return nil, fmt.Errorf("%w: bad number of topics for SCdeploy event = %d", errCannotRecognizeEvent, numTopics)
		}

		// "event.Address" is same as "event.Topics[0]"" (the address of the deployed contract).
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
	isBeforeSirius := !controller.provider.IsReleaseSiriusActive(tx.Epoch)
	rawEvents := controller.findManyEventsByIdentifier(tx, transactionEventTransferValueOnly)
	typedEvents := make([]*eventTransferValueOnly, 0)

	for _, event := range rawEvents {
		if string(event.Data) != transactionEventDataExecuteOnDestContext && string(event.Data) != transactionEventDataAsyncCall {
			continue
		}

		if isBeforeSirius {
			typedEvent, err := controller.decideEffectiveEventTransferValueOnlyBeforeSirius(event)
			if err != nil {
				return nil, err
			}

			if typedEvent != nil {
				typedEvents = append(typedEvents, typedEvent)
			}
		} else {
			typedEvent, err := controller.decideEffectiveEventTransferValueOnlyAfterSirius(event)
			if err != nil {
				return nil, err
			}

			if typedEvent != nil {
				typedEvents = append(typedEvents, typedEvent)
			}
		}
	}

	return typedEvents, nil
}

func (controller *transactionEventsController) decideEffectiveEventTransferValueOnlyBeforeSirius(event *transaction.Events) (*eventTransferValueOnly, error) {
	numTopics := len(event.Topics)

	if numTopics != numTopicsOfEventTransferValueOnlyBeforeSirius {
		return nil, fmt.Errorf("%w: bad number of topics for 'transferValueOnly' = %d", errCannotRecognizeEvent, numTopics)
	}

	senderPubKey := event.Topics[0]
	receiverPubKey := event.Topics[1]
	valueBytes := event.Topics[2]

	if len(valueBytes) == 0 {
		return nil, nil
	}

	isIntrashard := controller.provider.ComputeShardIdOfPubKey(senderPubKey) == controller.provider.ComputeShardIdOfPubKey(receiverPubKey)
	if !isIntrashard {
		return nil, nil
	}

	sender := controller.provider.ConvertPubKeyToAddress(senderPubKey)
	receiver := controller.provider.ConvertPubKeyToAddress(receiverPubKey)
	value := big.NewInt(0).SetBytes(valueBytes)

	return &eventTransferValueOnly{
		sender:   sender,
		receiver: receiver,
		value:    value.String(),
	}, nil
}

// See: https://github.com/multiversx/mx-specs/blob/main/releases/protocol/release-specs-v1.6.0-Sirius.md#17-logs--events-changes-5490
func (controller *transactionEventsController) decideEffectiveEventTransferValueOnlyAfterSirius(event *transaction.Events) (*eventTransferValueOnly, error) {
	numTopics := len(event.Topics)

	if numTopics != numTopicsOfEventTransferValueOnlyAfterSirius {
		return nil, fmt.Errorf("%w: bad number of topics for 'transferValueOnly' = %d", errCannotRecognizeEvent, numTopics)
	}

	valueBytes := event.Topics[0]
	receiverPubKey := event.Topics[1]

	if len(valueBytes) == 0 {
		return nil, nil
	}

	sender := event.Address
	senderPubKey, err := controller.provider.ConvertAddressToPubKey(sender)
	if err != nil {
		return nil, err
	}

	isIntrashard := controller.provider.ComputeShardIdOfPubKey(senderPubKey) == controller.provider.ComputeShardIdOfPubKey(receiverPubKey)
	if !isIntrashard {
		return nil, nil
	}

	receiver := controller.provider.ConvertPubKeyToAddress(receiverPubKey)
	value := big.NewInt(0).SetBytes(valueBytes)

	return &eventTransferValueOnly{
		sender:   sender,
		receiver: receiver,
		value:    value.String(),
	}, nil
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
		if numTopics != numTopicsOfEventESDTTransfer {
			return nil, fmt.Errorf("%w: bad number of topics for (ESDT|ESDTNFT)Transfer event = %d", errCannotRecognizeEvent, numTopics)
		}

		typedEvent, err := newEventESDTFromBasicTopics(event.Topics)
		if err != nil {
			return nil, err
		}

		receiverPubkey := event.Topics[3]
		typedEvent.receiverAddress = controller.provider.ConvertPubKeyToAddress(receiverPubkey)
		typedEvent.senderAddress = event.Address
		typedEvents = append(typedEvents, typedEvent)
	}

	// Then, handle multi transfers
	for _, event := range rawEventsMultiESDTNFTTransfer {
		numTopics := len(event.Topics)
		numTopicsExceptLast := numTopics - 1
		numTopicsPerTransfer := numTopicsPerTransferOfEventMultiESDTNFTTransfer

		if numTopicsExceptLast%numTopicsPerTransfer != 0 {
			return nil, fmt.Errorf("%w: bad number of topics for MultiESDTNFTTransfer event = %d", errCannotRecognizeEvent, numTopics)
		}

		numTransfers := numTopicsExceptLast / numTopicsPerTransfer
		receiverPubkey := event.Topics[numTopics-1]
		receiver := controller.provider.ConvertPubKeyToAddress(receiverPubkey)

		for i := 0; i < numTransfers; i++ {
			typedEvent, err := newEventESDTFromBasicTopics(event.Topics[i*numTopicsPerTransfer+0 : i*numTopicsPerTransfer+3])
			if err != nil {
				return nil, err
			}

			typedEvent.receiverAddress = receiver
			typedEvent.senderAddress = event.Address
			typedEvents = append(typedEvents, typedEvent)
		}
	}

	return typedEvents, nil
}

func (controller *transactionEventsController) extractEventsESDTLocalBurn(tx *transaction.ApiTransactionResult) ([]*eventESDT, error) {
	rawEvents := controller.findManyEventsByIdentifier(tx, transactionEventESDTLocalBurn)
	typedEvents := make([]*eventESDT, 0, len(rawEvents))

	for _, event := range rawEvents {
		numTopics := len(event.Topics)
		if numTopics != numTopicsOfEventESDTLocalBurn {
			return nil, fmt.Errorf("%w: bad number of topics for ESDTLocalBurn event = %d", errCannotRecognizeEvent, numTopics)
		}

		typedEvent, err := newEventESDTFromBasicTopics(event.Topics)
		if err != nil {
			return nil, err
		}

		typedEvent.otherAddress = event.Address
		typedEvents = append(typedEvents, typedEvent)
	}

	return typedEvents, nil
}

func (controller *transactionEventsController) extractEventsESDTLocalMint(tx *transaction.ApiTransactionResult) ([]*eventESDT, error) {
	rawEvents := controller.findManyEventsByIdentifier(tx, transactionEventESDTLocalMint)
	typedEvents := make([]*eventESDT, 0, len(rawEvents))

	for _, event := range rawEvents {
		numTopics := len(event.Topics)
		if numTopics != numTopicsOfEventESDTLocalMint {
			return nil, fmt.Errorf("%w: bad number of topics for ESDTLocalMint event = %d", errCannotRecognizeEvent, numTopics)
		}

		typedEvent, err := newEventESDTFromBasicTopics(event.Topics)
		if err != nil {
			return nil, err
		}

		typedEvent.otherAddress = event.Address
		typedEvents = append(typedEvents, typedEvent)
	}

	return typedEvents, nil
}

func (controller *transactionEventsController) extractEventsESDTWipe(tx *transaction.ApiTransactionResult) ([]*eventESDT, error) {
	rawEvents := controller.findManyEventsByIdentifier(tx, transactionEventESDTWipe)
	typedEvents := make([]*eventESDT, 0, len(rawEvents))

	for _, event := range rawEvents {
		numTopics := len(event.Topics)
		if numTopics != numTopicsOfEventESDTWipe {
			return nil, fmt.Errorf("%w: bad number of topics for ESDTWipe event = %d", errCannotRecognizeEvent, numTopics)
		}

		typedEvent, err := newEventESDTFromBasicTopics(event.Topics)
		if err != nil {
			return nil, err
		}

		accountPubkey := event.Topics[3]
		typedEvent.otherAddress = controller.provider.ConvertPubKeyToAddress(accountPubkey)
		typedEvents = append(typedEvents, typedEvent)
	}

	return typedEvents, nil
}

func (controller *transactionEventsController) extractEventsESDTNFTCreate(tx *transaction.ApiTransactionResult) ([]*eventESDT, error) {
	rawEvents := controller.findManyEventsByIdentifier(tx, transactionEventESDTNFTCreate)
	typedEvents := make([]*eventESDT, 0, len(rawEvents))

	for _, event := range rawEvents {
		numTopics := len(event.Topics)
		if numTopics != numTopicsOfEventESDTNFTCreate {
			return nil, fmt.Errorf("%w: bad number of topics for %s event = %d", errCannotRecognizeEvent, transactionEventESDTNFTCreate, numTopics)
		}

		typedEvent, err := newEventESDTFromBasicTopics(event.Topics)
		if err != nil {
			return nil, err
		}

		typedEvent.otherAddress = event.Address
		typedEvents = append(typedEvents, typedEvent)
	}

	return typedEvents, nil
}

func (controller *transactionEventsController) extractEventsESDTNFTBurn(tx *transaction.ApiTransactionResult) ([]*eventESDT, error) {
	rawEvents := controller.findManyEventsByIdentifier(tx, transactionEventESDTNFTBurn)
	typedEvents := make([]*eventESDT, 0, len(rawEvents))

	for _, event := range rawEvents {
		numTopics := len(event.Topics)
		if numTopics != numTopicsOfEventESDTNFTBurn {
			return nil, fmt.Errorf("%w: bad number of topics for %s event = %d", errCannotRecognizeEvent, transactionEventESDTNFTBurn, numTopics)
		}

		typedEvent, err := newEventESDTFromBasicTopics(event.Topics)
		if err != nil {
			return nil, err
		}

		typedEvent.otherAddress = event.Address
		typedEvents = append(typedEvents, typedEvent)
	}

	return typedEvents, nil
}

func (controller *transactionEventsController) extractEventsESDTNFTAddQuantity(tx *transaction.ApiTransactionResult) ([]*eventESDT, error) {
	rawEvents := controller.findManyEventsByIdentifier(tx, transactionEventESDTNFTAddQuantity)
	typedEvents := make([]*eventESDT, 0, len(rawEvents))

	for _, event := range rawEvents {
		numTopics := len(event.Topics)
		if numTopics != numTopicsOfEventESDTNFTAddQuantity {
			return nil, fmt.Errorf("%w: bad number of topics for %s event = %d", errCannotRecognizeEvent, transactionEventESDTNFTAddQuantity, numTopics)
		}

		typedEvent, err := newEventESDTFromBasicTopics(event.Topics)
		if err != nil {
			return nil, err
		}

		typedEvent.otherAddress = event.Address
		typedEvents = append(typedEvents, typedEvent)
	}

	return typedEvents, nil
}

func (controller *transactionEventsController) extractEventsClaimDeveloperRewards(tx *transaction.ApiTransactionResult) ([]*eventClaimDeveloperRewards, error) {
	rawEvents := controller.findManyEventsByIdentifier(tx, transactionEventClaimDeveloperRewards)
	typedEvents := make([]*eventClaimDeveloperRewards, 0, len(rawEvents))

	for _, event := range rawEvents {
		numTopics := len(event.Topics)
		if numTopics != numTopicsOfEventClaimDeveloperRewards {
			return nil, fmt.Errorf("%w: bad number of topics for %s event = %d", errCannotRecognizeEvent, transactionEventClaimDeveloperRewards, numTopics)
		}

		valueBytes := event.Topics[0]
		receiverPubkey := event.Topics[1]

		value := big.NewInt(0).SetBytes(valueBytes)
		receiver := controller.provider.ConvertPubKeyToAddress(receiverPubkey)

		typedEvents = append(typedEvents, &eventClaimDeveloperRewards{
			value:           value.String(),
			receiverAddress: receiver,
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
