package services

type constructionPreprocessMetadata struct {
	Sender         string `json:"sender"`
	Receiver       string `json:"receiver"`
	Amount         string `json:"amount"`
	CurrencySymbol string `json:"currencySymbol"`
	GasLimit       uint64 `json:"gasLimit"`
	GasPrice       uint64 `json:"gasPrice"`
	Data           []byte `json:"data"`
}

func newConstructionPreprocessMetadata(obj objectsMap) (*constructionPreprocessMetadata, error) {
	result := &constructionPreprocessMetadata{}
	err := fromObjectsMap(obj, result)
	if err != nil {
		return nil, err
	}

	return result, nil
}
