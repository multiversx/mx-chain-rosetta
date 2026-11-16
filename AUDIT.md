# Audit and review of mx-chain-rosetta

Reviewed at commit `81f96eb` (tagged `v0.8.2`) on 2026-11-14. The companion file [IMPROVEMENTS.md](IMPROVEMENTS.md) turns each finding into a concrete change and orders the work.

## Verdict

The core of the project, turning MultiversX blocks into balance-changing Rosetta operations, is careful and well tested. The weak spots are at the edges: process wiring, the offline mode, request validation, and repository hygiene.

I found one issue that could let a third party stall every Rosetta instance on a shard, and seven medium-severity issues, five of which are plain bugs. Most of them sit in code that no test exercises.

| Severity | Count | Findings |
|---|---|---|
| High | 1 | A1 |
| Medium | 7 | A2 – A8 |
| Low | 8 | A9 – A16 |
| Repository and process | 6 | B1 – B6 |

## Scope and method

All Go sources outside tests were read in full (about 6,000 lines), along with the CI workflows, the README and the system-test configuration. The test files and the Python system tests were skimmed, not reviewed line by line.

| Check | Result |
|---|---|
| `go build ./...` | passes |
| `go vet ./...` | clean |
| `gofmt -l .` | clean |
| `go test ./...` and `go test -race ./...` | pass |
| `go mod tidy -diff`, `go mod verify` | no drift, all modules verified |
| Statement coverage | 74.9% total: `server/services` 81.8%, `server/provider` 82.1%, `cmd/rosetta` 12.8%, `server/factory` 0% |
| Binary started in offline mode and exercised over HTTP | done, results in A2, A3, A6, A7, B1 |

Not done:

- **Vulnerability and lint scanners.** `govulncheck`, `staticcheck` and `golangci-lint` are not installed on this machine, and fetching them to run was not permitted in this session. Dependency findings in B5 are therefore based on versions only.
- **Live network.** Nothing was run against an observer, and the `mesh-cli` system tests were not run.
- **On-chain confirmation of A1.** The Rosetta side is reproduced by a test; the protocol side is established by reading the VM source, not by deploying a contract.

Each finding states how it was verified. Reproduction code is in the appendix.

## What is in good shape

- **Layering.** `provider` talks to the observer, `services` implements the Rosetta endpoints, `factory` wires them. Dependencies point one way.
- **The transaction transformer.** It handles the hard cases explicitly and comments on why: scheduled miniblocks, invalid transactions, ineffective refunds, developer rewards, relayed V3 fee payers, failed cross-shard move-balance, Supernova proposal-only transactions.
- **Reading only final data.** The "latest" block is `highestFinalNonce - 2`, capped by the last executed nonce, so consumers are never served a block that can be rolled back.
- **Fee arithmetic.** It uses `big.Int` and mirrors the protocol's gas-price-modifier computation.
- **Continuous validation.** Scheduled workflows run `mesh-cli check:data` against mainnet and devnet every 12 hours. For a balance-reconciliation service this is the test that matters most.
- **CI basics.** Workflows declare `permissions: contents: read`.

## Findings

### High

#### A1. A single unrecognised event makes the whole block unservable, and any contract can produce one

**Where:** `server/services/transactionEventsController.go:27`, reached from `transactionsTransformer.go:351`.

Events are selected by identifier alone. When an event with a known identifier has an unexpected number of topics, the extractor returns `errCannotRecognizeEvent`, which aborts `transformBlockTxs` and makes `/block` return `ErrUnableToGetBlock` for that nonce. The error is marked retriable, but it will never succeed.

For most identifiers this is safe, because the protocol stops contracts from producing them. In `mx-chain-vm-go` v1.5.43 (the version pinned by `mx-chain-go` v1.11.1), a contract's log identifier is the name of the executing endpoint (`vmhost/contexts/output.go`, `WriteLog`). Endpoint names that match an active built-in function are rejected at deploy time, and so are five protected names: `internalVMErrors`, `transferValueOnly`, `writeLog`, `signalError`, `completedTxEvent` (`vmhost/contexts/validator.go`).

`SCDeploy` is in neither list. A contract can therefore export an endpoint called `SCDeploy` and emit an event from it with one topic. Rosetta then fails on the block containing that call.

**Impact:** a consumer that syncs blocks in order (an exchange, `mesh-cli`) stops at that block on that shard until Rosetta is patched. The cost to the attacker is one deployment and one call.

**Verified:** the Rosetta side by test (`TestRepro_SCDeployEventWithOneTopicFailsWholeBlock`, appendix). The protocol side by reading the VM source. It has not been confirmed on a localnet, which is the first thing to do.

The same fail-closed behaviour applies to an unknown transaction type (`transactionsTransformer.go:120`). That one is a deliberate trade-off, but it means a protocol upgrade that adds a type halts Rosetta until a release.

### Medium

#### A2. Offline mode panics on `/block`, `/account/*` and `/mempool*`

**Where:** `server/services/offline_services.go:15`.

`NewOfflineService` returns `&offlineService{}` without setting `errFactory`. Every method then calls `service.errFactory.newErr(ErrOfflineMode)` on a nil pointer.

**Impact:** the client gets an empty reply (connection closed) instead of the Rosetta error `rosetta server is in offline mode`, and each request writes a panic stack to the log file.

**Verified:** end to end. `curl` against an offline instance exits with code 52 on `/block` and `/account/balance`. No test references `offlineService`.

#### A3. `/construction/preprocess` panics on an operation without `account` or `amount`

**Where:** `server/services/constructionService.go:57`, `:67`, `:77`, `:87`.

The handler dereferences `request.Operations[i].Account` and `.Amount`. The SDK asserter that runs first accepts operations where either is absent.

**Impact:** an unauthenticated request causes a panic. `net/http` recovers it, so the process survives, but the client gets an empty reply and the log gets a stack trace per request.

**Verified:** end to end with `{"operations":[{"operation_identifier":{"index":0},"type":"Transfer"}]}`.

#### A4. `--gas-limit-custom-transfer` silently becomes 0 for values of 1,000,000 or more

**Where:** `cmd/rosetta/cli.go:141` declares a `Float64Flag`; `cli.go:282` reads it with `GlobalUint64`.

`urfave/cli` formats the float and re-parses it as an integer. From 1,000,000 upwards the float prints as `1e+06`, parsing fails, and the result is 0 with no error.

| Flag value | Parsed value |
|---|---|
| default (200000) | 200000 |
| 250000 | 250000 |
| 1000000 | 0 |
| 2000000 | 0 |
| 200000.5 | 0 |

**Impact:** with 0, `/construction/metadata` suggests a gas limit for ESDT transfers that covers data movement only, so transactions built from it run out of gas. The default works.

**Verified:** by test (`TestRepro_GasLimitCustomTransferFlag`).

#### A5. `--genesis-timestamp` is ignored

**Where:** `cmd/rosetta/main.go:63-86`.

The flag is parsed into `cliFlags.genesisTimestamp`, but `startRosetta` never copies it into `ArgsCreateNetworkProvider`. The provider's `genesisTimestamp` is always 0.

**Impact:** the genesis block returned by `/block` has `timestamp: 0` on every network, whatever the operator configures.

**Verified:** by reading; no assignment of `GenesisTimestamp` exists outside the factory and the test mock.

#### A6. No graceful shutdown on SIGTERM, and a failed bind leaves the process running

**Where:** `cmd/rosetta/main.go:107-125`.

- `signal.Notify(stop, os.Interrupt, os.Kill)` registers SIGINT and SIGKILL. SIGKILL cannot be caught, and SIGTERM, which Docker, systemd and Kubernetes send, is not registered. The process dies with exit code 143 without calling `Shutdown` or closing the log file.
- `ListenAndServe` runs in a goroutine. If the port is taken, the error is logged and the main goroutine keeps waiting for a signal. A supervisor sees a live process that serves nothing.
- The shutdown timeout is one second, shorter than a `/block` request can take.

**Verified:** end to end. A second instance on a busy port was still alive after 4 seconds; `kill -TERM` gave exit code 143.

#### A7. The HTTP server has no protective limits

**Where:** `cmd/rosetta/main.go:131-144`, `cmd/rosetta/cli.go:36-43`, `cmd/rosetta/pprof.go`.

- No `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout` or `IdleTimeout`, so slow clients can hold connections open indefinitely.
- No request body size limit.
- It listens on all interfaces (`":port"`), with no flag to bind elsewhere.
- CORS is `Access-Control-Allow-Origin: *` on every response (confirmed over HTTP).
- With `--pprof`, the profiling endpoints are served on the same public listener.
- The default log level is `*:DEBUG`, and the construction endpoints log request metadata and signed transactions at that level.

**Impact:** depends on deployment. Behind a reverse proxy on a private network most of this is mitigated; exposed directly, it is not. Rosetta has no authentication by design, so the README should say which of these the operator is expected to handle.

#### A8. `/construction/parse` reports any transaction as a plain transfer, ignoring `data`

**Where:** `server/services/constructionService.go:242-284`.

Unless `data` starts with `ESDTTransfer`, the parsed operations are a native transfer of `value` from sender to receiver. A contract call, `MultiESDTNFTTransfer` or `ESDTNFTTransfer` is not reflected.

**Impact:** `/parse` is the check an offline signer runs to confirm that what it is about to sign matches the intended operations. A payload introduced between `/metadata` and `/payloads` (for example by a compromised online component) would not show up in that check.

**Verified:** by test (`TestRepro_ParseReportsContractCallAsPlainTransfer`).

### Low

#### A9. The reported middleware version is stale

`version/constants.go:10` says `v0.7.2`. The reviewed commit is tagged `v0.8.2`; the constant was last changed in September 2025. `/network/options` and `--version` report the wrong release.

#### A10. Request values are placed in observer URL paths without validation

**Where:** `server/provider/urls.go`, `server/services/accountService.go:56-80`, `server/provider/networkProvider.go:330`, `:441`.

Account addresses, token identifiers, block hashes and transaction hashes from requests are formatted into paths. `url.URL.String()` escapes `?` and `#` but leaves `/` and `..` alone, so a currency symbol of `A-b/../../../../node/status` yields `/address/<addr>/esdt/A-b/../../../../node/status`.

**Impact:** limited. Requests are GET only, go only to the configured observer, and the response is decoded into a fixed struct. It is still input that should be rejected at the boundary.

Two related gaps in `/account/balance`:

- It accepts a currency that is not configured, queries the observer anyway, and reports it with `decimals: 0`.
- It does not check that the address belongs to the observed shard.

**Verified:** URL construction by test. The observer's handling of such paths was not tested.

#### A11. `/construction/metadata` does not validate its options

**Where:** `server/services/constructionService.go:118-174`.

`constructionOptions.validate()` is called in `/preprocess` only. Calling `/metadata` directly with `amount: "not-a-number"` and an unknown currency succeeds and returns `data = "ESDTTransfer@<hex>@"`, with an empty amount, because `amountToHex` returns `""` on failure after logging a warning.

**Verified:** by test.

#### A12. Transaction order is non-deterministic for blocks with scheduled miniblocks

`server/provider/scheduledMiniblocks.go:134` builds the result by iterating a map. Two requests for the same pre-Supernova block can return transactions in different orders. Rosetta does not require an order, but identical requests should produce identical responses.

#### A13. `/mempool/transaction` renders every pending transaction as a native transfer

`mempoolMoveBalanceTxToRosettaTx` (`transactionsTransformer.go:318`) is applied to any pending transaction, so ESDT transfers and contract calls appear as value transfers or as nothing. The `if err != nil` at `mempoolService.go:44` tests a variable that cannot have changed.

#### A14. The block cache is bounded by count, not by size, and node status is not cached

- `blocksCacheCapacity = 1024` (`server/provider/constants.go:8`) holds full blocks including transactions and logs. Worst-case memory is large and not configurable.
- Every `/block` call fetches `/node/status` first, and `/network/options` triggers four observer calls to read a version string.

#### A15. Custom-currency configuration is barely validated

`newCurrenciesProvider` rejects an empty symbol only. Duplicate symbols overwrite silently, and negative decimals or malformed identifiers are accepted. `--observer-projected-shard` is truncated to a byte without a range check (`networkProvider.go:360`).

#### A16. Dead code and small defects

- `server/factory/provider.go:88` checks an `err` that the preceding call does not set.
- `server/provider/errors.go:30` formats a transaction hash as `address = %s`.
- `server/services/errors.go:126` reads "gas price is to low"; the `ErrInvalidInputParam` message ends in a dangling colon.
- `server/services/logging.go` writes trace output with `fmt.Printf`, bypassing the logger.
- `NetworkProvider` is declared twice (`server/factory/interface.go`, `server/services/interface.go`) and must be kept in sync by hand.
- `--activation-epoch-sirius` and `--activation-epoch-spica` are accepted and ignored.
- `ErrUnableToGetNetworkConfig` is advertised in `/network/options` but never returned.

### Repository and process

#### B1. The README quick start does not work

- Both example commands omit `--first-historical-epoch` and `--num-historical-epochs`, which are required. Running the offline example prints `Required flags "first-historical-epoch, num-historical-epochs" not set` (confirmed).
- The build step says `cd rosetta/cmd/rosetta`; the clone directory is `mx-chain-rosetta`.
- The CodeQL badge points at `codeql-analysis.yml`, which does not exist in `.github/workflows`.
- Links go to `rosetta-api.org`, and one note is dated "as of September 2022".

#### B2. There is no LICENSE file

The repository is public and has no licence text, so the terms of use are undefined.

#### B3. CI gaps

- **Unpinned installer in jobs that hold secrets.** The mesh-cli workflows run `curl … | sh` from the mutable branch `install-rename` of another repository. Those jobs have access to `USERS_MNEMONIC`, `SPONSOR_SECRET_KEY` and the proxy URLs.
- **Actions pinned by tag, not commit.** This includes `codecov/codecov-action@v5`, which receives `CODECOV_TOKEN`.
- **Tests run only on pull requests,** inside the coverage job, without `-race`. Nothing runs on push to `main`.
- **Linting is weak.** `golangci-lint` runs with `only-new-issues: true` and no configuration file, so existing issues are never reported and only default linters apply.
- **No vulnerability scanning,** no Dependabot, no CodeQL workflow, no `SECURITY.md`.
- **Leftovers.** A `Gopkg.toml`/`dep` step that can never run, `go get -d` (deprecated), mixed `actions/checkout` versions. The mesh-cli workflows do not set up Go and rely on the runner's default.
- **Python is not linted.** `.flake8` and `pyrightconfig.json` exist, but no workflow runs them over 2,700 lines of system tests.
- The testnet check workflow has no schedule, unlike mainnet and devnet.

#### B4. Test coverage misses the code where the bugs are

| Area | Coverage | Bugs found there |
|---|---|---|
| `cmd/rosetta` flag parsing and startup | 0% | A4, A5, A6 |
| `server/factory` | 0% | dead check in A16 |
| `offline_services.go` | 0% | A2 |
| Genesis block, block by hash | 0% | A5 |
| `SendTransaction`, mempool lookup, receipt hash (provider) | 0% | — |

There are no tests at the HTTP level with the real asserter in front of the services. Such tests would have caught A2 and A3.

#### B5. Dependencies

- `mx-chain-core-go` is pinned to a pseudo-version (an untagged commit).
- `mx-chain-go` (the full node) is a direct dependency for one constructor, `sharding.NewMultiShardCoordinator`. It accounts for most of the module graph.
- Several indirect dependencies are old: `golang.org/x/crypto` v0.32.0, `golang.org/x/net` v0.34.0, `gin` v1.10.0. Whether any published advisory is reachable from this code is unknown, because the scan was not run.
- The signing payload is `json.Marshal` of `mx-chain-proxy-go`'s `data.Transaction`. Signature validity therefore depends on that struct's field order and `omitempty` tags staying identical to the protocol's canonical form across proxy upgrades. Nothing in this repository tests that.

#### B6. Test wallet secrets in source

`systemtests/config.py:6-8` hard-codes a secret key and a mnemonic as defaults. They are the public MultiversX test wallets, so this is not a leak. It will trip secret scanners, and those accounts must never hold real funds.

## Appendix: reproductions

The tests below were run with `go test -overlay`, so nothing was added to the repository. To re-run them, save each as a `_test.go` file in the package named in its first line.

### A1, A2, A3, A8, A11 — `package services`

```go
func TestRepro_OfflineServicePanics(t *testing.T) {
	require.Panics(t, func() {
		_, _ = NewOfflineService().Block(context.Background(), &types.BlockRequest{})
	})
}

func TestRepro_PreprocessPanicsOnOperationWithoutAccount(t *testing.T) {
	provider := testscommon.NewNetworkProviderMock()
	netID := &types.NetworkIdentifier{Blockchain: provider.GetBlockchainName(), Network: provider.GetNetworkConfig().NetworkName}
	a, err := asserter.NewServer(SupportedOperationTypes, true, []*types.NetworkIdentifier{netID}, nil, false, "")
	require.NoError(t, err)

	request := &types.ConstructionPreprocessRequest{
		NetworkIdentifier: netID,
		Operations: []*types.Operation{
			{OperationIdentifier: &types.OperationIdentifier{Index: 0}, Type: opTransfer},
		},
	}
	require.NoError(t, a.ConstructionPreprocessRequest(request)) // the asserter lets it through

	require.Panics(t, func() {
		_, _ = NewConstructionService(provider).ConstructionPreprocess(context.Background(), request)
	})
}

func TestRepro_SCDeployEventWithOneTopicFailsWholeBlock(t *testing.T) {
	provider := testscommon.NewNetworkProviderMock()
	transformer := newTransactionsTransformer(provider)

	good := &transaction.ApiTransactionResult{
		Hash: "aaaa", Type: string(transaction.TxTypeNormal), Status: transaction.TxStatusSuccess,
		Sender: testscommon.TestAddressAlice, Receiver: testscommon.TestAddressBob, Value: "100", InitiallyPaidFee: "10",
	}
	evil := &transaction.ApiTransactionResult{
		Hash: "bbbb", Type: string(transaction.TxTypeNormal), Status: transaction.TxStatusSuccess,
		Sender: testscommon.TestAddressAlice, Receiver: testscommon.TestAddressOfContract, Value: "0", InitiallyPaidFee: "10",
		Logs: &transaction.ApiLogs{Events: []*transaction.Events{
			{Identifier: "SCDeploy", Address: testscommon.TestAddressOfContract, Topics: [][]byte{[]byte("anything")}},
		}},
	}
	block := &api.Block{Nonce: 10, LastExecutionResult: &api.ExecutionResult{},
		MiniBlocks: []*api.MiniBlock{{Transactions: []*transaction.ApiTransactionResult{good, evil}}}}

	_, err := transformer.transformBlockTxs(block)
	require.ErrorIs(t, err, errCannotRecognizeEvent)
	// err: cannot recognize transaction event: bad number of topics for SCdeploy event = 1
}

func TestRepro_MetadataAcceptsGarbageAmountAndUnknownCurrency(t *testing.T) {
	provider := testscommon.NewNetworkProviderMock()
	provider.MockAccountsByAddress[testscommon.TestAddressAlice] = &resources.Account{Address: testscommon.TestAddressAlice, Nonce: 7}

	response, errTyped := NewConstructionService(provider).ConstructionMetadata(context.Background(), &types.ConstructionMetadataRequest{
		Options: objectsMap{
			"sender": testscommon.TestAddressAlice, "receiver": testscommon.TestAddressBob,
			"amount": "not-a-number", "currencySymbol": "UNKNOWN-abcdef",
		},
	})
	require.Nil(t, errTyped)
	meta, _ := newConstructionMetadata(response.Metadata)
	require.Equal(t, "ESDTTransfer@554e4b4e4f574e2d616263646566@", string(meta.Data))
}

func TestRepro_ParseReportsContractCallAsPlainTransfer(t *testing.T) {
	provider := testscommon.NewNetworkProviderMock()
	// data is base64("MultiESDTNFTTransfer@abc")
	tx := `{"nonce":1,"value":"1000","receiver":"` + testscommon.TestAddressOfContract + `","sender":"` + testscommon.TestAddressAlice +
		`","gasPrice":1000000000,"gasLimit":60000000,"data":"TXVsdGlFU0RUTkZUVHJhbnNmZXJAYWJj","chainID":"T","version":1}`
	response, errTyped := NewConstructionService(provider).ConstructionParse(context.Background(), &types.ConstructionParseRequest{Transaction: tx})
	require.Nil(t, errTyped)
	require.Equal(t, opTransfer, response.Operations[0].Type) // -1000 from sender, +1000 to the contract
}
```

### A4 — `package main` (`cmd/rosetta`)

```go
func TestRepro_GasLimitCustomTransferFlag(t *testing.T) {
	run := func(extra ...string) parsedCliFlags {
		var parsed parsedCliFlags
		app := cli.NewApp()
		app.Flags = getAllCliFlags()
		app.Action = func(ctx *cli.Context) error { parsed = getParsedCliFlags(ctx); return nil }
		args := append([]string{"rosetta", "--network-id=D", "--network-name=devnet",
			"--first-historical-epoch=1", "--num-historical-epochs=2"}, extra...)
		require.NoError(t, app.Run(args))
		return parsed
	}
	require.Equal(t, uint64(250000), run("--gas-limit-custom-transfer=250000").gasLimitCustomTransfer)
	require.Equal(t, uint64(0), run("--gas-limit-custom-transfer=1000000").gasLimitCustomTransfer) // the bug
}
```

### A2, A3, A6, A7 — over HTTP

```sh
cd cmd/rosetta && go build . && ./rosetta --offline --network-id=D --network-name=devnet \
  --native-currency=XeGLD --first-historical-epoch=1 --num-historical-epochs=1 --port=19091 &

NET='{"network_identifier":{"blockchain":"MultiversX","network":"devnet"}'

# A2: empty reply (curl exit 52) instead of the offline-mode error
curl -s -X POST localhost:19091/block -H 'content-type: application/json' \
  -d "$NET"',"block_identifier":{"index":5}}'; echo "exit $?"

# A3: empty reply (curl exit 52)
curl -s -X POST localhost:19091/construction/preprocess -H 'content-type: application/json' \
  -d "$NET"',"operations":[{"operation_identifier":{"index":0},"type":"Transfer"}]}'; echo "exit $?"

# A7: Access-Control-Allow-Origin: *
curl -si -X POST localhost:19091/network/list -H 'content-type: application/json' -d '{}' | grep -i access-control

# A6: a second instance on the same port logs "address already in use" and keeps running;
#     kill -TERM on the first gives exit code 143 with no "HTTP server stopped" line.
```
