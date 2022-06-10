## Set up the localnet

Clone `elrond-go` and `elrond-proxy-go` at `~/rosetta-localnet`:

```
mkdir -p ~/rosetta-localnet && cd ~/rosetta-localnet
git clone git@github.com:ElrondNetwork/elrond-go.git --branch development
git clone git@github.com:ElrondNetwork/elrond-proxy-go.git --branch rosetta-development
```

Start the localnet using `erdpy` (will also start the Rosetta API):

```
cd tests/localnet
erdpy testnet config
erdpy testnet start
```

## Execute a number of transactions

```
cd tests/localnet/snippets
npm install
```

Run snippets steps from the Test Explorer.

## Run the checks

```
rosetta-cli check:data --configuration-file localnet-data-001.json
```
