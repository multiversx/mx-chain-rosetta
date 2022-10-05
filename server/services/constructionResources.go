package services

import "encoding/json"

type constructionPreprocessMetadata struct {
	Sender   string `json:"sender"`
	Receiver string `json:"receiver"`
	Value    string `json:"value"`
	GasLimit uint64 `json:"gasLimit"`
	GasPrice uint64 `json:"gasPrice"`
	Data     []byte `json:"data"`
}

func newConstructionPreprocessMetadata(obj objectsMap) (*constructionPreprocessMetadata, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}

	result := &constructionPreprocessMetadata{}
	err = json.Unmarshal(data, result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

type constructionOptions struct {
	FirstOperationType string  `json:"type"`
	Sender             string  `json:"sender"`
	Receiver           string  `json:"receiver"`
	Value              string  `json:"value"`
	GasLimit           uint64  `json:"gasLimit"`
	GasPrice           uint64  `json:"gasPrice"`
	Data               []byte  `json:"data"`
	MaxFee             string  `json:"maxFee"`
	FeeMultiplier      float64 `json:"feeMultiplier"`
}

func newConstructionOptions(obj objectsMap) (*constructionOptions, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}

	result := &constructionOptions{}
	err = json.Unmarshal(data, result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (options *constructionOptions) toObjectsMap() (objectsMap, error) {
	data, err := json.Marshal(options)
	if err != nil {
		return nil, err
	}

	var result objectsMap
	err = json.Unmarshal(data, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

type constructionMetadata struct {
	Data     []byte `json:"data"`
	ChainID  string `json:"chainID"`
	Version  int    `json:"version"`
	Sender   string `json:"sender"`
	Receiver string `json:"receiver"`
	Value    string `json:"value"`
	Nonce    uint64 `json:"nonce"`
	GasLimit uint64 `json:"gasLimit"`
	GasPrice uint64 `json:"gasPrice"`
}

func (metadata *constructionMetadata) toObjectsMap() (objectsMap, error) {
	data, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}

	var result objectsMap
	err = json.Unmarshal(data, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}
