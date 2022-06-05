import sys
from argparse import ArgumentParser
from os.path import expanduser
from typing import Any, Dict, List

from erdpy import utils


def main(cli_args: List[str]):
    parser = ArgumentParser()
    parser.add_argument("--input", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--currency", default="XeGLD")
    args = parser.parse_args(cli_args)

    input_file = expanduser(args.input)
    output_file = expanduser(args.output)
    currency = args.currency

    genesis_data: List[Dict[str, Any]] = utils.read_json_file(input_file)
    balances_data = []

    for genesis_entry in genesis_data:
        address: str = genesis_entry["address"]
        balance: str = genesis_entry["balance"]
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
            balances_data.append(balances_entry)

    utils.write_json_file(output_file, balances_data)


if __name__ == "__main__":
    main(sys.argv[1:])
