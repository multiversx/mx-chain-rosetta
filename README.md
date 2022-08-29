# Rosetta API for Elrond Network

## Overview

**Elrond Network runs on a sharded architecture** - transaction, data and network sharding are leveraged. 

In the Rosetta implementation, we've decided to provide a single-shard perspective to the API consumer. That is, **one Rosetta instance** would observe **a single _regular_ shard** of the network - the shard is selected by the owner of the instance.

Currently, the Rosetta implementation only supports the native currency (EGLD), while custom currencies ([ESDTs](https://docs.elrond.com/developers/esdt-tokens)) will be supported in the near future. At that point, Rosetta would observe the _metachain_, as well.

## Docker setup

In order to set up Rosetta using Docker, use [ElrondNetwork/rosetta-docker](https://github.com/ElrondNetwork/rosetta-docker).

## Standalone setup

### Setup an Observer

Follow the official documentation to setup an observer:
 - [mainnet](https://docs.elrond.com/validators/mainnet/config-scripts/)
 - [devnet](https://docs.elrond.com/validators/elrond-go-scripts/config-scripts/)

Before starting the observer, make sure to edit the `config/prefs.toml`:

```
[Preferences]
   DestinationShardAsObserver = "0" # a chosen shard to observe
   FullArchive = true
```

Furthermore, adjust `config/config.toml`, as follows:

```
[GeneralSettings]
    StartInEpochEnabled = false

[StoragePruning]
    AccountsTrieCleanOldEpochsData = false
    NumEpochsToKeep = 128 # a desired history length

[StateTriesConfig]
    AccountsStatePruningEnabled = false

[DbLookupExtensions]
    Enabled = true
```

### Setup Observer's database

In order to set up a database supporting historical balances lookup, please follow the instructions in section [Setup a database](#setup-a-database).

### Setup Rosetta

Clone the repository:

```
cd $HOME
git clone https://github.com/ElrondNetwork/rosetta.git
```

Build the application:

```
cd rosetta/cmd/rosetta
go build .
```

Then, start `rosetta` as follows:

```
./rosetta --observer-http-url=http://localhost:8080 --observer-actual-shard=0 \
--chain-id=D --native-currency=XeGLD \
--port=9091
```

Or, in order to start using the `offline` mode:

```
./rosetta --observer-actual-shard=0 \
--chain-id=D --native-currency=XeGLD \
--port=9092 --offline
```

## Setup a database

In order to support historical balances lookup, Rosetta has to connect to an Observer whose database contains _non-pruned accounts tries_. Such databases can be re-built locally or downloaded from the Internet.

### Build archives

In order to locally re-build a database with historical lookup support, one should run [import-db](https://docs.elrond.com/validators/import-db/#docsNav), using the following configuration:

#### `config.toml`
```
[StoragePruning]
    AccountsTrieCleanOldEpochsData = false
    NumEpochsToKeep = 128 # a desired history length

[StateTriesConfig]
    AccountsStatePruningEnabled = false

[DbLookupExtensions]
    Enabled = true
```

The **source** database (e.g. located in `./import-db/db`) should normally be a recent node database including epoch `N`, while the **destination** database (e.g. located in `./db`) should contain epoch `N - NumEpochsToKeep - 1` with an intact `AccountsTries` (i.e. not removed due to the default `NumEpochsToKeep = 3` and `AccountsTrieCleanOldEpochsData = true`).

### Download archives

An archive supporting historical lookup is available to download **on demand**, from a cloud-based, S3-compatible storage.

The archive consists of:
 - Individual files per _epoch_: `Epoch_*.tar`
 - A file for the _static_ database: `Static.tar`

Before starting the download, set up the following environment variables - make sure to adjust them beforehand, as necessary:

#### `devnet` example

```
export CHAIN_ID=D
export EPOCH_FIRST=1160
export EPOCH_LAST=1807
export URL_BASE=https://location-of-devnet-archives
```

#### `mainnet` example
```
export CHAIN_ID=1
export EPOCH_FIRST=700
export EPOCH_LAST=760
export URL_BASE=https://location-of-mainnet-archives
```

If you've previosuly downloaded a set of archives in the past, and you'd like to (incrementally) download newer ones, make sure to re-download a couple of previously-downloaded epochs, as follows:

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
wget ${URL_BASE}/Static.tar

# Download epochs
for (( epoch = ${EPOCH_FIRST}; epoch <= ${EPOCH_LAST}; epoch++ )) 
do 
    wget ${URL_BASE}/Epoch_${epoch}.tar
done
```

Once the download has finished, extract the archived data:

```
cd ~/historical-workspace

# Extract "Static" folder
tar -xf Static.tar --directory db/${CHAIN_ID}

# Extract epochs
for (( epoch = ${EPOCH_FIRST}; epoch <= ${EPOCH_LAST}; epoch++ )) 
do 
    tar -xf Epoch_${epoch}.tar --directory db/${CHAIN_ID}
done
```

The folder `~/historical-workspace/db` is now ready to be copied to the observer's working directory.


## Implementation notes

 - We do not support the `related_transactions` property, since it's not feasible to properly filter the related transactions of a given transaction by source / destination shard (with respect to the observed shard).
 - The endpoint `/block/transaction` is not implemented, since all transactions are returned by the endpoint `/block`.
 - We chose not to support the optional property `Operation.related_operations`. Although the smart contract results (also known as _unsigned transactions_) form a DAG (directed acyclic graph) at the protocol level, operations within a transaction are in a simple sequence.
 - Only successful operations are listed in our Rosetta API implementation. For _invalid_ transactions, we only list the _fee_ operation.
 - Balance-changing operations that affect Smart Contract accounts are not emitted by our Rosetta implementation (thus are not available on the Rosetta API).

## Implementation validation

In order to validate the Rosetta implementation using `rosetta-cli`, please follow [ElrondNetwork/rosetta-checks](https://github.com/ElrondNetwork/rosetta-checks).
