# P2A-W2 Mission Decision Vertical Closure Deterministic GREEN

**Date**: 2026-07-30
**Baseline**: `5fd31a4`
**Verdict**: `GREEN`
**Live effect**: locked pending Implementation Review

## RED to GREEN

After reviewed Contract commit `5fd31a4`, the real production client test,
erased through `LocalProductClientProtocol` exactly as the native Store does,
failed:

```text
testRealClientAdvertisesDecisionProtocolForNativeStoreComposition
XCTAssertNotNil failed
```

The only production behavior change is:

```swift
LocalIPCClient:
    LocalProductClientProtocol,
    LocalProductDecisionClientProtocol,
    LocalProductSetupClientProtocol
```

The same focused test then passed. Existing strict
`readMissionDecision`/`decideMission` implementations are unchanged.

## Swift matrix

All output used attempt-local scratch paths:

```text
focused real-client test     PASS
Swift debug                  PASS
  XCTest                     51 executed, 1 expected visual-audit skip
  Swift Testing               4 passed
Swift Thread Sanitizer       PASS
  XCTest                     51 executed, 1 expected visual-audit skip
  Swift Testing               4 passed
Swift Release build          PASS
```

Strict decision decoding tests still reject unknown fields and null
collections. Store tests preserve presentation-only `Not now` and unprepared
Review fail-closed behavior.

## Go and cross-language matrix

After reviewed cache quarantine, every Swift probe build was routed through the
controlled `SWIFTPM_BUILD_DIR`:

```text
real Go IPC -> strict Swift prepared decision    PASS
go test -count=1 ./...                           PASS
go test -count=1 -race ./...                     PASS
go vet ./...                                     PASS
```

Focused gates passed for:

- exact Rules authority input;
- mislabeled Authorization context rejection;
- stale view and stale generation rejection;
- concurrent submission single winner;
- replay rejection after view advance;
- exact Review acceptance input and unbound digest rejection;
- exact Recovery input and unbound decision/claim rejection;
- fail-closed production decision registry; and
- controlled Mission fixture loading.

## Scope

The complete product diff is two files:

```text
LocalIPCClient.swift
  one conformance line

LocalIPCClientTests.swift
  one real production-composition regression test
  one private-socket test helper
```

Current SHA-256:

```text
LocalIPCClient.swift
ae1e82e86a369f64173186474434ba65c7a2055ab21123731be2b372ea0bc762

LocalIPCClientTests.swift
928baa71f396dbff2052224b88af8b17807fe2d3cdd0d59cbc7b8c4a61a28fb2
```

`git diff --check` passes. Repository `apps/macos/.build` is absent. No live
process or Attempt 003 state exists.

Fresh independent Cache Repair Result Review and Implementation Review remain
required before live.
