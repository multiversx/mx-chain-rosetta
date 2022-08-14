package provider

var (
	nativeCurrencyNumDecimals              = 18
	genesisBlockNonce                      = 0
	oldestPossibleNonceWithHistoricalState = int64(1)
)

type MiniblockProcessingType string

const (
	Normal    MiniblockProcessingType = "Normal"
	Scheduled MiniblockProcessingType = "Scheduled"
	Processed MiniblockProcessingType = "Processed"
)
