import signal
import subprocess
import sys
import time
from argparse import ArgumentParser
from typing import Any

import requests

from systemtests import constants
from systemtests.config import CONFIGURATIONS, Configuration


def main() -> int:
    parser = ArgumentParser()
    parser.add_argument("--network", choices=CONFIGURATIONS.keys(), required=True)
    args = parser.parse_args()

    configuration = CONFIGURATIONS[args.network]

    process_rosetta = run_rosetta(configuration)
    process_adapter = run_proxy_to_observer_adapter(configuration)
    process_checker = run_rosetta_checker(configuration)

    # Handle termination signals
    def signal_handler(sig: Any, frame: Any):
        process_rosetta.kill()
        process_adapter.kill()
        process_checker.kill()
        sys.exit(1)

    signal.signal(signal.SIGINT, signal_handler)
    signal.signal(signal.SIGTERM, signal_handler)

    # Wait for checker to finish
    exit_code = process_checker.wait()

    process_rosetta.kill()
    process_adapter.kill()

    time.sleep(1)

    print(f"Checker finished with exit code: {exit_code}.")
    return exit_code


def run_rosetta(configuration: Configuration):
    """
    E.g.

    rosetta --port=7091 --observer-http-url=http://localhost:8080 \
        --observer-actual-shard=0 --network-id=D --network-name=devnet --native-currency=EGLD \
        --first-historical-epoch=42 --num-historical-epochs=1
    """

    current_epoch = get_current_epoch(configuration)

    command = [
        str(constants.PATH_ROSETTA),
        f"--port={constants.PORT_ROSETTA}",
        f"--observer-http-url=http://localhost:{constants.PORT_OBSERVER_SURROGATE}",
        f"--observer-actual-shard={configuration.network_shard}",
        f"--network-id={configuration.network_id}",
        f"--network-name={configuration.network_name}",
        f"--native-currency={configuration.native_currency}",
        f"--first-historical-epoch={current_epoch}",
        f"--num-historical-epochs={configuration.num_historical_epochs}",
    ]

    return subprocess.Popen(command)


def get_current_epoch(configuration: Configuration) -> int:
    response = requests.get(f"{configuration.proxy_url}/network/status/{configuration.network_shard}")
    response.raise_for_status()
    return response.json()["data"]["status"]["erd_epoch_number"]


def run_proxy_to_observer_adapter(configuration: Configuration):
    command = [
        str(constants.PATH_PROXY_TO_OBSERVER_ADAPTER),
        f"{configuration.proxy_url}",
        f"{configuration.network_shard}",
    ]

    return subprocess.Popen(command)


def run_rosetta_checker(configuration: Configuration):
    """
    E.g.

    rosetta-cli check:construction --configuration-file devnet-construction.json \
        --online-url=http://localhost:7091 --offline-url=http://localhost:7091
    """

    command = [
        "rosetta-cli",
        "check:construction",
        f"--configuration-file={configuration.check_construction_configuration_file}",
        f"--online-url=http://localhost:{constants.PORT_ROSETTA}",
        f"--offline-url=http://localhost:{constants.PORT_ROSETTA}",
    ]

    return subprocess.Popen(command)


if __name__ == "__main__":
    exit_code = main()
    sys.exit(exit_code)
