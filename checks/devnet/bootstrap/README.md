# Bootstrap balances

These files were built using [balancesExporter](https://github.com/ElrondNetwork/elrond-tools-go/tree/main/trieTools/balancesExporter).

### `D_shard_0_epoch_1166_nonce_1399025_XeGLD.rosetta-json.metadata.json`

In order to build this file, follow the steps below:

```
mkdir -p ~/balances/devnet/epochs && cd ~/balances/devnet/epochs

wget ${ARCHIVES_URL}/devnet/shard-0/Epoch_1165.tar
wget ${ARCHIVES_URL}/devnet/shard-0/Epoch_1166.tar

tar -xf ./Epoch_1165.tar
tar -xf ./Epoch_1166.tar

cd ~/balances/devnet
balancesExporter --db-path=./epochs --shard=0 --num-shards=3 --epoch=1166 --currency=XeGLD --format=rosetta-json
```

### `D_shard_0(0)_epoch_1166_nonce_1399025_XeGLD.rosetta.json`

For this file, the procedure is similar, with the final command being:

```
balancesExporter --db-path=./epochs --shard=0 --num-shards=3 --by-projected-shard=0 --epoch=1166 --currency=XeGLD --format=rosetta-json
```


