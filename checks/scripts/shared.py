
from typing import Any, List

from erdpy.accounts import Address
import requests


def get_node_api_genesis_balance(node_api_url: str, address: Address) -> int:
    url = f"{node_api_url}/network/genesis-balances"
    balances = do_get(url)["data"]["balances"]
    balance = next((item for item in balances if item["address"] == address.bech32()), None)
    return int(balance) if balance else 0


def get_api_account_transfers(api_url: str, address: Address) -> List[Any]:
    url = f"{api_url}/accounts/{address.bech32()}/transfers?size=500"
    transfers = do_get(url)
    transfers.sort(key=lambda item: item["timestamp"])
    return transfers


def get_gateway_block_nonces_of_transactions(gateway_url: str, transactionHashes: List[str]) -> List[int]:
    nonces: List[int] = []

    for txHash in transactionHashes:
        try:
            tx = get_gateway_transaction(gateway_url, txHash)
            nonces.append(tx.get("blockNonce", 0))
        except:
            print("Probably intra-shard SCR", txHash)

    nonces.sort()
    return nonces


def get_gateway_transaction(gateway_url, transactionHash: str) -> Any:
    url = f"{gateway_url}/transaction/{transactionHash}?withResults=false&withLogs=false"
    return do_get(url)["data"]["transaction"]


def get_gateway_balance(gateway_url, address: Address) -> int:
    url = f"{gateway_url}/address/{address.bech32()}/balance"
    balance = do_get(url)["data"]["balance"]
    return int(balance)


def get_rosetta_blocks(rosetta_url: str, nonces: List[int]) -> List[Any]:
    blocks: List[Any] = []

    for nonce in nonces:
        block = get_rosetta_block(rosetta_url, nonce)
        blocks.append(block)

    return blocks


def get_rosetta_block(rosetta_url: str, nonce: int) -> Any:
    url = f"{rosetta_url}/block"
    request_data = {
        "network_identifier": {
            "blockchain": "Elrond",
            "network": "1"
        },
        "block_identifier": {
            "index": nonce
        }
    }

    response = do_post(url, request_data)
    return response["block"]


def do_get(url: str) -> Any:
    response = requests.get(url)
    response.raise_for_status()
    data = response.json()
    return data


def do_post(url: str, data: Any) -> Any:
    response = requests.post(url, json=data)
    response.raise_for_status()
    data = response.json()
    return data
