package services

import (
	"github.com/ElrondNetwork/elrond-go-core/core"
	"github.com/ElrondNetwork/elrond-go-core/core/pubkeyConverter"
)

var bech32PubkeyConverter, _ = pubkeyConverter.NewBech32PubkeyConverter(32, log)

func isSmartContractAddress(address string) bool {
	pubkey, err := bech32PubkeyConverter.Decode(address)
	if err != nil {
		// E.g., when address = "metachain"
		return false
	}

	return core.IsSmartContractAddress(pubkey)
}

func isUserAddress(address string) bool {
	pubkey, err := bech32PubkeyConverter.Decode(address)
	if err != nil {
		// E.g., when address = "metachain"
		return false
	}

	return !core.IsSmartContractAddress(pubkey)
}
