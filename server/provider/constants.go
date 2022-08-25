package provider

var (
	nativeCurrencyNumDecimals = 18
	genesisBlockNonce         = 0
	// Block with nonce = 0 is not actually retrievable from the observer
	oldestPossibleNonceWithHistoricalState = uint64(1)
	blocksCacheSize                        = 4096
)

type MiniblockProcessingType string

const (
	Normal    MiniblockProcessingType = "Normal"
	Scheduled MiniblockProcessingType = "Scheduled"
	Processed MiniblockProcessingType = "Processed"
)
