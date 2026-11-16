# Proposed improvements for mx-chain-rosetta

This file is the action plan for the findings in [AUDIT.md](AUDIT.md). Finding IDs (A1, B3, …) refer to that file. Nothing here has been applied; the code sketches show the intended shape of each change and have not been compiled against the repository.

## Recommended order

| Step | What | Resolves | Size |
|---|---|---|---|
| 1 | Confirm A1 on a localnet, then stop malformed events from failing a block | A1 | small |
| 2 | Fix the four wiring bugs: offline services, preprocess nil checks, gas-limit flag, genesis timestamp | A2, A3, A4, A5 | small |
| 3 | Fix startup and shutdown; give the HTTP server limits | A6, A7 | small |
| 4 | Add the tests that would have caught steps 1–3 | B4 | medium |
| 5 | Validate request input at the service boundary | A10, A11, A15 | medium |
| 6 | Make `/construction/parse` reflect `data` | A8 | medium, needs a decision |
| 7 | Repair the README, add a licence, tighten CI | B1, B2, B3, A9 | small |
| 8 | Performance, determinism, cleanup, dependency weight | A12 – A14, A16, B5 | medium, ongoing |

Steps 1 to 3 are each a few lines and can ship as one patch release. Step 4 should land with them, so the fixes are locked in.

## 1. Stop malformed events from failing a block (A1)

**First, confirm it.** Add a contract to `systemtests/contracts` that exports an endpoint named `SCDeploy` and emits an event with one topic. Call it on the localnet and run `check:data`. If the deployment is rejected, A1 is not exploitable today and what remains is hardening. If it is accepted, treat the fix as a security release.

**Then change the extractor** so that an `SCDeploy` event Rosetta has no use for is never parsed. The only consumer of these events requires `tx.Receiver` to be the system deploy address, so check that before looking at topics, and skip rather than fail on a bad shape:

```go
func (controller *transactionEventsController) extractEventSCDeploy(tx *transaction.ApiTransactionResult) ([]*eventSCDeploy, error) {
	if tx.Receiver != systemContractDeployAddress {
		// Not a direct deployment: such events are not used, and may be emitted by any contract.
		return nil, nil
	}

	rawEvents := controller.findManyEventsByIdentifier(tx, transactionEventSCDeploy)
	typedEvents := make([]*eventSCDeploy, 0, len(rawEvents))

	for _, event := range rawEvents {
		if len(event.Topics) < numTopicsOfEventSCDeployBeforeSirius {
			log.Warn("extractEventSCDeploy: skipping event with unexpected shape", "tx", tx.Hash, "numTopics", len(event.Topics))
			continue
		}
		// ... unchanged
	}

	return typedEvents, nil
}
```

**Write the rule down.** For each identifier Rosetta acts on, record in a comment why a contract cannot forge it (built-in function name, or VM-protected name). Any identifier without such a guarantee must be parsed tolerantly. Today that list has one entry, `SCDeploy`; the comment makes the next addition safe.

**Consider a general safety valve.** A flag such as `--skip-unrecognized-events` that downgrades `errCannotRecognizeEvent` to a warning would let an operator unblock a stalled shard without waiting for a release. The cost is that a skipped event may hide a balance change, so it should default to off and log loudly.

## 2. Fix the wiring bugs (A2, A3, A4, A5)

**A2 — offline services.** One line in `server/services/offline_services.go`:

```go
func NewOfflineService() *offlineService {
	return &offlineService{errFactory: newErrFactory()}
}
```

**A3 — preprocess.** Read the fallbacks through helpers that tolerate missing fields, in `server/services/constructionService.go`:

```go
func operationAddress(operations []*types.Operation, index int) (string, bool) {
	if index >= len(operations) || operations[index].Account == nil {
		return "", false
	}
	return operations[index].Account.Address, true
}

func operationAmount(operations []*types.Operation, index int) (*types.Amount, bool) {
	if index >= len(operations) || operations[index].Amount == nil || operations[index].Amount.Currency == nil {
		return nil, false
	}
	return operations[index].Amount, true
}
```

Use them in place of the four direct dereferences and return `ErrConstruction` ("cannot prepare sender", and so on) when they report false.

As a backstop for any panic that slips through later, wrap the router in a recovery middleware that answers with a Rosetta error instead of dropping the connection:

```go
func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Error("panic while serving request", "path", r.URL.Path, "panic", recovered, "stack", string(debug.Stack()))
				w.Header().Set("Content-Type", "application/json; charset=UTF-8")
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(&types.Error{Code: int32(services.ErrUnknown), Message: "unknown error"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}
```

**A4 — gas-limit flag.** In `cmd/rosetta/cli.go`, declare the flag with the type it is read as:

```go
cliFlagGasLimitCustomTransfer = cli.Uint64Flag{
	Name:  "gas-limit-custom-transfer",
	Usage: "Specifies the necessary gas limit for a custom transfer (for transaction construction).",
	Value: 200000,
}
```

**A5 — genesis timestamp.** In `cmd/rosetta/main.go`, add the missing field to the `ArgsCreateNetworkProvider` literal:

```go
GenesisTimestamp: cliFlags.genesisTimestamp,
```

This changes the genesis block's `timestamp` from 0 to the configured value. Mention it in the release notes, since consumers that stored the old response will see a difference.

## 3. Startup, shutdown and server limits (A6, A7)

Replace the tail of `startRosetta` so that a bind failure stops the process, SIGTERM is handled, and in-flight requests get time to finish:

```go
listener, err := net.Listen("tcp", httpServer.Addr)
if err != nil {
	return fmt.Errorf("cannot listen on %s: %w", httpServer.Addr, err)
}

ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()

serveErr := make(chan error, 1)
go func() { serveErr <- httpServer.Serve(listener) }()

select {
case err := <-serveErr:
	return fmt.Errorf("HTTP server stopped unexpectedly: %w", err)
case <-ctx.Done():
}

shutdownContext, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
_ = httpServer.Shutdown(shutdownContext)
_ = fileLogging.Close()
return nil
```

Give the server limits in `createHttpServer`:

```go
httpServer := &http.Server{
	Addr:              net.JoinHostPort(host, strconv.Itoa(port)),
	Handler:           recoverMiddleware(http.MaxBytesHandler(corsRouter, 1<<20)),
	ReadHeaderTimeout: 10 * time.Second,
	ReadTimeout:       30 * time.Second,
	WriteTimeout:      120 * time.Second, // above the 60s observer timeout
	IdleTimeout:       120 * time.Second,
}
```

Related changes, each a new flag with a default that keeps current behaviour unless noted:

- `--host` to choose the bind address (default: all interfaces, as today).
- `--cors-allowed-origin` to replace the unconditional `*`.
- `--pprof-address` so profiling listens separately, defaulting to `127.0.0.1:6060`. This changes behaviour for anyone using `--pprof` today.
- Change the default log level from `*:DEBUG` to `*:INFO`. This also changes behaviour; note it in the release.

Add a short "Deployment" section to the README saying that Rosetta has no authentication and should sit behind a reverse proxy or on a private network.

## 4. Tests that would have caught these (B4)

In order of value:

1. **HTTP-level tests with the real asserter.** Build the controllers through `factory.CreateControllers` over the mock provider, mount them on `httptest.NewServer`, and send JSON. A table of `{endpoint, body, expected error code}` for offline and online modes covers A2 and A3 and exercises `server/factory`, which is at 0%.
2. **A flag-mapping test.** Run the CLI app with every flag set to a non-default value and assert the resulting `ArgsCreateNetworkProvider` field by field. To make that possible, extract a pure function `toProviderArgs(parsedCliFlags) factory.ArgsCreateNetworkProvider` from `startRosetta`. This covers A4 and A5 and will catch the next forgotten field.
3. **Fuzz tests** for the code that parses untrusted input: `parseCustomCurrencyTransfer`, `parseInnerTxOfRelayedV1`, `parseTokenIdentifierIntoParts`, and `addOperationsGivenTransactionEvents` with arbitrary events. The property to assert is "no panic"; for events from non-protected identifiers, also "no error".
4. **Unit tests for the uncovered paths:** genesis block, block by hash, `ComputeReceiptHash`, `SendTransaction`, `GetMempoolTransactionByHash`.
5. **A signing-format test** (B5): build a transaction through `/construction/payloads` and compare the bytes with a fixture produced by `multiversx_sdk`. This turns a silent breakage on a proxy-go upgrade into a failing test.

Promote the reproductions in the audit appendix to permanent tests, inverting their assertions once the fixes are in.

## 5. Validate input at the service boundary (A10, A11, A15)

Add one small validation file in `server/services` and call it at the top of each handler, before anything reaches the provider:

| Input | Rule | Used by |
|---|---|---|
| Account address | decodes as bech32 with the expected prefix and length | `/account/balance`, `/construction/metadata`, `/construction/payloads` |
| Currency symbol | is the native symbol or a configured custom currency; decimals match | `/account/balance`, `/construction/metadata` |
| Block hash, transaction hash | exactly 64 hexadecimal characters | `/block`, `/account/balance`, `/mempool/transaction` |
| Amount | parses as a positive base-10 integer | `/construction/metadata`, `/construction/payloads` |
| Token identifier | matches `^[A-Z0-9]{3,10}-[0-9a-f]{6}(-[0-9a-f]+)?$` | provider URL builders |

Further points:

- In `/account/balance`, return `ErrInvalidAccountAddress` when `IsAddressObserved` is false, so a query for another shard fails clearly instead of depending on how the observer answers.
- Call `constructionOptions.validate()` in `/construction/metadata`, not only in `/preprocess`.
- Make `amountToHex` return an error instead of `""`.
- In the URL builders, apply `url.PathEscape` to each path segment as defence in depth.
- In `newCurrenciesProvider`, reject duplicate symbols, negative decimals, and symbols that fail the token-identifier pattern. In the CLI, reject `--observer-projected-shard` above 255.

## 6. Make `/construction/parse` reflect `data` (A8)

This one needs a product decision, because Rosetta has no standard way to express a contract call. Two options:

- **Strict (recommended).** `/construction/payloads` and `/construction/parse` reject any `data` that looks like a function call (contains `@`, or starts with a built-in function name) unless it is the single `ESDTTransfer` form Rosetta itself generates. Plain memos remain allowed. The construction API then only ever builds what `/parse` can describe.
- **Transparent.** Keep accepting arbitrary `data`, but return it in `ConstructionParseResponse.Metadata` and document that signers must inspect it.

The strict option closes the gap; the transparent one relies on every integrator reading the documentation. Either way, also check in `/parse` that the receiver of a custom transfer and the amount round-trip to what `/preprocess` was asked for.

## 7. Repository and CI (B1, B2, B3, A9)

**README.**

- Add `--first-historical-epoch` and `--num-historical-epochs` to both example commands.
- Change `cd rosetta/cmd/rosetta` to `cd mx-chain-rosetta/cmd/rosetta`.
- Remove the CodeQL badge or add the workflow.
- Update the `rosetta-api.org` links and the 2022 note.
- Add a table of all flags; it can be generated from `--help`.

**Licence and policy files.** Add `LICENSE` (the other MultiversX chain repositories use GPL-3.0; confirm with whoever owns licensing). Add `SECURITY.md` with a disclosure contact. A1 is the kind of report it exists for.

**Version (A9).** Stop maintaining the constant by hand. Set it at build time and fail the release if it disagrees with the tag:

```sh
go build -ldflags "-X github.com/multiversx/mx-chain-rosetta/version.RosettaMiddlewareVersion=$(git describe --tags --always)" .
```

If `mx-chain-rosetta-docker` builds the binary itself, the flag has to be added there too. Until then, bump the constant to `v0.8.2`.

**CI.**

- Add a `test.yml` that runs `go test -race ./...` on pull requests and on push to `main`.
- Add `.golangci.yml` enabling at least `errcheck`, `govet`, `staticcheck`, `ineffassign`, `gosec`, `bodyclose`, `revive`; turn off `only-new-issues` once the backlog is cleared.
- Add a `govulncheck ./...` job and a Dependabot configuration for `gomod` and `github-actions`. Run `govulncheck` locally first, since the audit could not.
- Pin every third-party action to a commit SHA.
- Replace `curl … | sh` from the `install-rename` branch with a download of a tagged release verified against a checksum. Those jobs hold wallet secrets.
- Use `actions/setup-go` with `go-version-file: go.mod` in every workflow that builds, including the mesh-cli ones.
- Delete the `Gopkg.toml`/`dep` step and replace `go get -v -t -d ./...` with `go mod download`.
- Add a job running `flake8` and `pyright` on `systemtests`; the configuration files already exist.
- Give the testnet check a schedule like mainnet and devnet, or state why it is manual.

**Developer convenience.** A `Makefile` with `build`, `test`, `lint` and `run-offline` targets would replace the commands scattered across the README and workflows.

## 8. Performance, determinism and cleanup

**Fewer observer calls (A14).**

- Cache the plain node status for about one round (a few hundred milliseconds to a second). Every `/block` request currently pays for a `/node/status` round trip.
- Cache the oldest-block summary per epoch; it changes once a day.
- Cache the node version for `/network/options`, which should not need four observer calls.
- Wrap `doGetBlockByNonce` in `singleflight`, so concurrent requests for neighbouring blocks do not fetch the same block several times.

**Bound the block cache by memory (A14).** Make the capacity a flag, and consider a size-aware cache (the `Put` call already takes a size argument, currently always 1). After the Supernova activation the previous and next blocks are no longer needed, so the capacity can drop sharply on those networks.

**Deterministic ordering (A12).** In `gatherEffectiveTransactions`, keep a slice of hashes in first-seen order next to the map and build the result from it.

**Mempool rendering (A13).** Either restrict `/mempool/transaction` to move-balance transactions and return `ErrCannotParsePoolTransaction` for others, or run pending transactions through the same classification as block transactions. Remove the dead `err` check.

**Context propagation.** Every service method discards its `context.Context`. Passing it down to the observer calls would let a disconnected client cancel its work. This needs support in `mx-chain-proxy-go`'s processors, so it is a longer-term item.

**Observability.** There are no metrics and no health endpoint. Useful additions: a `/health` route for supervisors (ties in with the bind fix in step 3), Prometheus counters for requests by endpoint and error code, a histogram of observer latency, and a gauge for the gap between the observer's head and the block Rosetta serves.

**Readability.**

- `addOperationsGivenTransactionEvents` is 230 lines of near-identical loops. A table of `{extractor, sign, operation type, address selector}` would cut it to a fraction and make adding an event one row.
- The six `extractEventsESDT*` functions differ only in identifier, topic count and which address they keep; one parameterised helper covers them.
- Keep one `NetworkProvider` interface. `factory` can embed the `services` one and add `LogDescription`.
- Remove the items listed in A16: the dead `err` check, the wrong format label, the typos, `fmt.Printf` tracing, the two ignored `--activation-epoch-*` flags (after a deprecation notice), and the unused error code.

**Dependency weight (B5).**

- `mx-chain-go` is imported for `sharding.NewMultiShardCoordinator` alone. Shard computation for a fixed shard count is about thirty lines; implementing it locally, with a test comparing results against the upstream coordinator, would remove the full node from the module graph, shorten builds and shrink the surface that vulnerability scans report on.
- Move `mx-chain-core-go` to a tagged release when one contains the needed commit.
- After running `govulncheck`, upgrade `golang.org/x/crypto`, `golang.org/x/net` and `gin` as it directs.

## Open questions for the maintainers

1. **A1:** is there a protocol-level reason a contract cannot export an endpoint named `SCDeploy` that the VM source reading missed? A localnet test settles it.
2. **A8:** strict or transparent handling of `data` in the construction API?
3. **A5:** is any consumer relying on the genesis block having `timestamp: 0`?
4. **B2:** which licence applies?
5. **Step 3:** are the two default changes (`INFO` logging, pprof on localhost) acceptable in a minor release, or do they need a major one?
