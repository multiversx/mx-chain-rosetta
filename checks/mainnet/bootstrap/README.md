# Bootstrap balances

The files were built using [balancesExporter](https://github.com/ElrondNetwork/elrond-tools-go/tree/main/trieTools/balancesExporter).

### `1_shard_0_epoch_703_nonce_10120628_EGLD.rosetta.json`

In order to build this file, follow the steps below:

```
mkdir -p ~/balances/mainnet/epochs && cd ~/balances/mainnet/epochs

wget ${ARCHIVES_URL}/mainnet/shard-0/Epoch_702.tar
wget ${ARCHIVES_URL}/mainnet/shard-0/Epoch_703.tar

tar -xf ./Epoch_702.tar
tar -xf ./Epoch_703.tar

cd ~/balances/mainnet
balancesExporter --db-path=./epochs --shard=0 --num-shards=3 --epoch=703 --currency=EGLD --format=rosetta-json
```

### `1_shard_0(0)_epoch_703_nonce_10120628_EGLD.rosetta.json`

For this file, the procedure is similar, with the final command being:

```
balancesExporter --db-path=./epochs --shard=0 --num-shards=3 --by-projected-shard=0 --epoch=703 --currency=EGLD --format=rosetta-json
```
