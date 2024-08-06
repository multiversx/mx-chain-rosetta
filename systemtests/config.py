from dataclasses import dataclass


@dataclass
class Configuration:
    network_shard: int
    network_id: str
    network_name: str
    native_currency: str
    config_file_custom_currencies: str
    num_historical_epochs: int
    proxy_url: str
    check_construction_native_configuration_file: str
    check_construction_custom_configuration_file: str
    check_data_configuration_file: str
    check_data_directory: str


CONFIGURATIONS = {
    "devnet": Configuration(
        network_shard=0,
        network_id="D",
        network_name="untitled",
        native_currency="EGLD",
        config_file_custom_currencies="systemtests/rosetta_config/devnet-custom-currencies.json",
        num_historical_epochs=2,
        proxy_url="https://devnet-gateway.multiversx.com",
        check_construction_native_configuration_file="systemtests/mesh_cli_config/devnet-construction.json",
        check_construction_custom_configuration_file="systemtests/mesh_cli_config/devnet-construction-custom.json",
        check_data_configuration_file="systemtests/mesh_cli_config/check-data.json",
        check_data_directory="systemtests/devnet-data",
    ),
    "testnet": Configuration(
        network_shard=0,
        network_id="T",
        network_name="untitled",
        native_currency="EGLD",
        config_file_custom_currencies="systemtests/rosetta_config/testnet-custom-currencies.json",
        num_historical_epochs=2,
        proxy_url="https://testnet-gateway.multiversx.com",
        check_construction_native_configuration_file="systemtests/mesh_cli_config/testnet-construction.json",
        check_construction_custom_configuration_file="systemtests/mesh_cli_config/testnet-construction-custom.json",
        check_data_configuration_file="systemtests/mesh_cli_config/check-data.json",
        check_data_directory="systemtests/testnet-data",
    ),
}
