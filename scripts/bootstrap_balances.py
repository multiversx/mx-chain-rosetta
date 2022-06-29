import sys
from argparse import ArgumentParser
from os.path import expanduser
from typing import Any, Dict, List

from erdpy import utils

"""
python3 ./scripts/bootstrap_balances.py --input=balances_epoch_678_nonce_9761083.json --output=rosetta_balances_epoch_678_nonce_9761083.json --currency=EGLD
"""

def main(cli_args: List[str]):
    parser = ArgumentParser()
    parser.add_argument("--input", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--currency", default="EGLD")
    args = parser.parse_args(cli_args)

    input_file = expanduser(args.input)
    output_file = expanduser(args.output)
    currency = args.currency

    input_balances_data: List[Dict[str, Any]] = utils.read_json_file(input_file)
    output_balances_data = []

    for item in input_balances_data:
        address: str = item["address"]
        balance: str = item["balance"]
        balances_entry = {
            "account_identifier": {
                "address": address
            },
            "currency": {
                "symbol": currency,
                "decimals": 18
            },
            "value": balance
        }

        if balance and balance != "0":
            output_balances_data.append(balances_entry)

    utils.write_json_file(output_file, output_balances_data)


if __name__ == "__main__":
    main(sys.argv[1:])
