package provider

import (
	"testing"

	"github.com/ElrondNetwork/rosetta/common"
	"github.com/ElrondNetwork/rosetta/server/resources"
	"github.com/ElrondNetwork/rosetta/testscommon"
	"github.com/stretchr/testify/require"
)

func TestBuildUrlGetAccount(t *testing.T) {
	url := buildUrlGetAccount(testscommon.TestAddressAlice)
	require.Equal(t, "/address/erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th?onFinalBlock=true", url)
}

func TestBuildUrlGetAccountNativeBalance(t *testing.T) {
	optionsOnFinal := resources.AccountQueryOptions{OnFinalBlock: true}
	optionsAtBlockNonce := resources.AccountQueryOptions{BlockNonce: common.OptionalUint64{Value: 7, HasValue: true}}
	optionsAtBlockHash := resources.AccountQueryOptions{BlockHash: []byte{0xaa, 0xbb, 0xcc, 0xdd}}

	url := buildUrlGetAccountNativeBalance(testscommon.TestAddressAlice, optionsOnFinal)
	require.Equal(t, "/address/erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th/balance?onFinalBlock=true", url)

	url = buildUrlGetAccountNativeBalance(testscommon.TestAddressAlice, optionsAtBlockNonce)
	require.Equal(t, "/address/erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th/balance?blockNonce=7", url)

	url = buildUrlGetAccountNativeBalance(testscommon.TestAddressAlice, optionsAtBlockHash)
	require.Equal(t, "/address/erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th/balance?blockHash=aabbccdd", url)
}

func TestBuildUrlGetAccountESDTBalance(t *testing.T) {
	optionsOnFinal := resources.AccountQueryOptions{OnFinalBlock: true}
	optionsAtBlockNonce := resources.AccountQueryOptions{BlockNonce: common.OptionalUint64{Value: 7, HasValue: true}}
	optionsAtBlockHash := resources.AccountQueryOptions{BlockHash: []byte{0xaa, 0xbb, 0xcc, 0xdd}}

	url := buildUrlGetAccountESDTBalance(testscommon.TestAddressAlice, "ABC-abcdef", optionsOnFinal)
	require.Equal(t, "/address/erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th/esdt/ABC-abcdef?onFinalBlock=true", url)

	url = buildUrlGetAccountESDTBalance(testscommon.TestAddressAlice, "ABC-abcdef", optionsAtBlockNonce)
	require.Equal(t, "/address/erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th/esdt/ABC-abcdef?blockNonce=7", url)

	url = buildUrlGetAccountESDTBalance(testscommon.TestAddressAlice, "ABC-abcdef", optionsAtBlockHash)
	require.Equal(t, "/address/erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th/esdt/ABC-abcdef?blockHash=aabbccdd", url)
}
