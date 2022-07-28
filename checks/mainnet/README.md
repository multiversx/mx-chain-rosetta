## Run the checks

Set the Rosetta URLs:

```
export ROSETTA_ONLINE=http://rosetta-mainnet:9090
export ROSETTA_OFFLINE=http://rosetta-mainnet:9091
```

Set a starting point for the data API checks:

```
export AFTER_BLOCK=10120628
```

Check the data API:

```
rosetta-cli check:data --configuration-file mainnet-data-start-after-${AFTER_BLOCK}.json \
--online-url=${ROSETTA_ONLINE} --data-dir=mainnet-${AFTER_BLOCK}
```
