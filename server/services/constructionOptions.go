package services

import (
	"errors"
)

type constructionOptions struct {
	Sender         string `json:"sender"`
	Receiver       string `json:"receiver"`
	Amount         string `json:"amount"`
	CurrencySymbol string `json:"currencySymbol"`
	GasLimit       uint64 `json:"gasLimit"`
	GasPrice       uint64 `json:"gasPrice"`
	Data           []byte `json:"data"`
}

func newConstructionOptions(obj objectsMap) (*constructionOptions, error) {
	result := &constructionOptions{}
	err := fromObjectsMap(obj, result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (options *constructionOptions) coalesceGasLimit(estimatedGasLimit uint64) uint64 {
	if options.GasLimit == 0 {
		return estimatedGasLimit
	}

	return options.GasLimit
}

func (options *constructionOptions) coalesceGasPrice(minGasPrice uint64) uint64 {
	if options.GasPrice == 0 {
		return minGasPrice
	}

	return options.GasPrice
}

func (options *constructionOptions) validate(nativeCurrencySymbol string) error {
	if len(options.Sender) == 0 {
		return errors.New("missing option 'sender'")
	}
	if len(options.Receiver) == 0 {
		return errors.New("missing option 'receive'")
	}
	if isZeroAmount(options.Amount) {
		return errors.New("missing option 'amount'")
	}
	if len(options.CurrencySymbol) == 0 {
		return errors.New("missing option 'currencySymbol'")
	}
	if len(options.Data) > 0 && options.CurrencySymbol != nativeCurrencySymbol {
		return errors.New("for custom currencies, cannot populate option 'data'")
	}

	return nil
}
