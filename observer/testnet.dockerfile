FROM golang:1.17.6 as builder

# Clone repositories
RUN git clone https://github.com/ElrondNetwork/elrond-config-testnet --branch=T1.3.23.1 --depth=1
RUN git clone https://github.com/ElrondNetwork/elrond-go.git --branch=v1.3.23 --depth=1

# Build node
WORKDIR /go/elrond-go/cmd/node
RUN go build -i -v -ldflags="-X main.appVersion=$(git -C /go/elrond-config-testnet describe --tags --long --dirty)"

RUN cp /go/pkg/mod/github.com/!elrond!network/arwen-wasm-vm@$(cat /go/elrond-go/go.mod | grep arwen-wasm-vm | sed 's/.* //' | tail -n 1)/wasmer/libwasmer_linux_amd64.so /lib/libwasmer_linux_amd64.so

# Enable DbLookupExtensions 
RUN sed -i '/\[DbLookupExtensions\]/!b;n;c\\tEnabled = true' /go/elrond-config-testnet/config.toml

# ===== SECOND STAGE ======
FROM ubuntu:20.04

COPY --from=builder "/go/elrond-go/cmd/node" "/elrond/"
COPY --from=builder "/go/elrond-config-testnet" "/elrond/config/"
COPY --from=builder "/lib/libwasmer_linux_amd64.so" "/lib/libwasmer_linux_amd64.so"

EXPOSE 8080
WORKDIR /elrond
ENTRYPOINT ["/elrond/node", "--log-save", "--log-level=*:INFO,core/dblookupext:WARN", "--log-logger-name", "--rest-api-interface=0.0.0.0:8080", "--working-directory=/data"]
