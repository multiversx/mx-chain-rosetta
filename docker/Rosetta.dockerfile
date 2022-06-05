FROM golang:1.17.6 as builder

# Clone repositories
RUN git clone https://github.com/ElrondNetwork/rosetta.git --branch=v0.2.0 --depth=1

# Build
WORKDIR /go/rosetta/cmd/rosetta
RUN go build

# ===== SECOND STAGE ======
FROM ubuntu:20.04

COPY --from=builder "/go/rosetta/cmd/rosetta" "/rosetta/"

EXPOSE 8080
WORKDIR /elrond
ENTRYPOINT ["/elrond/rosetta", "--logs-folder=/data"]
