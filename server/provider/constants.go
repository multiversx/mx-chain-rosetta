package provider

var (
	nativeCurrencyNumDecimals = 18
	genesisBlockNonce         = 0
)

type MiniblockProcessingType string

const (
	Normal    MiniblockProcessingType = "Normal"
	Scheduled MiniblockProcessingType = "Scheduled"
	Processed MiniblockProcessingType = "Processed"
)
