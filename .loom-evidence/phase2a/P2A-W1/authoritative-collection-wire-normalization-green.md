# P2A-W1 Authoritative Collection Wire Normalization GREEN

**Date**: 2026-07-29  
**Result**: `GREEN - COMPLETE NON-LIVE MATRIX PASS`  
**Live allowance**: `0`  
**Live action**: none

## Minimal implementation

One shared Go helper now copies every requested collection into a fresh slice,
including zero-length nil input. The snapshot path uses that helper for:

- `runtimes`, `teams`, `runs`, `evidence`, and `attention`;
- every Runtime's `model_ids` and `observed_capabilities`.

The existing timeline boundary continues canonicalizing `records`,
`board.nodes`, and `attention`. Non-empty values and order are preserved.
Projection records, stale-view storage, Journal facts, schema version, and IPC
framing are unchanged.

Production Swift source was not edited. Its three read-only inputs reproduce
the prior accepted source-lock SHA-256 values exactly:

| Input | SHA-256 |
|---|---|
| `LocalIPCClient.swift` | `870d84ada398fbda6e351ea45f37e5b82d2844929c6d8c8bf6c1ffdebfa29474` |
| `LocalProductModels.swift` | `6a941f939a4293097a17f05b8f8caf4f7a93fc1675442bf62d6c52d85e1f8b39` |
| contract probe `main.swift` | `19bb6355e5fabab8358800ac7fa52aacdd0c590dfc60341a604b28d4cf292d0d` |

## Contract-to-test closure

The genuine RED showed nil top-level snapshot collections encoded as JSON
`null`. After the helper repair, the focused regression passes and every frozen
snapshot collection encodes as an array.

The new real component fixture starts with the exact historical
`RuntimeInstanceDiscovered` Event containing `"model_ids":null`, rebuilds the
Projection, calls `LocalProductReadService` through the production
`localProductHandler` and real Go `localipc.Server`, then invokes the compiled
`LoomLocalAppContractProbe`. The strict Swift `LocalIPCClient` decodes snapshot
and timeline successfully and observes non-nil empty collections.

The existing malformed-response control still rejects null, missing,
duplicate, unknown, and wrong-type required fields as `invalid_response`.

## Complete deterministic matrix

All commands were local and non-live:

```text
focused API nil-to-array regression
PASS

real LocalProductReadService -> Go UDS -> strict Swift client fixture
PASS

strict Swift malformed-response Go fixture
PASS

go test ./internal/api ./internal/localipc ./cmd/loomd
PASS

go test -race ./internal/api ./internal/localipc ./cmd/loomd
PASS

go test ./...
PASS

go test -race ./...
PASS

go vet ./...
PASS

GOPROXY=off GOTOOLCHAIN=local go mod tidy -diff
PASS - no output

GOPROXY=off GOTOOLCHAIN=local go mod verify
all modules verified

cd apps/macos && swift test
27 tests, 0 failures, 1 governed visual-export skip

cd apps/macos && swift build -c release
PASS

gofmt -d on owned Go files
PASS - no output

git diff --check
PASS

git diff --cached --name-only
PASS - empty staging
```

The managed sandbox prevents Unix socket readiness; the exact UDS fixtures
were therefore executed outside that socket sandbox only. They used private
`/private/tmp` roots and did not connect to the resident socket or service.

## Boundaries

No W4, Swift decoder change, tolerant fallback, Event mutation, Projection or
StateWriter change, module-lock change, install, launchctl, App launch,
Provider, Runtime execution, network access, credential access, Candidate,
retry, replacement transaction, or live canary occurred. The prior failed live
result remains unchanged and live allowance remains `0`.

Fresh independent Implementation Review is the current gate.
