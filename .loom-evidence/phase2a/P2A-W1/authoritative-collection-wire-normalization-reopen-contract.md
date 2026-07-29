# P2A-W1 Authoritative Collection Wire Normalization Reopen Contract

**Date**: 2026-07-29  
**Status**: `CLOSED - FRESH INDEPENDENT IMPLEMENTATION REVIEW PASS`  
**Parent**: P2A-W1 Local App Shell and Read Experience  
**WorkItem count**: unchanged; this is not P2A-W4  
**Live allowance**: `0`  
**P2A-W2**: `LOCKED`

## 1. Evidence-led reason

The accepted non-live Runtime collection repair is present at the shared
`LocalProductReadService` boundary, but the closure proof is incomplete:

1. `cloneLocalProductSlice` still preserves nil for the five top-level
   snapshot collections, so collection normalization is not one uniform
   authoritative snapshot invariant;
2. timeline normalization exists for `records`, `board.nodes`, and
   `attention`, but is not bound together with snapshot normalization by a
   single wire-contract test;
3. the existing Go-server/Swift-client component fixture uses a hard-coded
   handler result, so it does not prove that a historical
   `RuntimeInstanceDiscovered` Event containing `"model_ids":null` crosses the
   real `LocalProductReadService`, production product handler, Go IPC server,
   and strict Swift client as `"model_ids":[]`.

The previously consumed live result remains
`FAIL - ROLLBACK_INCOMPLETE - HUMAN_REQUIRED - NO RETRY`. This reopen grants no
installation, launchd, native-window, Provider, Runtime, network, or live
canary authority.

## 2. Frozen behavior

Every required JSON collection emitted by the authoritative local-product
snapshot and timeline APIs must be an array, never `null`:

- snapshot: `runtimes`, each Runtime's `model_ids` and
  `observed_capabilities`, `teams`, `runs`, `evidence`, and `attention`;
- timeline: `records`, `board.nodes`, and `attention`.

Nil source collections become fresh zero-length slices. Non-empty values and
order remain byte-semantically unchanged. Returned snapshots must not alias
projection or stale-view storage. Schema version and field presence are
unchanged; `omitempty` is forbidden.

The Swift decoder remains strict. It must continue rejecting null, missing,
duplicate, unknown, and wrong-type required fields as `invalid_response`.

## 3. Exact owned files

This reopen may modify only:

```text
internal/api/local_product_read.go
internal/api/local_product_read_test.go
cmd/loomd/product_daemon_test.go
docs/CURRENT.md
```

It may create only:

```text
.loom-evidence/phase2a/P2A-W1/authoritative-collection-wire-normalization-reopen-contract.md
.loom-evidence/phase2a/P2A-W1/authoritative-collection-wire-normalization-reopen-contract-review.md
.loom-evidence/phase2a/P2A-W1/authoritative-collection-wire-normalization-red.md
.loom-evidence/phase2a/P2A-W1/authoritative-collection-wire-normalization-green.md
.loom-evidence/phase2a/P2A-W1/authoritative-collection-wire-normalization-implementation-review.md
```

The following inputs are read-only and must not change:

```text
internal/localipc/server.go
internal/localipc/protocol.go
internal/localipc/swift_contract_test.go
apps/macos/Sources/LoomLocalAppCore/LocalIPCClient.swift
apps/macos/Sources/LoomLocalAppCore/LocalProductModels.swift
apps/macos/Sources/LoomLocalAppContractProbe/main.swift
```

Journal, Projection, StateWriter, database schema, IPC envelope, Go module
lock, installed files, plist, resident service, credentials, and every other
dirty-worktree path are outside ownership.

## 4. Mandatory test-first evidence

Before changing production source:

1. add a focused Go API regression that demonstrates the common snapshot
   clone/wire boundary still emits `null` for nil top-level collections;
2. preserve the already-green historical Event to Projection to service
   Runtime-array tests as controls;
3. add one real cross-language fixture in `cmd/loomd` that creates a private
   SQLite Journal containing the exact historical `"model_ids":null` Event,
   starts the real `LocalProductReadService` through the production product
   handler and real `localipc.Server`, invokes the compiled
   `LoomLocalAppContractProbe`, and verifies strict Swift decodes both snapshot
   and timeline successfully;
4. assert the actual Go IPC snapshot wire contains `"model_ids":[]`, never
   `"model_ids":null`, and that every frozen snapshot/timeline collection is
   an array;
5. rerun the existing malformed Swift response suite to prove the strict
   decoder still rejects null and malformed collections.

The focused snapshot regression must be observed failing before the minimal Go
repair. The full-chain fixture is expected to exercise the previously repaired
Runtime path; if it passes immediately, that is honest component evidence, not
a fabricated RED. Any failure outside the frozen nil-to-array or fixture proof
stops for review.

## 5. Minimal implementation

The implementation may only make the shared Go snapshot clone/wire helper
canonicalize nil collections to fresh empty slices and, if required by a
genuine focused failure, consolidate equivalent timeline normalization without
changing its public shape.

It must not:

- modify or relax Swift decoding;
- add a tolerant decoding fallback;
- change Event payloads or append Events;
- introduce a second read model or wire schema;
- change IPC framing, peer authorization, daemon lifecycle, retry, or timeout;
- manufacture model IDs or capabilities;
- add a command, W4, live allowance, or Candidate/live transaction.

## 6. Deterministic verification

All verification is localhost/private-fixture only:

```text
gofmt on owned Go files
go test ./internal/api -run 'TestLocalProduct.*Collection' -count=1
go test ./cmd/loomd -run 'TestProductDaemon.*Swift' -count=1
go test ./internal/localipc -run 'TestSwiftClient(InteroperatesWithRealGoServer|RejectsMalformedLoopbackResponses)' -count=1
go test ./internal/api ./internal/localipc ./cmd/loomd
go test -race ./internal/api ./internal/localipc ./cmd/loomd
go test ./...
go test -race ./...
go vet ./...
go mod tidy -diff
go mod verify
cd apps/macos && swift test
cd apps/macos && swift build -c release
git diff --check
owned-file, Swift-read-only, authority, credential, and empty-staging audits
```

No command may connect to a Provider, execute a Runtime, mutate the resident
Journal, install an App/binary, call launchctl, open a native window, or perform
a live canary.

## 7. Review and exit

A fresh independent Contract Reviewer must return `PASS` before test/source
work. A fresh independent Implementation Reviewer must confirm exact
contract-to-test traceability, genuine RED/GREEN evidence, real full-chain
fixture provenance, unchanged strict Swift source, full deterministic matrix,
and zero live activity.

Implementation Review `PASS` closes only this non-live P2A-W1 reopen. It does
not accept P2A-W1, unlock P2A-W2, restore a retry, or authorize any live canary.
