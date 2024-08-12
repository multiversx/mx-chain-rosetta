package provider

import (
	"testing"

	"github.com/multiversx/mx-chain-rosetta/server/resources"
	"github.com/multiversx/mx-chain-rosetta/testscommon"
	"github.com/stretchr/testify/require"
)

func TestBuildUrlGetAccount(t *testing.T) {
	url := buildUrlGetAccount(testscommon.TestAddressAlice)
	require.Equal(t, "/address/erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th?onFinalBlock=true", url)
}

func TestBuildUrlGetAccountNativeBalance(t *testing.T) {
	optionsOnFinal := resources.NewAccountQueryOptionsOnFinalBlock()
	optionsAtBlockNonce := resources.NewAccountQueryOptionsWithBlockNonce(7)
	optionsAtBlockHash := resources.NewAccountQueryOptionsWithBlockHash([]byte{0xaa, 0xbb, 0xcc, 0xdd})

	url := buildUrlGetAccountNativeBalance(testscommon.TestAddressAlice, optionsOnFinal)
	require.Equal(t, "/address/erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th?onFinalBlock=true", url)

	url = buildUrlGetAccountNativeBalance(testscommon.TestAddressAlice, optionsAtBlockNonce)
	require.Equal(t, "/address/erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th?blockNonce=7", url)

	url = buildUrlGetAccountNativeBalance(testscommon.TestAddressAlice, optionsAtBlockHash)
	require.Equal(t, "/address/erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th?blockHash=aabbccdd", url)

	url = buildUrlGetAccountNativeBalance(testscommon.TestAddressAlice, resources.AccountQueryOptions{})
	require.Equal(t, "/address/erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th", url)
}

func TestBuildUrlGetAccountFungibleTokenBalance(t *testing.T) {
	optionsOnFinal := resources.NewAccountQueryOptionsOnFinalBlock()
	optionsAtBlockNonce := resources.NewAccountQueryOptionsWithBlockNonce(7)
	optionsAtBlockHash := resources.NewAccountQueryOptionsWithBlockHash([]byte{0xaa, 0xbb, 0xcc, 0xdd})

	url := buildUrlGetAccountFungibleTokenBalance(testscommon.TestAddressAlice, "ABC-abcdef", optionsOnFinal)
	require.Equal(t, "/address/erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th/esdt/ABC-abcdef?onFinalBlock=true", url)

	url = buildUrlGetAccountFungibleTokenBalance(testscommon.TestAddressAlice, "ABC-abcdef", optionsAtBlockNonce)
	require.Equal(t, "/address/erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th/esdt/ABC-abcdef?blockNonce=7", url)

	url = buildUrlGetAccountFungibleTokenBalance(testscommon.TestAddressAlice, "ABC-abcdef", optionsAtBlockHash)
	require.Equal(t, "/address/erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th/esdt/ABC-abcdef?blockHash=aabbccdd", url)

	url = buildUrlGetAccountFungibleTokenBalance(testscommon.TestAddressAlice, "ABC-abcdef", resources.AccountQueryOptions{})
	require.Equal(t, "/address/erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th/esdt/ABC-abcdef", url)
}

func TestBuildUrlGetAccountNonFungibleTokenBalance(t *testing.T) {
	optionsOnFinal := resources.NewAccountQueryOptionsOnFinalBlock()
	optionsAtBlockNonce := resources.NewAccountQueryOptionsWithBlockNonce(7)
	optionsAtBlockHash := resources.NewAccountQueryOptionsWithBlockHash([]byte{0xaa, 0xbb, 0xcc, 0xdd})

	url := buildUrlGetAccountNonFungibleTokenBalance(testscommon.TestAddressAlice, "ABC-abcdef", 10, optionsOnFinal)
	require.Equal(t, "/address/erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th/nft/ABC-abcdef/nonce/10?onFinalBlock=true", url)

	url = buildUrlGetAccountNonFungibleTokenBalance(testscommon.TestAddressAlice, "ABC-abcdef", 10, optionsAtBlockNonce)
	require.Equal(t, "/address/erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th/nft/ABC-abcdef/nonce/10?blockNonce=7", url)

	url = buildUrlGetAccountNonFungibleTokenBalance(testscommon.TestAddressAlice, "ABC-abcdef", 10, optionsAtBlockHash)
	require.Equal(t, "/address/erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th/nft/ABC-abcdef/nonce/10?blockHash=aabbccdd", url)

	url = buildUrlGetAccountNonFungibleTokenBalance(testscommon.TestAddressAlice, "ABC-abcdef", 10, resources.AccountQueryOptions{})
	require.Equal(t, "/address/erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th/nft/ABC-abcdef/nonce/10", url)
}
