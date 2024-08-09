from argparse import ArgumentParser
from pathlib import Path
from typing import Dict, List

from multiversx_sdk import (Address, AddressComputer, Mnemonic,  # type: ignore
                            ProxyNetworkProvider, RelayedTransactionsFactory,
                            SmartContractTransactionsFactory, Token,
                            TokenTransfer, Transaction, TransactionComputer,
                            TransactionsFactoryConfig,
                            TransferTransactionsFactory, UserSecretKey,
                            UserSigner)

from systemtests.config import CONFIGURATIONS, Configuration

CONTRACT_PATH_ADDER = Path(__file__).parent / "adder.wasm"


def main():
    parser = ArgumentParser()
    parser.add_argument("--network", choices=CONFIGURATIONS.keys(), required=True)
    parser.add_argument("--mode", choices=["setup", "run"], required=True)
    args = parser.parse_args()

    configuration = CONFIGURATIONS[args.network]
    mode = args.mode

    accounts = BunchOfAccounts(configuration)
    controller = Controller(configuration, accounts)

    if mode == "setup":
        controller.send_multiple(controller.create_airdrops())
        controller.send_multiple(controller.create_contract_deployments())
    elif mode == "run":
        # Intra-shard
        controller.send(controller.create_simple_move_balance_with_refund(
            sender=accounts.get_user(shard=0, index=0),
            receiver=accounts.get_user(shard=0, index=1).address,
        ))

        # Cross-shard
        controller.send(controller.create_simple_move_balance_with_refund(
            sender=accounts.get_user(shard=0, index=1),
            receiver=accounts.get_user(shard=1, index=0).address,
        ))

        # Intrashard
        controller.send(controller.create_invalid_move_balance_with_refund(
            sender=accounts.get_user(shard=0, index=2),
            receiver=accounts.get_user(shard=0, index=3).address,
        ))

        # Cross-shard
        controller.send(controller.create_invalid_move_balance_with_refund(
            sender=accounts.get_user(shard=0, index=4),
            receiver=accounts.get_user(shard=1, index=1).address,
        ))

        # Intra-shard, sending value to non-payable contract
        controller.send(controller.create_simple_move_balance_with_refund(
            sender=accounts.get_user(shard=0, index=0),
            receiver=accounts.contracts_by_shard[0][0],
        ))

        # Cross-shard, sending value to non-payable contract
        controller.send(controller.create_simple_move_balance_with_refund(
            sender=accounts.get_user(shard=0, index=1),
            receiver=accounts.contracts_by_shard[1][0],
        ))

        # Intra-shard, native transfer within MultiESDTTransfer
        controller.send(controller.create_native_transfer_within_multiesdt(
            sender=accounts.get_user(shard=0, index=0),
            receiver=accounts.get_user(shard=0, index=1).address,
        ))

        # Cross-shard, native transfer within MultiESDTTransfer
        controller.send(controller.create_native_transfer_within_multiesdt(
            sender=accounts.get_user(shard=0, index=1),
            receiver=accounts.get_user(shard=1, index=0).address,
        ))

        # Intra-shard, native transfer within MultiESDTTransfer, towards non-payable contract
        controller.send(controller.create_native_transfer_within_multiesdt(
            sender=accounts.get_user(shard=0, index=0),
            receiver=accounts.contracts_by_shard[0][0],
        ))

        # Cross-shard, native transfer within MultiESDTTransfer, towards non-payable contract
        controller.send(controller.create_native_transfer_within_multiesdt(
            sender=accounts.get_user(shard=0, index=1),
            receiver=accounts.contracts_by_shard[1][0],
        ))


class BunchOfAccounts:
    def __init__(self, configuration: Configuration) -> None:
        self.configuration = configuration
        self.mnemonic = Mnemonic(configuration.users_mnemonic)
        self.sponsor = self._create_sponsor()
        self.users: List[Account] = []
        self.users_by_shard: List[List[Account]] = [[], [], []]
        self.users_by_bech32: Dict[str, Account] = {}
        self.contracts: List[Address] = []
        self.contracts_by_shard: List[List[Address]] = [[], [], []]

        address_computer = AddressComputer()

        for i in range(32):
            user = self._create_user(i)
            shard = address_computer.get_shard_of_address(user.address)
            self.users.append(user)
            self.users_by_shard[shard].append(user)
            self.users_by_bech32[user.address.to_bech32()] = user

        for item in configuration.known_contracts:
            contract_address = Address.from_bech32(item)
            shard = address_computer.get_shard_of_address(contract_address)
            self.contracts.append(contract_address)
            self.contracts_by_shard[shard].append(contract_address)

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


class Controller:
    def __init__(self, configuration: Configuration, accounts: BunchOfAccounts) -> None:
        self.configuration = configuration
        self.accounts = accounts
        self.custom_currencies = CustomCurrencies(configuration)
        self.transactions_factory_config = TransactionsFactoryConfig(chain_id=configuration.network_id)
        self.network_provider = ProxyNetworkProvider(configuration.proxy_url)
        self.nonces_tracker = NoncesTracker(configuration.proxy_url)
        self.transfer_transactions_factory = TransferTransactionsFactory(self.transactions_factory_config)
        self.relayed_transactions_factory = RelayedTransactionsFactory(self.transactions_factory_config)
        self.contracts_transactions_factory = SmartContractTransactionsFactory(self.transactions_factory_config)

    def create_airdrops(self) -> List[Transaction]:
        transactions: List[Transaction] = []

        for user in self.accounts.users:
            transaction = self.transfer_transactions_factory.create_transaction_for_native_token_transfer(
                sender=self.accounts.sponsor.address,
                receiver=user.address,
                native_amount=10000000000000000
            )

            transaction = self.transfer_transactions_factory.create_transaction_for_esdt_token_transfer(
                sender=self.accounts.sponsor.address,
                receiver=user.address,
                token_transfers=[TokenTransfer(Token(self.custom_currencies.currency), 1000000)]
            )

            transactions.append(transaction)

        return transactions

    def create_contract_deployments(self) -> List[Transaction]:
        transactions: List[Transaction] = []

        # Non-payable contracts, in each shard
        transactions.append(self.contracts_transactions_factory.create_transaction_for_deploy(
            sender=self.accounts.get_user(shard=0, index=0).address,
            bytecode=CONTRACT_PATH_ADDER,
            gas_limit=5000000,
            arguments=[0]
        ))

        transactions.append(self.contracts_transactions_factory.create_transaction_for_deploy(
            sender=self.accounts.get_user(shard=1, index=0).address,
            bytecode=CONTRACT_PATH_ADDER,
            gas_limit=5000000,
            arguments=[0]
        ))

        transactions.append(self.contracts_transactions_factory.create_transaction_for_deploy(
            sender=self.accounts.get_user(shard=2, index=0).address,
            bytecode=CONTRACT_PATH_ADDER,
            gas_limit=5000000,
            arguments=[0]
        ))

        return transactions

    def create_simple_move_balance_with_refund(self, sender: "Account", receiver: Address) -> Transaction:
        transaction = self.transfer_transactions_factory.create_transaction_for_native_token_transfer(
            sender=sender.address,
            receiver=receiver,
            native_amount=42
        )

        transaction.gas_limit += 42000
        return transaction

    def create_invalid_move_balance_with_refund(self, sender: "Account", receiver: Address) -> Transaction:
        transaction = self.transfer_transactions_factory.create_transaction_for_native_token_transfer(
            sender=sender.address,
            receiver=receiver,
            native_amount=1000000000000000000000000
        )

        transaction.gas_limit += 42000
        return transaction

    def create_native_transfer_within_multiesdt(self, sender: "Account", receiver: Address) -> Transaction:
        transaction = self.transfer_transactions_factory.create_transaction_for_transfer(
            sender=sender.address,
            receiver=receiver,
            native_amount=42,
            token_transfers=[TokenTransfer(Token(self.custom_currencies.currency), 7)]
        )

        return transaction

    def send_multiple(self, transactions: List[Transaction]):
        for transaction in transactions:
            sender = self.accounts.get_account_by_bech32(transaction.sender)
            self._apply_nonce(transaction, sender)
            self._sign(transaction, sender)

        self.network_provider.send_transactions(transactions)

    def send(self, transaction: Transaction):
        sender = self.accounts.get_account_by_bech32(transaction.sender)
        self._apply_nonce(transaction, sender)
        self._sign(transaction, sender)
        transaction_hash = self.network_provider.send_transaction(transaction)
        print(f"{self.configuration.explorer_url}/transactions/{transaction_hash}")

    def _apply_nonce(self, transaction: Transaction, user: "Account"):
        transaction.nonce = self.nonces_tracker.get_then_increment_nonce(user.address)

    def _sign(self, transaction: Transaction, user: "Account"):
        computer = TransactionComputer()
        bytes_for_signing = computer.compute_bytes_for_signing(transaction)
        transaction.signature = user.signer.sign(bytes_for_signing)


class Account:
    def __init__(self, signer: UserSigner) -> None:
        self.signer = signer
        self.address: Address = signer.get_pubkey().to_address("erd")


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


class CustomCurrencies:
    def __init__(self, configuration: Configuration) -> None:
        self.currency = "ROSETTA-7783de"


if __name__ == '__main__':
    main()
