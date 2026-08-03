# P2B-W1 Parent Execution Binding Restart RED

Date: 2026-08-03  
Status: RED CAPTURED, THEN GREEN

## RED boundary

The reviewed amendment persists `parent_execution_digest` so a client restart
can decide a projected Side-task without relying on the transient Mission
start envelope. Two tests cleared the in-memory execution result while
retaining only the authoritative snapshot, timeline board and current Run.

Initial commands:

```text
go test ./internal/tui -run TestMissionSideTaskUsesZeroWriteProposalExplicitConfirmAndTypedDecision -count=1
swift test --filter LocalProductStoreTests/testRestartedStoreDecidesFromAuthoritativeSideTaskExecutionBinding
```

Observed RED:

```text
internal/tui: decision request list remained empty
Swift LocalProductStoreTests: expected SideTaskDecisionRequest was nil
```

The failures were behavioral, not fixture, syntax or infrastructure failures.
Both clients required an in-memory `executionResult` even though the projected
Side-task already contained the admitted digest.

## Green closure

The proposal path still requires the current Mission start envelope. The
decision path now reconstructs its binding from the exact projected Mission,
Team, WorkItem, Run, attempt, claim generation and admitted execution digest.
The same focused commands then passed.

