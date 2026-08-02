# P2A-W3 Acceptance/Recovery Repair 4 Verification 2

**Date**: 2026-08-03  
**Baseline commit**: `848f068cbc0307f14473f6a71961db949a8734ca`  
**Status**: `PASS — final Repair 4 candidate eligible for independent re-review`

## Repair result

Team replay now rejects a second `TeamNodeAttemptScheduled` for an already
recorded logical node/attempt and rejects every `TeamExecutionTerminal` after
the Team has already entered a terminal status. Corrupt Team history is exposed
to `ScheduleTeamNodeRecovery` as `ErrTeamExecutionConflict`, not reclassified as
a valid recovery input error.

This closes the duplicate-fact gap at the shared replay boundary while keeping
legitimate later execution of a newly scheduled retry intact. The exact
recovery matcher still verifies the recovery transaction prefix byte-for-byte;
full replay now guarantees that the prefix cannot be followed by a duplicate
scheduled-attempt or terminal fact.

## Focused proof

```text
go test -count=1 ./internal/work -run '^TestTeamRecoveryExactReplayRejectsDuplicate(ScheduledAttempt|TeamTerminal)$'
PASS

go test -count=1 ./internal/work
PASS

go test -count=1 -race ./internal/work -run '^(TestTeamRecoveryIsExplicitTimeBoundedAndStopsAtMaxAttempts|TestTeamTerminalRecoveryIsTerminalOnceAndExactRetryIsIdempotent|TestTeamRecoveryExactReplayRejects.*)$'
PASS
```

## Full candidate matrix

```text
go test -count=1 -p=1 ./...
PASS

go test -count=1 -race -p=1 ./...
PASS

go vet ./...
PASS

swift test --package-path apps/macos
PASS — 60 XCTest, 1 explicitly visual-only skip, 4 Swift Testing cases

swift build -c release --package-path apps/macos
PASS

git diff --check -- <nine source-locked files and Repair 4 evidence>
PASS
```

No Event/IPC/Swift schema, authority topology or scope changed. No product
process, live lineage, Provider, staging or commit was used.
