import sys
from argparse import ArgumentParser
from os.path import expanduser
from typing import Any, Dict, List

from erdpy import utils


def main(cli_args: List[str]):
    parser = ArgumentParser()
    parser.add_argument("--input", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--currency", default="eGLD")
    args = parser.parse_args(cli_args)

    input_file = expanduser(args.input)
    output_file = expanduser(args.output)
    currency = args.currency

    lines = utils.read_lines(input_file)
    balances_data = []

    for line in lines:
        parts = line.split(",")
        
        if len(parts) != 2:
            continue

        address: str = parts[0].strip()
        balance: str = parts[1].strip()
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
