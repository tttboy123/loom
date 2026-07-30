# P2A-W2 Interaction Continuity Exit Reopen Mandatory RED

**Baseline**: `65c719c`
**Status**: `RED — EXPECTED PRODUCT GAPS REPRODUCED`
**Scope**: test-only changes in the frozen owned boundary

## Preconditions

- The pure governance checkpoint was committed before any product edit.
- The real product socket and lock were not opened, removed, replaced or
  changed.
- No daemon, installed app, Keychain, Provider, network or resident service
  action occurred.
- The first local IPC attempt had a test-only fixture authoring error
  (`undefined: newFixture`). That attempt is discarded. The fixture was
  corrected without changing product code and the command was rerun.

## Test-only changes

- `internal/localipc/socket_test.go`
- `internal/localipc/server_test.go`
- `internal/tui/model_test.go`
- `apps/macos/Tests/LoomLocalAppTests/InteractionContinuityTests.swift`

## Valid RED evidence

### Product socket/lock transaction

```text
go test ./internal/localipc -run 'Test(PrepareSocketReclaimsAbandonedOwnedLockWithMissingSocket|ServerRemovesLockPathBeforeReleasingAdvisoryOwnership)$' -count=1
```

Exited `1`:

```text
TestServerRemovesLockPathBeforeReleasingAdvisoryOwnership:
active server lock had no advisory ownership

TestPrepareSocketReclaimsAbandonedOwnedLockWithMissingSocket:
prepareSocket(abandoned owned lock) error = invalid local IPC socket path
```

### Task-first TUI

```text
go test ./internal/tui -run 'TestInteractionContinuityStartsWithTasksInsteadOfHome$' -count=1
```

Exited `1` because the frozen `ScreenTasks` product symbol does not exist.

### Native interaction continuity

```text
swift test --package-path apps/macos --filter InteractionContinuityTests
```

Exited `1` because the frozen `LocalProductWorkspaceState`,
`LocalProductWorkspaceTask`, inspector cases and
`LocalProductInteractionCopy` product symbols do not exist.

## Gate result

The failures are causal product gaps and not environmental failures. Product
implementation is unlocked only inside the complete frozen owned boundary.
No live action is unlocked by this RED.
