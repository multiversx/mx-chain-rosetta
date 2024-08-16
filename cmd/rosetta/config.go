package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/multiversx/mx-chain-rosetta/server/resources"
)

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
