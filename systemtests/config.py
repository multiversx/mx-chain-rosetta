import os
from dataclasses import dataclass

# These work fine for localnet:
# https://github.com/multiversx/mx-sdk-testwallets/blob/main/users/alice.pem.
DEFAULT_SPONSOR_SECRET_KEY = "413f42575f7f26fad3317a778771212fdb80245850981e48b58a4f25e344e8f9"
# https://github.com/multiversx/mx-sdk-testwallets/blob/main/users/mnemonic.txt.
DEFAULT_USERS_MNEMONIC = "moral volcano peasant pass circle pen over picture flat shop clap goat never lyrics gather prepare woman film husband gravity behind test tiger improve"


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
    activation_epoch_relayed_v3: int
    check_construction_native_configuration_file: str
    check_construction_custom_configuration_file: str
    check_data_configuration_file: str
    check_data_directory: str
    check_data_num_blocks: int
    view_url: str
    custom_currency_issue_cost: int = 50000000000000000
    memento_file: str = ""
    sponsor_secret_key: bytes = bytes.fromhex(os.environ.get("SPONSOR_SECRET_KEY", DEFAULT_SPONSOR_SECRET_KEY))
    users_mnemonic: str = os.environ.get("USERS_MNEMONIC", DEFAULT_USERS_MNEMONIC)
    num_users_per_shard: int = 16
    users_in_projected_shard: bool = False
    generate_relayed_v1: bool = False
    generate_relayed_v2: bool = False
    generate_relayed_v3: bool = False


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
        activation_epoch_spica=1538,
        activation_epoch_relayed_v3=4294967295,
        check_construction_native_configuration_file="",
        check_construction_custom_configuration_file="",
        check_data_configuration_file="systemtests/mesh_cli_config/check-data.json",
        check_data_directory="systemtests/mainnet-data",
        check_data_num_blocks=4096,
        memento_file="systemtests/memento/mainnet.json",
        view_url="https://explorer.multiversx.com/transactions/{hash}",
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
        activation_epoch_spica=2327,
        activation_epoch_relayed_v3=2991,
        check_construction_native_configuration_file="systemtests/mesh_cli_config/devnet-construction-native.json",
        check_construction_custom_configuration_file="systemtests/mesh_cli_config/devnet-construction-custom.json",
        check_data_configuration_file="systemtests/mesh_cli_config/check-data.json",
        check_data_directory="systemtests/devnet-data",
        check_data_num_blocks=0,
        memento_file="systemtests/memento/devnet.json",
        view_url="https://devnet-explorer.multiversx.com/transactions/{hash}",
        users_in_projected_shard=True,
        generate_relayed_v3=True
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
        activation_epoch_spica=33,
        activation_epoch_relayed_v3=1182,
        check_construction_native_configuration_file="systemtests/mesh_cli_config/testnet-construction-native.json",
        check_construction_custom_configuration_file="systemtests/mesh_cli_config/testnet-construction-custom.json",
        check_data_configuration_file="systemtests/mesh_cli_config/check-data.json",
        check_data_directory="systemtests/testnet-data",
        check_data_num_blocks=0,
        memento_file="systemtests/memento/testnet.json",
        view_url="https://testnet-explorer.multiversx.com/transactions/{hash}",
        generate_relayed_v3=True
    ),
    "localnet": Configuration(
        network_shard=0,
        network_id="localnet",
        network_name="untitled",
        native_currency="EGLD",
        config_file_custom_currencies="systemtests/rosetta_config/localnet-custom-currencies.json",
        num_historical_epochs=3,
        observer_url="",
        proxy_url="http://localhost:7950",
        activation_epoch_sirius=1,
        activation_epoch_spica=1,
        activation_epoch_relayed_v3=2,
        check_construction_native_configuration_file="systemtests/mesh_cli_config/localnet-construction-native.json",
        check_construction_custom_configuration_file="systemtests/mesh_cli_config/localnet-construction-custom.json",
        check_data_configuration_file="systemtests/mesh_cli_config/check-data.json",
        check_data_directory="systemtests/localnet-data",
        check_data_num_blocks=0,
        memento_file="systemtests/memento/localnet.json",
        view_url="http://localhost:7950/transaction/{hash}?withResults=true&withLogs=true",
        custom_currency_issue_cost=5000000000000000000,
        generate_relayed_v3=True
    ),
    "internal": Configuration(
        network_shard=0,
        network_id="1",
        network_name="untitled",
        native_currency="EGLD",
        config_file_custom_currencies="systemtests/rosetta_config/internal-custom-currencies.json",
        num_historical_epochs=1,
        observer_url="",
        proxy_url=os.environ.get("INTERNAL_TESTNET_PROXY_URL", ""),
        activation_epoch_sirius=1,
        activation_epoch_spica=1,
        activation_epoch_relayed_v3=2,
        check_construction_native_configuration_file="systemtests/mesh_cli_config/internal-construction-native.json",
        check_construction_custom_configuration_file="systemtests/mesh_cli_config/internal-construction-custom.json",
        check_data_configuration_file="systemtests/mesh_cli_config/check-data.json",
        check_data_directory="systemtests/internal-data",
        check_data_num_blocks=0,
        memento_file="systemtests/memento/internal.json",
        view_url=f"{os.environ.get('INTERNAL_TESTNET_EXPLORER_URL', '')}/transactions/{{hash}}",
        custom_currency_issue_cost=5000000000000000000
    ),
}
