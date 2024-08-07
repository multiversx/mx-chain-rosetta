package main

import (
	"encoding/json"
	"os"

	"github.com/multiversx/mx-chain-rosetta/server/resources"
)

func loadConfigOfCustomCurrencies(configFile string) ([]resources.Currency, error) {
	fileContent, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	var customCurrencies []resources.Currency

	err = json.Unmarshal(fileContent, &customCurrencies)
	if err != nil {
		return nil, err
	}

	return customCurrencies, nil
}
