# P2A-W2 Isolated Pi Catalog Implementation Verification

**Date**: 2026-07-30
**Status**: PASS — awaiting fresh independent Implementation Review

## Implemented boundary

- The Pi metadata Runner optionally binds the already accepted exact
  llama-server/GGUF identities and revalidates them before every call.
- Each disposable `PI_CODING_AGENT_DIR` receives one atomic regular `0600`
  `models.json`.
- Metadata discovery and Pi RPC now share one serializer for the exact
  `loom-local/qwen2.5-coder-1.5b-instruct-q4-k-m` declaration.
- The probe factory, Runtime daemon and daemon CLI carry the catalog as one
  all-or-none private-root/executable/model tuple.
- Catalog discovery starts no model server, makes no network request and reads
  no user Pi home, credential or session state.
- Team Builder remains downstream of authoritative Runtime discovery; it does
  not synthesize model IDs.

## RED

The preserved RED evidence is
`isolated-pi-catalog-red.md`. Focused compilation failed only because the
catalog, Runner field/seam and daemon configuration did not yet exist.

## Deterministic verification

All commands passed:

```text
go test ./internal/runtime/piadapter ./cmd/loomd \
  -run 'TestPiLocalModelCatalog|TestPiMetadataProcessRunnerMaterializesBoundLocalCatalog|TestRunAcceptsLocalModelCatalogOnlyAsCompleteTuple' \
  -count=1

go test ./internal/runtime/piadapter \
  -run 'TestPiLocalModelCatalog|TestPiMetadataProcessRunnerMaterializesBoundLocalCatalog|TestPiLocalRuntimeProbeFactoryCarriesCatalogThroughRealParser' \
  -count=1

go test -race ./internal/runtime/piadapter ./internal/app ./cmd/loomd \
  -run 'TestPiLocalModelCatalog|TestPiMetadataProcessRunnerMaterializesBoundLocalCatalog|TestPiLocalRuntimeProbeFactoryCarriesCatalogThroughRealParser|TestLocalRuntimeObservationDaemonRealSQLiteRestart|TestProductDaemonRealSetupServiceConfirmsCandidateOverPrivateUDS|TestRunAcceptsLocalModelCatalogOnlyAsCompleteTuple' \
  -count=5

go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go mod tidy -diff
go mod verify
git diff --check
gofmt -l <owned Go files>

swift test --package-path apps/macos
swift build -c release --package-path apps/macos
swift test --sanitize=thread --package-path apps/macos
```

The Swift debug and thread-sanitizer matrices each executed 31 XCTest cases
with one expected visual-export skip plus four Swift Testing cases. The release
build passed.

## Layered vertical proof

The new deterministic tests prove the new portion of the vertical boundary:

1. catalog bytes are identical to the accepted RPC catalog and materialize as
   regular `0600`;
2. a real process fixture sees that file only in its fresh agent directory;
3. the configured probe factory carries it through the real Pi metadata parser
   to the exact canonical model ID;
4. existing daemon tests carry parsed Runtime models through authoritative
   Events and Projection;
5. existing real product-daemon IPC tests reconstruct non-empty Runtime model
   data and complete a compatible TeamDefinition-only Builder journey;
6. CLI tests reject every partial tuple before the builder is called.

Post-bind model mode drift fails before process launch. The existing no-catalog
daemon and strict JSON-array/Swift-decoder matrices remain green.

## Pre-review Repair 1

Controller audit before independent review found that the initial factory
validated the catalog but retained only its paths; a same-digest replacement
before `BuildProbe` could therefore become a new accepted binding. The review
was stopped before it produced a verdict.

Repair 1 preserves the original `piLocalModelCatalog` binding in the factory and
passes that exact binding to every Runner. Runner construction and every
metadata call revalidate file identity, mode, owner, size and digest. The
regression RED first returned a non-nil probe with `found=true`; after repair it
fails closed with no probe and
`ErrPiLocalRuntimeProbeConstructionFailed`.

The post-repair five-run focused race matrix passed. A first package-parallel
`go test ./...` rerun then showed three process-fixture failures while
`cmd/loomd` took about five times its normal duration: daemon recovery failed
one Pi version call and two pre-existing local-model server cases hit cleanup or
early-exit timing failures. No new catalog test failed. After load subsided:

```text
go test ./internal/app \
  -run '^TestLocalRuntimeObservationDaemonFailureThenRestartRecovery$' \
  -count=10

go test ./internal/runtime/piadapter \
  -run '^(TestPiLocalModelServerHealthAndCleanup|TestPiLocalModelServerFailsClosed)$' \
  -count=10
```

both passed. The final deterministic rerun used the repository's supported
serial package gate for process/port fixtures:

```text
go test -p 1 ./... -count=1
go test -race -p 1 ./... -count=1
```

Both passed, followed by fresh vet, module, format/diff and all three Swift
matrices. The transient parallel run is retained as evidence and is not
represented as a product pass.

## Safety and non-actions

Candidate-owned source/evidence contains no pasted Provider key, bearer token or
private key material. No installed Pi, llama-server, local model process,
Codex, Keychain, MiniMax request, network connection, native app, product
daemon, resident observer mutation, Journal clone or live canary was used.
P2A-W3 remains locked and no P2A-W4 exists.
