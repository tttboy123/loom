# P2A-W2 Mission Decision Vertical Closure RED

**Date**: 2026-07-30
**Contract commit**: `5fd31a4`
**Verdict**: `RED — expected failure reproduced`

## Test added before production change

`apps/macos/Tests/LoomLocalAppTests/LocalIPCClientTests.swift` now includes:

```text
testRealClientAdvertisesDecisionProtocolForNativeStoreComposition
```

The test creates a private owned Unix socket accepted by the real
`LocalIPCClient` initializer and asserts that the concrete production client is
recoverable as `LocalProductDecisionClientProtocol`, matching the way
`LocalProductStore` obtains its decision client.

## Command

```text
/usr/bin/swift test --package-path apps/macos \
  --scratch-path "/Users/lune/Library/Application Support/Loom/p2a-w2-mission-workbench-decision-client-reopen-001/swiftpm-scratch-red-reviewed" \
  --filter "LocalIPCClientTests/testRealClientAdvertisesDecisionProtocolForNativeStoreComposition"
```

## Expected failure

After Contract Review `PASS` was committed, the Controller removed the
speculative uncommitted conformance, rebuilt from a fresh attempt-local scratch
path and reproduced:

```text
Executed 1 test, with 1 failure
XCTAssertNotNil failed
```

This proves the frozen production client does not advertise
`LocalProductDecisionClientProtocol` even though it implements
`readMissionDecision` and `decideMission`.

## Cache boundary

SwiftPM output was written to the attempt-local scratch path. The repository
path `apps/macos/.build` remained absent after the RED run.
