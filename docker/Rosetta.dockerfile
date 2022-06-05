FROM golang:1.17.6 as builder

# Clone repositories
RUN git clone https://github.com/ElrondNetwork/elrond-proxy-go.git --branch=rosetta-0.1.0 --depth=1
RUN git clone https://github.com/ElrondNetwork/rosetta-images.git --branch=config-0.1.1 --depth=1

# Build proxy
WORKDIR /go/elrond-proxy-go/cmd/proxy
RUN go build

# ===== SECOND STAGE ======
FROM ubuntu:20.04

COPY --from=builder "/go/elrond-proxy-go/cmd/proxy" "/elrond/"
COPY --from=builder "/go/rosetta-images/configuration/config.toml" "/elrond/config/config.toml"
COPY --from=builder "/go/rosetta-images/configuration/offline_testnet.toml" "/elrond/config"
COPY --from=builder "/go/rosetta-images/configuration/offline_devnet.toml" "/elrond/config"
COPY --from=builder "/go/rosetta-images/configuration/offline_mainnet.toml" "/elrond/config"

EXPOSE 8080
WORKDIR /elrond
ENTRYPOINT ["/elrond/proxy", "--log-save", "--working-directory=/data"]
