package services

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ElrondNetwork/elrond-proxy-go/data"
)

type constructionMetadata struct {
	Sender         string `json:"sender"`
	Receiver       string `json:"receiver"`
	Nonce          uint64 `json:"nonce"`
	Amount         string `json:"amount"`
	CurrencySymbol string `json:"currencySymbol"`
	GasLimit       uint64 `json:"gasLimit"`
	GasPrice       uint64 `json:"gasPrice"`
	Data           []byte `json:"data"`
	ChainID        string `json:"chainID"`
	Version        int    `json:"version"`
}

func newConstructionMetadata(obj objectsMap) (*constructionMetadata, error) {
	result := &constructionMetadata{}
	err := fromObjectsMap(obj, result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (metadata *constructionMetadata) toTransactionJson() ([]byte, error) {
	tx, err := metadata.toTransaction()
	if err != nil {
		return nil, err
	}

	txJson, err := json.Marshal(tx)
	if err != nil {
		return nil, err
	}

	return txJson, nil
}

func (metadata *constructionMetadata) toTransaction() (*data.Transaction, error) {
	err := metadata.validate()
	if err != nil {
		return nil, err
	}

	tx := &data.Transaction{
		Sender:   metadata.Sender,
		Receiver: metadata.Receiver,
		Nonce:    metadata.Nonce,
		Value:    metadata.Amount,
		GasLimit: metadata.GasLimit,
		GasPrice: metadata.GasPrice,
		Data:     metadata.Data,
		ChainID:  metadata.ChainID,
		Version:  uint32(metadata.Version),
	}

	return tx, nil
}

func (metadata *constructionMetadata) validate() error {
	if len(metadata.Sender) == 0 {
		return errors.New("missing metadata: 'sender'")
	}
	if len(metadata.Receiver) == 0 {
		return errors.New("missing metadata: 'receiver'")
	}
	if metadata.GasLimit == 0 {
		return errors.New("missing metadata: 'gasLimit'")
	}
	if metadata.GasPrice == 0 {
		return errors.New("missing metadata: 'gasPrice'")
	}
	if metadata.Version != 1 {
		return fmt.Errorf("bad metadata: unexpected 'version' %v", metadata.Version)
	}
	if len(metadata.ChainID) == 0 {
		return errors.New("missing metadata: 'chainID'")
	}

	return nil
}
