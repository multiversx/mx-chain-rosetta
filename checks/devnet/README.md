## Run the checks

Set the Rosetta URLs:

```
export ROSETTA_ONLINE=http://localhost:7091
export ROSETTA_OFFLINE=http://localhost:7092
```

Check the construction API:

```
rosetta-cli check:construction --configuration-file devnet-construction.json \
--online-url=${ROSETTA_ONLINE} --offline-url=${ROSETTA_OFFLINE}
```

Set a starting point for the data API checks:

```
export AFTER_BLOCK=1399025
```

Check the data API:

```
rosetta-cli check:data --configuration-file devnet-data-start-after-${AFTER_BLOCK}.json \
--online-url=${ROSETTA_ONLINE} --data-dir=devnet-${AFTER_BLOCK}
```

Or, using historical balances:

```
rosetta-cli check:data --configuration-file devnet-data-start-historical.json \
--online-url=${ROSETTA_ONLINE} --data-dir=devnet-${AFTER_BLOCK}-historical
```

