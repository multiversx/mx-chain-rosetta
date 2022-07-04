FROM golang:1.18 as builder

# Clone repositories
RUN git clone https://github.com/ElrondNetwork/rosetta.git --branch=main --depth=1

# Build
WORKDIR /go/rosetta/cmd/rosetta
RUN go build

# ===== SECOND STAGE ======
FROM ubuntu:20.04

COPY --from=builder "/go/rosetta/cmd/rosetta" "/elrond/"

EXPOSE 8080
WORKDIR /elrond
ENTRYPOINT ["/elrond/rosetta", "--logs-folder=/data"]
