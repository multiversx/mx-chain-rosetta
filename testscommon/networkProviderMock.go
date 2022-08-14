package testscommon

import (
	"fmt"
	"math/big"

	"github.com/ElrondNetwork/elrond-go-core/core"
	"github.com/ElrondNetwork/elrond-go-core/core/pubkeyConverter"
	"github.com/ElrondNetwork/elrond-go-core/data/transaction"
	"github.com/ElrondNetwork/elrond-go/sharding"
	"github.com/ElrondNetwork/elrond-proxy-go/data"
	"github.com/ElrondNetwork/rosetta/server/resources"
)

const emptyHash = "0000000000000000000000000000000000000000000000000000000000000000"
const genesisTimestamp = int64(1596117600)

type networkProviderMock struct {
	pubKeyConverter core.PubkeyConverter

	MockIsOffline                   bool
	MockNumShards                   uint32
	MockObservedActualShard         uint32
	MockObservedProjectedShard      uint32
	MockObservedProjectedShardIsSet bool
	MockObserverPubkey              string
	MockNativeCurrencySymbol        string
	MockGenesisBlockHash            string
	MockGenesisTimestamp            int64
	MockNetworkConfig               *resources.NetworkConfig
	MockGenesisBalances             []*resources.GenesisBalance
	MockLatestBlockSummary          *resources.BlockSummary
	MockBlocksByNonce               map[uint64]*data.Block
	MockBlocksByHash                map[string]*data.Block
	MockAccountsByAddress           map[string]*resources.Account
	MockAccountsNativeBalances      map[string]*resources.AccountNativeBalance
	MockAccountsESDTBalances        map[string]*resources.AccountESDTBalance
	MockMempoolTransactionsByHash   map[string]*data.FullTransaction
	MockComputedTransactionHash     string
	MockComputedReceiptHash         string
	MockNextError                   error

	SendTransactionCalled func(tx *data.Transaction) (string, error)
}

// NewNetworkProviderMock -
func NewNetworkProviderMock() *networkProviderMock {
	pubKeyConverter, _ := pubkeyConverter.NewBech32PubkeyConverter(32, log)

	return &networkProviderMock{
		pubKeyConverter:                 pubKeyConverter,
		MockIsOffline:                   false,
		MockNumShards:                   3,
		MockObservedActualShard:         0,
		MockObservedProjectedShard:      0,
		MockObservedProjectedShardIsSet: false,
		MockObserverPubkey:              "observer",
		MockNativeCurrencySymbol:        "XeGLD",
		MockGenesisBlockHash:            emptyHash,
		MockGenesisTimestamp:            genesisTimestamp,
		MockNetworkConfig: &resources.NetworkConfig{
			ChainID:        "test",
			GasPerDataByte: 1500,
			MinGasPrice:    1000000000,
			MinGasLimit:    50000,
		},
		MockGenesisBalances: make([]*resources.GenesisBalance, 0),
		MockLatestBlockSummary: &resources.BlockSummary{
			Nonce:             0,
			Hash:              emptyHash,
			PreviousBlockHash: emptyHash,
			Timestamp:         genesisTimestamp,
		},
		MockBlocksByNonce:             make(map[uint64]*data.Block),
		MockBlocksByHash:              make(map[string]*data.Block),
		MockAccountsByAddress:         make(map[string]*resources.Account),
		MockAccountsNativeBalances:    make(map[string]*resources.AccountNativeBalance),
		MockAccountsESDTBalances:      make(map[string]*resources.AccountESDTBalance),
		MockMempoolTransactionsByHash: make(map[string]*data.FullTransaction),
		MockComputedTransactionHash:   emptyHash,
		MockNextError:                 nil,
	}
}

// IsOffline -
func (mock *networkProviderMock) IsOffline() bool {
	return mock.MockIsOffline
}

// GetBlockchainName -
func (mock *networkProviderMock) GetBlockchainName() string {
	return resources.BlockchainName
}

// GetChainID -
func (mock *networkProviderMock) GetChainID() string {
	return mock.MockNetworkConfig.ChainID
}

// GetNativeCurrency -
func (mock *networkProviderMock) GetNativeCurrency() resources.NativeCurrency {
	return resources.NativeCurrency{
		Symbol:   mock.MockNativeCurrencySymbol,
		Decimals: 18,
	}
}

// GetObserverPubkey -
func (mock *networkProviderMock) GetObserverPubkey() string {
	return mock.MockObserverPubkey
}

// GetNetworkConfig -
func (mock *networkProviderMock) GetNetworkConfig() *resources.NetworkConfig {
	return mock.MockNetworkConfig
}

// GetGenesisBlockSummary -
func (mock *networkProviderMock) GetGenesisBlockSummary() *resources.BlockSummary {
	return &resources.BlockSummary{
		Nonce:     0,
		Hash:      mock.MockGenesisBlockHash,
		Timestamp: mock.MockGenesisTimestamp,
	}
}

// GetGenesisTimestamp -
func (mock *networkProviderMock) GetGenesisTimestamp() int64 {
	return mock.MockGenesisTimestamp
}

// GetGenesisBalances -
func (mock *networkProviderMock) GetGenesisBalances() ([]*resources.GenesisBalance, error) {
	if mock.MockNextError != nil {
		return nil, mock.MockNextError
	}

	return mock.MockGenesisBalances, nil
}

// GetLatestBlockSummary -
func (mock *networkProviderMock) GetLatestBlockSummary() (*resources.BlockSummary, error) {
	if mock.MockNextError != nil {
		return nil, mock.MockNextError
	}

	return mock.MockLatestBlockSummary, nil
}

// GetBlockByNonce -
func (mock *networkProviderMock) GetBlockByNonce(nonce uint64) (*data.Block, error) {
	if mock.MockNextError != nil {
		return nil, mock.MockNextError
	}

	block, ok := mock.MockBlocksByNonce[nonce]
	if ok {
		return block, nil
	}

	return nil, fmt.Errorf("block %d not found", nonce)
}

// GetBlockByHash -
func (mock *networkProviderMock) GetBlockByHash(hash string) (*data.Block, error) {
	if mock.MockNextError != nil {
		return nil, mock.MockNextError
	}

	block, ok := mock.MockBlocksByHash[hash]
	if ok {
		return block, nil
	}

	return nil, fmt.Errorf("block %s not found", hash)
}

// GetAccount -
func (mock *networkProviderMock) GetAccount(address string) (*resources.AccountModel, error) {
	if mock.MockNextError != nil {
		return nil, mock.MockNextError
	}

	account, ok := mock.MockAccountsByAddress[address]
	if ok {
		return &resources.AccountModel{
			Account: *account,
			BlockCoordinates: resources.AccountBlockCoordinates{
				Nonce:    mock.MockLatestBlockSummary.Nonce,
				Hash:     mock.MockLatestBlockSummary.Hash,
				RootHash: emptyHash,
			},
		}, nil
	}

	return nil, fmt.Errorf("account %s not found", address)
}

func (mock *networkProviderMock) GetAccountNativeBalance(address string, options resources.AccountQueryOptions) (*resources.AccountNativeBalance, error) {
	if mock.MockNextError != nil {
		return nil, mock.MockNextError
	}

	accountBalance, ok := mock.MockAccountsNativeBalances[address]
	if ok {
		return &resources.AccountNativeBalance{
			Balance: accountBalance.Balance,
			BlockCoordinates: resources.AccountBlockCoordinates{
				Nonce:    mock.MockLatestBlockSummary.Nonce,
				Hash:     mock.MockLatestBlockSummary.Hash,
				RootHash: emptyHash,
			},
		}, nil
	}

	return nil, fmt.Errorf("account %s not found", address)
}

func (mock *networkProviderMock) GetAccountESDTBalance(address string, tokenIdentifier string, options resources.AccountQueryOptions) (*resources.AccountESDTBalance, error) {
	if mock.MockNextError != nil {
		return nil, mock.MockNextError
	}

	key := fmt.Sprintf("%s_%s", address, tokenIdentifier)
	accountBalance, ok := mock.MockAccountsESDTBalances[key]
	if ok {
		return &resources.AccountESDTBalance{
			Balance: accountBalance.Balance,
			BlockCoordinates: resources.AccountBlockCoordinates{
				Nonce:    mock.MockLatestBlockSummary.Nonce,
				Hash:     mock.MockLatestBlockSummary.Hash,
				RootHash: emptyHash,
			},
		}, nil
	}

	return nil, fmt.Errorf("account %s not found", address)
}

// IsAddressObserved -
func (mock *networkProviderMock) IsAddressObserved(address string) (bool, error) {
	if mock.MockNextError != nil {
		return false, mock.MockNextError
	}

	shardCoordinator, err := sharding.NewMultiShardCoordinator(mock.MockNumShards, mock.MockObservedActualShard)
	if err != nil {
		return false, err
	}

	pubKey, err := mock.ConvertAddressToPubKey(address)
	if err != nil {
		return false, err
	}

	shard := shardCoordinator.ComputeId(pubKey)

	isObservedActualShard := shard == mock.MockObservedActualShard
	isObservedProjectedShard := pubKey[len(pubKey)-1] == byte(mock.MockObservedProjectedShard)

	if mock.MockObservedProjectedShardIsSet {
		return isObservedProjectedShard, nil
	}

	return isObservedActualShard, nil
}

// ConvertPubKeyToAddress -
func (mock *networkProviderMock) ConvertPubKeyToAddress(pubkey []byte) string {
	return mock.pubKeyConverter.Encode(pubkey)
}

// ConvertAddressToPubKey -
func (mock *networkProviderMock) ConvertAddressToPubKey(address string) ([]byte, error) {
	return mock.pubKeyConverter.Decode(address)
}

// ComputeTransactionHash -
func (mock *networkProviderMock) ComputeTransactionHash(tx *data.Transaction) (string, error) {
	if mock.MockNextError != nil {
		return "", mock.MockNextError
	}

	return mock.MockComputedTransactionHash, nil
}

// ComputeReceiptHash -
func (mock *networkProviderMock) ComputeReceiptHash(apiReceipt *transaction.ApiReceipt) (string, error) {
	if mock.MockNextError != nil {
		return "", mock.MockNextError
	}

	return mock.MockComputedReceiptHash, nil
}

// ComputeTransactionFeeForMoveBalance -
func (mock *networkProviderMock) ComputeTransactionFeeForMoveBalance(tx *data.FullTransaction) *big.Int {
	minGasLimit := mock.MockNetworkConfig.MinGasLimit
	gasPerDataByte := mock.MockNetworkConfig.GasPerDataByte
	gasLimit := minGasLimit + gasPerDataByte*uint64(len(tx.Data))

	fee := core.SafeMul(gasLimit, tx.GasPrice)
	return fee
}

// SendTransaction -
func (mock *networkProviderMock) SendTransaction(tx *data.Transaction) (string, error) {
	if mock.MockNextError != nil {
		return "", mock.MockNextError
	}

	if mock.SendTransactionCalled != nil {
		return mock.SendTransactionCalled(tx)
	}

	return mock.MockComputedTransactionHash, nil
}

// GetMempoolTransactionByHash -
func (mock *networkProviderMock) GetMempoolTransactionByHash(hash string) (*data.FullTransaction, error) {
	if mock.MockNextError != nil {
		return nil, mock.MockNextError
	}

	transaction, ok := mock.MockMempoolTransactionsByHash[hash]
	if ok {
		return transaction, nil
	}

	return nil, nil
}
