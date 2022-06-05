package provider

import "time"

// Defined by the Network:
var transactionsHasherType = "blake2b"
var transactionsMarshalizerType = "gogo protobuf"
var pubKeyLength = 32
var nativeCurrencyNumDecimals = 18

// Defined in the scope of the Rosetta node:
var requestTimeoutInSeconds = 60
var nodeStatusCacheDuration = time.Duration(1 * time.Second)