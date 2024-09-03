import os
from dataclasses import dataclass


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
    activation_epoch_sirius: int
    activation_epoch_spica: int
    check_construction_native_configuration_file: str
    check_construction_custom_configuration_file: str
    check_data_configuration_file: str
    check_data_directory: str
    explorer_url: str
    memento_file: str = ""
    sponsor_secret_key: bytes = bytes.fromhex(os.environ.get("SPONSOR_SECRET_KEY", ""))
    users_mnemonic: str = os.environ.get("USERS_MNEMONIC", "")


CONFIGURATIONS = {
    "mainnet": Configuration(
        network_shard=0,
        network_id="1",
        network_name="untitled",
        native_currency="EGLD",
        config_file_custom_currencies="systemtests/rosetta_config/mainnet-custom-currencies.json",
        num_historical_epochs=1,
        observer_url="",
        proxy_url=os.environ.get("MAINNET_PROXY_URL", "https://gateway.multiversx.com"),
        activation_epoch_sirius=1265,
        activation_epoch_spica=4294967295,
        check_construction_native_configuration_file="",
        check_construction_custom_configuration_file="",
        check_data_configuration_file="systemtests/mesh_cli_config/check-data.json",
        check_data_directory="systemtests/mainnet-data",
        memento_file="systemtests/memento/mainnet.json",
        explorer_url="https://explorer.multiversx.com",
    ),
    "devnet": Configuration(
        network_shard=0,
        network_id="D",
        network_name="untitled",
        native_currency="EGLD",
        config_file_custom_currencies="systemtests/rosetta_config/devnet-custom-currencies.json",
        num_historical_epochs=1,
        observer_url="",
        proxy_url="https://devnet-gateway.multiversx.com",
        activation_epoch_sirius=629,
        activation_epoch_spica=4294967295,
        check_construction_native_configuration_file="systemtests/mesh_cli_config/devnet-construction-native.json",
        check_construction_custom_configuration_file="systemtests/mesh_cli_config/devnet-construction-custom.json",
        check_data_configuration_file="systemtests/mesh_cli_config/check-data.json",
        check_data_directory="systemtests/devnet-data",
        memento_file="systemtests/memento/devnet.json",
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
        activation_epoch_sirius=1,
        activation_epoch_spica=4294967295,
        check_construction_native_configuration_file="systemtests/mesh_cli_config/testnet-construction-native.json",
        check_construction_custom_configuration_file="systemtests/mesh_cli_config/testnet-construction-custom.json",
        check_data_configuration_file="systemtests/mesh_cli_config/check-data.json",
        check_data_directory="systemtests/testnet-data",
        memento_file="systemtests/memento/testnet.json",
        explorer_url="https://testnet-explorer.multiversx.com",
    ),
    "localnet": Configuration(
        network_shard=0,
        network_id="localnet",
        network_name="untitled",
        native_currency="EGLD",
        config_file_custom_currencies="systemtests/rosetta_config/localnet-custom-currencies.json",
        num_historical_epochs=2,
        observer_url="",
        proxy_url="http://localhost:7950",
        activation_epoch_sirius=1,
        activation_epoch_spica=4294967295,
        check_construction_native_configuration_file="systemtests/mesh_cli_config/localnet-construction-native.json",
        check_construction_custom_configuration_file="systemtests/mesh_cli_config/localnet-construction-custom.json",
        check_data_configuration_file="systemtests/mesh_cli_config/check-data.json",
        check_data_directory="systemtests/localnet-data",
        memento_file="systemtests/memento/localnet.json",
        explorer_url="",
    ),
}
