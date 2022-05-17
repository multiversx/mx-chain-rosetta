## Build the images

```
docker image build . -t rosetta-observer-testnet:latest -f ./observer/testnet.dockerfile
```

## Generate keys for Observers

```
./generate_keys.sh ${HOME}/rosetta/keys
```

## Run on Testnet

```
export DATA_FOLDER=${HOME}/rosetta/testnet
export KEYS_FOLDER=${HOME}/rosetta/keys
```

```
docker compose --file ./docker-compose-testnet.yml up 
```

