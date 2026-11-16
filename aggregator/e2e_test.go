package aggregator

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/coinbase/rosetta-sdk-go/asserter"
	"github.com/coinbase/rosetta-sdk-go/client"
	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/multiversx/mx-chain-core-go/data/api"
	"github.com/multiversx/mx-chain-core-go/data/transaction"
	"github.com/multiversx/mx-chain-proxy-go/data"
	"github.com/multiversx/mx-chain-rosetta/aggregator/backend"
	"github.com/multiversx/mx-chain-rosetta/aggregator/config"
	"github.com/multiversx/mx-chain-rosetta/aggregator/index"
	"github.com/multiversx/mx-chain-rosetta/aggregator/metachain"
	"github.com/multiversx/mx-chain-rosetta/aggregator/mock"
	"github.com/multiversx/mx-chain-rosetta/aggregator/services"
	"github.com/multiversx/mx-chain-rosetta/server/factory"
	"github.com/multiversx/mx-chain-rosetta/server/resources"
	shardTestscommon "github.com/multiversx/mx-chain-rosetta/testscommon"
	"github.com/stretchr/testify/require"
)

const (
	e2eNativeSymbol = "XeGLD"
	e2eTxHash       = "1111111111111111111111111111111111111111111111111111111111111111"
	e2eIntraTxHash  = "2222222222222222222222222222222222222222222222222222222222222222"
	e2eResultHash   = "3333333333333333333333333333333333333333333333333333333333333333"
	e2eCallHash     = "4444444444444444444444444444444444444444444444444444444444444444"
	e2eSubmitHash   = "5555555555555555555555555555555555555555555555555555555555555555"
)

// e2eShardInstance is a real shard instance (the services of this repository, served over HTTP), on top of a mocked network provider.
// It stands for the "rosetta" binary observing one shard.
type e2eShardInstance struct {
	shard     uint32
	server    *httptest.Server
	submitted []*data.Transaction

	setBlocks          func(blocks ...*api.Block)
	setBalance         func(address string, balance string, atBlock *api.Block)
	setAccount         func(address string, nonce uint64)
	setGenesisBalances func(balances []*resources.GenesisBalance)
}

func newE2EShardInstance(t *testing.T, shard uint32) *e2eShardInstance {
	provider := shardTestscommon.NewNetworkProviderMock()
	provider.MockObservedActualShard = shard
	provider.MockNumShards = 3
	provider.MockNativeCurrencySymbol = e2eNativeSymbol
	provider.MockGenesisBlockHash = mock.FakeHash("shard-genesis", shard)
	provider.MockComputedTransactionHash = e2eSubmitHash
	provider.MockNodeStatus.ObserverPublicKey = mock.FakeHash("observer", shard)
	provider.MockNodeStatus.Version = "v1.11.1-mock"
	provider.MockNodeStatus.LatestBlock.Hash = provider.MockGenesisBlockHash
	provider.MockNodeStatus.OldestBlockWithHistoricalState.Hash = provider.MockGenesisBlockHash

	instance := &e2eShardInstance{shard: shard}

	provider.SendTransactionCalled = func(tx *data.Transaction) (string, error) {
		instance.submitted = append(instance.submitted, tx)
		return e2eSubmitHash, nil
	}

	instance.setBlocks = func(blocks ...*api.Block) {
		for _, block := range blocks {
			provider.MockBlocksByNonce[block.Nonce] = block
			provider.MockBlocksByHash[block.Hash] = block

			if block.Nonce >= provider.MockNodeStatus.LatestBlock.Nonce {
				provider.MockNodeStatus.LatestBlock = resources.BlockSummary{
					Nonce:             block.Nonce,
					Hash:              block.Hash,
					PreviousBlockHash: block.PrevBlockHash,
					Timestamp:         block.Timestamp,
				}
			}
		}
	}

	instance.setBalance = func(address string, balance string, atBlock *api.Block) {
		provider.MockAccountsNativeBalances[address] = &resources.AccountBalanceOnBlock{Balance: balance}
		provider.MockNextAccountBlockCoordinates = &resources.BlockCoordinates{Nonce: atBlock.Nonce, Hash: atBlock.Hash}
	}

	instance.setAccount = func(address string, nonce uint64) {
		provider.MockAccountsByAddress[address] = &resources.Account{Address: address, Nonce: nonce, Balance: "1000000"}
	}

	instance.setGenesisBalances = func(balances []*resources.GenesisBalance) {
		provider.MockGenesisBalances = balances
	}

	// The same wiring as in the "rosetta" binary.
	controllers, err := factory.CreateControllers(provider)
	require.NoError(t, err)

	instance.server = httptest.NewServer(server.NewRouter(controllers...))
	t.Cleanup(instance.server.Close)
	return instance
}

func newE2EShardBlock(shard uint32, nonce uint64, txs ...*transaction.ApiTransactionResult) *api.Block {
	return &api.Block{
		Nonce:         nonce,
		Hash:          mock.FakeHash("e2e-shard-block", shard, nonce),
		PrevBlockHash: mock.FakeHash("e2e-shard-block", shard, nonce-1),
		Shard:         shard,
		Epoch:         1,
		Round:         nonce + 10,
		Status:        "on-chain",
		Timestamp:     mock.GenesisTimestamp + int64(nonce)*6,
		MiniBlocks: []*api.MiniBlock{
			{Transactions: txs},
		},
	}
}

func notarize(blocks ...*api.Block) []metachain.NotarizedBlock {
	notarized := make([]metachain.NotarizedBlock, 0, len(blocks))
	for _, block := range blocks {
		notarized = append(notarized, metachain.NotarizedBlock{Shard: block.Shard, Nonce: block.Nonce, Hash: block.Hash})
	}

	return notarized
}

func newE2EMetaBlock(nonce uint64, notarized []metachain.NotarizedBlock) *metachain.Block {
	return &metachain.Block{
		Nonce:       nonce,
		Hash:        mock.FakeHash("e2e-meta-block", nonce),
		PrevHash:    mock.FakeHash("e2e-meta-block", nonce-1),
		TimestampMs: (mock.GenesisTimestamp + int64(nonce)*6) * 1000,
		Epoch:       1,
		Round:       nonce + 10,
		Notarized:   notarized,
	}
}

// TestEndToEnd_WithRealShardInstances runs the whole stack over HTTP, on the loopback interface:
//
//	Rosetta client -> aggregator -> 3 shard instances (the actual services of this repository, on mocked network providers)
//	                             -> a fake metachain observer (REST)
//
// It checks that what the real shard instances return (transaction metadata, operations, errors) is what the aggregator expects.
// No real network is involved.
func TestEndToEnd_WithRealShardInstances(t *testing.T) {
	ctx := context.Background()

	// Accounts: Alice and Carol in shard 0, Bob in shard 1, Dan in shard 2.
	addresses := mock.NewSimulator(mock.ArgsNewSimulator{NumShards: 3, NumAccountsPerShard: 1, Seed: 99})
	alice := addresses.NewAddressInShard(0)
	carol := addresses.NewAddressInShard(0)
	bob := addresses.NewAddressInShard(1)
	dan := addresses.NewAddressInShard(2)

	instances := []*e2eShardInstance{
		newE2EShardInstance(t, 0),
		newE2EShardInstance(t, 1),
		newE2EShardInstance(t, 2),
	}

	// All shard instances are given the same (complete) genesis file: each one reports the accounts of its own shard.
	genesisBalances := []*resources.GenesisBalance{
		{Address: alice, Balance: "1000000"},
		{Address: carol, Balance: "2000000"},
		{Address: bob, Balance: "3000000"},
		{Address: dan, Balance: "4000000"},
	}
	for _, instance := range instances {
		instance.setGenesisBalances(genesisBalances)
	}

	// A cross-shard transfer (Alice -> Bob), as returned by the observers of the two shards.
	crossShardTransfer := &transaction.ApiTransactionResult{
		Hash:                        e2eTxHash,
		Type:                        string(transaction.TxTypeNormal),
		ProcessingTypeOnSource:      "MoveBalance",
		ProcessingTypeOnDestination: "MoveBalance",
		Sender:                      alice,
		Receiver:                    bob,
		SourceShard:                 0,
		DestinationShard:            1,
		Value:                       "1000",
		InitiallyPaidFee:            "50",
		MiniBlockType:               "TxBlock",
		Status:                      transaction.TxStatusSuccess,
	}

	intraShardTransfer := &transaction.ApiTransactionResult{
		Hash:                        e2eIntraTxHash,
		Type:                        string(transaction.TxTypeNormal),
		ProcessingTypeOnSource:      "MoveBalance",
		ProcessingTypeOnDestination: "MoveBalance",
		Sender:                      alice,
		Receiver:                    carol,
		SourceShard:                 0,
		DestinationShard:            0,
		Value:                       "7",
		InitiallyPaidFee:            "50",
		MiniBlockType:               "TxBlock",
		Status:                      transaction.TxStatusSuccess,
	}

	// A cross-shard result (value sent from shard 1 to Dan, in shard 2), originated by another transaction.
	crossShardResult := &transaction.ApiTransactionResult{
		Hash:                    e2eResultHash,
		Type:                    string(transaction.TxTypeUnsigned),
		Sender:                  bob,
		Receiver:                dan,
		SourceShard:             1,
		DestinationShard:        2,
		Value:                   "300",
		MiniBlockType:           "SmartContractResultBlock",
		OriginalTransactionHash: e2eCallHash,
		PreviousTransactionHash: e2eCallHash,
		Status:                  transaction.TxStatusSuccess,
	}

	shard0Block1 := newE2EShardBlock(0, 1, crossShardTransfer, intraShardTransfer)
	shard1Block1 := newE2EShardBlock(1, 1)
	shard1Block2 := newE2EShardBlock(1, 2, crossShardTransfer, crossShardResult)
	shard2Block1 := newE2EShardBlock(2, 1)
	shard2Block2 := newE2EShardBlock(2, 2, crossShardResult)

	instances[0].setBlocks(shard0Block1)
	instances[1].setBlocks(shard1Block1, shard1Block2)
	instances[2].setBlocks(shard2Block1, shard2Block2)

	// The metachain notarizes the block of the source shard first, the block of the destination shard later.
	metaSource := mock.NewMetaSource(3)
	metaSource.AddBlock(newE2EMetaBlock(0, nil))
	metaSource.AddBlock(newE2EMetaBlock(1, notarize(shard0Block1, shard1Block1, shard2Block1)))
	metaSource.AddBlock(newE2EMetaBlock(2, notarize(shard1Block2)))
	metaSource.AddBlock(newE2EMetaBlock(3, notarize(shard2Block2)))
	metaSource.SetHighestFinalNonce(3)

	metaObserver := httptest.NewServer(mock.NewObserverHandler(metaSource))
	defer metaObserver.Close()

	// A replica that cannot be reached, to exercise the fail-over.
	deadServer := httptest.NewServer(http.NotFoundHandler())
	deadUrl := deadServer.URL
	deadServer.Close()

	cfg := &config.Config{}
	cfg.Network.Name = "testnet"
	cfg.Network.NumShards = 3
	cfg.Network.GenesisBlockHash = mock.FakeHash("e2e-meta-block", 0)
	cfg.Network.GenesisTimestamp = mock.GenesisTimestamp
	cfg.Metachain.ObserverUrls = []string{deadUrl, metaObserver.URL}
	cfg.Shards = []config.ShardConfig{
		{ID: 0, RosettaUrls: []string{instances[0].server.URL}},
		{ID: 1, RosettaUrls: []string{deadUrl, instances[1].server.URL}},
		{ID: 2, RosettaUrls: []string{instances[2].server.URL + "/"}},
	}
	cfg.Index.Path = filepath.Join(t.TempDir(), "index")
	cfg.ApplyDefaults()
	require.NoError(t, cfg.Validate())

	// The same wiring as in the "rosetta-aggregator" binary (see "createApp" in "cmd/rosetta-aggregator").
	httpClient := &http.Client{Timeout: 10 * time.Second}

	backendsByShard := make(map[uint32]backend.ShardBackend)
	for _, shard := range cfg.Shards {
		shardBackend, err := backend.NewHttpBackend(shard.RosettaUrls, httpClient)
		require.NoError(t, err)
		backendsByShard[shard.ID] = shardBackend
	}

	backends, err := backend.NewSet(backendsByShard)
	require.NoError(t, err)

	observerSource, err := metachain.NewObserverSource(cfg.Metachain.ObserverUrls, httpClient)
	require.NoError(t, err)

	store, err := index.NewLevelDBStore(cfg.Index.Path)
	require.NoError(t, err)

	app, err := NewApp(ArgsNewApp{Config: cfg, Version: "v0.0.0-test", Backends: backends, MetaSource: observerSource, Store: store})
	require.NoError(t, err)

	defer func() {
		_ = app.Close()
	}()

	require.NoError(t, app.Start(ctx))
	require.NoError(t, app.Step(ctx))

	aggregatorServer := httptest.NewServer(app.Handler())
	defer aggregatorServer.Close()

	rosettaClient := client.NewAPIClient(client.NewConfiguration(aggregatorServer.URL, "test", aggregatorServer.Client()))
	network := &types.NetworkIdentifier{Blockchain: "MultiversX", Network: "testnet"}

	// Responses are validated as Rosetta consumers do.
	status, rosettaErr, err := rosettaClient.NetworkAPI.NetworkStatus(ctx, &types.NetworkRequest{NetworkIdentifier: network})
	require.NoError(t, err)
	require.Nil(t, rosettaErr)

	options, rosettaErr, err := rosettaClient.NetworkAPI.NetworkOptions(ctx, &types.NetworkRequest{NetworkIdentifier: network})
	require.NoError(t, err)
	require.Nil(t, rosettaErr)

	responsesAsserter, err := asserter.NewClientWithResponses(network, status, options, "")
	require.NoError(t, err)

	getBlock := func(blockIndex int64) *types.Block {
		response, rosettaErr, err := rosettaClient.BlockAPI.Block(ctx, &types.BlockRequest{
			NetworkIdentifier: network,
			BlockIdentifier:   &types.PartialBlockIdentifier{Index: &blockIndex},
		})
		require.NoError(t, err, types.PrintStruct(rosettaErr))
		require.Nil(t, rosettaErr)
		require.NoError(t, responsesAsserter.Block(response.Block))
		return response.Block
	}

	findTx := func(block *types.Block, hash string) *types.Transaction {
		for _, tx := range block.Transactions {
			if tx.TransactionIdentifier.Hash == hash {
				return tx
			}
		}

		require.Fail(t, "transaction not found", "hash = %s, block = %d", hash, block.BlockIdentifier.Index)
		return nil
	}

	type simpleOperation struct {
		operationType string
		address       string
		value         string
		status        string
	}

	simplify := func(tx *types.Transaction) []simpleOperation {
		result := make([]simpleOperation, 0, len(tx.Operations))
		for _, operation := range tx.Operations {
			result = append(result, simpleOperation{operation.Type, operation.Account.Address, operation.Amount.Value, *operation.Status})
		}

		return result
	}

	t.Run("network status and options", func(t *testing.T) {
		require.Equal(t, int64(3), status.CurrentBlockIdentifier.Index)
		require.Equal(t, mock.FakeHash("e2e-meta-block", 3), status.CurrentBlockIdentifier.Hash)
		require.Equal(t, &types.BlockIdentifier{Index: 0, Hash: mock.FakeHash("e2e-meta-block", 0)}, status.GenesisBlockIdentifier)
		require.True(t, *status.SyncStatus.Synced)
		require.Len(t, status.Peers, 3)

		require.Equal(t, "v0.0.0-test", *options.Version.MiddlewareVersion)
		require.Equal(t, "v1.11.1-mock", options.Version.NodeVersion)
		require.NotEmpty(t, options.Version.Metadata["shardMiddlewareVersion"])
		require.Contains(t, options.Allow.OperationTypes, "Transfer")

		// The errors of the real shard instances (codes 1 - 19), plus the ones of the aggregator.
		codes := make(map[int32]struct{})
		for _, item := range options.Allow.Errors {
			codes[item.Code] = struct{}{}
		}
		require.Contains(t, codes, int32(2))
		require.Contains(t, codes, int32(services.ErrBlockNotYetAvailable))
		require.Len(t, codes, len(options.Allow.Errors))
	})

	t.Run("genesis block", func(t *testing.T) {
		block := getBlock(0)
		require.Len(t, block.Transactions, 1)

		balancesByAddress := make(map[string]string)
		for _, operation := range block.Transactions[0].Operations {
			require.Equal(t, "GenesisBalanceMovement", operation.Type)
			balancesByAddress[operation.Account.Address] = operation.Amount.Value
		}

		require.Equal(t, map[string]string{alice: "1000000", carol: "2000000", bob: "3000000", dan: "4000000"}, balancesByAddress)
	})

	t.Run("source leg of the cross-shard transfer", func(t *testing.T) {
		block := getBlock(1)
		require.Equal(t, mock.FakeHash("e2e-meta-block", 1), block.BlockIdentifier.Hash)
		require.Equal(t, mock.FakeHash("e2e-meta-block", 0), block.ParentBlockIdentifier.Hash)
		require.Len(t, block.Transactions, 2)

		// Only the operations of the sender (the receiver is in another shard).
		tx := findTx(block, e2eTxHash)
		require.Equal(t, []simpleOperation{
			{"Transfer", alice, "-1000", "Success"},
			{"Fee", alice, "-50", "Success"},
		}, simplify(tx))

		require.Equal(t, []*types.RelatedTransaction{
			{TransactionIdentifier: &types.TransactionIdentifier{Hash: e2eTxHash + "-dst"}, Direction: types.Forward},
		}, tx.RelatedTransactions)
		require.Equal(t, map[string]interface{}{"leg": "source", "sourceShard": float64(0), "destinationShard": float64(1)}, tx.Metadata["crossShard"])

		// An intra-shard transfer is passed through.
		intraTx := findTx(block, e2eIntraTxHash)
		require.Equal(t, []simpleOperation{
			{"Transfer", alice, "-7", "Success"},
			{"Transfer", carol, "7", "Success"},
			{"Fee", alice, "-50", "Success"},
		}, simplify(intraTx))
		require.Empty(t, intraTx.RelatedTransactions)
		require.NotContains(t, intraTx.Metadata, "crossShard")
	})

	t.Run("destination leg of the cross-shard transfer", func(t *testing.T) {
		block := getBlock(2)
		require.Len(t, block.Transactions, 2)

		tx := findTx(block, e2eTxHash+"-dst")
		require.Equal(t, []simpleOperation{{"Transfer", bob, "1000", "Success"}}, simplify(tx))
		require.Equal(t, []*types.RelatedTransaction{
			{TransactionIdentifier: &types.TransactionIdentifier{Hash: e2eTxHash}, Direction: types.Backward},
		}, tx.RelatedTransactions)
		require.Equal(t, map[string]interface{}{"leg": "destination", "hash": e2eTxHash, "sourceShard": float64(0), "destinationShard": float64(1)}, tx.Metadata["crossShard"])

		// The metadata set by the shard instance is preserved.
		require.Equal(t, "1000", tx.Metadata["value"])
		require.Equal(t, bob, tx.Metadata["receiver"])
	})

	t.Run("cross-shard result, linked to the transaction that originated it", func(t *testing.T) {
		// Source leg (shard 1): the debit of the sender.
		sourceLeg := findTx(getBlock(2), e2eResultHash)
		require.Equal(t, []simpleOperation{{"SmartContractResult", bob, "-300", "Success"}}, simplify(sourceLeg))
		require.Equal(t, []*types.RelatedTransaction{
			{TransactionIdentifier: &types.TransactionIdentifier{Hash: e2eCallHash}, Direction: types.Backward},
		}, sourceLeg.RelatedTransactions)

		// Destination leg (shard 2), in a later block: the credit of the receiver.
		destinationLeg := findTx(getBlock(3), e2eResultHash+"-dst")
		require.Equal(t, []simpleOperation{{"SmartContractResult", dan, "300", "Success"}}, simplify(destinationLeg))
		require.Equal(t, []*types.RelatedTransaction{
			{TransactionIdentifier: &types.TransactionIdentifier{Hash: e2eResultHash}, Direction: types.Backward},
			{TransactionIdentifier: &types.TransactionIdentifier{Hash: e2eCallHash}, Direction: types.Backward},
		}, destinationLeg.RelatedTransactions)
	})

	t.Run("account balance is read from the shard of the account, at the frontier", func(t *testing.T) {
		// At unified block 3, the frontier of shard 1 is its block 2.
		instances[1].setBalance(bob, "3000700", shard1Block2)

		response, rosettaErr, err := rosettaClient.AccountAPI.AccountBalance(ctx, &types.AccountBalanceRequest{
			NetworkIdentifier: network,
			AccountIdentifier: &types.AccountIdentifier{Address: bob},
		})
		require.NoError(t, err)
		require.Nil(t, rosettaErr)

		require.Equal(t, &types.BlockIdentifier{Index: 3, Hash: mock.FakeHash("e2e-meta-block", 3)}, response.BlockIdentifier)
		require.Equal(t, "3000700", response.Balances[0].Value)
		require.Equal(t, e2eNativeSymbol, response.Balances[0].Currency.Symbol)
		require.Equal(t, float64(1), response.Metadata["shard"])
		require.Equal(t, map[string]interface{}{"nonce": float64(2), "hash": shard1Block2.Hash}, response.Metadata["shardBlock"])

		// If the shard instance answers for another block than the frontier, the balance is not returned.
		instances[1].setBalance(bob, "3000700", shard1Block1)
		_, rosettaErr, _ = rosettaClient.AccountAPI.AccountBalance(ctx, &types.AccountBalanceRequest{
			NetworkIdentifier: network,
			AccountIdentifier: &types.AccountIdentifier{Address: bob},
		})
		requireErrorCode(t, int32(services.ErrBackendInconsistent), rosettaErr)

		// Errors of the shard instances are passed through (here: "unable to get account", for an account unknown to the mock).
		_, rosettaErr, _ = rosettaClient.AccountAPI.AccountBalance(ctx, &types.AccountBalanceRequest{
			NetworkIdentifier: network,
			AccountIdentifier: &types.AccountIdentifier{Address: dan},
		})
		requireErrorCode(t, 2, rosettaErr)
		require.Contains(t, rosettaErr.Details["originalError"], "not found")
	})

	t.Run("construction of a cross-shard transfer", func(t *testing.T) {
		// Only the instance of shard 0 knows the account of Alice: the flow succeeds only if the requests are routed correctly.
		instances[0].setAccount(alice, 42)

		currency := &types.Currency{Symbol: e2eNativeSymbol, Decimals: 18}
		operations := []*types.Operation{
			{
				OperationIdentifier: &types.OperationIdentifier{Index: 0},
				Type:                "Transfer",
				Account:             &types.AccountIdentifier{Address: alice},
				Amount:              &types.Amount{Value: "-1000", Currency: currency},
			},
			{
				OperationIdentifier: &types.OperationIdentifier{Index: 1},
				Type:                "Transfer",
				Account:             &types.AccountIdentifier{Address: bob},
				Amount:              &types.Amount{Value: "1000", Currency: currency},
			},
		}

		preprocess, rosettaErr, _ := rosettaClient.ConstructionAPI.ConstructionPreprocess(ctx, &types.ConstructionPreprocessRequest{
			NetworkIdentifier: network,
			Operations:        operations,
		})
		require.Nil(t, rosettaErr)
		require.Equal(t, alice, preprocess.Options["sender"])

		metadata, rosettaErr, _ := rosettaClient.ConstructionAPI.ConstructionMetadata(ctx, &types.ConstructionMetadataRequest{
			NetworkIdentifier: network,
			Options:           preprocess.Options,
		})
		require.Nil(t, rosettaErr)
		require.Equal(t, float64(42), metadata.Metadata["nonce"])
		require.Len(t, metadata.SuggestedFee, 1)

		payloads, rosettaErr, _ := rosettaClient.ConstructionAPI.ConstructionPayloads(ctx, &types.ConstructionPayloadsRequest{
			NetworkIdentifier: network,
			Operations:        operations,
			Metadata:          metadata.Metadata,
		})
		require.Nil(t, rosettaErr)
		require.Len(t, payloads.Payloads, 1)
		require.Equal(t, alice, payloads.Payloads[0].AccountIdentifier.Address)

		parsed, rosettaErr, _ := rosettaClient.ConstructionAPI.ConstructionParse(ctx, &types.ConstructionParseRequest{
			NetworkIdentifier: network,
			Transaction:       payloads.UnsignedTransaction,
		})
		require.Nil(t, rosettaErr)
		require.Len(t, parsed.Operations, 2)
		require.Equal(t, "-1000", parsed.Operations[0].Amount.Value)
		require.Equal(t, bob, parsed.Operations[1].Account.Address)

		publicKey := &types.PublicKey{Bytes: newTestPublicKeyBytes(), CurveType: types.Edwards25519}
		combined, rosettaErr, _ := rosettaClient.ConstructionAPI.ConstructionCombine(ctx, &types.ConstructionCombineRequest{
			NetworkIdentifier:   network,
			UnsignedTransaction: payloads.UnsignedTransaction,
			Signatures: []*types.Signature{
				{
					SigningPayload: payloads.Payloads[0],
					PublicKey:      publicKey,
					SignatureType:  types.Ed25519,
					Bytes:          append(newTestPublicKeyBytes(), newTestPublicKeyBytes()...),
				},
			},
		})
		require.Nil(t, rosettaErr)

		hashed, rosettaErr, _ := rosettaClient.ConstructionAPI.ConstructionHash(ctx, &types.ConstructionHashRequest{
			NetworkIdentifier: network,
			SignedTransaction: combined.SignedTransaction,
		})
		require.Nil(t, rosettaErr)
		require.Equal(t, e2eSubmitHash, hashed.TransactionIdentifier.Hash)

		submitted, rosettaErr, _ := rosettaClient.ConstructionAPI.ConstructionSubmit(ctx, &types.ConstructionSubmitRequest{
			NetworkIdentifier: network,
			SignedTransaction: combined.SignedTransaction,
		})
		require.Nil(t, rosettaErr)
		require.Equal(t, e2eSubmitHash, submitted.TransactionIdentifier.Hash)

		// The transaction was broadcast in the shard of the sender, only.
		require.Len(t, instances[0].submitted, 1)
		require.Equal(t, alice, instances[0].submitted[0].Sender)
		require.Equal(t, bob, instances[0].submitted[0].Receiver)
		require.Equal(t, uint64(42), instances[0].submitted[0].Nonce)
		require.Empty(t, instances[1].submitted)
		require.Empty(t, instances[2].submitted)

		// Deriving an address needs no chain access.
		derived, rosettaErr, _ := rosettaClient.ConstructionAPI.ConstructionDerive(ctx, &types.ConstructionDeriveRequest{
			NetworkIdentifier: network,
			PublicKey:         publicKey,
		})
		require.Nil(t, rosettaErr)
		require.Contains(t, derived.AccountIdentifier.Address, "erd1")
	})

	t.Run("the index is persisted", func(t *testing.T) {
		require.NoError(t, app.Close())

		reopened, err := index.NewLevelDBStore(cfg.Index.Path)
		require.NoError(t, err)

		defer func() {
			_ = reopened.Close()
		}()

		first, last, ok, err := reopened.Bounds()
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, uint64(0), first)
		require.Equal(t, uint64(3), last)
	})
}
