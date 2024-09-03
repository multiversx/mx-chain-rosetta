import json
import time
from argparse import ArgumentParser
from pathlib import Path
from typing import Any, Dict, List

from multiversx_sdk import (Address, AddressComputer, Mnemonic,
                            ProxyNetworkProvider, RelayedTransactionsFactory,
                            SmartContractTransactionsFactory, Token,
                            TokenManagementTransactionsFactory,
                            TokenManagementTransactionsOutcomeParser,
                            TokenTransfer, Transaction, TransactionAwaiter,
                            TransactionComputer, TransactionsConverter,
                            TransactionsFactoryConfig,
                            TransferTransactionsFactory, UserSecretKey,
                            UserSigner)
from multiversx_sdk.network_providers.transactions import TransactionOnNetwork

from systemtests.config import CONFIGURATIONS, Configuration

CONTRACT_PATH_ADDER = Path(__file__).parent / "contracts" / "adder.wasm"
CONTRACT_PATH_DUMMY = Path(__file__).parent / "contracts" / "dummy.wasm"
CONTRACT_PATH_DEVELOPER_REWARDS = Path(__file__).parent / "contracts" / "developer_rewards.wasm"


def main():
    parser = ArgumentParser()

    subparsers = parser.add_subparsers()
    subparser_setup = subparsers.add_parser("setup")
    subparser_setup.add_argument("--network", choices=CONFIGURATIONS.keys(), required=True)
    subparser_setup.set_defaults(func=do_setup)

    subparser_run = subparsers.add_parser("run")
    subparser_run.add_argument("--network", choices=CONFIGURATIONS.keys(), required=True)
    subparser_run.add_argument("--without-spica", action="store_true")
    subparser_run.set_defaults(func=do_run)

    args = parser.parse_args()

    if not hasattr(args, "func"):
        parser.print_help()
    else:
        args.func(args)


def do_setup(args: Any):
    network = args.network
    configuration = CONFIGURATIONS[network]
    memento = Memento(Path(configuration.memento_file))
    accounts = BunchOfAccounts(configuration, memento)
    controller = Controller(configuration, accounts, memento)

    print("Do airdrops for native currency...")
    controller.do_airdrops_for_native_currency()

    print("Issue custom currency...")
    controller.issue_custom_currency("ROSETTA")

    print("Do airdrops for custom currencies...")
    controller.do_airdrops_for_custom_currencies()

    print("Do contract deployments...")
    controller.do_create_contract_deployments()


def do_run(args: Any):
    network = args.network
    with_spica = not args.without_spica
    configuration = CONFIGURATIONS[network]
    memento = Memento(Path(configuration.memento_file))
    accounts = BunchOfAccounts(configuration, memento)
    controller = Controller(configuration, accounts, memento)

    print("Intra-shard, simple MoveBalance with refund")
    controller.send(controller.create_simple_move_balance_with_refund(
        sender=accounts.get_user(shard=0, index=0),
        receiver=accounts.get_user(shard=0, index=1).address,
    ))

    print("Cross-shard, simple MoveBalance with refund")
    controller.send(controller.create_simple_move_balance_with_refund(
        sender=accounts.get_user(shard=0, index=1),
        receiver=accounts.get_user(shard=1, index=0).address,
    ))

    print("Intra-shard, invalid MoveBalance with refund")
    controller.send(controller.create_invalid_move_balance_with_refund(
        sender=accounts.get_user(shard=0, index=2),
        receiver=accounts.get_user(shard=0, index=3).address,
    ))

    print("Cross-shard, invalid MoveBalance with refund")
    controller.send(controller.create_invalid_move_balance_with_refund(
        sender=accounts.get_user(shard=0, index=4),
        receiver=accounts.get_user(shard=1, index=1).address,
    ))

    print("Intra-shard, sending value to non-payable contract")
    controller.send(controller.create_simple_move_balance_with_refund(
        sender=accounts.get_user(shard=0, index=0),
        receiver=accounts.get_contract_address("adder", 0, 0),
    ))

    print("Cross-shard, sending value to non-payable contract")
    controller.send(controller.create_simple_move_balance_with_refund(
        sender=accounts.get_user(shard=0, index=1),
        receiver=accounts.get_contract_address("adder", 1, 0),
    ))

    if with_spica:
        # Intra-shard, native transfer within MultiESDTTransfer
        controller.send(controller.create_native_transfer_within_multiesdt(
            sender=accounts.get_user(shard=0, index=0),
            receiver=accounts.get_user(shard=0, index=1).address,
        ))

        print("Cross-shard, native transfer within MultiESDTTransfer")
        controller.send(controller.create_native_transfer_within_multiesdt(
            sender=accounts.get_user(shard=0, index=1),
            receiver=accounts.get_user(shard=1, index=0).address,
        ))

        print("Intra-shard, native transfer within MultiESDTTransfer, towards non-payable contract")
        controller.send(controller.create_native_transfer_within_multiesdt(
            sender=accounts.get_user(shard=0, index=0),
            receiver=accounts.get_contract_address("adder", 0, 0),
        ))

        print("Cross-shard, native transfer within MultiESDTTransfer, towards non-payable contract")
        controller.send(controller.create_native_transfer_within_multiesdt(
            sender=accounts.get_user(shard=0, index=1),
            receiver=accounts.get_contract_address("adder", 1, 0),
        ))

    print("Intra-shard, relayed v1 transaction with MoveBalance")
    controller.send(controller.create_relayed_v1_with_move_balance(
        relayer=accounts.get_user(shard=0, index=0),
        sender=accounts.get_user(shard=0, index=1),
        receiver=accounts.get_user(shard=0, index=2).address,
        amount=42
    ))

    if with_spica:
        print("Relayed v3, senders and receivers in same shard")
        controller.send(controller.create_relayed_v3_with_a_few_inner_move_balances(
            relayer=accounts.get_user(shard=0, index=0),
            senders=accounts.users_by_shard[0][1:3],
            receivers=[account.address for account in accounts.users_by_shard[0][3:5]],
            amount=42
        ))

    if with_spica:
        print("Relayed v3, senders and receivers in different shards")
        controller.send(controller.create_relayed_v3_with_a_few_inner_move_balances(
            relayer=accounts.get_user(shard=0, index=0),
            senders=accounts.users_by_shard[0][1:3],
            receivers=[account.address for account in accounts.users_by_shard[1][3:5]],
            amount=42
        ))

    if with_spica:
        print("Relayed v3, senders and receivers in same shard (insufficient balance)")
        controller.send(controller.create_relayed_v3_with_a_few_inner_move_balances(
            relayer=accounts.get_user(shard=0, index=0),
            senders=accounts.users_by_shard[0][1:3],
            receivers=[account.address for account in accounts.users_by_shard[0][3:5]],
            amount=1000000000000000000000
        ))

    if with_spica:
        print("Relayed v3, senders and receivers in different shards (insufficient balance)")
        controller.send(controller.create_relayed_v3_with_a_few_inner_move_balances(
            relayer=accounts.get_user(shard=0, index=0),
            senders=accounts.users_by_shard[0][1:3],
            receivers=[account.address for account in accounts.users_by_shard[1][3:5]],
            amount=1000000000000000000000
        ))

    if with_spica:
        print("Relayed v3, senders and receivers in same shard, sending to non-payable contract")
        controller.send(controller.create_relayed_v3_with_a_few_inner_move_balances(
            relayer=accounts.get_user(shard=0, index=0),
            senders=[accounts.get_user(shard=0, index=5)],
            receivers=[accounts.get_contract_address("adder", 0, 0)],
            amount=1000000000000000000
        ))

    print("Intra-shard, relayed v1 transaction with MoveBalance")
    controller.send(controller.create_relayed_v1_with_move_balance(
        relayer=accounts.get_user(shard=1, index=0),
        sender=accounts.get_user(shard=1, index=9),
        receiver=Address.from_bech32("erd1qqqqqqqqqqqqqqqpqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqzllls8a5w6u"),
        amount=1000000000000000000
    ))

    print("Direct contract deployment with MoveBalance")
    controller.send(controller.create_contract_deployment_with_move_balance(
        sender=accounts.get_user(shard=0, index=0),
        amount=10000000000000000
    ))

    print("Intra-shard, contract call with MoveBalance, with signal error")
    controller.send(controller.create_contract_call_with_move_balance_with_signal_error(
        sender=accounts.get_user(shard=0, index=0),
        contract=accounts.get_contract_address("adder", 0, 0),
        amount=10000000000000000
    ))

    print("Cross-shard, contract call with MoveBalance, with signal error")
    controller.send(controller.create_contract_call_with_move_balance_with_signal_error(
        sender=accounts.get_user(shard=0, index=0),
        contract=accounts.get_contract_address("adder", 1, 0),
        amount=10000000000000000
    ))

    print("Direct contract deployment with MoveBalance, with signal error")
    controller.send(controller.create_contract_deployment_with_move_balance_with_signal_error(
        sender=accounts.get_user(shard=0, index=0),
        amount=77
    ))

    if with_spica:
        print("ClaimDeveloperRewards on directly owned contract")
        controller.send(controller.create_claim_developer_rewards_on_directly_owned_contract(
            sender=accounts.get_user(shard=0, index=0),
            contract=accounts.get_contract_address("adder", 0, 0),
        ))

        # TODO: claim developer rewards with parent-child contracts
        # TODO: claim developer rewards on directly owned contracts, cross shard (use change owner address).

    print("Intra-shard, relayed v1 transaction with contract call with MoveBalance, with signal error")
    controller.send(controller.create_relayed_v1_with_contract_call_with_move_balance_with_signal_error(
        relayer=accounts.get_user(shard=0, index=0),
        sender=accounts.get_user(shard=0, index=1),
        contract=accounts.get_contract_address("adder", 0, 0),
        amount=1
    ))


class BunchOfAccounts:
    def __init__(self, configuration: Configuration, memento: "Memento") -> None:
        self.configuration = configuration
        self.mnemonic = Mnemonic(configuration.users_mnemonic)
        self.sponsor = self._create_sponsor()
        self.users: List[Account] = []
        self.users_by_shard: List[List[Account]] = [[], [], []]
        self.users_by_bech32: Dict[str, Account] = {}
        self.contracts: List[SmartContract] = []
        self.contracts_by_shard: List[List[SmartContract]] = [[], [], []]

        address_computer = AddressComputer()

        for i in range(32):
            user = self._create_user(i)
            shard = address_computer.get_shard_of_address(user.address)
            self.users.append(user)
            self.users_by_shard[shard].append(user)
            self.users_by_bech32[user.address.to_bech32()] = user

        for item in memento.get_contracts():
            contract_address = Address.from_bech32(item.address)
            shard = address_computer.get_shard_of_address(contract_address)
            self.contracts.append(item)
            self.contracts_by_shard[shard].append(item)

    def _create_sponsor(self) -> "Account":
        sponsor_secret_key = UserSecretKey(self.configuration.sponsor_secret_key)
        sponsor_signer = UserSigner(sponsor_secret_key)
        return Account(sponsor_signer)

    def _create_user(self, index: int) -> "Account":
        user_secret_key = self.mnemonic.derive_key(index)
        user_signer = UserSigner(user_secret_key)
        return Account(user_signer)

    def get_user(self, shard: int, index: int) -> "Account":
        return self.users_by_shard[shard][index]

    def get_account_by_bech32(self, address: str) -> "Account":
        if self.sponsor.address.to_bech32() == address:
            return self.sponsor

        return self.users_by_bech32[address]

    def get_contract_address(self, tag: str, shard: int, index: int) -> Address:
        addresses: List[str] = [contract.address for contract in self.contracts_by_shard[shard] if contract.tag == tag]
        return Address.from_bech32(addresses[index])


class Controller:
    def __init__(self, configuration: Configuration, accounts: BunchOfAccounts, memento: "Memento") -> None:
        self.configuration = configuration
        self.memento = memento
        self.accounts = accounts
        self.network_provider = ProxyNetworkProvider(configuration.proxy_url)
        self.transaction_computer = TransactionComputer()
        self.transactions_converter = TransactionsConverter()
        self.transactions_factory_config = TransactionsFactoryConfig(chain_id=configuration.network_id)
        self.nonces_tracker = NoncesTracker(configuration.proxy_url)
        self.token_management_transactions_factory = TokenManagementTransactionsFactory(self.transactions_factory_config)
        self.token_management_outcome_parser = TokenManagementTransactionsOutcomeParser()
        self.transfer_transactions_factory = TransferTransactionsFactory(self.transactions_factory_config)
        self.relayed_transactions_factory = RelayedTransactionsFactory(self.transactions_factory_config)
        self.contracts_transactions_factory = SmartContractTransactionsFactory(self.transactions_factory_config)
        self.transaction_awaiter = TransactionAwaiter(self)

    # Temporary workaround, until the SDK is updated to simplify transaction awaiting.
    def get_transaction(self, tx_hash: str) -> TransactionOnNetwork:
        return self.network_provider.get_transaction(tx_hash, with_process_status=True)

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

        self.send_multiple(transactions)

        time.sleep(3)

        self.await_completed(transactions)

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

        time.sleep(3)

        [transaction_on_network] = self.await_completed([transaction])
        transaction_outcome = self.transactions_converter.transaction_on_network_to_outcome(transaction_on_network)
        [issue_fungible_outcome] = self.token_management_outcome_parser.parse_issue_fungible(transaction_outcome)
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

        self.send_multiple(transactions)

        time.sleep(3)

        self.await_completed(transactions)

    def do_create_contract_deployments(self):
        transactions_adder: List[Transaction] = []
        transactions_dummy: List[Transaction] = []
        transactions_developer_rewards: List[Transaction] = []
        address_computer = AddressComputer()

        # Adder
        transactions_adder.append(self.contracts_transactions_factory.create_transaction_for_deploy(
            sender=self.accounts.get_user(shard=0, index=0).address,
            bytecode=CONTRACT_PATH_ADDER,
            gas_limit=5000000,
            arguments=[0]
        ))

        transactions_adder.append(self.contracts_transactions_factory.create_transaction_for_deploy(
            sender=self.accounts.get_user(shard=1, index=0).address,
            bytecode=CONTRACT_PATH_ADDER,
            gas_limit=5000000,
            arguments=[0]
        ))

        transactions_adder.append(self.contracts_transactions_factory.create_transaction_for_deploy(
            sender=self.accounts.get_user(shard=2, index=0).address,
            bytecode=CONTRACT_PATH_ADDER,
            gas_limit=5000000,
            arguments=[0]
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

        transactions_all = transactions_adder + transactions_dummy + transactions_developer_rewards

        for transaction in transactions_all:
            self.apply_nonce(transaction)
            self.sign(transaction)

        for transaction in transactions_adder:
            sender = Address.from_bech32(transaction.sender)
            contract_address = address_computer.compute_contract_address(sender, transaction.nonce)
            self.memento.add_contract("adder", contract_address.to_bech32())

        for transaction in transactions_dummy:
            sender = Address.from_bech32(transaction.sender)
            contract_address = address_computer.compute_contract_address(sender, transaction.nonce)
            self.memento.add_contract("adder", contract_address.to_bech32())

        for transaction in transactions_developer_rewards:
            sender = Address.from_bech32(transaction.sender)
            contract_address = address_computer.compute_contract_address(sender, transaction.nonce)
            self.memento.add_contract("developerRewards", contract_address.to_bech32())

        self.send_multiple(transactions_all)

        time.sleep(3)

        self.await_completed(transactions_all)

    def create_simple_move_balance_with_refund(self, sender: "Account", receiver: Address) -> Transaction:
        transaction = self.transfer_transactions_factory.create_transaction_for_native_token_transfer(
            sender=sender.address,
            receiver=receiver,
            native_amount=42
        )

        transaction.gas_limit += 42000

        self.apply_nonce(transaction)
        self.sign(transaction)

        return transaction

    def create_invalid_move_balance_with_refund(self, sender: "Account", receiver: Address) -> Transaction:
        transaction = self.transfer_transactions_factory.create_transaction_for_native_token_transfer(
            sender=sender.address,
            receiver=receiver,
            native_amount=1000000000000000000000000
        )

        transaction.gas_limit += 42000

        self.apply_nonce(transaction)
        self.sign(transaction)

        return transaction

    def create_native_transfer_within_multiesdt(self, sender: "Account", receiver: Address) -> Transaction:
        custom_currency = self.memento.get_custom_currencies()[0]

        transaction = self.transfer_transactions_factory.create_transaction_for_transfer(
            sender=sender.address,
            receiver=receiver,
            native_amount=42,
            token_transfers=[TokenTransfer(Token(custom_currency), 7)]
        )

        self.apply_nonce(transaction)
        self.sign(transaction)

        return transaction

    def create_relayed_v1_with_move_balance(self, relayer: "Account", sender: "Account", receiver: Address, amount: int) -> Transaction:
        # Relayer nonce is reserved before sender nonce, to ensure good ordering (if sender and relayer are the same account).
        relayer_nonce = self._reserve_nonce(relayer)

        inner_transaction = self.transfer_transactions_factory.create_transaction_for_native_token_transfer(
            sender=sender.address,
            receiver=receiver,
            native_amount=amount
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

    def create_relayed_v1_with_esdt_transfer(self, relayer: "Account", sender: "Account", receiver: Address, amount: int) -> Transaction:
        custom_currency = self.memento.get_custom_currencies()[0]

        # Relayer nonce is reserved before sender nonce, to ensure good ordering (if sender and relayer are the same account).
        relayer_nonce = self._reserve_nonce(relayer)

        inner_transaction = self.transfer_transactions_factory.create_transaction_for_esdt_token_transfer(
            sender=sender.address,
            receiver=receiver,
            token_transfers=[TokenTransfer(Token(custom_currency), amount)]
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

    def create_relayed_v2_with_move_balance(self, relayer: "Account", sender: "Account", receiver: Address, amount: int) -> Transaction:
        # Relayer nonce is reserved before sender nonce, to ensure good ordering (if sender and relayer are the same account).
        relayer_nonce = self._reserve_nonce(relayer)

        inner_transaction = self.transfer_transactions_factory.create_transaction_for_native_token_transfer(
            sender=sender.address,
            receiver=receiver,
            native_amount=amount
        )

        inner_transaction.gas_limit = 0

        self.apply_nonce(inner_transaction)
        self.sign(inner_transaction)

        transaction = self.relayed_transactions_factory.create_relayed_v2_transaction(
            inner_transaction=inner_transaction,
            inner_transaction_gas_limit=100000,
            relayer_address=relayer.address,
        )

        transaction.nonce = relayer_nonce
        self.sign(transaction)

        return transaction

    def create_relayed_v3_with_a_few_inner_move_balances(self, relayer: "Account", senders: List["Account"], receivers: List[Address], amount: int) -> Transaction:
        # Relayer nonce is reserved before sender nonce, to ensure good ordering (if sender and relayer are the same account).
        relayer_nonce = self._reserve_nonce(relayer)

        if len(senders) != len(receivers):
            raise ValueError("senders and receivers must have the same length", len(senders), len(receivers))

        inner_transactions: List[Transaction] = []

        for sender, receiver in zip(senders, receivers):
            inner_transaction = self.transfer_transactions_factory.create_transaction_for_native_token_transfer(
                sender=sender.address,
                receiver=receiver,
                native_amount=amount,
            )

            inner_transaction.relayer = relayer.address.to_bech32()
            self.apply_nonce(inner_transaction)
            self.sign(inner_transaction)
            inner_transactions.append(inner_transaction)

        transaction = self.relayed_transactions_factory.create_relayed_v3_transaction(
            relayer_address=relayer.address,
            inner_transactions=inner_transactions,
        )

        transaction.nonce = relayer_nonce
        self.sign(transaction)

        return transaction

    def create_contract_deployment_with_move_balance(self, sender: "Account", amount: int) -> Transaction:
        transaction = self.contracts_transactions_factory.create_transaction_for_deploy(
            sender=sender.address,
            bytecode=CONTRACT_PATH_DUMMY,
            gas_limit=5000000,
            arguments=[0],
            native_transfer_amount=amount
        )

        self.apply_nonce(transaction)
        self.sign(transaction)

        return transaction

    def create_contract_deployment_with_move_balance_with_signal_error(self, sender: "Account", amount: int) -> Transaction:
        transaction = self.contracts_transactions_factory.create_transaction_for_deploy(
            sender=sender.address,
            bytecode=CONTRACT_PATH_ADDER,
            gas_limit=5000000,
            arguments=[1, 2, 3, 4, 5],
            native_transfer_amount=amount
        )

        self.apply_nonce(transaction)
        self.sign(transaction)

        return transaction

    def create_contract_call_with_move_balance_with_signal_error(self, sender: "Account", contract: Address, amount: int) -> Transaction:
        transaction = self.contracts_transactions_factory.create_transaction_for_execute(
            sender=sender.address,
            contract=contract,
            function="missingFunction",
            gas_limit=5000000,
            arguments=[1, 2, 3, 4, 5],
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

    def create_relayed_v1_with_contract_call_with_move_balance_with_signal_error(self, relayer: "Account", sender: "Account", contract: Address, amount: int) -> Transaction:
        # Relayer nonce is reserved before sender nonce, to ensure good ordering (if sender and relayer are the same account).
        relayer_nonce = self._reserve_nonce(relayer)

        inner_transaction = self.contracts_transactions_factory.create_transaction_for_execute(
            sender=sender.address,
            contract=contract,
            function="add",
            gas_limit=5000000,
            arguments=[1, 2, 3, 4, 5],
            native_transfer_amount=amount
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

    def apply_nonce(self, transaction: Transaction):
        sender = self.accounts.get_account_by_bech32(transaction.sender)
        transaction.nonce = self.nonces_tracker.get_then_increment_nonce(sender.address)

    def _reserve_nonce(self, account: "Account"):
        sender = self.accounts.get_account_by_bech32(account.address.to_bech32())
        return self.nonces_tracker.get_then_increment_nonce(sender.address)

    def sign(self, transaction: Transaction):
        sender = self.accounts.get_account_by_bech32(transaction.sender)
        bytes_for_signing = self.transaction_computer.compute_bytes_for_signing(transaction)
        transaction.signature = sender.signer.sign(bytes_for_signing)

    def send_multiple(self, transactions: List[Transaction]):
        self.network_provider.send_transactions(transactions)

    def send(self, transaction: Transaction):
        transaction_hash = self.network_provider.send_transaction(transaction)
        print(f"{self.configuration.explorer_url}/transactions/{transaction_hash}")

    def await_completed(self, transactions: List[Transaction]) -> List[TransactionOnNetwork]:
        print(f"Awaiting completion of {len(transactions)} transactions...")

        transactions_on_network: List[TransactionOnNetwork] = []

        # We do sequential awaiting (perfectly fine in this context).
        for transaction in transactions:
            transaction_hash = self.transaction_computer.compute_transaction_hash(transaction).hex()
            transaction_on_network = self.transaction_awaiter.await_completed(transaction_hash)
            transactions_on_network.append(transaction_on_network)

            print(f"Completed: {self.configuration.explorer_url}/transactions/{transaction_hash}")

        return transactions_on_network


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
    def __init__(self, path: Path) -> None:
        self.path = path
        self._contracts: List[SmartContract] = []
        self._custom_currencies: List[str] = []

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

    def get_contracts(self) -> List[SmartContract]:
        self.load()
        return self._contracts

    def load(self):
        if not self.path.exists():
            return

        data = json.loads(self.path.read_text())

        contracts_raw = data.get("contracts", [])
        self._contracts = [SmartContract.from_dictionary(item) for item in contracts_raw]
        self._custom_currencies = data.get("customCurrencies", [])

    def save(self):
        contracts_raw = [contract.to_dictionary() for contract in self._contracts]

        data = {
            "contracts": contracts_raw,
            "customCurrencies": self._custom_currencies,
        }

        self.path.parent.mkdir(parents=True, exist_ok=True)
        self.path.write_text(json.dumps(data, indent=4))


if __name__ == '__main__':
    main()
