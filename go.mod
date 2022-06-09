module github.com/ElrondNetwork/rosetta

go 1.15

require (
	github.com/ElrondNetwork/elrond-go v1.2.4-0.20210702131210-721bd99ea5bd
	github.com/ElrondNetwork/elrond-go-logger v1.0.4
	github.com/ElrondNetwork/elrond-proxy-go v1.1.20-0.20220609134827-bb6f12e86a86
	github.com/coinbase/rosetta-sdk-go v0.7.9
	github.com/stretchr/testify v1.7.0
	github.com/urfave/cli v1.22.5
)

replace github.com/ElrondNetwork/arwen-wasm-vm/v1_3 v1.3.19 => github.com/ElrondNetwork/arwen-wasm-vm v1.3.20-0.20210702122719-c891907234fa
