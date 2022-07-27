FROM golang:1.17.6 as builder

# Clone repositories
RUN git clone https://github.com/ElrondNetwork/elrond-config-mainnet --branch=rc-2022-july --depth=1
RUN git clone https://github.com/ElrondNetwork/elrond-go.git --branch=rc/2022-july --depth=1

# Build node
WORKDIR /go/elrond-go/cmd/node
RUN go build -i -v -ldflags="-X main.appVersion=$(git -C /go/elrond-config-mainnet describe --tags --long --dirty)"

RUN cp /go/pkg/mod/github.com/!elrond!network/arwen-wasm-vm@$(cat /go/elrond-go/go.mod | grep arwen-wasm-vm | sed 's/.* //' | tail -n 1)/wasmer/libwasmer_linux_amd64.so /lib/libwasmer_linux_amd64.so

# Adjust configuration files
RUN apt-get update && apt-get -y install python3-pip && pip3 install toml
RUN wget -O adjust_config.py https://raw.githubusercontent.com/ElrondNetwork/rosetta/main/docker/adjust_config.py && \
    python3 adjust_config.py --mode=main --file=/go/elrond-config-mainnet/config.toml && \
    python3 adjust_config.py --mode=prefs --file=/go/elrond-config-mainnet/prefs.toml

# ===== SECOND STAGE ======
FROM ubuntu:20.04

COPY --from=builder "/go/elrond-go/cmd/node" "/elrond/"
COPY --from=builder "/go/elrond-config-mainnet" "/elrond/config/"
COPY --from=builder "/lib/libwasmer_linux_amd64.so" "/lib/libwasmer_linux_amd64.so"

EXPOSE 8080
WORKDIR /elrond
ENTRYPOINT ["/elrond/node", "--log-save", "--log-level=*:INFO,core/dblookupext:WARN", "--log-logger-name", "--rest-api-interface=0.0.0.0:8080", "--working-directory=/data"]
