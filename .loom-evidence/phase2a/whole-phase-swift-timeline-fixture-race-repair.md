# Phase 2A Whole-Phase Swift Timeline Fixture Race Repair

**Date**: 2026-08-03  
**Boundary**: test-only repair inside the existing P2A-W3/Whole-Phase gate  
**New WorkItem**: none

## Fresh RED

The controller ran:

```text
swift test --sanitize=thread --package-path apps/macos
```

The suite's functional assertions passed, but Thread Sanitizer returned exit
status `1` with two warnings in
`SequencedSuspendedTimelineStubClient`. The reported accesses were concurrent
mutation/read of its continuation array from `timeline`,
`waitForRequestCount`, and `resolveRequest`.

This is a race in the newly accepted W3 test fixture, not evidence that the
product authority or production client has a data race. It nevertheless makes
the required Whole-Phase race matrix fail and must not be waived.

## Scoped repair

Both suspended timeline test clients are isolated as Swift actors. Test-side
continuation resolution crosses the actor boundary explicitly with `await`.
No production source, IPC schema, Journal fact, Projection, writer, Provider,
Runtime, Grant, Evidence, scheduler, or live allowance is changed.

## Required GREEN

- focused stale-load fencing tests pass;
- normal Swift suite passes;
- full Thread Sanitizer suite exits `0` with no sanitizer warning;
- full Phase 2A matrix and fresh independent Whole-Phase Review still pass.

## Fresh GREEN

```text
swift test --package-path apps/macos \
  --filter LocalProductStoreTests/testNewerLoadForSameSelectionFencesOlderSuccessAndError
PASS — 1 test, 0 failures

swift test --sanitize=thread --package-path apps/macos
PASS — 75 XCTest, 1 governed visual-only skip, 0 failures;
       4 Swift Testing checks; 0 Thread Sanitizer warnings

swift test --package-path apps/macos
PASS — 75 XCTest, 1 governed visual-only skip, 0 failures;
       4 Swift Testing checks
```

The repair changes only
`apps/macos/Tests/LoomLocalAppTests/LocalProductStoreTests.swift`: two private
test clients became actors and six test-side resolver calls now cross the actor
boundary with `await`.
