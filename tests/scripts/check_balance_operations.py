
from argparse import ArgumentParser
from multiprocessing.dummy import Pool, current_process
import sys
from time import sleep
from typing import Any, Dict, List

import requests
from erdpy import utils
from erdpy.accounts import Account, Address
from erdpy.contracts import SmartContract
from tests.scripts import shared

"""
export ADDRESS=erd1...

python3 ./tests/scripts/check_balance_operations.py --address=${ADDRESS} \
--workspace=./tests/scripts/workspace \
--api-url=https://api.elrond.com \
--gateway-url=http://rosetta-mainnet:8079 \
--rosetta-api-url=http://rosetta-mainnet:9090 \
--node-api-url=http://rosetta-mainnet:8080
"""


def main(cli_args: List[str]):
    parser = ArgumentParser()
    parser.add_argument("--address", required=True)
    parser.add_argument("--workspace", required=True)
    parser.add_argument("--api-url", required=True)
    parser.add_argument("--gateway-url", required=True)
    parser.add_argument("--rosetta-api-url", required=True)
    parser.add_argument("--node-api-url", required=True)

    parsed_args = parser.parse_args(cli_args)

    address = Address(parsed_args.address)
    workspace = parsed_args.workspace
    api_url = parsed_args.api_url
    gateway_url = parsed_args.gateway_url
    rosetta_api_url = parsed_args.rosetta_api_url
    node_api_url = parsed_args.node_api_url

    utils.ensure_folder(workspace)

    # Fetch genesis balance
    genesis_balance = shared.get_node_api_genesis_balance(node_api_url, address)
    print("Genesis balance:", genesis_balance)

    # Fetch current balance
    current_balance = shared.get_gateway_balance(gateway_url, address)
    print("Current balance:", current_balance)

    # Fetch transfers from API
    transfers = shared.get_api_account_transfers(api_url, address)
    analyze_balance_changes_on_api(address, transfers, genesis_balance, current_balance)

    # Fetch blocks nonces for blocks holding the transfers
    transfers_hashes = [item["txHash"] for item in transfers]
    blocks_nonces = shared.get_gateway_block_nonces_of_transactions(gateway_url, transfers_hashes)
    print("Block nonces of interest:", blocks_nonces)

    # Now fetch rosetta blocks
    rosetta_blocks = shared.get_rosetta_blocks(rosetta_api_url, blocks_nonces)
    analyze_balance_changes_on_rosetta(address, rosetta_blocks, genesis_balance, current_balance)

def analyze_balance_changes_on_api(
    address: Address,
    transfers: List[Any],
    genesis_balance: int,
    current_balance: int
):
    api_balance = genesis_balance

    print("Balance changes on API:")

    claim_developer_rewards_txs: List[str] = []

    for transfer in transfers:
        tx_hash = str(transfer["txHash"])
        func = transfer.get("function", None)

        if func == "ClaimDeveloperRewards":
            claim_developer_rewards_txs.append(tx_hash)

    for transfer in transfers:
        tx_hash = str(transfer["txHash"])
        original_tx_hash = transfer.get("originalTxHash", None)
        type = transfer["type"]
        round = transfer.get("round", -1)
        sender = transfer["sender"]
        receiver = transfer["receiver"]
        value = int(transfer["value"])
        fee = int(transfer.get("fee", "0"))

        if type == "SmartContractResult" and sender == receiver and original_tx_hash in claim_developer_rewards_txs:
            api_balance += value
            continue

        if sender == address.bech32():
            api_balance -= value
            api_balance -= fee
        if receiver == address.bech32():
            api_balance += value

        print("...", api_balance, "after round", round, "tx", tx_hash)

    ok = api_balance == current_balance
    if ok:
        print("API balance (computed) == Live balance")
    else:
        print("API balance (computed) != Live balance", api_balance, current_balance)


def analyze_balance_changes_on_rosetta(
    address: Address,
    blocks: List[Any],
    genesis_balance: int,
    current_balance: int
):
    rosetta_balance = genesis_balance

    for block in blocks:
        nonce = block["block_identifier"]["index"]
        txs = block["transactions"]

        for tx in txs:
            operations = tx["operations"]

            for operation in operations:
                op_address = operation["account"]["address"]
                value = int(operation["amount"]["value"])

                if op_address == address.bech32():
                    rosetta_balance += value
                    print("...", value, "in block", nonce)

    ok = rosetta_balance == current_balance
    if ok:
        print("Rosetta balance (computed) == Live balance")
    else:
        print("Rosetta balance (computed) != Live balance", rosetta_balance, current_balance)

if __name__ == "__main__":
    main(sys.argv[1:])
