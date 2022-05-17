BASE_FOLDER=$1
NETWORKS=("testnet" "devnet" "mainnet")
SHARDS=("0" "1" "2" "metachain")

for NETWORK in ${NETWORKS[@]};
do
    mkdir -p ${BASE_FOLDER}/${NETWORK}

    for SHARD in ${SHARDS[@]};
    do
        mkdir -p ${BASE_FOLDER}/${NETWORK}/node-${SHARD}/db
        mkdir -p ${BASE_FOLDER}/${NETWORK}/node-${SHARD}/logs
    done
done
