import subprocess

import requests

from systemtests import constants
from systemtests.config import Configuration


def run_proxy_to_observer_adapter(configuration: Configuration):
    command = [
        str(constants.PATH_PROXY_TO_OBSERVER_ADAPTER),
        f"--proxy={configuration.proxy_url}",
        f"--shard={configuration.network_shard}",
        f"--sleep={constants.ADAPTER_DELAY_IN_MILLISECONDS}"
    ]

    return subprocess.Popen(command)


def get_current_epoch(configuration: Configuration) -> int:
    response = requests.get(f"{configuration.proxy_url}/network/status/{configuration.network_shard}")
    response.raise_for_status()
    return response.json()["data"]["status"]["erd_epoch_number"]
