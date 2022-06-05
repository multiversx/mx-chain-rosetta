KEYS_FOLDER=$1
OBSERVERS=("0" "1" "2" "metachain")

for OBSERVER in ${OBSERVERS[@]};
do
    docker run --rm --mount type=bind,source=${KEYS_FOLDER},destination=/keys --workdir /keys elrondnetwork/elrond-go-keygenerator:latest
    sudo chown $(whoami) ${KEYS_FOLDER}/validatorKey.pem
    mv ${KEYS_FOLDER}/validatorKey.pem ${KEYS_FOLDER}/$OBSERVER.pem
done
