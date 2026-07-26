# S3-W5 Mandatory RED

- Date: `2026-07-26`
- Baseline: `cd1e594`
- Command:

```text
go test ./internal/journal ./internal/teams ./internal/projection \
  ./internal/supervisor -count=1
```

## Observed failure

The test build failed for the intended missing S3-W5 product surfaces:

```text
internal/journal/journal_test.go:746:25:
  store.ReadStreamSet undefined
internal/journal/journal_test.go:756:27:
  undefined: StreamHead

internal/teams/execution_plan_test.go:11:11:
  undefined: ExecutionPlanInput
internal/teams/execution_plan_test.go:41:16:
  undefined: BuildExecutionPlan

internal/projection/global_read_view_test.go:38:21:
  projection.GlobalReadView undefined

internal/supervisor/managed_execution_test.go:105:28:
  unknown field FrameSink in AdapterRequest
internal/supervisor/managed_execution_test.go:106:24:
  unknown field FrameObserver in ExecuteInput
```

The failures are compile-time absence of the frozen APIs, not fixture,
environment, or unrelated baseline failures.

VERDICT: RED

## Amendment 3 durable recovery RED

The reviewed restart amendment added tests before implementation for the
durable attempt capture, Team generation rebound, Projection rebound, latest
Grant lookup, and application restart paths.

Commands included:

```text
go test ./internal/evidence -count=1
go test ./internal/work \
  -run TestTeamAttemptRebindFencesExpiredGenerationAndIsIdempotent -count=1
go test ./internal/projection \
  -run TestTeamExecutionProjectionTracksLogicalAttemptsAndDeepCopies -count=1
go test ./internal/app \
  -run '^TestTeamCoordinatorRecoversDurableAttemptWindows$' -count=1
```

The first two builds failed only because the newly frozen capture and rebound
APIs did not exist. The Projection RED then failed with:

```text
invalid projection event: unknown Team execution event TeamNodeAttemptRebound
```

The application RED preserved two exact missing restart capabilities:

```text
terminal Run capture finalizes without duplicate execution:
  Team status remained running; error Team execution incomplete

expired claim rebinds generation before one execution:
  Team status remained running; error Team execution incomplete
```

After those REDs, implementation began. The recovery test later exposed and
preserved two additional cross-stream defects before repair:

- an explicitly revoked Grant followed by a higher-generation issue used the
  Run head rather than the prior Grant-stream Event as causation; and
- Projection incorrectly required a rebound's referenced `RunClaimed` Event to
  remain the final Run head after later start/terminal Events.

Both were repaired without widening the frozen authority boundary.

VERDICT: RED

## Implementation Review 1 repair RED

The independent implementation Review 1 produced three bounded product
findings. Tests added before Repair 1 reproduced all three:

```text
wrong previous rebound retry error = <nil>

missing capture rejects later same-generation Run activity:
  Run() error = Team execution incomplete

divergent capture conflicts before lease expiry:
  Run() error = Team execution incomplete
```

The unrelated dirty `AGENTS.md` edit was classified as user-owned and remains
outside Candidate staging.

VERDICT: RED

Fault injection at the two accepted N+1 split states then exposed one more
bounded recovery RED:

```text
after Run reclaim before capture rebind:
  Team status remained running; error Team execution incomplete

after capture rebind before Team rebound:
  Team status remained running; error Team execution incomplete
```

The Coordinator was incorrectly waiting for the new N+1 lease to expire before
finishing an already-authorized reclaim sequence. Repair restricts lease
waiting to the same-generation claimed state.

VERDICT: RED
