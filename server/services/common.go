package services

type objectsMap map[string]interface{}

func isZeroAmount(amount string) bool {
	return amount == "" || amount == "0" || amount == "-0"
}

func isNonZeroAmount(amount string) bool {
	return !isZeroAmount(amount)
}
