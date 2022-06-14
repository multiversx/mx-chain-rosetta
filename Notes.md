# Implementation notes

 - We do not support the `related_transactions` property, since it's not feasible to properly filter the related transactions of a given transaction by source / destination shard (with respect to the observed shard).
 - The endpoint `/block/transaction` is not implemented, since all transactions are returned by the endpoint `/block`.
 - We chose not to support the optional property `Operation.related_operations`. Although the smart contract results (also known as _unsigned transactions_) form a DAG (directed acyclic graph) at the protocol level, operations within a transaction are in a simple sequence.
 - Only successful operations are listed in our Rosetta API implementation. For _invalid_ transactions, we only list the _fee_ operation.
 - Balance-changing operations that affect Smart Contract accounts are not emitted by our Rosetta implementation (thus are not available on the Rosetta API).
