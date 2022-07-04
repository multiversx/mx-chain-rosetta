## Run the checks

Set the starting point and the Rosetta URLs:

```
export AFTER_BLOCK=1399025
export ROSETTA_ONLINE=http://rosetta-devnet:8091
export ROSETTA_OFFLINE=http://rosetta-devnet:8092
```

Check the construction API:

```
rosetta-cli check:construction --configuration-file devnet-construction.json \
--online-url=${ROSETTA_ONLINE} --offline-url=${ROSETTA_OFFLINE}
```

Check the data API:

```
rosetta-cli check:data --configuration-file devnet-data-start-after-${AFTER_BLOCK}.json \
--online-url ${ROSETTA} --data-dir devnet-${AFTER_BLOCK}
```

Or, to continue checking the data API:

```
rosetta-cli check:data --configuration-file devnet-data-continue.json \
--online-url ${ROSETTA} --data-dir devnet-${AFTER_BLOCK}
```

