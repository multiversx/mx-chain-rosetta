package provider

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"github.com/ElrondNetwork/elrond-go-core/core"
	"github.com/ElrondNetwork/elrond-go-core/data/receipt"
	"github.com/ElrondNetwork/elrond-go-core/data/transaction"
	"github.com/ElrondNetwork/elrond-go-core/hashing"
	"github.com/ElrondNetwork/elrond-go-core/marshal"
	logger "github.com/ElrondNetwork/elrond-go-logger"
	"github.com/ElrondNetwork/elrond-proxy-go/common"
	"github.com/ElrondNetwork/elrond-proxy-go/data"
	"github.com/ElrondNetwork/rosetta/server/resources"
)

var (
	urlPathGetNetworkConfig   = "/network/config"
	urlPathGetNodeStatus      = "/node/status"
	urlPathGetGenesisBalances = "/network/genesis-balances"
)

var log = logger.GetOrCreate("server/provider")

type ArgsNewNetworkProvider struct {
	IsOffline                   bool
	ObservedActualShard         uint32
	ObservedProjectedShard      uint32
	ObservedProjectedShardIsSet bool
	ObserverUrl                 string
	ObserverPubkey              string
	ChainID                     string
	GasPerDataByte              uint64
	MinGasPrice                 uint64
	MinGasLimit                 uint64
	NativeCurrencySymbol        string
	GenesisBlockHash            string
	GenesisTimestamp            int64
	ObserveNotFinalBlocks       bool

	ObserverFacade observerFacade

	Hasher                hashing.Hasher
	MarshalizerForHashing marshal.Marshalizer
	PubKeyConverter       core.PubkeyConverter
}

type networkProvider struct {
	isOffline                   bool
	observedActualShard         uint32
	observedProjectedShard      uint32
	observedProjectedShardIsSet bool
	observerUrl                 string
	observerPubkey              string
	nativeCurrencySymbol        string
	genesisBlockHash            string
	genesisTimestamp            int64
	observeNotFinalBlocks       bool

	observerFacade observerFacade

	hasher                hashing.Hasher
	marshalizerForHashing marshal.Marshalizer
	pubKeyConverter       core.PubkeyConverter

	networkConfig *resources.NetworkConfig
}

func NewNetworkProvider(args ArgsNewNetworkProvider) (*networkProvider, error) {
	return &networkProvider{
		isOffline: args.IsOffline,

		observedActualShard:         args.ObservedActualShard,
		observedProjectedShard:      args.ObservedProjectedShard,
		observedProjectedShardIsSet: args.ObservedProjectedShardIsSet,
		observerUrl:                 args.ObserverUrl,
		observerPubkey:              args.ObserverPubkey,
		nativeCurrencySymbol:        args.NativeCurrencySymbol,
		genesisBlockHash:            args.GenesisBlockHash,
		genesisTimestamp:            args.GenesisTimestamp,
		observeNotFinalBlocks:       args.ObserveNotFinalBlocks,

		observerFacade: args.ObserverFacade,

		hasher:                args.Hasher,
		marshalizerForHashing: args.MarshalizerForHashing,
		pubKeyConverter:       args.PubKeyConverter,

		networkConfig: &resources.NetworkConfig{
			ChainID:        args.ChainID,
			GasPerDataByte: args.GasPerDataByte,
			MinGasPrice:    args.MinGasPrice,
			MinGasLimit:    args.MinGasLimit,
		},
	}, nil
}

// IsOffline returns whether the network provider is in the "offline" mode (i.e. no connection to the observer)
func (provider *networkProvider) IsOffline() bool {
	return provider.isOffline
}

// GetBlockchainName returns the name of the network ("Elrond")
func (provider *networkProvider) GetBlockchainName() string {
	return resources.BlockchainName
}

// GetChainID gets the chain identifier ("1" for mainnet, "D" for devnet etc.)
func (provider *networkProvider) GetChainID() string {
	return provider.networkConfig.ChainID
}

// GetNativeCurrency gets the native currency (EGLD, 18 decimals)
func (provider *networkProvider) GetNativeCurrency() resources.NativeCurrency {
	return resources.NativeCurrency{
		Symbol:   provider.nativeCurrencySymbol,
		Decimals: int32(nativeCurrencyNumDecimals),
	}
}

// GetObserverPubkey gets the pubkey of the connected observer
func (provider *networkProvider) GetObserverPubkey() string {
	return provider.observerPubkey
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

	_, err := provider.observerFacade.CallGetRestEndPoint(provider.observerUrl, urlPathGetGenesisBalances, &response)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, errors.New(response.Error)
	}

	return response.Data.Balances, nil
}

// GetLatestBlockSummary gets a summary of the latest block
func (provider *networkProvider) GetLatestBlockSummary() (*resources.BlockSummary, error) {
	if provider.isOffline {
		return nil, errIsOffline
	}

	latestBlockNonce, err := provider.getLatestBlockNonce()
	if err != nil {
		return nil, err
	}

	log.Debug("GetLatestBlockSummary()", "latestBlockNonce", latestBlockNonce)

	queryOptions := common.BlockQueryOptions{
		WithTransactions: false,
		WithLogs:         false,
	}

	blockResponse, err := provider.observerFacade.GetBlockByNonce(
		provider.observedActualShard,
		latestBlockNonce,
		queryOptions,
	)
	if err != nil {
		return nil, newErrCannotGetBlockByNonce(latestBlockNonce, err)
	}

	return &resources.BlockSummary{
		Nonce:             blockResponse.Data.Block.Nonce,
		Hash:              blockResponse.Data.Block.Hash,
		PreviousBlockHash: blockResponse.Data.Block.PrevBlockHash,
		Timestamp:         int64(blockResponse.Data.Block.Timestamp),
	}, nil
}

func (provider *networkProvider) getLatestBlockNonce() (uint64, error) {
	nodeStatus, err := provider.getNodeStatus()
	if err != nil {
		return 0, err
	}

	if provider.observeNotFinalBlocks {
		return nodeStatus.HighestNonce, nil
	}

	return nodeStatus.HighestFinalNonce, nil
}

func (provider *networkProvider) getNodeStatus() (*resources.NodeStatus, error) {
	if provider.isOffline {
		return nil, errIsOffline
	}

	response := &resources.NodeStatusApiResponse{}

	_, err := provider.observerFacade.CallGetRestEndPoint(provider.observerUrl, urlPathGetNodeStatus, &response)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, errors.New(response.Error)
	}

	return &response.Data.Status, nil
}

// GetBlockByNonce gets a block by nonce
func (provider *networkProvider) GetBlockByNonce(nonce uint64) (*data.Block, error) {
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
		return nil, err
	}

	provider.simplifyBlockWithScheduledTransactions(block)
	return block, nil
}

func (provider *networkProvider) doGetBlockByNonce(nonce uint64) (*data.Block, error) {
	queryOptions := common.BlockQueryOptions{
		WithTransactions: true,
		WithLogs:         true,
	}

	response, err := provider.observerFacade.GetBlockByNonce(provider.observedActualShard, nonce, queryOptions)
	if err != nil {
		return nil, newErrCannotGetBlockByNonce(nonce, err)
	}
	if response.Error != "" {
		return nil, newErrCannotGetBlockByNonce(nonce, errors.New(response.Error))
	}

	return &response.Data.Block, nil
}

// GetBlockByHash gets a block by hash
func (provider *networkProvider) GetBlockByHash(hash string) (*data.Block, error) {
	if provider.isOffline {
		return nil, errIsOffline
	}

	block, err := provider.doGetBlockByHash(hash)
	if err != nil {
		return nil, err
	}

	provider.simplifyBlockWithScheduledTransactions(block)
	return block, nil
}

func (provider *networkProvider) doGetBlockByHash(hash string) (*data.Block, error) {
	queryOptions := common.BlockQueryOptions{
		WithTransactions: true,
		WithLogs:         true,
	}

	response, err := provider.observerFacade.GetBlockByHash(provider.observedActualShard, hash, queryOptions)
	if err != nil {
		return nil, newErrCannotGetBlockByHash(hash, err)
	}
	if response.Error != "" {
		return nil, newErrCannotGetBlockByHash(hash, errors.New(response.Error))
	}

	return &response.Data.Block, nil
}

// GetAccount gets an account by address
func (provider *networkProvider) GetAccount(address string) (*data.AccountModel, error) {
	if provider.isOffline {
		return nil, errIsOffline
	}

	onFinalBlock := !provider.observeNotFinalBlocks
	options := common.AccountQueryOptions{OnFinalBlock: onFinalBlock}
	account, err := provider.observerFacade.GetAccount(address, options)
	if err != nil {
		return nil, newErrCannotGetAccount(address, err)
	}

	log.Trace(fmt.Sprintf("GetAccount(onFinal=%t)", onFinalBlock),
		"address", account.Account.Address,
		"balance", account.Account.Balance,
		"block", account.BlockInfo.Nonce,
		"blockHash", account.BlockInfo.Hash,
		"blockRootHash", account.BlockInfo.RootHash,
	)

	return account, nil
}

// IsAddressObserved returns whether the address is observed (i.e. is located in an observed shard)
func (provider *networkProvider) IsAddressObserved(address string) (bool, error) {
	pubKey, err := provider.ConvertAddressToPubKey(address)
	if err != nil {
		return false, err
	}

	shard, err := provider.observerFacade.ComputeShardId(pubKey)
	if err != nil {
		return false, err
	}

	isObservedActualShard := shard == provider.observedActualShard
	isObservedProjectedShard := pubKey[len(pubKey)-1] == byte(provider.observedProjectedShard)

	if provider.observedProjectedShardIsSet {
		return isObservedProjectedShard, nil
	}

	return isObservedActualShard, nil
}

// ConvertPubKeyToAddress converts a public key to an address
func (provider *networkProvider) ConvertPubKeyToAddress(pubkey []byte) string {
	return provider.pubKeyConverter.Encode(pubkey)
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

	receipt := &receipt.Receipt{
		TxHash:  txHash,
		SndAddr: senderPubkey,
		Value:   apiReceipt.Value,
		Data:    []byte(apiReceipt.Data),
	}

	receiptHash, err := core.CalculateHash(provider.marshalizerForHashing, provider.hasher, receipt)
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
		return "", err
	}

	return hash, nil
}

// GetMempoolTransactionByHash gets a transaction from the pool
func (provider *networkProvider) GetMempoolTransactionByHash(hash string) (*data.FullTransaction, error) {
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

// ComputeTransactionFeeForMoveBalance computes the fee for a move-balance transaction.
// TODO: when freeze account feature is merged, this will need to be adapted as well, as for guarded transactions we have an additional gas (limit).
func (provider *networkProvider) ComputeTransactionFeeForMoveBalance(tx *data.FullTransaction) *big.Int {
	minGasLimit := provider.networkConfig.MinGasLimit
	gasPerDataByte := provider.networkConfig.GasPerDataByte
	gasLimit := minGasLimit + gasPerDataByte*uint64(len(tx.Data))

	fee := core.SafeMul(gasLimit, tx.GasPrice)
	return fee
}

// LogDescription writes a description of the network provider in the log output
func (provider *networkProvider) LogDescription() {
	log.Info("Description of network provider",
		"isOffline", provider.isOffline,
		"observerUrl", provider.observerUrl,
		"observedActualShard", provider.observedActualShard,
		"observedProjectedShard", provider.observedProjectedShard,
		"observedProjectedShardIsSet", provider.observedProjectedShardIsSet,
		"observeNotFinalBlocks", provider.observeNotFinalBlocks,
		"nativeCurrency", provider.nativeCurrencySymbol,
	)
}
