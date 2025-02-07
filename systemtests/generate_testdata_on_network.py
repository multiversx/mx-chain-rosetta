import json
import os
import time
from argparse import ArgumentParser
from multiprocessing.dummy import Pool
from pathlib import Path
from typing import Any, Callable, Dict, List, Optional

from multiversx_sdk import (AccountOnNetwork, Address, AddressComputer,
                            AwaitingOptions, Mnemonic, ProxyNetworkProvider,
                            RelayedTransactionsFactory,
                            SmartContractController,
                            SmartContractTransactionsFactory, Token,
                            TokenManagementTransactionsFactory,
                            TokenManagementTransactionsOutcomeParser,
                            TokenTransfer, Transaction, TransactionComputer,
                            TransactionOnNetwork, TransactionsFactoryConfig,
                            TransferTransactionsFactory, UserSecretKey,
                            UserSigner)
from multiversx_sdk.abi import (AddressValue, BigUIntValue, BytesValue,
                                I32Value, I64Value, Serializer, StringValue,
                                U32Value)
from multiversx_sdk.core.address import get_shard_of_pubkey

from systemtests.config import CONFIGURATIONS, Configuration
from systemtests.constants import (ADDITIONAL_GAS_LIMIT_FOR_RELAYED_V3,
                                   AWAITING_PATIENCE_IN_MILLISECONDS,
                                   AWAITING_POLLING_TIMEOUT_IN_MILLISECONDS,
                                   NUM_SHARDS)

CONTRACT_PATH_ADDER = Path(__file__).parent / "contracts" / "adder.wasm"
CONTRACT_PATH_DUMMY = Path(__file__).parent / "contracts" / "dummy.wasm"
CONTRACT_PATH_FORWARDER = Path(__file__).parent / "contracts" / "forwarder.wasm"
CONTRACT_PATH_DEVELOPER_REWARDS = Path(__file__).parent / "contracts" / "developer_rewards.wasm"

SOME_SHARD = 0
OTHER_SHARD = 1


def main():
    parser = ArgumentParser()

    subparsers = parser.add_subparsers()
    subparser_setup = subparsers.add_parser("setup")
    subparser_setup.add_argument("--network", choices=CONFIGURATIONS.keys(), required=True)
    subparser_setup.set_defaults(func=do_setup)

    subparser_run = subparsers.add_parser("run")
    subparser_run.add_argument("--network", choices=CONFIGURATIONS.keys(), required=True)
    subparser_run.set_defaults(func=do_run)

    args = parser.parse_args()

    if not hasattr(args, "func"):
        parser.print_help()
    else:
        args.func(args)


def do_setup(args: Any):
    print("Phase [setup] started...")

    network = args.network
    configuration = CONFIGURATIONS[network]
    memento = Memento(Path(configuration.memento_file))
    accounts = BunchOfAccounts(configuration, memento)
    controller = Controller(configuration, accounts, memento)

    memento.clear()

    controller.wait_until_epoch(configuration.activation_epoch_sirius)

    print("Do airdrops for native currency...")
    controller.do_airdrops_for_native_currency()

    print("Issue custom currency...")
    controller.issue_custom_currency("ROSETTA")

    print("Do airdrops for custom currencies...")
    controller.do_airdrops_for_custom_currencies()

    print("Do contract deployments...")
    controller.do_create_contract_deployments()

    print("Setup done.")


def do_run(args: Any):
    print("Phase [run] started...")

    network = args.network
    configuration = CONFIGURATIONS[network]
    memento = Memento(Path(configuration.memento_file))
    accounts = BunchOfAccounts(configuration, memento)
    controller = Controller(configuration, accounts, memento)

    controller.wait_until_epoch(configuration.activation_epoch_relayed_v3)

    print("## Intra-shard, simple MoveBalance with refund")
    controller.send(controller.create_transfer(
        sender=accounts.get_user(shard=SOME_SHARD, index=0),
        receiver=accounts.get_user(shard=SOME_SHARD, index=1).address,
        native_amount=42,
        custom_amount=0,
        additional_gas_limit=42000,
    ), await_processing_started=True)

    print("## Cross-shard, simple MoveBalance with refund")
    controller.send(controller.create_transfer(
        sender=accounts.get_user(shard=SOME_SHARD, index=1),
        receiver=accounts.get_user(shard=OTHER_SHARD, index=0).address,
        native_amount=42,
        custom_amount=0,
        additional_gas_limit=42000,
    ), await_processing_started=True)

    print("## Intra-shard, invalid MoveBalance with refund")
    controller.send(controller.create_transfer(
        sender=accounts.get_user(shard=SOME_SHARD, index=2),
        receiver=accounts.get_user(shard=SOME_SHARD, index=3).address,
        native_amount=1000000000000000000000000,
        custom_amount=0,
        additional_gas_limit=42000,
    ), await_processing_started=True)

    print("## Cross-shard, invalid MoveBalance with refund")
    controller.send(controller.create_transfer(
        sender=accounts.get_user(shard=SOME_SHARD, index=3),
        receiver=accounts.get_user(shard=OTHER_SHARD, index=1).address,
        native_amount=1000000000000000000000000,
        custom_amount=0,
        additional_gas_limit=42000,
    ), await_processing_started=True)

    print("## Intra-shard, sending value to non-payable contract")
    controller.send(controller.create_transfer(
        sender=accounts.get_user(shard=SOME_SHARD, index=0),
        receiver=accounts.get_contract_address("adder", shard=SOME_SHARD, index=0),
        native_amount=42,
        custom_amount=0,
        additional_gas_limit=42000,
    ), await_processing_started=True)

    print("## Cross-shard, sending value to non-payable contract")
    controller.send(controller.create_transfer(
        sender=accounts.get_user(shard=SOME_SHARD, index=1),
        receiver=accounts.get_contract_address("adder", shard=OTHER_SHARD, index=0),
        native_amount=42,
        custom_amount=0,
        additional_gas_limit=42000,
    ), await_processing_started=True)

    print("## Intra-shard, native transfer within MultiESDTTransfer")
    controller.send(controller.create_transfer(
        sender=accounts.get_user(shard=SOME_SHARD, index=0),
        receiver=accounts.get_user(shard=SOME_SHARD, index=1).address,
        native_amount=42,
        custom_amount=7
    ), await_processing_started=True)

    print("## Cross-shard, native transfer within MultiESDTTransfer")
    controller.send(controller.create_transfer(
        sender=accounts.get_user(shard=SOME_SHARD, index=1),
        receiver=accounts.get_user(shard=OTHER_SHARD, index=0).address,
        native_amount=42,
        custom_amount=7
    ), await_processing_started=True)

    print("## Intra-shard, native transfer within MultiESDTTransfer, towards non-payable contract")
    controller.send(controller.create_transfer(
        sender=accounts.get_user(shard=SOME_SHARD, index=0),
        receiver=accounts.get_contract_address("adder", shard=SOME_SHARD, index=0),
        native_amount=42,
        custom_amount=7
    ), await_processing_started=True)

    print("## Cross-shard, native transfer within MultiESDTTransfer, towards non-payable contract")
    controller.send(controller.create_transfer(
        sender=accounts.get_user(shard=SOME_SHARD, index=1),
        receiver=accounts.get_contract_address("adder", shard=OTHER_SHARD, index=0),
        native_amount=42,
        custom_amount=7
    ), await_processing_started=True)

    print("## Cross-shard, transfer & execute with native & custom transfer")
    controller.send(controller.create_transfer_and_execute(
        sender=accounts.get_user(shard=SOME_SHARD, index=1),
        contract=accounts.get_contract_address("dummy", shard=SOME_SHARD, index=0),
        function="doSomething",
        arguments=[],
        gas_limit=3_000_000,
        native_amount=42,
        custom_amount=7,
    ), await_processing_started=True)

    print("## Intra-shard, transfer & execute with native & custom transfer")
    controller.send(controller.create_transfer_and_execute(
        sender=accounts.get_user(shard=SOME_SHARD, index=3),
        contract=accounts.get_contract_address("dummy", shard=SOME_SHARD, index=0),
        function="doSomething",
        arguments=[],
        gas_limit=3_000_000,
        native_amount=42,
        custom_amount=7,
    ), await_processing_started=True)

    print("## Direct contract deployment with MoveBalance")
    controller.send(controller.create_contract_deployment(
        sender=accounts.get_user(shard=SOME_SHARD, index=0),
        bytecode=CONTRACT_PATH_DUMMY,
        gas_limit=5_000_000,
        arguments=[],
        amount=10000000000000000,
    ), await_processing_started=True)

    print("## Direct contract deployment with MoveBalance, with signal error")
    controller.send(controller.create_contract_deployment(
        sender=accounts.get_user(shard=SOME_SHARD, index=0),
        bytecode=CONTRACT_PATH_ADDER,
        gas_limit=5_000_000,
        arguments=[BigUIntValue(41), BigUIntValue(42)],
        amount=77
    ), await_processing_started=True)

    print("## Intra-shard, contract call with MoveBalance, with signal error")
    controller.send(controller.create_transfer_and_execute(
        sender=accounts.get_user(shard=SOME_SHARD, index=0),
        contract=accounts.get_contract_address("adder", shard=SOME_SHARD, index=0),
        function="missingFunction",
        arguments=[BigUIntValue(42)],
        gas_limit=5_000_000,
        native_amount=10000000000000000,
        custom_amount=0,
    ), await_processing_started=True)

    print("## Cross-shard, contract call with MoveBalance, with signal error")
    controller.send(controller.create_transfer_and_execute(
        sender=accounts.get_user(shard=SOME_SHARD, index=0),
        contract=accounts.get_contract_address("adder", shard=OTHER_SHARD, index=0),
        function="missingFunction",
        arguments=[BigUIntValue(42)],
        gas_limit=5_000_000,
        native_amount=10000000000000000,
        custom_amount=0,
    ), await_processing_started=True)

    print("## Intra-shard ClaimDeveloperRewards on directly owned contract")
    controller.send(controller.create_claim_developer_rewards_on_directly_owned_contract(
        sender=accounts.get_user(shard=SOME_SHARD, index=0),
        contract=accounts.get_contract_address("adder", shard=SOME_SHARD, index=0),
    ), await_processing_started=True)

    print("## Cross-shard ClaimDeveloperRewards on directly owned contract")
    controller.do_change_contract_owner(
        contract=accounts.get_contract_address("adder", shard=OTHER_SHARD, index=0),
        new_owner=accounts.get_user(shard=SOME_SHARD, index=0)
    )

    controller.send(controller.create_claim_developer_rewards_on_directly_owned_contract(
        sender=accounts.get_user(shard=SOME_SHARD, index=0),
        contract=accounts.get_contract_address("adder", shard=OTHER_SHARD, index=0),
    ))

    print("## ClaimDeveloperRewards on child contract (owned by a contract); user & parent contract in same shard")
    controller.send(controller.create_claim_developer_rewards_on_child_contract(
        sender=accounts.get_user(shard=SOME_SHARD, index=0),
        parent_contract=accounts.get_contract_address("developerRewards", shard=SOME_SHARD, index=0),
    ), await_processing_started=True)

    print("## ClaimDeveloperRewards on child contract (owned by a contract); user & parent contract in different shards")
    controller.send(controller.create_claim_developer_rewards_on_child_contract(
        sender=accounts.get_user(shard=SOME_SHARD, index=0),
        parent_contract=accounts.get_contract_address("developerRewards", shard=OTHER_SHARD, index=0),
    ), await_processing_started=True)

    print("## Intra-shard, transfer native within multi-transfer (fuzzy, with tx.value != 0)")
    controller.send(controller.create_arbitrary_transaction(
        sender=accounts.get_user(shard=SOME_SHARD, index=2),
        # receiver := sender, since we're using multi-transfer.
        receiver=accounts.get_user(shard=SOME_SHARD, index=2).address,
        value=42,
        data="MultiESDTNFTTransfer@" + Serializer().serialize([
            AddressValue.new_from_address(accounts.get_user(shard=SOME_SHARD, index=3).address),
            U32Value(2),
            StringValue("EGLD-000000"),
            U32Value(0),
            U32Value(42),
            StringValue("EGLD-000000"),
            U32Value(0),
            U32Value(43)
        ]),
        gas_limit=5_000_000
    ), await_processing_started=True)

    print("## Intra-shard, transfer native within multi-transfer (fuzzy, with tx.value == 0, but transfer to self)")
    controller.send(controller.create_arbitrary_transaction(
        sender=accounts.get_user(shard=SOME_SHARD, index=3),
        # receiver := sender, since we're using multi-transfer.
        receiver=accounts.get_user(shard=SOME_SHARD, index=3).address,
        value=0,
        data="MultiESDTNFTTransfer@" + Serializer().serialize([
            AddressValue.new_from_address(accounts.get_user(shard=SOME_SHARD, index=3).address),
            U32Value(2),
            StringValue("EGLD-000000"),
            U32Value(0),
            U32Value(42),
            StringValue("EGLD-000000"),
            U32Value(0),
            U32Value(43)
        ]),
        gas_limit=5_000_000
    ), await_processing_started=True)

    print("## Cross-shard, transfer native within multi-transfer (fuzzy, with tx.value == 0)")
    controller.send(controller.create_arbitrary_transaction(
        sender=accounts.get_user(shard=SOME_SHARD, index=2),
        # receiver := sender, since we're using multi-transfer.
        receiver=accounts.get_user(shard=SOME_SHARD, index=2).address,
        value=0,
        data="MultiESDTNFTTransfer@" + Serializer().serialize([
            AddressValue.new_from_address(accounts.get_user(shard=OTHER_SHARD, index=3).address),
            U32Value(2),
            StringValue("EGLD-000000"),
            U32Value(0),
            U32Value(42),
            StringValue("EGLD-000000"),
            U32Value(0),
            U32Value(43)
        ]),
        gas_limit=5_000_000
    ), await_processing_started=True)

    relayed_v1_marker = [1] if configuration.generate_relayed_v1 else []
    relayed_v2_marker = [2] if configuration.generate_relayed_v2 else []
    relayed_v3_marker = [3] if configuration.generate_relayed_v3 else []

    for relayed_version in relayed_v1_marker + relayed_v3_marker:
        print(f"## Relayed v{relayed_version}, intra-shard MoveBalance")
        controller.send(controller.create_relayed_transfer(
            relayer=accounts.get_user(shard=SOME_SHARD, index=0),
            sender=accounts.get_user(shard=SOME_SHARD, index=1),
            receiver=accounts.get_user(shard=SOME_SHARD, index=2).address,
            native_amount=42,
            custom_amount=0,
            additional_gas_limit=0,
            relayed_version=relayed_version,
        ), await_completion=True)

    for relayed_version in relayed_v1_marker + relayed_v3_marker:
        print(f"## Relayed v{relayed_version}, cross-shard MoveBalance")
        controller.send(controller.create_relayed_transfer(
            relayer=accounts.get_user(shard=SOME_SHARD, index=0),
            sender=accounts.get_user(shard=SOME_SHARD, index=1),
            receiver=accounts.get_user(shard=OTHER_SHARD, index=0).address,
            native_amount=42,
            custom_amount=0,
            additional_gas_limit=0,
            relayed_version=relayed_version,
        ), await_completion=True)

    for relayed_version in relayed_v3_marker:
        print(f"## Relayed v{relayed_version}, intra-shard MoveBalance, with refund")
        controller.send(controller.create_relayed_transfer(
            relayer=accounts.get_user(shard=SOME_SHARD, index=0),
            sender=accounts.get_user(shard=SOME_SHARD, index=1),
            receiver=accounts.get_user(shard=SOME_SHARD, index=2).address,
            native_amount=42,
            custom_amount=0,
            additional_gas_limit=42000,
            relayed_version=relayed_version,
        ), await_completion=True)

    for relayed_version in relayed_v3_marker:
        print(f"## Relayed v{relayed_version}, cross-shard MoveBalance, with refund")
        controller.send(controller.create_relayed_transfer(
            relayer=accounts.get_user(shard=SOME_SHARD, index=0),
            sender=accounts.get_user(shard=SOME_SHARD, index=1),
            receiver=accounts.get_user(shard=OTHER_SHARD, index=0).address,
            native_amount=42,
            custom_amount=0,
            additional_gas_limit=42000,
            relayed_version=relayed_version,
        ), await_completion=True)

    for relayed_version in relayed_v3_marker:
        print(f"## Relayed v{relayed_version}, invalid, intra-shard MoveBalance, with refund")
        controller.send(controller.create_relayed_transfer(
            relayer=accounts.get_user(shard=SOME_SHARD, index=0),
            sender=accounts.get_user(shard=SOME_SHARD, index=1),
            receiver=accounts.get_user(shard=SOME_SHARD, index=2).address,
            native_amount=1000000000000000000000000,
            custom_amount=0,
            additional_gas_limit=42000,
            relayed_version=relayed_version,
        ), await_completion=True)

    for relayed_version in relayed_v3_marker:
        print(f"## Relayed v{relayed_version}, invalid, cross-shard MoveBalance, with refund")
        controller.send(controller.create_relayed_transfer(
            relayer=accounts.get_user(shard=SOME_SHARD, index=0),
            sender=accounts.get_user(shard=SOME_SHARD, index=1),
            receiver=accounts.get_user(shard=OTHER_SHARD, index=0).address,
            native_amount=1000000000000000000000000,
            custom_amount=0,
            additional_gas_limit=42000,
            relayed_version=relayed_version,
        ), await_completion=True)

    for relayed_version in relayed_v3_marker:
        print(f"## Relayed v{relayed_version}, intra-shard multi-transfer (with native)")
        controller.send(controller.create_relayed_transfer(
            relayer=accounts.get_user(shard=SOME_SHARD, index=0),
            sender=accounts.get_user(shard=SOME_SHARD, index=1),
            receiver=accounts.get_user(shard=SOME_SHARD, index=2).address,
            native_amount=42,
            custom_amount=43,
            additional_gas_limit=0,
            relayed_version=relayed_version,
        ), await_completion=True)

    for relayed_version in relayed_v3_marker:
        print(f"## Relayed v{relayed_version}, intra-shard custom transfer")
        controller.send(controller.create_relayed_transfer(
            relayer=accounts.get_user(shard=SOME_SHARD, index=0),
            sender=accounts.get_user(shard=SOME_SHARD, index=1),
            receiver=accounts.get_user(shard=SOME_SHARD, index=2).address,
            native_amount=0,
            custom_amount=43,
            additional_gas_limit=0,
            relayed_version=relayed_version,
        ), await_completion=True)

    for relayed_version in relayed_v1_marker + relayed_v3_marker:
        print(f"## Relayed v{relayed_version}, intra-shard MoveBalance (with bad receiver, system smart contract)")
        controller.send(controller.create_relayed_transfer(
            relayer=accounts.get_user(shard=SOME_SHARD, index=0),
            sender=accounts.get_user(shard=SOME_SHARD, index=2),
            receiver=Address.from_bech32("erd1qqqqqqqqqqqqqqqpqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqzllls8a5w6u"),
            native_amount=42,
            custom_amount=0,
            additional_gas_limit=0,
            relayed_version=relayed_version,
        ), await_completion=True)

    for relayed_version in relayed_v1_marker + relayed_v2_marker + relayed_v3_marker:
        print(f"## Relayed v{relayed_version}, intra-shard, with contract call")
        controller.send(controller.create_relayed_transfer_and_execute(
            relayer=accounts.get_user(shard=SOME_SHARD, index=0),
            sender=accounts.get_user(shard=SOME_SHARD, index=1),
            contract=accounts.get_contract_address("adder", shard=SOME_SHARD, index=0),
            function="add",
            arguments=[BigUIntValue(42)],
            gas_limit=5_000_000,
            native_amount=0,
            custom_amount=0,
            relayed_version=relayed_version,
        ), await_completion=True)

    for relayed_version in relayed_v1_marker + relayed_v2_marker + relayed_v3_marker:
        print(f"## Relayed v{relayed_version}, cross-shard, with contract call")
        controller.send(controller.create_relayed_transfer_and_execute(
            relayer=accounts.get_user(shard=SOME_SHARD, index=0),
            sender=accounts.get_user(shard=SOME_SHARD, index=1),
            contract=accounts.get_contract_address("adder", shard=OTHER_SHARD, index=0),
            function="add",
            arguments=[BigUIntValue(42)],
            gas_limit=5_000_000,
            native_amount=0,
            custom_amount=0,
            relayed_version=relayed_version,
        ), await_completion=True)

    for relayed_version in relayed_v1_marker + relayed_v2_marker + relayed_v3_marker:
        print(f"## Relayed v{relayed_version}, intra-shard, with contract call, with signal error")
        controller.send(controller.create_relayed_transfer_and_execute(
            relayer=accounts.get_user(shard=SOME_SHARD, index=0),
            sender=accounts.get_user(shard=SOME_SHARD, index=1),
            contract=accounts.get_contract_address("adder", shard=SOME_SHARD, index=0),
            function="add",
            arguments=[BigUIntValue(41), BigUIntValue(42)],
            gas_limit=5_000_000,
            native_amount=0,
            custom_amount=0,
            relayed_version=relayed_version,
        ), await_completion=True)

    for relayed_version in relayed_v1_marker + relayed_v2_marker + relayed_v3_marker:
        print(f"## Relayed v{relayed_version}, cross-shard, with contract call, with signal error")
        controller.send(controller.create_relayed_transfer_and_execute(
            relayer=accounts.get_user(shard=SOME_SHARD, index=0),
            sender=accounts.get_user(shard=SOME_SHARD, index=1),
            contract=accounts.get_contract_address("adder", shard=OTHER_SHARD, index=0),
            function="add",
            arguments=[BigUIntValue(41), BigUIntValue(42)],
            gas_limit=5_000_000,
            native_amount=0,
            custom_amount=0,
            relayed_version=relayed_version,
        ), await_completion=True)

    for relayed_version in relayed_v1_marker + relayed_v3_marker:
        print(f"## Relayed v{relayed_version}, intra-shard, with contract call with MoveBalance, with signal error")
        controller.send(controller.create_relayed_transfer_and_execute(
            relayer=accounts.get_user(shard=SOME_SHARD, index=0),
            sender=accounts.get_user(shard=SOME_SHARD, index=1),
            contract=accounts.get_contract_address("adder", shard=SOME_SHARD, index=0),
            function="add",
            arguments=[BigUIntValue(41), BigUIntValue(42)],
            gas_limit=5_000_000,
            native_amount=1,
            custom_amount=0,
            relayed_version=relayed_version,
        ), await_completion=True)

    for relayed_version in relayed_v1_marker + relayed_v3_marker:
        print(f"## Relayed v{relayed_version}, cross-shard, with contract call with MoveBalance, with signal error")
        controller.send(controller.create_relayed_transfer_and_execute(
            relayer=accounts.get_user(shard=SOME_SHARD, index=0),
            sender=accounts.get_user(shard=SOME_SHARD, index=1),
            contract=accounts.get_contract_address("adder", shard=OTHER_SHARD, index=0),
            function="add",
            arguments=[BigUIntValue(41), BigUIntValue(42)],
            gas_limit=5_000_000,
            native_amount=1,
            custom_amount=0,
            relayed_version=relayed_version,
        ), await_completion=True)

    for relayed_version in relayed_v3_marker:
        print(f"## Relayed v{relayed_version}, intra-shard, with contract call, with MoveBalance, via forwarder (promises)")
        controller.send(controller.create_relayed_transfer_and_execute(
            relayer=accounts.get_user(shard=SOME_SHARD, index=0),
            sender=accounts.get_user(shard=SOME_SHARD, index=1),
            contract=accounts.get_contract_address("forwarder", shard=SOME_SHARD, index=0),
            function="forward",
            arguments=[
                I32Value(1),
                AddressValue.new_from_address(accounts.get_contract_address("dummy", shard=SOME_SHARD, index=0)),
                BigUIntValue(42),
                BytesValue(b"doSomething"),
                I64Value(15_000_000),
            ],
            gas_limit=30_000_000,
            native_amount=43, custom_amount=0,
            relayed_version=relayed_version,
        ), await_completion=True)

    for relayed_version in relayed_v3_marker:
        print(f"## Relayed v{relayed_version}, cross-shard, with contract call, with MoveBalance, via forwarder (promises)")
        controller.send(controller.create_relayed_transfer_and_execute(
            relayer=accounts.get_user(shard=SOME_SHARD, index=0),
            sender=accounts.get_user(shard=SOME_SHARD, index=1),
            contract=accounts.get_contract_address("forwarder", shard=SOME_SHARD, index=0),
            function="forward",
            arguments=[
                I32Value(1),
                AddressValue.new_from_address(accounts.get_contract_address("dummy", shard=OTHER_SHARD, index=0)),
                BigUIntValue(42),
                BytesValue(b"doSomething"),
                I64Value(15_000_000),
            ],
            gas_limit=30_000_000,
            native_amount=43,
            custom_amount=0,
            relayed_version=relayed_version,
        ), await_completion=True)

    for relayed_version in relayed_v3_marker:
        print(f"## Relayed v{relayed_version}, intra-shard, with contract call, with MoveBalance, via forwarder (promises), with signal error")
        controller.send(controller.create_relayed_transfer_and_execute(
            relayer=accounts.get_user(shard=SOME_SHARD, index=0),
            sender=accounts.get_user(shard=SOME_SHARD, index=1),
            contract=accounts.get_contract_address("forwarder", shard=SOME_SHARD, index=0),
            function="forward",
            arguments=[
                I32Value(1),
                AddressValue.new_from_address(accounts.get_contract_address("dummy", shard=SOME_SHARD, index=0)),
                BigUIntValue(42),
                BytesValue(b"missingMethod"),
                I64Value(15_000_000),
            ],
            gas_limit=30_000_000,
            native_amount=43,
            custom_amount=0,
            relayed_version=relayed_version,
        ), await_completion=True)

    for relayed_version in relayed_v3_marker:
        print(f"## Relayed v{relayed_version}, cross-shard, with contract call, with MoveBalance, via forwarder, with signal error")
        controller.send(controller.create_relayed_transfer_and_execute(
            relayer=accounts.get_user(shard=SOME_SHARD, index=0),
            sender=accounts.get_user(shard=SOME_SHARD, index=1),
            contract=accounts.get_contract_address("forwarder", shard=SOME_SHARD, index=0),
            function="forward",
            arguments=[
                I32Value(1),
                AddressValue.new_from_address(accounts.get_contract_address("dummy", shard=OTHER_SHARD, index=0)),
                BigUIntValue(42),
                BytesValue(b"missingMethod"),
                I64Value(15_000_000),
            ],
            gas_limit=30_000_000,
            native_amount=43,
            custom_amount=0,
            relayed_version=relayed_version,
        ), await_completion=True)

    for relayed_version in relayed_v3_marker:
        print(f"## Relayed v{relayed_version}, intra-shard MoveBalance, when relayer is same as sender")
        controller.send(controller.create_relayed_transfer(
            relayer=accounts.get_user(shard=SOME_SHARD, index=0),
            sender=accounts.get_user(shard=SOME_SHARD, index=0),
            receiver=accounts.get_user(shard=SOME_SHARD, index=2).address,
            native_amount=42,
            custom_amount=0,
            additional_gas_limit=0,
            relayed_version=relayed_version,
        ), await_completion=True)

    for relayed_version in relayed_v3_marker:
        print(f"## Relayed v{relayed_version}, cross-shard MoveBalance, when relayer is same as sender")
        controller.send(controller.create_relayed_transfer(
            relayer=accounts.get_user(shard=SOME_SHARD, index=0),
            sender=accounts.get_user(shard=SOME_SHARD, index=0),
            receiver=accounts.get_user(shard=OTHER_SHARD, index=0).address,
            native_amount=42,
            custom_amount=0,
            additional_gas_limit=0,
            relayed_version=relayed_version,
        ), await_completion=True)

    for relayed_version in relayed_v3_marker:
        print(f"## Relayed v{relayed_version}, intra-shard MoveBalance, with refund, when relayer is same as sender")
        controller.send(controller.create_relayed_transfer(
            relayer=accounts.get_user(shard=SOME_SHARD, index=0),
            sender=accounts.get_user(shard=SOME_SHARD, index=0),
            receiver=accounts.get_user(shard=SOME_SHARD, index=2).address,
            native_amount=42,
            custom_amount=0,
            additional_gas_limit=42000,
            relayed_version=relayed_version,
        ), await_completion=True)

    for relayed_version in relayed_v3_marker:
        print(f"## Relayed v{relayed_version}, cross-shard MoveBalance, with refund, when relayer is same as sender")
        controller.send(controller.create_relayed_transfer(
            relayer=accounts.get_user(shard=SOME_SHARD, index=0),
            sender=accounts.get_user(shard=SOME_SHARD, index=0),
            receiver=accounts.get_user(shard=OTHER_SHARD, index=0).address,
            native_amount=42,
            custom_amount=0,
            additional_gas_limit=42000,
            relayed_version=relayed_version,
        ), await_completion=True)

    for relayed_version in relayed_v3_marker:
        print(f"## Relayed v{relayed_version}, intra-shard MoveBalance, when relayer is same as receiver")
        controller.send(controller.create_relayed_transfer(
            relayer=accounts.get_user(shard=SOME_SHARD, index=0),
            sender=accounts.get_user(shard=SOME_SHARD, index=1),
            receiver=accounts.get_user(shard=SOME_SHARD, index=0).address,
            native_amount=42,
            custom_amount=0,
            additional_gas_limit=0,
            relayed_version=relayed_version,
        ), await_completion=True)

    for relayed_version in relayed_v3_marker:
        print(f"## Relayed v{relayed_version}, intra-shard MoveBalance, with refund, when relayer is same as receiver")
        controller.send(controller.create_relayed_transfer(
            relayer=accounts.get_user(shard=SOME_SHARD, index=0),
            sender=accounts.get_user(shard=SOME_SHARD, index=1),
            receiver=accounts.get_user(shard=SOME_SHARD, index=0).address,
            native_amount=42,
            custom_amount=0,
            additional_gas_limit=42000,
            relayed_version=relayed_version,
        ), await_completion=True)

    for relayed_version in relayed_v3_marker:
        print(f"## Relayed v{relayed_version}, intra-shard MoveBalance, when relayer is same as sender & receiver")
        controller.send(controller.create_relayed_transfer(
            relayer=accounts.get_user(shard=SOME_SHARD, index=0),
            sender=accounts.get_user(shard=SOME_SHARD, index=0),
            receiver=accounts.get_user(shard=SOME_SHARD, index=0).address,
            native_amount=42,
            custom_amount=0,
            additional_gas_limit=0,
            relayed_version=relayed_version,
        ), await_completion=True)

    for relayed_version in relayed_v3_marker:
        print(f"## Relayed v{relayed_version}, intra-shard MoveBalance, with refund, when relayer is same as sender & receiver")
        controller.send(controller.create_relayed_transfer(
            relayer=accounts.get_user(shard=SOME_SHARD, index=0),
            sender=accounts.get_user(shard=SOME_SHARD, index=0),
            receiver=accounts.get_user(shard=SOME_SHARD, index=0).address,
            native_amount=42,
            custom_amount=0,
            additional_gas_limit=42000,
            relayed_version=relayed_version,
        ), await_completion=True)

    for relayed_version in relayed_v3_marker:
        print(f"## Relayed v{relayed_version}, intra-shard, with contract call, when relayer is same as sender")
        controller.send(controller.create_relayed_transfer_and_execute(
            relayer=accounts.get_user(shard=SOME_SHARD, index=0),
            sender=accounts.get_user(shard=SOME_SHARD, index=0),
            contract=accounts.get_contract_address("adder", shard=SOME_SHARD, index=0),
            function="add",
            arguments=[BigUIntValue(42)],
            gas_limit=5_000_000,
            native_amount=0,
            custom_amount=0,
            relayed_version=relayed_version,
        ), await_completion=True)

    for relayed_version in relayed_v3_marker:
        print(f"## Relayed v{relayed_version}, cross-shard, with contract call, when relayer is same as sender")
        controller.send(controller.create_relayed_transfer_and_execute(
            relayer=accounts.get_user(shard=SOME_SHARD, index=0),
            sender=accounts.get_user(shard=SOME_SHARD, index=0),
            contract=accounts.get_contract_address("adder", shard=OTHER_SHARD, index=0),
            function="add",
            arguments=[BigUIntValue(42)],
            gas_limit=5_000_000,
            native_amount=0,
            custom_amount=0,
            relayed_version=relayed_version,
        ), await_completion=True)

    for relayed_version in relayed_v3_marker:
        print(f"## Relayed v{relayed_version}, intra-shard, with contract call, with MoveBalance, via forwarder (promises), when relayer is same as sender")
        controller.send(controller.create_relayed_transfer_and_execute(
            relayer=accounts.get_user(shard=SOME_SHARD, index=0),
            sender=accounts.get_user(shard=SOME_SHARD, index=0),
            contract=accounts.get_contract_address("forwarder", shard=SOME_SHARD, index=0),
            function="forward",
            arguments=[
                I32Value(1),
                AddressValue.new_from_address(accounts.get_contract_address("dummy", shard=SOME_SHARD, index=0)),
                BigUIntValue(42),
                BytesValue(b"doSomething"),
                I64Value(15_000_000),
            ],
            gas_limit=30_000_000,
            native_amount=43,
            custom_amount=0,
            relayed_version=relayed_version,
        ), await_completion=True)

    for relayed_version in relayed_v3_marker:
        print(f"## Relayed v{relayed_version}, cross-shard, with contract call, with MoveBalance, via forwarder (promises), when relayer is same as sender")
        controller.send(controller.create_relayed_transfer_and_execute(
            relayer=accounts.get_user(shard=SOME_SHARD, index=0),
            sender=accounts.get_user(shard=SOME_SHARD, index=0),
            contract=accounts.get_contract_address("forwarder", shard=SOME_SHARD, index=0),
            function="forward",
            arguments=[
                I32Value(1),
                AddressValue.new_from_address(accounts.get_contract_address("dummy", shard=OTHER_SHARD, index=0)),
                BigUIntValue(42),
                BytesValue(b"doSomething"),
                I64Value(15_000_000),
            ],
            gas_limit=30_000_000,
            native_amount=43,
            custom_amount=0,
            relayed_version=relayed_version,
        ), await_completion=True)

    for relayed_version in relayed_v3_marker:
        print(f"## Relayed v{relayed_version}, intra-shard multi-transfer (with native), when relayer is same as receiver")
        controller.send(controller.create_relayed_transfer(
            relayer=accounts.get_user(shard=SOME_SHARD, index=0),
            sender=accounts.get_user(shard=SOME_SHARD, index=1),
            receiver=accounts.get_user(shard=SOME_SHARD, index=0).address,
            native_amount=42,
            custom_amount=43,
            additional_gas_limit=0,
            relayed_version=relayed_version,
        ), await_completion=True)

    for relayed_version in relayed_v3_marker:
        print(f"## Relayed v{relayed_version}, intra-shard custom transfer, when relayer is same as receiver")
        controller.send(controller.create_relayed_transfer(
            relayer=accounts.get_user(shard=SOME_SHARD, index=0),
            sender=accounts.get_user(shard=SOME_SHARD, index=1),
            receiver=accounts.get_user(shard=SOME_SHARD, index=0).address,
            native_amount=0,
            custom_amount=43,
            additional_gas_limit=0,
            relayed_version=relayed_version,
        ), await_completion=True)

    print("## Relayed v3, intra-shard, transfer native within multi-transfer (fuzzy, with tx.value != 0)")
    controller.send(controller.create_arbitrary_transaction(
        sender=accounts.get_user(shard=SOME_SHARD, index=2),
        # receiver := sender, since we're using multi-transfer.
        receiver=accounts.get_user(shard=SOME_SHARD, index=2).address,
        value=42,
        data="MultiESDTNFTTransfer@" + Serializer().serialize([
            AddressValue.new_from_address(accounts.get_user(shard=SOME_SHARD, index=3).address),
            U32Value(2),
            StringValue("EGLD-000000"),
            U32Value(0),
            U32Value(42),
            StringValue("EGLD-000000"),
            U32Value(0),
            U32Value(43)
        ]),
        gas_limit=5_000_000,
        relayer=accounts.get_user(shard=SOME_SHARD, index=0),
    ), await_processing_started=True)

    print("## Relayed v3, intra-shard, transfer native within multi-transfer (fuzzy, with tx.value == 0, but transfer to self)")
    controller.send(controller.create_arbitrary_transaction(
        sender=accounts.get_user(shard=SOME_SHARD, index=3),
        # receiver := sender, since we're using multi-transfer.
        receiver=accounts.get_user(shard=SOME_SHARD, index=3).address,
        value=0,
        data="MultiESDTNFTTransfer@" + Serializer().serialize([
            AddressValue.new_from_address(accounts.get_user(shard=SOME_SHARD, index=3).address),
            U32Value(2),
            StringValue("EGLD-000000"),
            U32Value(0),
            U32Value(42),
            StringValue("EGLD-000000"),
            U32Value(0),
            U32Value(43)
        ]),
        gas_limit=5_000_000,
        relayer=accounts.get_user(shard=SOME_SHARD, index=0),
    ), await_processing_started=True)

    print("## Relayed v3, cross-shard, transfer native within multi-transfer (fuzzy, with tx.value == 0)")
    controller.send(controller.create_arbitrary_transaction(
        sender=accounts.get_user(shard=SOME_SHARD, index=2),
        # receiver := sender, since we're using multi-transfer.
        receiver=accounts.get_user(shard=SOME_SHARD, index=2).address,
        value=0,
        data="MultiESDTNFTTransfer@" + Serializer().serialize([
            AddressValue.new_from_address(accounts.get_user(shard=OTHER_SHARD, index=3).address),
            U32Value(2),
            StringValue("EGLD-000000"),
            U32Value(0),
            U32Value(42),
            StringValue("EGLD-000000"),
            U32Value(0),
            U32Value(43)
        ]),
        gas_limit=5_000_000,
        relayer=accounts.get_user(shard=SOME_SHARD, index=0),
    ), await_processing_started=True)

    print("## Relayed v3, fuzzy, SaveKeyValue")
    controller.send(controller.create_arbitrary_transaction(
        sender=accounts.get_user(shard=SOME_SHARD, index=0),
        receiver=accounts.get_user(shard=SOME_SHARD, index=0).address,
        value=0,
        data=f"SaveKeyValue@{os.urandom(4).hex()}@{os.urandom(4).hex()}",
        gas_limit=1_000_000,
        relayer=accounts.get_user(shard=SOME_SHARD, index=0),
    ), await_processing_started=True)

    print("## Relayed v3, fuzzy, SetGuardian")
    controller.send(controller.create_arbitrary_transaction(
        sender=accounts.get_user(shard=SOME_SHARD, index=1),
        receiver=accounts.get_user(shard=SOME_SHARD, index=1).address,
        value=0,
        data=f"SetGuardian@{os.urandom(32).hex()}@{os.urandom(4).hex()}",
        gas_limit=1_000_000,
        relayer=accounts.get_user(shard=SOME_SHARD, index=1),
    ), await_processing_started=True)

    memento.replace_run_transactions(controller.transactions_hashes_accumulator)


class BunchOfAccounts:
    def __init__(self, configuration: Configuration, memento: "Memento") -> None:
        self.configuration = configuration
        self.mnemonic = Mnemonic(configuration.users_mnemonic)
        self.mnemonic_last_index = 0
        self.sponsor = self._create_sponsor()
        self.users: List[Account] = []
        self.users_by_bech32: Dict[str, Account] = {}
        self.contracts: List[SmartContract] = []

        self._create_users()

        for item in memento.get_contracts():
            self.contracts.append(item)

    def _create_sponsor(self) -> "Account":
        sponsor_secret_key = UserSecretKey(self.configuration.sponsor_secret_key)
        sponsor_signer = UserSigner(sponsor_secret_key)
        return Account(sponsor_signer)

    def _create_users(self):
        num_users_per_shard = self.configuration.num_users_per_shard
        use_projected_shard = self.configuration.users_in_projected_shard

        user_secret_keys_by_shard: list[list[UserSecretKey]] = [[] for _ in range(NUM_SHARDS)]
        shard_identifiers = list(range(NUM_SHARDS))

        while True:
            user_secret_key = self.mnemonic.derive_key(self.mnemonic_last_index)
            self.mnemonic_last_index += 1

            user_pubkey = user_secret_key.generate_public_key()
            user_shard = get_shard_of_pubkey(user_pubkey.get_bytes(), NUM_SHARDS)
            user_projected_shard = user_pubkey.get_bytes()[-1]

            if len(user_secret_keys_by_shard[user_shard]) == num_users_per_shard:
                continue

            if use_projected_shard:
                if user_projected_shard in shard_identifiers:
                    user_secret_keys_by_shard[user_shard].append(user_secret_key)
            else:
                user_secret_keys_by_shard[user_shard].append(user_secret_key)

            if all(len(items) == num_users_per_shard for items in user_secret_keys_by_shard):
                break

        for user_secret_keys in user_secret_keys_by_shard:
            for key in user_secret_keys:
                user_signer = UserSigner(key)
                user = Account(user_signer)
                self.users.append(user)
                self.users_by_bech32[user.address.to_bech32()] = user

    def get_user(self, shard: int, index: int) -> "Account":
        return [user for user in self.users if self._is_address_in_shard(user.address, shard)][index]

    def get_users_by_shard(self, shard: int) -> List["Account"]:
        return [user for user in self.users if self._is_address_in_shard(user.address, shard)]

    def get_account_by_bech32(self, address: str) -> "Account":
        if self.sponsor.address.to_bech32() == address:
            return self.sponsor

        return self.users_by_bech32[address]

    def get_contract_address(self, tag: str, shard: int, index: int) -> Address:
        addresses: List[str] = [contract.address for contract in self.contracts if contract.tag == tag and self._is_bech32_in_shard(contract.address, shard)]
        return Address.new_from_bech32(addresses[index])

    def get_contracts_addresses(self, tag: Optional[str] = None, shard: Optional[int] = None) -> List[Address]:
        contracts = self.contracts

        if tag is not None:
            contracts = [contract for contract in contracts if contract.tag == tag]
        if shard is not None:
            contracts = [contract for contract in contracts if self._is_bech32_in_shard(contract.address, shard)]

        return [Address.from_bech32(contract.address) for contract in contracts]

    def _is_bech32_in_shard(self, address: str, shard: int) -> bool:
        return self._is_address_in_shard(Address.new_from_bech32(address), shard)

    def _is_address_in_shard(self, address: Address, shard: int) -> bool:
        address_computer = AddressComputer()
        address_shard = address_computer.get_shard_of_address(address)
        return address_shard == shard


class Account:
    def __init__(self, signer: UserSigner) -> None:
        self.signer = signer
        self.address: Address = signer.get_pubkey().to_address("erd")


class SmartContract:
    def __init__(self, tag: str, address: str) -> None:
        self.tag = tag
        self.address = address

    @classmethod
    def from_dictionary(cls, data: Dict[str, str]) -> "SmartContract":
        return cls(
            tag=data["tag"],
            address=data["address"]
        )

    def to_dictionary(self) -> Dict[str, str]:
        return {
            "tag": self.tag,
            "address": self.address
        }


class Controller:
    def __init__(self, configuration: Configuration, accounts: BunchOfAccounts, memento: "Memento") -> None:
        self.configuration = configuration
        self.memento = memento
        self.accounts = accounts
        self.network_provider = ProxyNetworkProvider(configuration.proxy_url)
        self.transaction_computer = TransactionComputer()
        self.transactions_factory_config = TransactionsFactoryConfig(chain_id=configuration.network_id)
        self.transactions_factory_config.issue_cost = configuration.custom_currency_issue_cost
        self.nonces_tracker = NoncesTracker(configuration.proxy_url)
        self.token_management_transactions_factory = TokenManagementTransactionsFactory(self.transactions_factory_config)
        self.token_management_outcome_parser = TokenManagementTransactionsOutcomeParser()
        self.transfer_transactions_factory = TransferTransactionsFactory(self.transactions_factory_config)
        self.relayed_transactions_factory = RelayedTransactionsFactory(self.transactions_factory_config)
        self.contracts_transactions_factory = SmartContractTransactionsFactory(self.transactions_factory_config)
        self.contracts_query_controller = SmartContractController(configuration.network_id, self.network_provider)
        self.transactions_hashes_accumulator: list[str] = []
        self.awaiting_options = AwaitingOptions(
            polling_interval_in_milliseconds=AWAITING_POLLING_TIMEOUT_IN_MILLISECONDS,
            patience_in_milliseconds=AWAITING_PATIENCE_IN_MILLISECONDS
        )

    def do_airdrops_for_native_currency(self):
        transactions: List[Transaction] = []

        for user in self.accounts.users:
            transaction = self.transfer_transactions_factory.create_transaction_for_native_token_transfer(
                sender=self.accounts.sponsor.address,
                receiver=user.address,
                native_amount=100000000000000000
            )

            self.apply_nonce(transaction)
            self.sign(transaction)

            transactions.append(transaction)

        self.send_multiple(transactions, chunk_size=99, wait_between_chunks=7)

        print("Wait for the last transaction to be processed (optimization)...")
        self.await_processing_started(transactions[-1:])

    def issue_custom_currency(self, name: str):
        transaction = self.token_management_transactions_factory.create_transaction_for_issuing_fungible(
            sender=self.accounts.sponsor.address,
            token_name=name,
            token_ticker=name,
            initial_supply=1000000000,
            num_decimals=2,
            can_freeze=True,
            can_wipe=True,
            can_pause=True,
            can_change_owner=True,
            can_upgrade=True,
            can_add_special_roles=True,
        )

        self.apply_nonce(transaction)
        self.sign(transaction)
        self.send(transaction)

        [transaction_on_network] = self.await_completed([transaction])
        [issue_fungible_outcome] = self.token_management_outcome_parser.parse_issue_fungible(transaction_on_network)
        token_identifier = issue_fungible_outcome.token_identifier

        print(f"Token identifier: {token_identifier}")

        self.memento.add_custom_currency(token_identifier)

    def do_airdrops_for_custom_currencies(self):
        currencies = self.memento.get_custom_currencies()
        transactions: List[Transaction] = []

        for user in self.accounts.users:
            for currency in currencies:
                transaction = self.transfer_transactions_factory.create_transaction_for_esdt_token_transfer(
                    sender=self.accounts.sponsor.address,
                    receiver=user.address,
                    token_transfers=[TokenTransfer(Token(currency), 1000000)]
                )

                self.apply_nonce(transaction)
                self.sign(transaction)
                transactions.append(transaction)

        self.send_multiple(transactions, chunk_size=99, wait_between_chunks=7)

        print("Wait for the last transaction to be processed (optimization)...")
        self.await_processing_started(transactions[-1:])

    def do_create_contract_deployments(self):
        transactions_adder: List[Transaction] = []
        transactions_dummy: List[Transaction] = []
        transactions_forwarder: List[Transaction] = []
        transactions_developer_rewards: List[Transaction] = []
        address_computer = AddressComputer()

        # Adder
        transactions_adder.append(self.contracts_transactions_factory.create_transaction_for_deploy(
            sender=self.accounts.get_user(shard=0, index=0).address,
            bytecode=CONTRACT_PATH_ADDER,
            gas_limit=5000000,
            arguments=[BigUIntValue(0)]
        ))

        transactions_adder.append(self.contracts_transactions_factory.create_transaction_for_deploy(
            sender=self.accounts.get_user(shard=1, index=0).address,
            bytecode=CONTRACT_PATH_ADDER,
            gas_limit=5000000,
            arguments=[BigUIntValue(0)]
        ))

        transactions_adder.append(self.contracts_transactions_factory.create_transaction_for_deploy(
            sender=self.accounts.get_user(shard=2, index=0).address,
            bytecode=CONTRACT_PATH_ADDER,
            gas_limit=5000000,
            arguments=[BigUIntValue(0)]
        ))

        # Dummy
        transactions_dummy.append(self.contracts_transactions_factory.create_transaction_for_deploy(
            sender=self.accounts.get_user(shard=0, index=0).address,
            bytecode=CONTRACT_PATH_DUMMY,
            gas_limit=5000000,
            arguments=[]
        ))

        transactions_dummy.append(self.contracts_transactions_factory.create_transaction_for_deploy(
            sender=self.accounts.get_user(shard=1, index=0).address,
            bytecode=CONTRACT_PATH_DUMMY,
            gas_limit=5000000,
            arguments=[]
        ))

        transactions_dummy.append(self.contracts_transactions_factory.create_transaction_for_deploy(
            sender=self.accounts.get_user(shard=2, index=0).address,
            bytecode=CONTRACT_PATH_DUMMY,
            gas_limit=5000000,
            arguments=[]
        ))

        # Forwarder
        transactions_forwarder.append(self.contracts_transactions_factory.create_transaction_for_deploy(
            sender=self.accounts.get_user(shard=0, index=0).address,
            bytecode=CONTRACT_PATH_FORWARDER,
            gas_limit=25000000,
            arguments=[]
        ))

        transactions_forwarder.append(self.contracts_transactions_factory.create_transaction_for_deploy(
            sender=self.accounts.get_user(shard=1, index=0).address,
            bytecode=CONTRACT_PATH_FORWARDER,
            gas_limit=25000000,
            arguments=[]
        ))

        transactions_forwarder.append(self.contracts_transactions_factory.create_transaction_for_deploy(
            sender=self.accounts.get_user(shard=2, index=0).address,
            bytecode=CONTRACT_PATH_FORWARDER,
            gas_limit=25000000,
            arguments=[]
        ))

        # Developer rewards
        transactions_developer_rewards.append(self.contracts_transactions_factory.create_transaction_for_deploy(
            sender=self.accounts.get_user(shard=0, index=0).address,
            bytecode=CONTRACT_PATH_DEVELOPER_REWARDS,
            gas_limit=5000000,
            arguments=[]
        ))

        transactions_developer_rewards.append(self.contracts_transactions_factory.create_transaction_for_deploy(
            sender=self.accounts.get_user(shard=1, index=0).address,
            bytecode=CONTRACT_PATH_DEVELOPER_REWARDS,
            gas_limit=5000000,
            arguments=[]
        ))

        transactions_developer_rewards.append(self.contracts_transactions_factory.create_transaction_for_deploy(
            sender=self.accounts.get_user(shard=2, index=0).address,
            bytecode=CONTRACT_PATH_DEVELOPER_REWARDS,
            gas_limit=5000000,
            arguments=[]
        ))

        transactions_all = transactions_adder + transactions_dummy + transactions_forwarder + transactions_developer_rewards

        for transaction in transactions_all:
            self.apply_nonce(transaction)
            self.sign(transaction)

        for transaction in transactions_adder:
            contract_address = address_computer.compute_contract_address(transaction.sender, transaction.nonce)
            self.memento.add_contract("adder", contract_address.to_bech32())

        for transaction in transactions_dummy:
            contract_address = address_computer.compute_contract_address(transaction.sender, transaction.nonce)
            self.memento.add_contract("dummy", contract_address.to_bech32())

        for transaction in transactions_forwarder:
            contract_address = address_computer.compute_contract_address(transaction.sender, transaction.nonce)
            self.memento.add_contract("forwarder", contract_address.to_bech32())

        for transaction in transactions_developer_rewards:
            contract_address = address_computer.compute_contract_address(transaction.sender, transaction.nonce)
            self.memento.add_contract("developerRewards", contract_address.to_bech32())

        self.send_multiple(transactions_all)
        self.await_completed(transactions_all)

        # Let's do some indirect deployments, as well (children of "developerRewards" contracts).
        transactions: list[Transaction] = []

        for contract in self.memento.get_contracts("developerRewards"):
            transaction = self.contracts_transactions_factory.create_transaction_for_execute(
                sender=self.accounts.get_user(shard=SOME_SHARD, index=0).address,
                contract=Address.new_from_bech32(contract.address),
                function="deployChild",
                gas_limit=5000000,
            )

            self.apply_nonce(transaction)
            self.sign(transaction)

            transactions.append(transaction)

        self.send_multiple(transactions)
        self.await_completed(transactions)

        # Save the addresses of the newly deployed children.
        for contract in self.memento.get_contracts("developerRewards"):
            [child_address_pubkey] = self.contracts_query_controller.query(
                contract=Address.new_from_bech32(contract.address),
                function="getChildAddress",
                arguments=[],
            )

            child_address = Address(child_address_pubkey, "erd")
            self.memento.add_contract("developerRewardsChild", child_address.to_bech32())

    def do_change_contract_owner(self, contract: Address, new_owner: "Account"):
        contract_account = self.network_provider.get_account(contract)
        current_owner_address = contract_account.contract_owner_address
        assert current_owner_address is not None
        current_owner = self.accounts.get_account_by_bech32(current_owner_address.to_bech32())
        new_owner_address = new_owner.address

        if current_owner_address == new_owner_address.to_bech32():
            return

        transaction = self.create_change_owner_address(
            contract=contract,
            owner=current_owner,
            new_owner=new_owner_address,
        )

        self.send(transaction)
        self.await_completed([transaction])

    def create_arbitrary_transaction(self, sender: "Account", receiver: Address, value: int, data: str, gas_limit: int, relayer: Optional["Account"] = None) -> Transaction:
        transaction = Transaction(
            sender=sender.address,
            receiver=receiver,
            gas_limit=gas_limit,
            chain_id=self.configuration.network_id,
            value=value,
            data=data.encode()
        )

        if relayer is not None:
            transaction.relayer = relayer.address

        self.apply_nonce(transaction)
        self.sign(transaction)

        if relayer is not None:
            self.sign_as_relayer_v3(transaction)

        return transaction

    def create_transfer(self, sender: "Account", receiver: Address, native_amount: int, custom_amount: int, additional_gas_limit: int = 0) -> Transaction:
        token_transfers: List[TokenTransfer] = []

        if custom_amount:
            custom_currency = self.memento.get_custom_currencies()[0]
            token_transfers = [TokenTransfer(Token(custom_currency), custom_amount)]

        transaction = self.transfer_transactions_factory.create_transaction_for_transfer(
            sender=sender.address,
            receiver=receiver,
            native_amount=native_amount,
            token_transfers=token_transfers
        )

        transaction.gas_limit += additional_gas_limit

        self.apply_nonce(transaction)
        self.sign(transaction)

        return transaction

    def create_transfer_and_execute(self, sender: "Account", contract: Address, function: str, arguments: list[Any], gas_limit: int, native_amount: int, custom_amount: int) -> Transaction:
        token_transfers: List[TokenTransfer] = []

        if custom_amount:
            custom_currency = self.memento.get_custom_currencies()[0]
            token_transfers = [TokenTransfer(Token(custom_currency), custom_amount)]

        transaction = self.contracts_transactions_factory.create_transaction_for_execute(
            sender=sender.address,
            contract=contract,
            function=function,
            arguments=arguments,
            gas_limit=gas_limit,
            native_transfer_amount=native_amount,
            token_transfers=token_transfers
        )

        self.apply_nonce(transaction)
        self.sign(transaction)

        return transaction

    def create_contract_deployment(self, sender: "Account", bytecode: Path, arguments: list[Any], gas_limit: int, amount: int) -> Transaction:
        transaction = self.contracts_transactions_factory.create_transaction_for_deploy(
            sender=sender.address,
            bytecode=bytecode,
            gas_limit=gas_limit,
            arguments=arguments,
            native_transfer_amount=amount
        )

        self.apply_nonce(transaction)
        self.sign(transaction)

        return transaction

    def create_claim_developer_rewards_on_directly_owned_contract(self, sender: "Account", contract: Address) -> Transaction:
        transaction = self.contracts_transactions_factory.create_transaction_for_execute(
            sender=sender.address,
            contract=contract,
            function="ClaimDeveloperRewards",
            gas_limit=8000000,
        )

        self.apply_nonce(transaction)
        self.sign(transaction)

        return transaction

    def create_claim_developer_rewards_on_child_contract(self, sender: "Account", parent_contract: Address) -> Transaction:
        transaction = self.contracts_transactions_factory.create_transaction_for_execute(
            sender=sender.address,
            contract=parent_contract,
            function="claimDeveloperRewardsOnChild",
            gas_limit=8000000,
        )

        self.apply_nonce(transaction)
        self.sign(transaction)

        return transaction

    def create_change_owner_address(self, contract: Address, owner: "Account", new_owner: Address) -> Transaction:
        transaction = self.contracts_transactions_factory.create_transaction_for_execute(
            sender=owner.address,
            contract=contract,
            function="ChangeOwnerAddress",
            gas_limit=6000000,
            arguments=[AddressValue.new_from_address(new_owner)]
        )

        self.apply_nonce(transaction)
        self.sign(transaction)

        return transaction

    def create_relayed_transfer(self, relayer: "Account", sender: "Account", receiver: Address, native_amount: int, custom_amount: int, additional_gas_limit: int, relayed_version: int) -> Transaction:
        token_transfers: List[TokenTransfer] = []

        if custom_amount:
            custom_currency = self.memento.get_custom_currencies()[0]
            token_transfers = [TokenTransfer(Token(custom_currency), custom_amount)]

        if relayed_version == 1:
            # Relayer nonce is reserved before sender nonce, to ensure good ordering (if sender and relayer are the same account).
            relayer_nonce = self._reserve_nonce(relayer)

            inner_transaction = self.transfer_transactions_factory.create_transaction_for_transfer(
                sender=sender.address,
                receiver=receiver,
                native_amount=native_amount,
                token_transfers=token_transfers
            )

            inner_transaction.gas_limit += additional_gas_limit

            self.apply_nonce(inner_transaction)
            self.sign(inner_transaction)

            transaction = self.relayed_transactions_factory.create_relayed_v1_transaction(
                inner_transaction=inner_transaction,
                relayer_address=relayer.address,
            )

            transaction.nonce = relayer_nonce
            self.sign(transaction)

            return transaction

        if relayed_version == 2:
            raise ValueError("not implemented")

        if relayed_version == 3:
            transaction = self.transfer_transactions_factory.create_transaction_for_transfer(
                sender=sender.address,
                receiver=receiver,
                native_amount=native_amount,
                token_transfers=token_transfers
            )

            transaction.relayer = relayer.address
            transaction.gas_limit += additional_gas_limit
            transaction.gas_limit += ADDITIONAL_GAS_LIMIT_FOR_RELAYED_V3

            self.apply_nonce(transaction)
            self.sign(transaction)
            self.sign_as_relayer_v3(transaction)

            return transaction

        raise ValueError(f"Unsupported relayed version: {relayed_version}")

    def create_relayed_transfer_and_execute(self, relayer: "Account", sender: "Account", contract: Address, function: str, arguments: list[Any], gas_limit: int, native_amount: int, custom_amount: int, relayed_version: int) -> Transaction:
        token_transfers: List[TokenTransfer] = []

        if custom_amount:
            custom_currency = self.memento.get_custom_currencies()[0]
            token_transfers = [TokenTransfer(Token(custom_currency), custom_amount)]

        if relayed_version == 1:
            # Relayer nonce is reserved before sender nonce, to ensure good ordering (if sender and relayer are the same account).
            relayer_nonce = self._reserve_nonce(relayer)

            inner_transaction = self.contracts_transactions_factory.create_transaction_for_execute(
                sender=sender.address,
                contract=contract,
                function=function,
                gas_limit=gas_limit,
                arguments=arguments,
                native_transfer_amount=native_amount,
                token_transfers=token_transfers
            )

            self.apply_nonce(inner_transaction)
            self.sign(inner_transaction)

            transaction = self.relayed_transactions_factory.create_relayed_v1_transaction(
                inner_transaction=inner_transaction,
                relayer_address=relayer.address,
            )

            transaction.nonce = relayer_nonce
            self.sign(transaction)

            return transaction

        if relayed_version == 2:
            # Relayer nonce is reserved before sender nonce, to ensure good ordering (if sender and relayer are the same account).
            relayer_nonce = self._reserve_nonce(relayer)

            inner_transaction = self.contracts_transactions_factory.create_transaction_for_execute(
                sender=sender.address,
                contract=contract,
                function=function,
                gas_limit=0,
                arguments=arguments,
                native_transfer_amount=native_amount,
                token_transfers=token_transfers
            )

            self.apply_nonce(inner_transaction)
            self.sign(inner_transaction)

            transaction = self.relayed_transactions_factory.create_relayed_v2_transaction(
                inner_transaction=inner_transaction,
                inner_transaction_gas_limit=gas_limit,
                relayer_address=relayer.address,
            )

            transaction.nonce = relayer_nonce
            self.sign(transaction)

            return transaction

        if relayed_version == 3:
            transaction = self.contracts_transactions_factory.create_transaction_for_execute(
                sender=sender.address,
                contract=contract,
                function=function,
                gas_limit=gas_limit,
                arguments=arguments,
                native_transfer_amount=native_amount,
                token_transfers=token_transfers
            )

            transaction.relayer = relayer.address
            transaction.gas_limit += ADDITIONAL_GAS_LIMIT_FOR_RELAYED_V3
            self.apply_nonce(transaction)
            self.sign(transaction)
            self.sign_as_relayer_v3(transaction)

            return transaction

        raise ValueError(f"Unsupported relayed version: {relayed_version}")

    def apply_nonce(self, transaction: Transaction):
        sender = self.accounts.get_account_by_bech32(transaction.sender.to_bech32())
        transaction.nonce = self.nonces_tracker.get_then_increment_nonce(sender.address)

    def _reserve_nonce(self, account: "Account"):
        sender = self.accounts.get_account_by_bech32(account.address.to_bech32())
        return self.nonces_tracker.get_then_increment_nonce(sender.address)

    def sign(self, transaction: Transaction):
        sender = self.accounts.get_account_by_bech32(transaction.sender.to_bech32())
        bytes_for_signing = self.transaction_computer.compute_bytes_for_signing(transaction)
        transaction.signature = sender.signer.sign(bytes_for_signing)

    def sign_as_relayer_v3(self, transaction: Transaction):
        assert transaction.relayer is not None
        relayer = self.accounts.get_account_by_bech32(transaction.relayer.to_bech32())
        bytes_for_signing = self.transaction_computer.compute_bytes_for_signing(transaction)
        transaction.relayer_signature = relayer.signer.sign(bytes_for_signing)

    def send_multiple(self, transactions: List[Transaction], chunk_size: int = 1024, wait_between_chunks: float = 0, await_processing_started: bool = False, await_completion: bool = False):
        print(f"Sending {len(transactions)} transactions...")

        chunks = list(split_to_chunks(transactions, chunk_size))

        for chunk in chunks:
            num_sent, hashes = self.network_provider.send_transactions(chunk)
            print(f"Sent {num_sent} transactions. Waiting {wait_between_chunks} seconds...")

            self.transactions_hashes_accumulator.extend([hash.hex() for hash in hashes if hash is not None])
            time.sleep(wait_between_chunks)

        if await_processing_started or await_completion:
            self.await_processing_started(transactions)

        if await_completion:
            self.await_completed(transactions)

    def send(self, transaction: Transaction, await_processing_started: bool = False, await_completion: bool = False):
        transaction_hash = self.network_provider.send_transaction(transaction)
        print(f"    🌍 {self.configuration.view_url.replace('{hash}', transaction_hash.hex())}")

        if await_processing_started or await_completion:
            self.await_processing_started([transaction])

        if await_completion:
            self.await_completed([transaction])

        self.transactions_hashes_accumulator.append(transaction_hash.hex())

    def await_completed(self, transactions: List[Transaction]) -> List[TransactionOnNetwork]:
        print(f"    ⏳ Awaiting completion of {len(transactions)} transactions...")

        def await_completed_one(transaction: Transaction) -> TransactionOnNetwork:
            transaction_hash = self.transaction_computer.compute_transaction_hash(transaction).hex()
            transaction_on_network = self.network_provider.await_transaction_completed(transaction_hash, self.awaiting_options)

            print(f"    ✓ Completed: {self.configuration.view_url.replace('{hash}', transaction_hash)}")
            return transaction_on_network

        transactions_on_network = Pool(8).map(await_completed_one, transactions)
        return transactions_on_network

    def await_processing_started(self, transactions: List[Transaction]) -> List[TransactionOnNetwork]:
        print(f"    ⏳ Awaiting processing start of {len(transactions)} transactions...")

        def await_processing_started_one(transaction: Transaction) -> TransactionOnNetwork:
            condition: Callable[[AccountOnNetwork], bool] = lambda account: account.nonce > transaction.nonce
            self.network_provider.await_account_on_condition(transaction.sender, condition, self.awaiting_options)

            transaction_hash = self.transaction_computer.compute_transaction_hash(transaction).hex()
            transaction_on_network = self.network_provider.get_transaction(transaction_hash)

            print(f"    ✓ Processing started: {self.configuration.view_url.replace('{hash}', transaction_hash)}")
            return transaction_on_network

        transactions_on_network = Pool(8).map(await_processing_started_one, transactions)
        return transactions_on_network

    def wait_until_epoch(self, epoch: int):
        print(f"⏳ Waiting until epoch {epoch}...")

        while True:
            current_epoch = self.network_provider.get_network_status().current_epoch
            if current_epoch >= epoch:
                print(f"✓ Reached epoch {current_epoch} >= {epoch}")
                break

            time.sleep(30)


class NoncesTracker:
    def __init__(self, proxy_url: str) -> None:
        self.nonces_by_address: Dict[str, int] = {}
        self.network_provider = ProxyNetworkProvider(proxy_url)

    def get_then_increment_nonce(self, address: Address):
        nonce = self.get_nonce(address)
        self.increment_nonce(address)
        return nonce

    def get_nonce(self, address: Address) -> int:
        if address.to_bech32() not in self.nonces_by_address:
            self.recall_nonce(address)

        return self.nonces_by_address[address.to_bech32()]

    def recall_nonce(self, address: Address):
        account = self.network_provider.get_account(address)
        self.nonces_by_address[address.to_bech32()] = account.nonce

    def increment_nonce(self, address: Address):
        self.nonces_by_address[address.to_bech32()] += 1


class Memento:
    """
    The memento is used to store some state (contract addresses, tokens identifiers) among multiple runs.
    """

    def __init__(self, path: Path) -> None:
        self.path = path
        self._contracts: List[SmartContract] = []
        self._custom_currencies: List[str] = []
        self._run_transactions: List[str] = []

    def clear(self):
        self._contracts = []
        self._custom_currencies = []
        self.save()

    def add_custom_currency(self, currency: str):
        self.load()
        self._custom_currencies.append(currency)
        self.save()

    def get_custom_currencies(self) -> List[str]:
        self.load()
        return self._custom_currencies

    def add_contract(self, tag: str, address: str):
        self.load()
        self._contracts.append(SmartContract(tag, address))
        self.save()

    def get_contracts(self, tag: Optional[str] = None) -> List[SmartContract]:
        self.load()

        contracts = self._contracts

        if tag is not None:
            contracts = [contract for contract in contracts if contract.tag == tag]

        return contracts

    def replace_run_transactions(self, transactions_hashes: list[str]):
        self.load()
        self._run_transactions = transactions_hashes
        self.save()

    def load(self):
        if not self.path.exists():
            return

        data = json.loads(self.path.read_text())

        contracts_raw = data.get("contracts", [])
        self._contracts = [SmartContract.from_dictionary(item) for item in contracts_raw]
        self._custom_currencies = data.get("customCurrencies", [])
        self._run_transactions = data.get("runTransactions", [])

    def save(self):
        contracts_raw = [contract.to_dictionary() for contract in self._contracts]

        data = {
            "contracts": contracts_raw,
            "customCurrencies": self._custom_currencies,
            "runTransactions": self._run_transactions,
        }

        self.path.parent.mkdir(parents=True, exist_ok=True)
        self.path.write_text(json.dumps(data, indent=4) + "\n")


def split_to_chunks(items: Any, chunk_size: int):
    for i in range(0, len(items), chunk_size):
        yield items[i:i + chunk_size]


if __name__ == '__main__':
    main()
