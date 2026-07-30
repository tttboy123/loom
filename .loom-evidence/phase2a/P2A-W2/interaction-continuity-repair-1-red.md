# P2A-W2 Interaction Continuity Repair 1 RED

**Status**: `RED — THREE REVIEW FINDINGS REPRODUCED`

Only frozen owned test files changed before Repair 1 product implementation.

## Native preflight and filtering

```text
swift test --package-path apps/macos --filter InteractionContinuityTests
```

Exited `1` because `LocalProductWorkspaceState.filteredTasks(matching:)` and
`LocalProductPreflightReview` do not exist.

## TUI preflight and filtering

```text
go test ./internal/tui -run 'Test(TasksFilterWithoutReplacingCurrentSelection|TeamBuilderPreflightShowsBoundProviderModelAndLimits)$' -count=1
```

Exited `1`: `/` did not enter task-search mode, and the confirm surface omitted
the selected Coordinator, Codex, model, native auth, permission,
compatibility, budget and cost.

## Exact-remove replacement interleaving

```text
go test ./internal/localipc -run 'TestExactRemovePreservesReplacementInsertedAfterInitialCheck$' -count=1
```

Exited `1` because the frozen observed-remove boundary does not exist.

The RED used only copied Builder data and private temporary directories. It did
not access the real product lock, daemon, app, Keychain or Provider.
