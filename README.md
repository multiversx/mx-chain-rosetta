# Rosetta API for MultiversX

## Overview

**MultiversX runs on a sharded architecture** - transaction, data and network sharding are leveraged. 

In the Rosetta implementation, we've decided to provide a single-shard perspective to the API consumer. That is, **one Rosetta instance** would observe **a single _regular_ shard** of the network - the shard is selected by the owner of the instance.

Currently, the Rosetta implementation only supports the native currency (EGLD), while custom currencies ([ESDTs](https://docs.multiversx.com/tokens/esdt-tokens)) will be supported in the near future. At that point, Rosetta would observe the _metachain_, as well.

## Docker setup

In order to set up Rosetta using Docker, use [MultiversX/rosetta-docker](https://github.com/multiversx/mx-chain-rosetta-docker).

## Standalone setup

### Setup an Observer

Follow the official documentation to [set up an observer](https://docs.multiversx.com/validators/nodes-scripts/config-scripts/).

Before starting the observer, make sure to edit the `config/prefs.toml`:

```
[Preferences]
   DestinationShardAsObserver = "0" # a chosen shard to observe
   FullArchive = true
```

Furthermore, adjust `config/config.toml`, as follows:

```
...
[GeneralSettings]
    StartInEpochEnabled = false
...
[StoragePruning]
    AccountsTrieCleanOldEpochsData = false
    NumEpochsToKeep = 128 # a desired history length
...
[StateTriesConfig]
    AccountsStatePruningEnabled = false
...
[DbLookupExtensions]
    Enabled = true
...
```

### Setup Observer's database

In order to set up a database supporting historical balances lookup, please follow the instructions in section [Set up a database](#setup-a-database).

### Setup Rosetta

Clone the repository:

```
cd $HOME
git clone https://github.com/multiversx/mx-chain-rosetta
```

Build the application:

```
cd rosetta/cmd/rosetta
go build .
```

Then, start `rosetta` as follows:

```
./rosetta --observer-http-url=http://localhost:8080 --observer-actual-shard=0 \
--network-id=D --network-name=devnet --native-currency=XeGLD \
--port=9091
```

Or, in order to start using the `offline` mode:

```
./rosetta --observer-actual-shard=0 \
--network-id=D --network-name=devnet --native-currency=XeGLD \
--port=9092 --offline
```

## Setup a database

In order to support historical balances' lookup, Rosetta has to connect to an Observer whose database contains _non-pruned accounts tries_. Such databases can be re-built locally or downloaded from the public archive - the URL being available [on request](https://t.me/MultiversXDevelopers).

### Build archives

In order to locally re-build a database with historical lookup support, one should run [import-db](https://docs.multiversx.com/validators/import-db/#docsNav), using the following configuration:

#### `config.toml`
```
...
[StoragePruning]
    AccountsTrieCleanOldEpochsData = false
...
[StateTriesConfig]
    AccountsStatePruningEnabled = false
...
[DbLookupExtensions]
    Enabled = true
...
```

The **source** database (e.g. located in `./import-db/db`) should normally be a recent node database, while the **destination** database (e.g. located in `./db`) should contain a desired _starting epoch_ `N` as it's latest epoch - with intact `AccountsTries` (i.e. not removed due to `AccountsTrieCleanOldEpochsData = true`).

### Download archives

An archive supporting historical lookup is available to download [on request](https://t.me/MultiversXDevelopers), from a cloud-based, S3-compatible storage.

The archive consists of:
 - Individual files per _epoch_: `Epoch_*.tar`
 - A file for the _static_ database: `Static.tar`

Before starting the download, set up the following environment variables - make sure to adjust them beforehand, as necessary:

`devnet` example

```
export CHAIN_ID=D
export EPOCH_FIRST=1160
export EPOCH_LAST=1807
export URL_BASE=https://location-of-devnet-archives
```

`mainnet` example

```
export CHAIN_ID=1
export EPOCH_FIRST=700
export EPOCH_LAST=760
export URL_BASE=https://location-of-mainnet-archives
```

If you've previously downloaded a set of archives in the past, and you'd like to (incrementally) download newer ones, make sure to re-download a couple of previously-downloaded epochs, as follows:

```
export LATEST_DOWNLOADED_EPOCH=760
export EPOCH_FIRST=LATEST_DOWNLOADED_EPOCH-2
export EPOCH_LAST=770
```

Then, set up the download & extraction workspace:

```
mkdir ~/historical-workspace
mkdir -p ~/historical-workspace/db/${CHAIN_ID}
```

Let's proceed with the download:

```
cd ~/historical-workspace

# Download "Static" folder
echo "Downloading: Static"
wget ${URL_BASE}/Static.tar || exit 1
echo "Downloaded: Static"

# Download epochs
for (( epoch = ${EPOCH_FIRST}; epoch <= ${EPOCH_LAST}; epoch++ )) 
do 
    echo "Downloading: epoch ${epoch}"
    wget ${URL_BASE}/Epoch_${epoch}.tar || exit 1
    echo "Downloaded: epoch ${epoch}"
done
```

Once the download has finished, extract the archived data:

```
cd ~/historical-workspace

# Extract "Static" folder
tar -xf Static.tar --directory db/${CHAIN_ID} || exit 1
echo "Extracted: Static"

# Extract epochs
for (( epoch = ${EPOCH_FIRST}; epoch <= ${EPOCH_LAST}; epoch++ )) 
do 
    tar -xf Epoch_${epoch}.tar --directory db/${CHAIN_ID} || exit 1
    echo "Extracted: epoch ${epoch}"
done
```

The folder `~/historical-workspace/db` is now ready to be copied to the observer's working directory.

## Storage Pruning

At this moment, we only support a [storage pruning](https://www.rosetta-api.org/docs/storage_pruning.html#docsNav) mechanism that is **manually triggered** - the Observer behind Rosetta does not perform any _automatic pruning_. Instead, removing old epochs should be performed as follows:
 - stop Observer and Rosetta
 - remove epochs `[oldest, ..., latest - N - s]` (where `N` is the desired number of historical epochs, and `s = 2`, see below) from `/db/{chainID}` (or `/data/{chainID}` when using the Docker setup)
 - start Observer and Rosetta

The constant `s = 2` is needed to overcome possible snapshotting-related edge-cases, still present as of September 2022 (the use of this constant will become obsolete in future releases).

### **Pruning example**

For example, let's say that the `/db/{chainID}` (or `/data/{chainID}`) folder contains:
 - the folder `Static`
 - the folders `Epoch_500, Epoch_501, ..., Epoch_1022`.

Now, assume that you'd like to only have historical support for the latest `N = 22` epochs. Therefore, let's remove `[oldest, ..., latest - N - s] = [500, ..., 998]` epochs. In the end, the `/db/{chainID}` (or `/data/{chainID}`) folder contains:  
 - the folder `Static`
 - the folders `Epoch_999`, `Epoch_1000`, ... `Epoch_1022` (a number of `N + s = 24` epochs)

Then, when starting the Rosetta instance, you need to specify the `--first-historical-epoch` and `--num-historical-epochs` as follows:
 - `--first-historical-epoch = oldest + s = 999 + 2 = 1001`
 - `--num-historical-epochs = N = 22`

The parameters and `--first-historical-epoch` and `--num-historical-epochs` are used to compute the [`oldest_block_identifier`](https://www.rosetta-api.org/docs/models/NetworkStatusResponse.html), using this formula:

```
oldest_epoch = max(first_historical_epoch, current_epoch - num_historical_epochs)

oldest_block_identifier = first block of oldest_epoch
```

## Implementation notes

 - We do not support the `related_transactions` property, since it's not feasible to properly filter the related transactions of a given transaction by source / destination shard (with respect to the observed shard).
 - The endpoint `/block/transaction` is not implemented, since all transactions are returned by the endpoint `/block`.
 - We chose not to support the optional property `Operation.related_operations`. Although the smart contract results (also known as _unsigned transactions_) form a DAG (directed acyclic graph) at the protocol level, operations within a transaction are in a simple sequence.
 - Balance-changing operations that affect Smart Contract accounts are not emitted by our Rosetta implementation (thus are not available on the Rosetta API).

## Implementation validation

In order to validate the Rosetta implementation using `rosetta-cli`, please follow [MultiversX/rosetta-checks](https://github.com/multiversx/mx-chain-rosetta-checks).
