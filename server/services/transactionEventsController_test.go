package services

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/multiversx/mx-chain-core-go/data/transaction"
	"github.com/multiversx/mx-chain-rosetta/testscommon"
	"github.com/stretchr/testify/require"
)

func TestTransactionEventsController_HasAnySignalError(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	controller := newTransactionEventsController(networkProvider)

	t.Run("arbitrary tx", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{}
		txMatches := controller.hasAnySignalError(tx)
		require.False(t, txMatches)
	})

	t.Run("tx with event 'signalError'", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			Logs: &transaction.ApiLogs{

				Events: []*transaction.Events{
					{
						Identifier: transactionEventSignalError,
					},
				},
			},
		}

		txMatches := controller.hasAnySignalError(tx)
		require.True(t, txMatches)
	})
}

func TestTransactionEventsController_FindManyEventsByIdentifier(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	controller := newTransactionEventsController(networkProvider)

	t.Run("no matching events", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			Logs: &transaction.ApiLogs{
				Events: []*transaction.Events{
					{
						Identifier: "a",
					},
				},
			},
		}

		events := controller.findManyEventsByIdentifier(tx, "b")
		require.Len(t, events, 0)
	})

	t.Run("more than one matching event", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			Logs: &transaction.ApiLogs{
				Events: []*transaction.Events{
					{
						Identifier: "a",
						Data:       []byte("1"),
					},
					{
						Identifier: "a",
						Data:       []byte("2"),
					},
					{
						Identifier: "b",
						Data:       []byte("3"),
					},
				},
			},
		}

		events := controller.findManyEventsByIdentifier(tx, "a")
		require.Len(t, events, 2)
		require.Equal(t, []byte("1"), events[0].Data)
		require.Equal(t, []byte("2"), events[1].Data)
	})
}

func TestTransactionEventsController_HasSignalErrorOfSendingValueToNonPayableContract(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	controller := newTransactionEventsController(networkProvider)

	t.Run("arbitrary tx", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{}
		txMatches := controller.hasSignalErrorOfSendingValueToNonPayableContract(tx)
		require.False(t, txMatches)
	})

	t.Run("invalid tx with event 'sending value to non-payable contract'", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			Logs: &transaction.ApiLogs{

				Events: []*transaction.Events{
					{
						Identifier: transactionEventSignalError,
						Data:       []byte(sendingValueToNonPayableContractDataPrefix + "aaaabbbbccccdddd"),
					},
				},
			},
		}

		txMatches := controller.hasSignalErrorOfSendingValueToNonPayableContract(tx)
		require.True(t, txMatches)
	})
}

func TestTransactionEventsController_HasSignalErrorOfMetaTransactionIsInvalid(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	controller := newTransactionEventsController(networkProvider)

	t.Run("arbitrary tx", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{}
		txMatches := controller.hasSignalErrorOfMetaTransactionIsInvalid(tx)
		require.False(t, txMatches)
	})

	t.Run("invalid tx with event 'meta transaction is invalid'", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			Logs: &transaction.ApiLogs{

				Events: []*transaction.Events{
					{
						Identifier: transactionEventSignalError,
						Topics: [][]byte{
							[]byte(transactionEventTopicInvalidMetaTransaction),
						},
					},
				},
			},
		}

		txMatches := controller.hasSignalErrorOfMetaTransactionIsInvalid(tx)
		require.True(t, txMatches)
	})

	t.Run("invalid tx with event 'meta transaction is invalid: not enough gas'", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			Logs: &transaction.ApiLogs{

				Events: []*transaction.Events{
					{
						Identifier: transactionEventSignalError,
						Topics: [][]byte{
							[]byte(transactionEventTopicInvalidMetaTransactionNotEnoughGas),
						},
					},
				},
			},
		}

		txMatches := controller.hasSignalErrorOfMetaTransactionIsInvalid(tx)
		require.True(t, txMatches)
	})
}

func TestEventHasTopic(t *testing.T) {
	event := transaction.Events{
		Identifier: transactionEventSignalError,
		Topics: [][]byte{
			[]byte("foo"),
		},
	}

	require.True(t, eventHasTopic(&event, "foo"))
	require.False(t, eventHasTopic(&event, "bar"))
}

func TestTransactionEventsController_ExtractEvents(t *testing.T) {
	networkProvider := testscommon.NewNetworkProviderMock()
	controller := newTransactionEventsController(networkProvider)

	networkProvider.MockActivationEpochSirius = 42

	t.Run("SCDeploy", func(t *testing.T) {
		topic0, _ := hex.DecodeString("00000000000000000500def8dad1161f8f0c38f3e6e73515ed81058f0b5606b8")
		topic1, _ := hex.DecodeString("5cf4abc83e50c5309d807fc3f676988759a1e301001bc9a0265804f42af806b8")
		topic2, _ := hex.DecodeString("be5560e0d7d3857d438a3678269039f8f80ded90dcbc2cda268a0847ba9cb379")

		tx := &transaction.ApiTransactionResult{
			Logs: &transaction.ApiLogs{
				Events: []*transaction.Events{
					{
						Identifier: "SCDeploy",
						Address:    "erd1qqqqqqqqqqqqqpgqmmud45gkr78scw8numnn290dsyzc7z6kq6uqw2jcza",
						Topics: [][]byte{
							topic0,
							topic1,
							topic2,
						},
					},
				},
			},
		}

		events, err := controller.extractEventSCDeploy(tx)
		require.NoError(t, err)
		require.Len(t, events, 1)
		require.Equal(t, "erd1qqqqqqqqqqqqqpgqmmud45gkr78scw8numnn290dsyzc7z6kq6uqw2jcza", events[0].contractAddress)
		require.Equal(t, "erd1tn62hjp72rznp8vq0lplva5csav6rccpqqdungpxtqz0g2hcq6uq9k4cc6", events[0].deployerAddress)
	})

	t.Run("transferValueOnly, before Sirius (not handled at all)", func(t *testing.T) {
		topic0 := testscommon.TestContractFooShard0.PubKey
		topic1 := testscommon.TestContractBarShard0.PubKey
		topic2 := big.NewInt(100).Bytes()

		tx := &transaction.ApiTransactionResult{
			Epoch: 41,
			Logs: &transaction.ApiLogs{
				Events: []*transaction.Events{
					{
						Identifier: "transferValueOnly",
						Address:    "erd1qqqqqqqqqqqqqpgqmmud45gkr78scw8numnn290dsyzc7z6kq6uqw2jcza",
						Topics: [][]byte{
							topic0,
							topic1,
							topic2,
						},
					},
				},
			},
		}

		events, err := controller.extractEventTransferValueOnly(tx)
		require.NoError(t, err)
		require.Len(t, events, 0)
	})

	t.Run("transferValueOnly, after Sirius, effective (intra-shard ExecuteOnDestContext)", func(t *testing.T) {
		topic0 := big.NewInt(100).Bytes()
		topic1 := testscommon.TestContractBarShard0.PubKey

		tx := &transaction.ApiTransactionResult{
			Epoch: 43,
			Logs: &transaction.ApiLogs{
				Events: []*transaction.Events{
					{
						Identifier: "transferValueOnly",
						Address:    testscommon.TestContractFooShard0.Address,
						Topics: [][]byte{
							topic0,
							topic1,
						},
						Data: []byte("ExecuteOnDestContext"),
					},
				},
			},
		}

		events, err := controller.extractEventTransferValueOnly(tx)
		require.NoError(t, err)
		require.Len(t, events, 1)
		require.Equal(t, testscommon.TestContractFooShard0.Address, events[0].sender)
		require.Equal(t, testscommon.TestContractBarShard0.Address, events[0].receiver)
		require.Equal(t, "100", events[0].value)
	})

	t.Run("transferValueOnly, after Sirius, ineffective (cross-shard AsyncCall)", func(t *testing.T) {
		topic0 := big.NewInt(100).Bytes()
		topic1 := testscommon.TestContractBarShard1.PubKey

		tx := &transaction.ApiTransactionResult{
			Epoch: 43,
			Logs: &transaction.ApiLogs{
				Events: []*transaction.Events{
					{
						Identifier: "transferValueOnly",
						Address:    testscommon.TestContractFooShard0.Address,
						Topics: [][]byte{
							topic0,
							topic1,
						},
						Data: []byte("AsyncCall"),
					},
				},
			},
		}

		events, err := controller.extractEventTransferValueOnly(tx)
		require.NoError(t, err)
		require.Len(t, events, 0)
	})

	t.Run("transferValueOnly, after Sirius, ineffective, intra-shard BackTransfer", func(t *testing.T) {
		topic0 := big.NewInt(100).Bytes()
		topic1 := testscommon.TestContractBarShard0.PubKey

		tx := &transaction.ApiTransactionResult{
			Epoch: 43,
			Logs: &transaction.ApiLogs{
				Events: []*transaction.Events{
					{
						Identifier: "transferValueOnly",
						Address:    testscommon.TestContractFooShard0.Address,
						Topics: [][]byte{
							topic0,
							topic1,
						},
						Data: []byte("BackTransfer"),
					},
				},
			},
		}

		events, err := controller.extractEventTransferValueOnly(tx)
		require.NoError(t, err)
		require.Len(t, events, 0)
	})

	t.Run("ESDTNFTCreate", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			Logs: &transaction.ApiLogs{
				Events: []*transaction.Events{
					{
						Identifier: "ESDTNFTCreate",
						Address:    testscommon.TestAddressAlice,
						Topics: [][]byte{
							[]byte("EXAMPLE-abcdef"),
							{0x2a},
							{0x1},
							{0x0},
						},
					},
				},
			},
		}

		events, err := controller.extractEventsESDTNFTCreate(tx)
		require.NoError(t, err)
		require.Len(t, events, 1)
		require.Equal(t, "EXAMPLE-abcdef", events[0].identifier)
		require.Equal(t, testscommon.TestAddressAlice, events[0].otherAddress)
		require.Equal(t, []byte{0x2a}, events[0].nonceAsBytes)
		require.Equal(t, "1", events[0].value)
	})

	t.Run("ESDTNFTBurn", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			Logs: &transaction.ApiLogs{
				Events: []*transaction.Events{
					{
						Identifier: "ESDTNFTBurn",
						Address:    testscommon.TestAddressAlice,
						Topics: [][]byte{
							[]byte("EXAMPLE-abcdef"),
							{0x2a},
							{0x1},
						},
					},
				},
			},
		}

		events, err := controller.extractEventsESDTNFTBurn(tx)
		require.NoError(t, err)
		require.Len(t, events, 1)
		require.Equal(t, "EXAMPLE-abcdef", events[0].identifier)
		require.Equal(t, testscommon.TestAddressAlice, events[0].otherAddress)
		require.Equal(t, []byte{0x2a}, events[0].nonceAsBytes)
		require.Equal(t, "1", events[0].value)
	})

	t.Run("ESDTNFTAddQuantity", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			Logs: &transaction.ApiLogs{
				Events: []*transaction.Events{
					{
						Identifier: "ESDTNFTAddQuantity",
						Address:    testscommon.TestAddressAlice,
						Topics: [][]byte{
							[]byte("EXAMPLE-aabbcc"),
							{0x2a},
							{0x64},
						},
					},
				},
			},
		}

		events, err := controller.extractEventsESDTNFTAddQuantity(tx)
		require.NoError(t, err)
		require.Len(t, events, 1)
		require.Equal(t, "EXAMPLE-aabbcc", events[0].identifier)
		require.Equal(t, testscommon.TestAddressAlice, events[0].otherAddress)
		require.Equal(t, []byte{0x2a}, events[0].nonceAsBytes)
		require.Equal(t, "100", events[0].value)
	})

	t.Run("ClaimDeveloperRewards", func(t *testing.T) {
		topic0 := []byte{0x64}
		topic1, _ := hex.DecodeString("5cf4abc83e50c5309d807fc3f676988759a1e301001bc9a0265804f42af806b8")

		tx := &transaction.ApiTransactionResult{
			Logs: &transaction.ApiLogs{
				Events: []*transaction.Events{
					{
						Identifier: "ClaimDeveloperRewards",
						Topics: [][]byte{
							topic0,
							topic1,
						},
					},
				},
			},
		}

		events, err := controller.extractEventsClaimDeveloperRewards(tx)
		require.NoError(t, err)
		require.Len(t, events, 1)
		require.Equal(t, "100", events[0].value)
		require.Equal(t, "erd1tn62hjp72rznp8vq0lplva5csav6rccpqqdungpxtqz0g2hcq6uq9k4cc6", events[0].receiverAddress)
	})
}
