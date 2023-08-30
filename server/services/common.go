package services

import (
	"encoding/json"
	"math/big"
	"strings"
)

type objectsMap map[string]interface{}

func toObjectsMap(value interface{}) (objectsMap, error) {
	data, err := json.Marshal(value)
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

func fromObjectsMap(obj objectsMap, value interface{}) error {
	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, value)
	if err != nil {
		return err
	}

	return nil
}

func isZeroAmount(amount string) bool {
	if amount == "" {
		return true
	}

	value, ok := big.NewInt(0).SetString(amount, 10)
	if ok {
		return value.Sign() == 0
	}

	return false
}

func isZeroBigIntOrNil(value *big.Int) bool {
	if value == nil {
		return true
	}

	return value.Sign() == 0
}

func getMagnitudeOfAmount(amount string) string {
	return strings.Trim(amount, "-")
}

func multiplyUint64(a uint64, b uint64) *big.Int {
	return big.NewInt(0).Mul(big.NewInt(0).SetUint64(a), big.NewInt(0).SetUint64(b))
}

func addBigInt(a *big.Int, b *big.Int) *big.Int {
	return big.NewInt(0).Add(a, b)
}
