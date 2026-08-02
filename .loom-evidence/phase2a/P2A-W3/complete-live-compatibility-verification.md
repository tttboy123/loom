# P2A-W3 Complete Live Compatibility Verification

**Date**: 2026-08-02  
**Baseline**: `848f068cbc0307f14473f6a71961db949a8734ca`  
**Contract**: `8e7d77ef06c9867799d5dd759db6c826d77a76a7a2960b17ad55ba35946d5f75`  
**Status**: `DETERMINISTIC PASS / IMPLEMENTATION REVIEW PENDING / LIVE LOCKED`

## Closed behavior

- Pi discovery accepts only the exact qualified identity
  `loom-local/qwen2.5-coder-1.5b-instruct-q4-k-m`, derives the exact adapter
  model, and rejects aliases, empty components, extra slashes and multiple
  models.
- Only typed Pi version/list-model metadata timeouts are contained. Product IPC
  remains responsive, performs no hidden retry and closes cleanly on controlled
  cancellation. Every leaf in a joined or wrapped failure must be that exact
  typed timeout or one of the two known structural observation/discovery
  wrappers. The real Pi process component fixture proves `--list-models` times
  out once while UDS ping, snapshot and setup/Codex reads remain available.
  The existing product snapshot becomes `partial = true` with exact reason
  `observer_models_timeout`, while daemon remains `serving_request`, Journal
  remains `available` and Projection remains `current`; this diagnostic is
  process-local and writes no fact. A real schema v2 Go UDS -> strict Swift
  client fixture proves this snapshot decodes without weakening the Swift
  closed health enum.
  Mixed metadata binding, process, cleanup, cancellation and unknown chains
  remain fatal.
- Every Swift MiniMax Test emits a fresh lowercase UUID v4 `operation_id`.
  Missing, malformed, duplicate and unknown method parameters fail closed.
  Same-operation recovery binds the latest immutable Journal idempotency key;
  a different stale operation conflicts without another Provider call or write.
- The real Swift client -> Go UDS server -> LocalProductSetupService ->
  CredentialBroker -> SQLite Journal fixture proves one Provider observation,
  revision `1 -> 2`, one `ProviderCredentialVerified` fact and no secret bytes
  in the payload.
- Native presentation exposes `Testing` and closed
  `Verified|Rejected|Unavailable|Conflict`, disables Test in flight, and accepts
  success only when the returned result and refreshed authoritative setup
  snapshot match exactly.
- Saved Team summaries resolve the human TeamDefinition name from the same
  GlobalReadView and fall back to `Saved team` on inconsistent metadata.
- The deterministic TUI KeyRunes path already preserves objective spaces; the
  reported collapse remains classified as a PTY/computer-input artifact and no
  speculative TUI production change was made.

## Deterministic matrix

All commands ran against the exact hashes in
`complete-live-compatibility-source-lock.json`:

```text
focused Go tests for Pi, observer, credential, display and operation identity PASS
real Pi --list-models timeout -> private UDS availability (5 consecutive)     PASS
focused Go race tests for observer and credential concurrency                 PASS
real Go UDS + strict Swift fixtures, including partial Runtime health          PASS
go test ./internal/api ./internal/app ./internal/credentials
        ./internal/localipc ./internal/tui ./cmd/loomd -count=1               PASS
go test -race on the same package set -count=1                                PASS
go test -timeout=8m -p=1 -count=1 ./...                                       PASS
go test -race -timeout=12m -p=1 -count=1 ./...                                PASS
go vet ./...                                                                  PASS
go mod verify                                                                 PASS
swift test --package-path apps/macos                                          PASS (60 tests; 1 visual-export skip)
swift test --sanitize=thread --package-path apps/macos                        PASS (60 tests; 1 visual-export skip)
swift build --package-path apps/macos -c release --product LoomLocalApp       PASS
git diff --check                                                              PASS
scope, protocol, authority, legacy-binding and secret scans                    PASS
```

The one Swift skip is the pre-existing opt-in visual preview export. It is not
a functional, security, concurrency or contract test.

Two deterministic-harness incidents were resolved without weakening a gate:

- The first post-repair package-set run observed the older engineering demo
  one-shot report one incomplete competing coordinator. Its focused test then
  passed five consecutive runs, and the subsequent package, whole-repository
  non-race and race matrices all passed. No product change was made for that
  non-reproducing result.
- A later package-set invocation reached Go's ten-minute default timeout while
  compiling a second copy of the real Swift setup probe. The new credential
  assertion was refactored to reuse the already-built setup probe. The focused
  real IPC fixture, package sets and both whole-repository matrices then passed.
  This removed redundant test compilation and did not change production code
  or any acceptance assertion.

## Locked boundaries

The generic local IPC protocol, credential StateWriter, credential Projection,
Supervisor managed execution and `loom.bridge.v1` frame hashes remain exactly
locked. No new Event type, Event field, StateWriter, Projection, Scheduler,
daemon, queue, Grant, Evidence or bridge protocol was introduced. Raw
credentials and authorization material were absent from all reopened source
and evidence scans.

## Gate

No daemon, Provider, credential action, Pi process or live canary was started
by this verification. The three replacement live lineages remain locked until
a fresh independent Implementation Reviewer returns PASS against the exact
source-lock digest.
