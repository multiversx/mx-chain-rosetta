# Rosetta images for Docker

**Elrond Network runs on a sharded architecture** - transaction, data and network sharding are leveraged. 

In the Rosetta implementation, we've decided to provide a single-shard perspective to the API consumer. That is, **one Rosetta instance** would observe **a single _regular_ shard** of the network (plus the _metachain_) - the shard is selected by the owner of the instance.

The Rosetta deployment for Elrond takes the shape of two Docker images (Elrond Rosetta and Elrond Observer), plus a Docker Compose definition to orchestrate the `1 + 1 + 1 + 1 = 4` containers: 

 - one Elrond Rosetta instance in **online mode**
 - one Elrond Rosetta instance in **offline mode**
 - one Elrond observer for a chosen regular shard
 - one Elrond observer for the _metachain_ (necessary for some pieces of information such as [ESDT](https://docs.elrond.com/developers/esdt-tokens) properties)
 
This `1 + 1 + 1 + 1 = 4` setup is usually referred to as an **Elrond Rosetta Squad**.

## Prerequisites

### Clone this repository

```
cd $HOME
git clone https://github.com/ElrondNetwork/rosetta.git
```

### Give permissions to the current user

Make sure you read [this article](https://docs.docker.com/engine/install/linux-postinstall/) carefully, before performing the step.

The following command adds the current user to the group "docker":

```
sudo usermod -aG docker $USER
```

After running the command, you may need to log out from the user session and log back in.

### Build the images

Below, we build all the images (including for  _devnet_).

```
cd $HOME/rosetta

docker image build . -t elrond-rosetta:latest -f ./docker/Rosetta.dockerfile
docker image build . -t elrond-rosetta-observer-devnet:latest -f ./docker/ObserverDevnet.dockerfile
docker image build . -t elrond-rosetta-observer-mainnet:latest -f ./docker/ObserverMainnet.dockerfile
```

### Prepare folders on host

The following script prepares the required folder structure on host:

```
cd $HOME/rosetta/scripts

export OBSERVED_SHARD=0
./prepare_host.sh ${HOME}/rosetta-workdir ${OBSERVED_SHARD}
```

### Generate keys for observers

The following script generates the node keys, required by the observers (chosen shard, plus metachain):

```
cd $HOME/rosetta/scripts

export OBSERVED_SHARD=0
./generate_keys.sh ${HOME}/rosetta-workdir/keys ${OBSERVED_SHARD}
```

Note that the script above downloads [this docker image](https://hub.docker.com/r/elrondnetwork/elrond-go-keygenerator). In order to change the ownership of the generated keys (from _owned by Docker_ to _owned by the current user_), superuser access will be requested.

## Run rosetta

### Run on devnet

```
cd $HOME/rosetta/docker

export CHAIN_ID=D
export NUM_SHARDS=3
export OBSERVER_PUBKEY="... get public key from rosetta-workdir/keys ..."
export OBSERVED_SHARD=0
export GENESIS_BLOCK=0000000000000000000000000000000000000000000000000000000000000000
export GENESIS_TIMESTAMP=1648551600
export NATIVE_CURRENCY=XeGLD
export ROSETTA_IMAGE=elrond-rosetta:latest
export OBSERVER_IMAGE=elrond-rosetta-observer-devnet:latest
export DATA_FOLDER=${HOME}/rosetta-workdir/devnet
export KEYS_FOLDER=${HOME}/rosetta-workdir/keys

docker compose --file ./docker-compose.yml up --detach
```

### Run on mainnet

```
cd $HOME/rosetta/docker

export CHAIN_ID=1
export NUM_SHARDS=3
export OBSERVER_PUBKEY="... get public key from rosetta-workdir/keys ..."
export OBSERVED_SHARD=0
export GENESIS_BLOCK=cd229e4ad2753708e4bab01d7f249affe29441829524c9529e84d51b6d12f2a7
export GENESIS_TIMESTAMP=1596117600
export NATIVE_CURRENCY=EGLD
export ROSETTA_IMAGE=elrond-rosetta:latest
export OBSERVER_IMAGE=elrond-rosetta-observer-mainnet:latest
export DATA_FOLDER=${HOME}/rosetta-workdir/mainnet
export KEYS_FOLDER=${HOME}/rosetta-workdir/keys

docker compose --file ./docker-compose.yml up --detach
```

## View logs of the running containers

Using `docker logs`:

```
docker logs docker-observer-1 -f
docker logs docker-rosetta-1 -f
```

By inspecting the files in the `logs` folder:

```
~/rosetta-workdir/(devnet|mainnet)/node-0/logs
```

## Update rosetta

Update the repository (repositories):

```
cd $HOME/rosetta
git pull origin
```

Stop the running containers:

```
docker stop docker-observer-1
docker stop docker-observer-metachain-1
docker stop docker-rosetta-1
docker stop docker-rosetta-offline-1
```

Re-build the images as described above, then run the containers again.
