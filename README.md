# Rosetta API for Elrond Network

## Overview

**Elrond Network runs on a sharded architecture** - transaction, data and network sharding are leveraged. 

In the Rosetta implementation, we've decided to provide a single-shard perspective to the API consumer. That is, **one Rosetta instance** would observe **a single _regular_ shard** of the network - the shard is selected by the owner of the instance.

Currently, the Rosetta implementation only supports the native currency (EGLD), while custom currencies ([ESDTs](https://docs.elrond.com/developers/esdt-tokens)) will be supported in the near future. At that point, Rosetta would observe the _metachain_, as well.

## Standalone setup

### Setup an Observer

Follow the official documentation to setup an observer:
 - [mainnet](https://docs.elrond.com/validators/mainnet/config-scripts/)
 - [devnet](https://docs.elrond.com/validators/elrond-go-scripts/config-scripts/)

Before starting the observer, make sure to edit the `config/prefs.toml` and set the target shard (for example, shard 0):

```
[Preferences]
   DestinationShardAsObserver = "0"
```

Furthermore, enable the **database lookup extensions** in `config/config.toml`, as follows:

```
[DbLookupExtensions]
    Enabled = true
```

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

## Docker setup

The Docker setup takes the shape of two Docker images (Elrond Rosetta and Elrond Observer), plus a Docker Compose definition to orchestrate the `1 + 1 + 1 = 3` containers: 

 - one Elrond Rosetta instance in **online mode**
 - one Elrond Rosetta instance in **offline mode**
 - one Elrond observer for a chosen regular shard
  
This `1 + 1 + 1 = 3` setup is usually referred to as an **Elrond Rosetta Squad**.

Currently, the Rosetta implementation only supports the native currency (EGLD), while custom currencies ([ESDTs](https://docs.elrond.com/developers/esdt-tokens)) will be supported in the near future. At that point, the Docker setup would contain `1 + 1 + 1 + 1 = 4` containers - the additional container being an Elrond observer for the _metachain_ (necessary for some pieces of information such as ESDT properties).


### Give permissions to the current user

Make sure you read [this article](https://docs.docker.com/engine/install/linux-postinstall/) carefully, before performing the step.

The following command adds the current user to the group "docker":

```
sudo usermod -aG docker $USER
```

After running the command, you may need to log out from the user session and log back in.

#### Prepare folders on host

The following script prepares the required folder structure on host:

```
cd $HOME/rosetta/scripts

export OBSERVED_SHARD=0
./prepare_host.sh ${HOME}/rosetta-workdir ${OBSERVED_SHARD}
```

#### Generate keys for observers

The following script generates the node keys, required by the observers (chosen shard, plus metachain):

```
cd $HOME/rosetta/scripts

export OBSERVED_SHARD=0
./generate_keys.sh ${HOME}/rosetta-workdir/keys ${OBSERVED_SHARD}
```

Note that the script above downloads [this docker image](https://hub.docker.com/r/elrondnetwork/elrond-go-keygenerator). 

In order to change the ownership of the generated keys (from _owned by Docker_ to _owned by the current user_), superuser access will be requested.

### Build the images

Below, we build all the images (including for  _devnet_).

```
cd $HOME/rosetta

docker image build --no-cache . -t elrond-rosetta:latest -f ./docker/Rosetta.dockerfile
docker image build --no-cache . -t elrond-rosetta-observer-devnet:latest -f ./docker/ObserverDevnet.dockerfile
docker image build --no-cache . -t elrond-rosetta-observer-mainnet:latest -f ./docker/ObserverMainnet.dockerfile
```

### Run rosetta

Run on **devnet**:

```
cd $HOME/rosetta/docker
docker compose --file ./docker-compose-devnet.yml --env-file ./devnet.env --project-name elrond-devnet up --detach
```

Run on **mainnet**:

```
cd $HOME/rosetta/docker
docker compose --file ./docker-compose-mainnet.yml --env-file ./mainnet.env --project-name elrond-mainnet up --detach
```

### Inspect logs of the running containers

Using `docker logs`:

```
# For devnet
docker logs elrond-rosetta-observer-devnet -f
docker logs elrond-rosetta-online-devnet -f
docker logs elrond-rosetta-offline-devnet -f

# For mainnet
docker logs elrond-rosetta-observer-mainnet -f
docker logs elrond-rosetta-online-mainnet -f
docker logs elrond-rosetta-offline-mainnet -f
```

By inspecting the files in the `logs` folder:

```
~/rosetta-workdir/(devnet|mainnet)/node-0/logs
```

### Update the Docker setup

Update the repository (repositories):

```
cd $HOME/rosetta
git pull origin
```

Stop the running containers:

```
# For devnet:
docker stop elrond-rosetta-observer-devnet
docker stop elrond-rosetta-online-devnet
docker stop elrond-rosetta-offline-devnet

# For mainnet:
docker stop elrond-rosetta-observer-mainnet
docker stop elrond-rosetta-online-mainnet
docker stop elrond-rosetta-offline-mainnet
```

Re-build the images as described above, then run the containers again.


## Implementation notes

 - We do not support the `related_transactions` property, since it's not feasible to properly filter the related transactions of a given transaction by source / destination shard (with respect to the observed shard).
 - The endpoint `/block/transaction` is not implemented, since all transactions are returned by the endpoint `/block`.
 - We chose not to support the optional property `Operation.related_operations`. Although the smart contract results (also known as _unsigned transactions_) form a DAG (directed acyclic graph) at the protocol level, operations within a transaction are in a simple sequence.
 - Only successful operations are listed in our Rosetta API implementation. For _invalid_ transactions, we only list the _fee_ operation.
 - Balance-changing operations that affect Smart Contract accounts are not emitted by our Rosetta implementation (thus are not available on the Rosetta API).

## Validation notes

### Data API

 - Make sure to set `"pruning_disabled": true` in the configuration file of the checker. Otherwise, the information gathered from `bootstrap/*.rosetta.json` will be lost at some point due to pruning, and balance reconciliations will start to fail.

### Construction API

 - Make sure to set a large enough `"stale_depth"`, since the implementation only returns _final_ blocks (notarized by the Metachain and built upon), by default. There is a delay between the broadcast of the transaction and the moment at which the container block is marked as _final_. For example, use `"stale_depth": 10`.
 - In the construction DSL, `generate_account()` cannot be used, since it cannot be constrained to create accounts in the observed shard, at the moment. As a workaround, the accounts involved in a transfer (sender, recipient) should be explicitly specified in the `*.ros` file. 
