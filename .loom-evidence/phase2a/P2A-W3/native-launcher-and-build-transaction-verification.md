# P2A-W3 Native Launcher and Build Transaction Verification

**Date**: 2026-08-02  
**Baseline**: `848f068cbc0307f14473f6a71961db949a8734ca`  
**Branch**: `codex/loom-platform-slice2`  
**Verdict**: `PASS / IMPLEMENTATION REVIEW PENDING`  
**Live action**: none

## Causal and focused gates

- mandatory launcher/build causal RED: PASS as a causal failure, recorded in
  `native-launcher-and-build-transaction-red.md`;
- final fail-closed attribution Repair RED: PASS as three causal failures,
  recorded in `native-launcher-and-build-transaction-repair-red.md`;
- `go test -count=1 ./internal/provider ./cmd/loomd`: PASS when rerun alone;
- focused normal/race run in parallel produced one invalid overloaded normal
  fixture result while the race command passed; that result is not counted;
- `go test -count=1 -race ./internal/provider ./cmd/loomd`: PASS;
- frozen launcher/reason regressions at `-count=20`: PASS;
- all three final attribution regression tests: PASS;
- setup-only MiniMax real-product/IPC proof: PASS once and at `-count=20`;
- existing production decision real-IPC race proof: PASS at `-count=20` after
  increasing only its test client budget from one to five seconds.

## Setup-only MiniMax construction gate

`TestProductDaemonSetupOnlyMiniMaxTestOverRealIPCDoesNotInitializeExecution`
passed through the real product runner, setup service, Go Unix socket, framed
client, Credential Broker and StateWriter with `Execution: nil` and no three
local-model inputs. One strict credential Test produced one Journal-backed
next-revision `rejected/unavailable` terminal through a deterministic missing
credential-store read and made zero Provider requests. The execution bundle
remained nil, the execution directory never existed, execution facts remained
zero, and the socket was removed on shutdown.

## Source-locked component gate

```text
LOOM_P2A_W3_LOCKED_MODEL_ROOT="/Users/lune/Library/Application Support/Loom/phase1-live" \
go test -count=1 ./cmd/loomd \
  -run TestProductDaemonCopiedRuntimeStateObservesPi0821WithoutRewrite
```

PASS. The gate validated exact regular private components:

- model size `1117320768` and SHA-256
  `cc324af070c2ecbfd324a30884d2f951a7ff756aba85cb811a6ec436933bb046`;
- llama server SHA-256
  `a4998768a70ba2be02617ec9d8773accc2952516f4f5a8f38f621ece54cbf04b`.

It crossed the production execution-enabled builder, real Go IPC snapshot and
zero-write saved-Team preflight. The retained six-fact SQLite remained
byte-identical. It did not start Pi, llama-server, Provider, native UI, Keychain
operation or a live canary.

## Complete Go gates

- `go test -count=1 -p 1 ./...`: PASS;
- `go test -count=1 -race -p 1 ./...`: PASS;
- `go vet ./...`: PASS;
- `go mod tidy -diff`: PASS, empty diff;
- `gofmt -l` over the six owned files: PASS, empty output;
- `git diff --check` over owned source, W3 evidence and CURRENT: PASS.

## Swift/platform gates

- `/usr/bin/swift test --package-path apps/macos --quiet`: PASS, 60 XCTest
  cases with one intentional visual-export skip, plus four Swift Testing cases;
- `/usr/bin/swift test --package-path apps/macos --sanitize thread --quiet`:
  PASS with the same intentional visual-export skip;
- `/usr/bin/swift build --package-path apps/macos -c release --arch arm64 --quiet`:
  PASS.

## Scope, authority and non-disclosure

- production/test drift after the previous immutable source lock is confined
  to the six files owned by the reviewed vertical contract;
- previously locked Credential, Swift store/contract, IPC protocol, StateWriter,
  Projection, Supervisor, Bridge and retained SQLite hashes are unchanged;
- no known API-key prefix pattern occurs in the reviewed source/evidence scope;
- long opaque matches are only the reviewed compressed SQLite fixture and
  existing deterministic test sentinels, not bearer credentials;
- no new Journal, StateWriter, Projection, Scheduler, schema, WorkItem or W4;
- no staging, commit, live manifest, retry or walkthrough occurred.

The next gate is an immutable implementation source lock followed by a fresh
independent read-only Implementation Review. Live remains locked until PASS.
