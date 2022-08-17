package provider

import (
	"testing"

	"github.com/ElrondNetwork/rosetta/testscommon"
	"github.com/stretchr/testify/require"
)

func TestBuildUrlGetAccount(t *testing.T) {
	url := buildUrlGetAccount(testscommon.TestAddressAlice)
	require.Equal(t, "/address/erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th?onFinalBlock=true", url)
}

func TestBuildUrlGetAccountNativeBalance(t *testing.T) {
	url := buildUrlGetAccountNativeBalance(testscommon.TestAddressAlice)
	require.Equal(t, "/address/erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th/balance?onFinalBlock=true", url)
}

func TestBuildUrlGetAccountESDTBalance(t *testing.T) {
	url := buildUrlGetAccountESDTBalance(testscommon.TestAddressAlice, "ABC-abcdef")
	require.Equal(t, "/address/erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th/esdt/ABC-abcdef?onFinalBlock=true", url)
}
