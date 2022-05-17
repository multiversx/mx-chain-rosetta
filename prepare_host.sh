BASE_FOLDER=$1
NETWORKS=("testnet" "devnet" "mainnet")
SHARDS=("0" "1" "2" "metachain")

mkdir -p ${BASE_FOLDER}/keys

for NETWORK in ${NETWORKS[@]};
do
    mkdir -p ${BASE_FOLDER}/${NETWORK}
    mkdir -p ${BASE_FOLDER}/${NETWORK}/proxy
    mkdir -p ${BASE_FOLDER}/${NETWORK}/proxy-rosetta
    mkdir -p ${BASE_FOLDER}/${NETWORK}/proxy-rosetta-offline

    for SHARD in ${SHARDS[@]};
    do
        mkdir -p ${BASE_FOLDER}/${NETWORK}/node-${SHARD}/db
        mkdir -p ${BASE_FOLDER}/${NETWORK}/node-${SHARD}/logs
    done
done
