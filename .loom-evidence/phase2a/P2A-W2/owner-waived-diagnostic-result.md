# P2A-W2 Owner-waived Diagnostic and Deterministic Test Result

**Date**: 2026-07-30
**Status**: TEST MATRIX PASS — LIVE PRODUCT PATH INCOMPATIBLE
**Parent**: `P2A-W2 Owner-waived Diagnostic Continuation`

## Product diagnostic result

The Product Owner explicitly waived the prior secret-source stop condition for
diagnostic testing. The existing attempt-002 app and daemon were started only
for the reviewed bounded diagnostic. The credential remained inside the
Credential Broker and OS Secret Store boundary; its value was not read,
repeated, logged, placed in evidence, or inspected in SQLite.

The native product reported:

- MiniMax: `verified`, auth mode `brokered`;
- Codex: `unsupported`, auth mode `native_auth`, reason `unknown_output`;
- Pi Runtime: `online`, adapter `pi-cli`, capacity `1`;
- Pi model IDs: empty collection;
- Team Builder role options: empty collection;
- blank Candidate start: `incompatible`.

The exact Codex 0.144.1 native status command exits successfully and emits the
reviewed complete line `Logged in using ChatGPT` on stderr while leaving stdout
empty. The W2 observer accepts that line only from stdout and rejects any
non-empty stderr, so the product status is a compatibility defect rather than
a missing OAuth login.

The Pi 0.82.1 metadata probe is deliberately executed with a fresh isolated
agent directory and no credential inheritance. Its authoritative Runtime
observation therefore contains `model_ids=[]`. Production setup catalog
construction requires at least one online positive-capacity Runtime observation
with a model ID before creating the two role options. The empty model catalog
therefore deterministically explains the Team Builder `incompatible` result.

## Authority and side-effect result

After the one user-authorized MiniMax `Test`, the isolated Journal contains
only:

```text
ProviderCredentialConfigured | 1
ProviderCredentialVerified   | 3
RuntimeInstanceDiscovered    | 1
```

There is no TeamDefinition, TeamInstance, AgentInstance, WorkItem, Run, Grant,
Evidence, dispatch, generation, archive, restore, or revocation Event.

The diagnostic Loom app and attempt daemon were stopped. The product socket is
absent. The separate resident Runtime observer remains running outside the
attempt. The attempt SQLite and configured Keychain item remain because the
Product Owner previously declined product-level revocation.

## Deterministic verification

The following sequential verification passed:

```text
go test ./...
go test -race ./...
go vet ./...
go mod tidy -diff
go mod verify
gofmt -l cmd internal protocol migrations
git diff --check
swift test
swift build -c release
swift test --sanitize=thread
```

Swift debug and thread-sanitizer runs each executed 27 XCTest cases with one
expected visual-audit-only skip and no failures, plus three Swift Testing cases
with no failures.

An initial intentionally parallel invocation of Go tests, Swift tests, and vet
caused one five-second Pi fixture process failure in
`TestLocalRuntimeObservationDaemonRealSQLiteRestart`. The isolated test then
passed ten consecutive runs, and the normal sequential complete Go suite and
complete race suite passed. This is recorded as test-runner resource contention,
not a reproduced product defect.

## Verdict

```text
Deterministic W2 test matrix = PASS
Owner-waived live diagnostic = COMPLETED
Live Team Builder journey    = INCOMPATIBLE
Original secret-negative gate = NOT REINTERPRETED
P2A-W2 product acceptance    = NOT CLAIMED
P2A-W3                       = LOCKED
```

Ignoring the prior Gate allowed the requested tests to complete. It does not
turn the two observed compatibility defects into passing product behavior.
