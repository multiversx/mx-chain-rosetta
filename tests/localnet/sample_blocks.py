from typing import Any

import requests
from erdpy import utils

hyperblock_start = 1
hyperblock_end = 80

# Fetch hyperblocks
for nonce in range(hyperblock_start, hyperblock_end):
    url = f"http://localhost:8090/hyperblock/by-nonce/{nonce}"
    print(url)
    response = requests.get(url)
    response.raise_for_status()
    data: Any = response.json()

    data["data"]["hyperblock"]["transactions"].sort(key=lambda tx: tx["hash"])

    utils.write_json_file(f"localnet_{nonce}_hyperblock.json", data)

# Fetch Rosetta API blocks
for nonce in range(hyperblock_start, hyperblock_end):
    url = "http://localhost:8091/block"
    print(url)
    response = requests.post(url, json={
        "network_identifier": {
            "blockchain": "Elrond",
            "network": "localnet"
        },
        "block_identifier": {
            "index": nonce
        }
    })
    response.raise_for_status()
    data: Any = response.json()

    data["block"]["transactions"].sort(key=lambda tx: tx["transaction_identifier"]["hash"])

    utils.write_json_file(f"localnet_{nonce}_rosetta.json", data)
