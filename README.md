## Prerequisites

### Build the images

```
docker image build . -t rosetta-observer-testnet:latest -f ./observer/testnet.dockerfile
docker image build . -t rosetta-observer-devnet:latest -f ./observer/devnet.dockerfile
docker image build . -t rosetta-observer-mainnet:latest -f ./observer/mainnet.dockerfile
```

### Prepare folders on host

The following script prepares the required folder structure on host:

```
./prepare_host.sh ${HOME}/rosetta
```

### Generate keys for observers

```
./generate_keys.sh ${HOME}/rosetta/keys
```

## Run rosetta

## Run on testnet

```
export OBSERVER_IMAGE=rosetta-observer-testnet:latest
export DATA_FOLDER=${HOME}/rosetta/testnet
export KEYS_FOLDER=${HOME}/rosetta/keys

docker compose --file ./docker-compose.yml up
```

## Run on devnet

```
export OBSERVER_IMAGE=rosetta-observer-devnet:latest
export DATA_FOLDER=${HOME}/rosetta/devnet
export KEYS_FOLDER=${HOME}/rosetta/keys

docker compose --file ./docker-compose.yml up
```

## Run on mainnet

```
export OBSERVER_IMAGE=rosetta-observer-mainnet:latest
export DATA_FOLDER=${HOME}/rosetta/mainnet
export KEYS_FOLDER=${HOME}/rosetta/keys

docker compose --file ./docker-compose.yml up
```
