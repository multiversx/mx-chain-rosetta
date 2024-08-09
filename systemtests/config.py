import os
from dataclasses import dataclass
from typing import List


@dataclass
class Configuration:
    network_shard: int
    network_id: str
    network_name: str
    native_currency: str
    config_file_custom_currencies: str
    num_historical_epochs: int
    observer_url: str
    proxy_url: str
    check_construction_native_configuration_file: str
    check_construction_custom_configuration_file: str
    check_data_configuration_file: str
    check_data_directory: str
    legacy_delegation_contract: str
    known_contracts: List[str]
    explorer_url: str
    sponsor_secret_key: bytes = bytes.fromhex(os.environ.get("SPONSOR_SECRET_KEY", ""))
    users_mnemonic: str = os.environ.get("USERS_MNEMONIC", "")


CONFIGURATIONS = {
    "devnet": Configuration(
        network_shard=0,
        network_id="D",
        network_name="untitled",
        native_currency="EGLD",
        config_file_custom_currencies="systemtests/rosetta_config/devnet-custom-currencies.json",
        num_historical_epochs=2,
        observer_url="",
        proxy_url="https://devnet-gateway.multiversx.com",
        check_construction_native_configuration_file="systemtests/mesh_cli_config/devnet-construction-native.json",
        check_construction_custom_configuration_file="systemtests/mesh_cli_config/devnet-construction-custom.json",
        check_data_configuration_file="systemtests/mesh_cli_config/check-data.json",
        check_data_directory="systemtests/devnet-data",
        legacy_delegation_contract="erd1qqqqqqqqqqqqqpgq97wezxw6l7lgg7k9rxvycrz66vn92ksh2tssxwf7ep",
        known_contracts=[],
        explorer_url="https://devnet-explorer.multiversx.com",
    ),
    "testnet": Configuration(
        network_shard=0,
        network_id="T",
        network_name="untitled",
        native_currency="EGLD",
        config_file_custom_currencies="systemtests/rosetta_config/testnet-custom-currencies.json",
        num_historical_epochs=1,
        observer_url="",
        proxy_url="https://testnet-gateway.multiversx.com",
        check_construction_native_configuration_file="systemtests/mesh_cli_config/testnet-construction-native.json",
        check_construction_custom_configuration_file="systemtests/mesh_cli_config/testnet-construction-custom.json",
        check_data_configuration_file="systemtests/mesh_cli_config/check-data.json",
        check_data_directory="systemtests/testnet-data",
        known_contracts=[
            "erd1qqqqqqqqqqqqqpgqagjekf5mxv86hy5c62vvtug5vc6jmgcsq6uq8reras",
            "erd1qqqqqqqqqqqqqpgq89t5xm4x04tnt9lv747wdrsaycf3rcwcggzsa7crse",
            "erd1qqqqqqqqqqqqqpgqeesfamasje5zru7ku79m8p4xqfqnywvqxj0qhtyzdr"
        ],
        legacy_delegation_contract="erd1qqqqqqqqqqqqqpgq97wezxw6l7lgg7k9rxvycrz66vn92ksh2tssxwf7ep",
        explorer_url="https://testnet-explorer.multiversx.com",
    ),
}
