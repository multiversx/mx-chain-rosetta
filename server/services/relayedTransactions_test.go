package services

import (
	"encoding/hex"
	"testing"

	"github.com/multiversx/mx-chain-core-go/data/transaction"
	"github.com/stretchr/testify/require"
)

func Test_IsRelayedV1Transaction(t *testing.T) {
	t.Run("arbitrary tx", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{}
		require.False(t, isRelayedV1Transaction(tx))
	})

	t.Run("relayed v1 tx", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			Type:                        string(transaction.TxTypeNormal),
			ProcessingTypeOnSource:      transactionProcessingTypeRelayed,
			ProcessingTypeOnDestination: transactionProcessingTypeRelayed,
		}

		require.True(t, isRelayedV1Transaction(tx))
	})
}

func Test_ParseInnerTxOfRelayedV1(t *testing.T) {
	t.Run("arbitrary tx", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{}
		innerTx, err := parseInnerTxOfRelayedV1(tx)
		require.ErrorIs(t, err, errCannotParseRelayedV1)
		require.Nil(t, innerTx)
	})

	t.Run("relayed v1 tx (Alice to Bob, 1 EGLD)", func(t *testing.T) {
		tx := &transaction.ApiTransactionResult{
			Data: []byte("relayedTx@7b226e6f6e6365223a372c2273656e646572223a2241546c484c76396f686e63616d433877673970645168386b77704742356a6949496f3349484b594e6165453d222c227265636569766572223a2267456e574f65576d6d413063306a6b71764d354241707a61644b46574e534f69417643575163776d4750673d222c2276616c7565223a313030303030303030303030303030303030302c226761735072696365223a313030303030303030302c226761734c696d6974223a35303030302c2264617461223a22222c227369676e6174757265223a222b4161696451714c4d6150314b4f414d42506a557554774955775137724f6d62586976446c6b4944775a315a48353053366377714a4163576a496a744f732f435177502b79597a6643356730637571526b55437842413d3d222c22636861696e4944223a224d513d3d222c2276657273696f6e223a327d"),
		}

		innerTx, err := parseInnerTxOfRelayedV1(tx)
		require.NoError(t, err)
		require.NotNil(t, innerTx)

		require.Equal(t, uint64(7), innerTx.Nonce)
		require.Equal(t, "1000000000000000000", innerTx.Value.String())
		require.Equal(t, "0139472eff6886771a982f3083da5d421f24c29181e63888228dc81ca60d69e1", hex.EncodeToString(innerTx.SenderPubKey))
		require.Equal(t, "8049d639e5a6980d1cd2392abcce41029cda74a1563523a202f09641cc2618f8", hex.EncodeToString(innerTx.ReceiverPubKey))
	})
}
