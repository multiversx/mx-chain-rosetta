from dataclasses import dataclass


@dataclass
class Configuration:
    network_shard: int
    network_id: str
    network_name: str
    native_currency: str
    num_historical_epochs: int
    proxy_url: str
    check_construction_configuration_file: str
    check_data_configuration_file: str
    check_data_directory: str


CONFIGURATIONS = {
    "devnet": Configuration(
        network_shard=0,
        network_id="D",
        network_name="untitled",
        native_currency="EGLD",
        num_historical_epochs=1,
        proxy_url="https://devnet-gateway.multiversx.com",
        check_construction_configuration_file="systemtests/devnet-construction.json",
        check_data_configuration_file="systemtests/check-data.json",
        check_data_directory="systemtests/devnet-data",
    ),
    "testnet": Configuration(
        network_shard=0,
        network_id="T",
        network_name="untitled",
        native_currency="EGLD",
        num_historical_epochs=3,
        proxy_url="https://testnet-gateway.multiversx.com",
        check_construction_configuration_file="systemtests/testnet-construction.json",
        check_data_configuration_file="systemtests/check-data.json",
        check_data_directory="systemtests/testnet-data",
    ),
}
