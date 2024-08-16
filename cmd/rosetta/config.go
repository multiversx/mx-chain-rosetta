package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/multiversx/mx-chain-rosetta/server/resources"
)

func decideCustomCurrencies(configFileCustomCurrencies string) ([]resources.Currency, error) {
	if len(configFileCustomCurrencies) == 0 {
		return make([]resources.Currency, 0), nil
	}

	return loadConfigOfCustomCurrencies(configFileCustomCurrencies)
}

func loadConfigOfCustomCurrencies(configFile string) ([]resources.Currency, error) {
	fileContent, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("error when reading custom currencies config file: %w", err)
	}

	var customCurrencies []resources.Currency

	err = json.Unmarshal(fileContent, &customCurrencies)
	if err != nil {
		return nil, fmt.Errorf("error when loading custom currencies from file: %w", err)
	}

	return customCurrencies, nil
}
