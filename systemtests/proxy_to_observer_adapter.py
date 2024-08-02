import logging
from argparse import ArgumentParser
from typing import Any

import requests
from bottle import Bottle, request, response  # type: ignore

app: Any = Bottle()


class Configuration:
    def __init__(self):
        self.shard = 0
        self.proxy = ""


configuration = Configuration()

logging.basicConfig(level=logging.INFO)


@app.route("/node/status")
def get_node_status() -> Any:
    url = f"{configuration.proxy}/network/status/{configuration.shard}"
    logging.debug(f"get_node_status(): {url}")

    proxy_response = requests.get(url)
    proxy_response.raise_for_status()
    proxy_response_json = proxy_response.json()

    proxy_response_json["data"]["metrics"] = proxy_response_json["data"]["status"]
    proxy_response_json["data"]["metrics"]["erd_app_version"] = "v1.2.3"
    proxy_response_json["data"]["metrics"]["erd_public_key_block_sign"] = "abba"
    del proxy_response_json["data"]["status"]

    return proxy_response_json


@app.route("/node/epoch-start/<epoch:int>")
def get_epoch_start(epoch: int) -> Any:
    url = f"{configuration.proxy}/network/epoch-start/{configuration.shard}/by-epoch/{epoch}"
    logging.debug(f"get_epoch_start(): {url}")

    proxy_response = requests.get(url)
    proxy_response.raise_for_status()
    return proxy_response.json()


@app.route("/block/by-nonce/<nonce:int>")
def get_block_by_nonce(nonce: int) -> Any:
    url = f"{configuration.proxy}/block/{configuration.shard}/by-nonce/{nonce}"
    logging.debug(f"get_block_by_nonce(): {url}")

    params: Any = dict(request.query)  # type: ignore
    proxy_response = requests.get(url, params=params)
    proxy_response.raise_for_status()
    return proxy_response.json()


@app.route("/address/<address>/esdt/<token>")
def get_account_esdt(address: str, token: str) -> Any:
    url = f"{configuration.proxy}/address/{address}/esdt/{token}"
    logging.debug(f"get_account_esdt(): {url}")

    params: Any = dict(request.query)  # type: ignore
    proxy_response = requests.get(url, params=params)
    proxy_response.raise_for_status()
    return proxy_response.json()


@app.route("/address/<address>")
def get_account(address: str) -> Any:
    url = f"{configuration.proxy}/address/{address}"
    logging.debug(f"get_account(): {url}")

    params: Any = dict(request.query)  # type: ignore
    proxy_response = requests.get(url, params=params)
    proxy_response_json = proxy_response.json()
    proxy_response.raise_for_status()

    return proxy_response_json


@app.route("/transaction/send", method="POST")
def send_transaction():
    url = f"{configuration.proxy}/transaction/send"
    logging.debug(f"send_transaction(): {url}")

    data = request.json
    proxy_response = requests.post(url, json=data)
    proxy_response_json = proxy_response.json()

    response.status = proxy_response.status_code

    return proxy_response_json


def main():
    parser = ArgumentParser()
    parser.add_argument("--proxy", required=True)
    parser.add_argument("--shard", type=int, required=True)
    args = parser.parse_args()

    configuration.proxy = args.proxy
    configuration.shard = args.shard

    app.run(host="localhost", port=8080, quiet=True)


if __name__ == "__main__":
    main()
