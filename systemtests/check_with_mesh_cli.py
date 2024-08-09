import shutil
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
    parser.add_argument("--mode", choices=["data", "construction-native", "construction-custom"], required=True)
    parser.add_argument("--network", choices=CONFIGURATIONS.keys(), required=True)
    args = parser.parse_args()

    mode = args.mode
    configuration = CONFIGURATIONS[args.network]

    process_rosetta = run_rosetta(configuration)
    process_adapter = run_proxy_to_observer_adapter(configuration)
    process_checker = run_mesh_cli(mode, configuration)

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
        f"--observer-http-url={constants.URL_OBSERVER_SURROGATE}",
        f"--observer-actual-shard={configuration.network_shard}",
        f"--network-id={configuration.network_id}",
        f"--network-name={configuration.network_name}",
        f"--native-currency={configuration.native_currency}",
        f"--config-custom-currencies={configuration.config_file_custom_currencies}",
        f"--first-historical-epoch={current_epoch}",
        f"--num-historical-epochs={configuration.num_historical_epochs}",
    ]

    return subprocess.Popen(command)


def run_proxy_to_observer_adapter(configuration: Configuration):
    command = [
        str(constants.PATH_PROXY_TO_OBSERVER_ADAPTER),
        f"--proxy={configuration.proxy_url}",
        f"--shard={configuration.network_shard}",
        f"--sleep={constants.ADAPTER_DELAY_IN_MILLISECONDS}"
    ]

    return subprocess.Popen(command)


def run_mesh_cli(mode: str, configuration: Configuration):
    if mode == "data":
        return run_mesh_cli_with_check_data(configuration)
    elif mode == "construction-native":
        return run_mesh_cli_with_check_construction_native(configuration)
    elif mode == "construction-custom":
        return run_mesh_cli_with_check_construction_custom(configuration)
    else:
        raise ValueError(f"Unknown mode: {mode}")


def run_mesh_cli_with_check_construction_native(configuration: Configuration):
    """
    E.g.

    rosetta-cli check:construction --configuration-file devnet-construction.json \
        --online-url=http://localhost:7091 --offline-url=http://localhost:7091
    """

    command = [
        "rosetta-cli",
        "check:construction",
        f"--configuration-file={configuration.check_construction_native_configuration_file}",
        f"--online-url=http://localhost:{constants.PORT_ROSETTA}",
        f"--offline-url=http://localhost:{constants.PORT_ROSETTA}",
    ]

    return subprocess.Popen(command)


def run_mesh_cli_with_check_construction_custom(configuration: Configuration):
    command = [
        "rosetta-cli",
        "check:construction",
        f"--configuration-file={configuration.check_construction_custom_configuration_file}",
        f"--online-url=http://localhost:{constants.PORT_ROSETTA}",
        f"--offline-url=http://localhost:{constants.PORT_ROSETTA}",
    ]

    return subprocess.Popen(command)


def run_mesh_cli_with_check_data(configuration: Configuration):
    """
    E.g.

    rosetta-cli check:data --configuration-file devnet-data.json \
        --online-url=http://localhost:7091 --data-dir=devnet-data
    """

    shutil.rmtree(configuration.check_data_directory, ignore_errors=True)

    current_epoch = get_current_epoch(configuration)
    first_historical_epoch = current_epoch - configuration.num_historical_epochs + 1
    start_block = get_start_of_epoch(configuration, first_historical_epoch)

    command = [
        "rosetta-cli",
        "check:data",
        f"--configuration-file={configuration.check_data_configuration_file}",
        f"--online-url=http://localhost:{constants.PORT_ROSETTA}",
        f"--data-dir={configuration.check_data_directory}",
        f"--start-block={start_block}"
    ]

    return subprocess.Popen(command)


def get_current_epoch(configuration: Configuration) -> int:
    response = requests.get(f"{configuration.proxy_url}/network/status/{configuration.network_shard}")
    response.raise_for_status()
    return response.json()["data"]["status"]["erd_epoch_number"]


def get_start_of_epoch(configuration: Configuration, epoch: int) -> int:
    response = requests.get(f"{configuration.proxy_url}/network/epoch-start/{configuration.network_shard}/by-epoch/{epoch}")
    response.raise_for_status()
    return response.json()["data"]["epochStart"]["nonce"]


if __name__ == "__main__":
    exit_code = main()
    sys.exit(exit_code)
