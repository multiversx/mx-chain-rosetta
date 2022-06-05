BASE_FOLDER=$1
NETWORKS=("testnet" "devnet" "mainnet")
SHARDS=("0")

mkdir -p ${BASE_FOLDER}/keys

for NETWORK in ${NETWORKS[@]};
do
    mkdir -p ${BASE_FOLDER}/${NETWORK}
    mkdir -p ${BASE_FOLDER}/${NETWORK}/rosetta
    mkdir -p ${BASE_FOLDER}/${NETWORK}/rosetta-offline

    for SHARD in ${SHARDS[@]};
    do
        mkdir -p ${BASE_FOLDER}/${NETWORK}/node-${SHARD}/db
        mkdir -p ${BASE_FOLDER}/${NETWORK}/node-${SHARD}/logs
    done
done
