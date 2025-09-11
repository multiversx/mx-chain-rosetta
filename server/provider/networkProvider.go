package provider

import (
	"encoding/hex"
	"errors"
	"math/big"

	"github.com/multiversx/mx-chain-core-go/core"
	"github.com/multiversx/mx-chain-core-go/data/api"
	"github.com/multiversx/mx-chain-core-go/data/receipt"
	"github.com/multiversx/mx-chain-core-go/data/transaction"
	"github.com/multiversx/mx-chain-core-go/hashing"
	"github.com/multiversx/mx-chain-core-go/marshal"
	logger "github.com/multiversx/mx-chain-logger-go"
	"github.com/multiversx/mx-chain-proxy-go/common"
	"github.com/multiversx/mx-chain-proxy-go/data"
	"github.com/multiversx/mx-chain-rosetta/server/resources"
	"github.com/multiversx/mx-chain-storage-go/lrucache"
)

var log = logger.GetOrCreate("server/provider")

type ArgsNewNetworkProvider struct {
	IsOffline                   bool
	ObservedActualShard         uint32
	ObservedProjectedShard      uint32
	ObservedProjectedShardIsSet bool
	ObserverUrl                 string
	BlockchainName              string
	NetworkID                   string
	NetworkName                 string
	GasPerDataByte              uint64
	GasPriceModifier            float64
	GasLimitCustomTransfer      uint64
	MinGasPrice                 uint64
	MinGasLimit                 uint64
	ExtraGasLimitGuardedTx      uint64
	ExtraGasLimitRelayedTxV3    uint64
	NativeCurrencySymbol        string
	CustomCurrencies            []resources.Currency
	GenesisBlockHash            string
	GenesisTimestamp            int64
	FirstHistoricalEpoch        uint32
	NumHistoricalEpochs         uint32
	ShouldHandleContracts       bool

	ObserverFacade observerFacade

	Hasher                hashing.Hasher
	MarshalizerForHashing marshal.Marshalizer
	PubKeyConverter       core.PubkeyConverter
}

// In the future, we might rename this to "networkFacade" (which, in turn, depends on networkProvider, currencyProvider, blocksProvider and so on).
type networkProvider struct {
	*currenciesProvider

	isOffline                   bool
	observedActualShard         uint32
	observedProjectedShard      uint32
	observedProjectedShardIsSet bool
	observerUrl                 string
	genesisBlockHash            string
	genesisTimestamp            int64
	firstHistoricalEpoch        uint32
	numHistoricalEpochs         uint32
	shouldHandleContracts       bool

	observerFacade observerFacade

	hasher                hashing.Hasher
	marshalizerForHashing marshal.Marshalizer
	pubKeyConverter       core.PubkeyConverter

	networkConfig *resources.NetworkConfig

	blocksCache blocksCache
}

// NewNetworkProvider (future-to-be renamed to NewNetworkFacade) creates a new networkProvider
func NewNetworkProvider(args ArgsNewNetworkProvider) (*networkProvider, error) {
	// Since for each block N we also have to fetch block N-1 and block N+1 (see "simplifyBlockWithScheduledTransactions"),
	// it makes sense to cache the block response (using an LRU cache).
	blocksCache, err := lrucache.NewCache(blocksCacheCapacity)
	if err != nil {
		return nil, err
	}

	currenciesProvider, err := newCurrenciesProvider(args.NativeCurrencySymbol, args.CustomCurrencies)
	if err != nil {
		return nil, err
	}

	return &networkProvider{
		currenciesProvider: currenciesProvider,

		isOffline: args.IsOffline,

		observedActualShard:         args.ObservedActualShard,
		observedProjectedShard:      args.ObservedProjectedShard,
		observedProjectedShardIsSet: args.ObservedProjectedShardIsSet,
		observerUrl:                 args.ObserverUrl,
		genesisBlockHash:            args.GenesisBlockHash,
		genesisTimestamp:            args.GenesisTimestamp,
		firstHistoricalEpoch:        args.FirstHistoricalEpoch,
		numHistoricalEpochs:         args.NumHistoricalEpochs,
		shouldHandleContracts:       args.ShouldHandleContracts,

		observerFacade: args.ObserverFacade,

		hasher:                args.Hasher,
		marshalizerForHashing: args.MarshalizerForHashing,
		pubKeyConverter:       args.PubKeyConverter,

		networkConfig: &resources.NetworkConfig{
			BlockchainName:           args.BlockchainName,
			NetworkID:                args.NetworkID,
			NetworkName:              args.NetworkName,
			GasPerDataByte:           args.GasPerDataByte,
			GasPriceModifier:         args.GasPriceModifier,
			GasLimitCustomTransfer:   args.GasLimitCustomTransfer,
			MinGasPrice:              args.MinGasPrice,
			MinGasLimit:              args.MinGasLimit,
			ExtraGasLimitGuardedTx:   args.ExtraGasLimitGuardedTx,
			ExtraGasLimitRelayedTxV3: args.ExtraGasLimitRelayedTxV3,
		},

		blocksCache: blocksCache,
	}, nil
}

// IsOffline returns whether the network provider is in the "offline" mode (i.e. no connection to the observer)
func (provider *networkProvider) IsOffline() bool {
	return provider.isOffline
}

// GetBlockchainName returns the name of the network
func (provider *networkProvider) GetBlockchainName() string {
	return provider.networkConfig.BlockchainName
}

// GetNetworkConfig gets the network config (once fetched, the network config is indefinitely held in memory)
func (provider *networkProvider) GetNetworkConfig() *resources.NetworkConfig {
	return provider.networkConfig
}

// GetGenesisBlockSummary gets a summary of the genesis block
func (provider *networkProvider) GetGenesisBlockSummary() *resources.BlockSummary {
	return &resources.BlockSummary{
		Nonce:     uint64(genesisBlockNonce),
		Hash:      provider.genesisBlockHash,
		Timestamp: provider.genesisTimestamp,
	}
}

// GetGenesisTimestamp gets the timestamp of the genesis block
func (provider *networkProvider) GetGenesisTimestamp() int64 {
	return provider.genesisTimestamp
}

func (provider *networkProvider) GetGenesisBalances() ([]*resources.GenesisBalance, error) {
	if provider.isOffline {
		return nil, errIsOffline
	}

	response := &resources.GenesisBalancesApiResponse{}
	err := provider.getResource(urlPathGetGenesisBalances, response)
	if err != nil {
		return nil, err
	}

	return response.Data.Balances, nil
}

func (provider *networkProvider) getBlockSummaryByNonce(nonce uint64) (resources.BlockSummary, error) {
	if provider.isOffline {
		return resources.BlockSummary{}, errIsOffline
	}

	queryOptions := common.BlockQueryOptions{
		WithTransactions: false,
		WithLogs:         false,
	}

	blockResponse, err := provider.observerFacade.GetBlockByNonce(
		provider.observedActualShard,
		nonce,
		queryOptions,
	)
	if err != nil {
		return resources.BlockSummary{}, newErrCannotGetBlockByNonce(nonce, err)
	}

	return resources.BlockSummary{
		Nonce:             blockResponse.Data.Block.Nonce,
		Hash:              blockResponse.Data.Block.Hash,
		PreviousBlockHash: blockResponse.Data.Block.PrevBlockHash,
		Timestamp:         int64(blockResponse.Data.Block.Timestamp),
	}, nil
}

// GetBlockByNonce gets a block by nonce
func (provider *networkProvider) GetBlockByNonce(nonce uint64) (*api.Block, error) {
	if provider.isOffline {
		return nil, errIsOffline
	}

	latestNonce, err := provider.getLatestBlockNonce()
	if err != nil {
		return nil, err
	}

	if nonce > latestNonce {
		return nil, errCannotGetBlock
	}

	block, err := provider.doGetBlockByNonce(nonce)
	if err != nil {
		log.Warn("GetBlockByNonce()", "nonce", nonce, "err", err)
		return nil, err
	}

	// The block (copy) returned by doGetBlockByNonce() is now mutated.
	// The mutated copy is not held in a cache (not needed).
	err = provider.simplifyBlockWithScheduledTransactions(block)
	if err != nil {
		return nil, err
	}

	return block, nil
}

func (provider *networkProvider) doGetBlockByNonce(nonce uint64) (*api.Block, error) {
	queryOptions := common.BlockQueryOptions{
		WithTransactions: true,
		WithLogs:         true,
	}

	block, ok := provider.getBlockByNonceCached(nonce)
	if ok {
		return createBlockCopy(block), nil
	}

	response, err := provider.observerFacade.GetBlockByNonce(provider.observedActualShard, nonce, queryOptions)
	if err != nil {
		return nil, newErrCannotGetBlockByNonce(nonce, convertStructuredApiErrToFlatErr(err))
	}
	if response.Error != "" {
		return nil, newErrCannotGetBlockByNonce(nonce, errors.New(response.Error))
	}

	block = &response.Data.Block

	provider.cacheBlockByNonce(nonce, block)

	return createBlockCopy(block), nil
}

// createBlockCopy creates a somehow shallow copy of a block.
func createBlockCopy(block *api.Block) *api.Block {
	miniblocksCopy := make([]*api.MiniBlock, len(block.MiniBlocks))

	for i, miniblock := range block.MiniBlocks {
		miniblocksCopy[i] = &api.MiniBlock{
			Type:              miniblock.Type,
			Hash:              miniblock.Hash,
			ProcessingType:    miniblock.ProcessingType,
			ConstructionState: miniblock.ConstructionState,
			SourceShard:       miniblock.SourceShard,
			DestinationShard:  miniblock.DestinationShard,
			// This is sufficient for our purposes (we don't mutate the transactions themselves, we only mutate the list of transactions within a miniblock).
			Transactions: miniblock.Transactions,
			Receipts:     miniblock.Receipts,
		}
	}

	return &api.Block{
		Nonce:         block.Nonce,
		Round:         block.Round,
		Epoch:         block.Epoch,
		Shard:         block.Shard,
		NumTxs:        block.NumTxs,
		Hash:          block.Hash,
		PrevBlockHash: block.PrevBlockHash,
		StateRootHash: block.StateRootHash,
		Status:        block.Status,
		Timestamp:     block.Timestamp,
		MiniBlocks:    miniblocksCopy,
	}
}

func (provider *networkProvider) getBlockByNonceCached(nonce uint64) (*api.Block, bool) {
	blockUntyped, ok := provider.blocksCache.Get(blockNonceToBytes(nonce))
	if ok {
		block, ok := blockUntyped.(*api.Block)
		if ok {
			return block, true
		}
	}

	return nil, false
}

func (provider *networkProvider) cacheBlockByNonce(nonce uint64, block *api.Block) {
	_ = provider.blocksCache.Put(blockNonceToBytes(nonce), block, 1)
}

// GetBlockByHash gets a block by hash
func (provider *networkProvider) GetBlockByHash(hash string) (*api.Block, error) {
	if provider.isOffline {
		return nil, errIsOffline
	}

	block, err := provider.doGetBlockByHash(hash)
	if err != nil {
		log.Warn("GetBlockByHash()", "hash", hash, "err", err)
		return nil, err
	}

	err = provider.simplifyBlockWithScheduledTransactions(block)
	if err != nil {
		return nil, err
	}

	return block, nil
}

func (provider *networkProvider) doGetBlockByHash(hash string) (*api.Block, error) {
	queryOptions := common.BlockQueryOptions{
		WithTransactions: true,
		WithLogs:         true,
	}

	response, err := provider.observerFacade.GetBlockByHash(provider.observedActualShard, hash, queryOptions)
	if err != nil {
		return nil, newErrCannotGetBlockByHash(hash, convertStructuredApiErrToFlatErr(err))
	}
	if response.Error != "" {
		return nil, newErrCannotGetBlockByHash(hash, errors.New(response.Error))
	}

	return &response.Data.Block, nil
}

// IsAddressObserved returns whether the address is observed (i.e. is located in an observed shard)
func (provider *networkProvider) IsAddressObserved(address string) (bool, error) {
	pubKey, err := provider.ConvertAddressToPubKey(address)
	if err != nil {
		return false, err
	}

	shard := provider.observerFacade.ComputeShardId(pubKey)

	noConstraintAboutProjectedShard := !provider.observedProjectedShardIsSet
	noConstraintAboutHandlingContracts := provider.shouldHandleContracts

	passesConstraintAboutObservedActualShard := shard == provider.observedActualShard
	passesConstraintAboutObservedProjectedShard := noConstraintAboutProjectedShard || pubKey[len(pubKey)-1] == byte(provider.observedProjectedShard)
	passesConstraintAboutHandlingContracts := noConstraintAboutHandlingContracts || !core.IsSmartContractAddress(pubKey)

	return passesConstraintAboutObservedActualShard && passesConstraintAboutObservedProjectedShard && passesConstraintAboutHandlingContracts, nil
}

// ComputeShardIdOfPubKey computes the shard ID of a public key
func (provider *networkProvider) ComputeShardIdOfPubKey(pubKey []byte) uint32 {
	shard := provider.observerFacade.ComputeShardId(pubKey)
	return shard
}

// ConvertPubKeyToAddress converts a public key to an address
func (provider *networkProvider) ConvertPubKeyToAddress(pubkey []byte) string {
	address, err := provider.pubKeyConverter.Encode(pubkey)
	if err != nil {
		log.Warn("networkProvider.ConvertPubKeyToAddress()", "pubkey", pubkey, "err", err)
		return ""
	}

	return address
}

// ConvertAddressToPubKey converts an address to a pubkey
func (provider *networkProvider) ConvertAddressToPubKey(address string) ([]byte, error) {
	return provider.pubKeyConverter.Decode(address)
}

// ComputeTransactionHash computes the hash of a provided transaction
func (provider *networkProvider) ComputeTransactionHash(tx *data.Transaction) (string, error) {
	return provider.observerFacade.ComputeTransactionHash(tx)
}

func (provider *networkProvider) ComputeReceiptHash(apiReceipt *transaction.ApiReceipt) (string, error) {
	txHash, err := hex.DecodeString(apiReceipt.TxHash)
	if err != nil {
		return "", err
	}

	senderPubkey, err := provider.ConvertAddressToPubKey(apiReceipt.SndAddr)
	if err != nil {
		return "", err
	}

	receiptObj := &receipt.Receipt{
		TxHash:  txHash,
		SndAddr: senderPubkey,
		Value:   apiReceipt.Value,
		Data:    []byte(apiReceipt.Data),
	}

	receiptHash, err := core.CalculateHash(provider.marshalizerForHashing, provider.hasher, receiptObj)
	if err != nil {
		return "", err
	}

	receiptHashHex := hex.EncodeToString(receiptHash)
	return receiptHashHex, nil
}

// SendTransaction broadcasts an already-signed transaction
func (provider *networkProvider) SendTransaction(tx *data.Transaction) (string, error) {
	if provider.isOffline {
		return "", errIsOffline
	}

	_, hash, err := provider.observerFacade.SendTransaction(tx)
	if err != nil {
		log.Warn("SendTransaction()", "sender", tx.Sender, "nonce", tx.Nonce, "err", err)
		return "", err
	}

	return hash, nil
}

// GetMempoolTransactionByHash gets a transaction from the pool
func (provider *networkProvider) GetMempoolTransactionByHash(hash string) (*transaction.ApiTransactionResult, error) {
	if provider.isOffline {
		return nil, errIsOffline
	}

	tx, _, err := provider.observerFacade.GetTransactionByHashAndSenderAddress(hash, "", false)
	if err != nil {
		return nil, newErrCannotGetTransaction(hash, err)
	}

	if tx.Status == transaction.TxStatusPending {
		return tx, nil
	}

	return nil, nil
}

// ComputeTransactionFeeForMoveBalance computes the fee for a move-balance transaction
func (provider *networkProvider) ComputeTransactionFeeForMoveBalance(tx *transaction.ApiTransactionResult) *big.Int {
	minGasLimit := provider.networkConfig.MinGasLimit
	extraGasLimitGuardedTx := provider.networkConfig.ExtraGasLimitGuardedTx
	extraGasLimitRelayedTxV3 := provider.networkConfig.ExtraGasLimitRelayedTxV3
	gasPerDataByte := provider.networkConfig.GasPerDataByte
	gasLimit := minGasLimit + gasPerDataByte*uint64(len(tx.Data))

	isGuarded := len(tx.GuardianAddr) > 0
	if isGuarded {
		gasLimit += extraGasLimitGuardedTx
	}

	isRelayedV3 := IsRelayedTxV3(tx)
	if isRelayedV3 {
		gasLimit += extraGasLimitRelayedTxV3
	}

	fee := core.SafeMul(gasLimit, tx.GasPrice)
	return fee
}

// LogDescription writes a description of the network provider in the log output
func (provider *networkProvider) LogDescription() {
	log.Info("Description of network provider",
		"blockchain", provider.networkConfig.BlockchainName,
		"network", provider.networkConfig.NetworkName,
		"isOffline", provider.isOffline,
		"observerUrl", provider.observerUrl,
		"observedActualShard", provider.observedActualShard,
		"observedProjectedShard", provider.observedProjectedShard,
		"observedProjectedShardIsSet", provider.observedProjectedShardIsSet,
		"firstHistoricalEpoch", provider.firstHistoricalEpoch,
		"numHistoricalEpochs", provider.numHistoricalEpochs,
		"shouldHandleContracts", provider.shouldHandleContracts,
		"nativeCurrency", provider.GetNativeCurrency().Symbol,
		"customCurrencies", provider.GetCustomCurrenciesSymbols(),
	)
}
