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
    parser.add_argument("--shard", type=int, choices=constants.SHARDS, required=True)
    parser.add_argument("--num-blocks", type=int, default=0)
    args = parser.parse_args()

    mode = args.mode
    configuration = CONFIGURATIONS[args.network]
    shard = args.shard
    num_blocks = args.num_blocks

    process_rosetta = run_rosetta(configuration, shard)
    process_adapter = optionally_run_proxy_to_observer_adapter(configuration, shard)
    process_checker = run_mesh_cli(mode, configuration, shard, num_blocks)

    # Handle termination signals
    def signal_handler(sig: Any, frame: Any):
        process_rosetta.kill()
        process_adapter.kill() if process_adapter else None
        process_checker.kill()
        sys.exit(1)

    signal.signal(signal.SIGINT, signal_handler)
    signal.signal(signal.SIGTERM, signal_handler)

    # Wait for checker to finish
    exit_code = process_checker.wait()

    process_rosetta.kill()
    process_adapter.kill() if process_adapter else None

    time.sleep(1)

    print(f"Checker finished with exit code: {exit_code}.")
    return exit_code


def run_rosetta(configuration: Configuration, shard: int):
    """
    E.g.

    rosetta --port=7091 --observer-http-url=http://localhost:8080 \
        --observer-actual-shard=0 --network-id=D --network-name=devnet --native-currency=EGLD \
        --first-historical-epoch=42 --num-historical-epochs=1
    """

    current_epoch = get_current_epoch(configuration, shard)
    observer_surrogate = f"http://localhost:{configuration.proxy_adapter_port}"
    observer_url = configuration.observer_url or observer_surrogate

    command = [
        str(constants.PATH_ROSETTA),
        f"--port={configuration.rosetta_port}",
        f"--observer-http-url={observer_url}",
        f"--observer-actual-shard={shard}",
        f"--network-id={configuration.network_id}",
        f"--network-name={configuration.network_name}",
        f"--native-currency={configuration.native_currency}",
        f"--config-custom-currencies={configuration.config_file_custom_currencies}",
        f"--first-historical-epoch={current_epoch}",
        f"--num-historical-epochs={configuration.num_historical_epochs}",
        f"--activation-epoch-sirius={configuration.activation_epoch_sirius}",
        f"--activation-epoch-spica={configuration.activation_epoch_spica}",
        "--handle-contracts",
        "--pprof"
    ]

    return subprocess.Popen(command)


def optionally_run_proxy_to_observer_adapter(configuration: Configuration, shard: int) -> Any:
    if configuration.observer_url:
        # If observer URL is provided, we don't need the adapter.
        return None

    command = [
        str(constants.PATH_PROXY_TO_OBSERVER_ADAPTER),
        f"--proxy={configuration.proxy_url}",
        f"--shard={shard}",
        f"--sleep={constants.ADAPTER_DELAY_IN_MILLISECONDS}",
        f"--port={configuration.proxy_adapter_port}"
    ]

    return subprocess.Popen(command)


def run_mesh_cli(mode: str, configuration: Configuration, shard: int, num_blocks: int):
    if mode == "data":
        return run_mesh_cli_with_check_data(configuration, shard, num_blocks)
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
        f"--online-url=http://localhost:{configuration.rosetta_port}",
        f"--offline-url=http://localhost:{configuration.rosetta_port}",
    ]

    return subprocess.Popen(command)


def run_mesh_cli_with_check_construction_custom(configuration: Configuration):
    command = [
        "rosetta-cli",
        "check:construction",
        f"--configuration-file={configuration.check_construction_custom_configuration_file}",
        f"--online-url=http://localhost:{configuration.rosetta_port}",
        f"--offline-url=http://localhost:{configuration.rosetta_port}",
    ]

    return subprocess.Popen(command)


def run_mesh_cli_with_check_data(configuration: Configuration, shard: int, num_blocks: int):
    """
    E.g.

    rosetta-cli check:data --configuration-file devnet-data.json \
        --online-url=http://localhost:7091 --data-dir=devnet-data
    """

    data_dir = f"{configuration.check_data_directory}-{shard}"
    shutil.rmtree(data_dir, ignore_errors=True)

    start_block = get_start_block_for_check_data(configuration, shard, num_blocks)

    command = [
        "rosetta-cli",
        "check:data",
        f"--configuration-file={configuration.check_data_configuration_file}",
        f"--online-url=http://localhost:{configuration.rosetta_port}",
        f"--data-dir={data_dir}",
        f"--start-block={start_block}"
    ]

    return subprocess.Popen(command)


def get_start_block_for_check_data(configuration: Configuration, shard: int, num_blocks: int) -> int:
    if num_blocks:
        latest_block = get_latest_block(configuration, shard)
        return latest_block - num_blocks

    current_epoch = get_current_epoch(configuration, shard)
    first_historical_epoch = max(0, current_epoch - configuration.num_historical_epochs + 1)
    start_block = max(1, get_start_of_epoch(configuration, first_historical_epoch, shard))
    return start_block


def get_latest_block(configuration: Configuration, shard: int) -> int:
    response = requests.get(f"{configuration.proxy_url}/network/status/{shard}")
    response.raise_for_status()
    return response.json()["data"]["status"]["erd_nonce"]


def get_current_epoch(configuration: Configuration, shard: int) -> int:
    response = requests.get(f"{configuration.proxy_url}/network/status/{shard}")
    response.raise_for_status()
    return response.json()["data"]["status"]["erd_epoch_number"]


def get_start_of_epoch(configuration: Configuration, epoch: int, shard: int) -> int:
    response = requests.get(f"{configuration.proxy_url}/network/epoch-start/{shard}/by-epoch/{epoch}")
    response.raise_for_status()
    return response.json()["data"]["epochStart"]["nonce"]


if __name__ == "__main__":
    exit_code = main()
    sys.exit(exit_code)
