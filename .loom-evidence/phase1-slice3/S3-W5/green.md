# S3-W5 Controller Verification

- Date: `2026-07-26`
- Baseline: `cd1e594`
- Candidate: uncommitted S3-W5 implementation

## Focused package matrix

```text
go test ./internal/evidence ./internal/journal ./internal/teams \
  ./internal/work ./internal/authorization ./internal/projection \
  ./internal/supervisor ./internal/runtime/piadapter ./internal/app -count=1
```

Result: PASS.

```text
go test -race ./internal/evidence ./internal/journal ./internal/teams \
  ./internal/work ./internal/authorization ./internal/projection \
  ./internal/supervisor ./internal/runtime/piadapter ./internal/app -count=1
```

Result: PASS.

## Controlled execution and restart repetition

```text
go test ./internal/app \
  -run '^(TestTeamDAGExecutionControlledCanary|TestTeamCoordinatorRecoversDurableAttemptWindows)$' \
  -count=10
```

Result: PASS.

```text
go test -race ./internal/app \
  -run '^(TestTeamDAGExecutionControlledCanary|TestTeamCoordinatorRecoversDurableAttemptWindows)$' \
  -count=3
```

Result: PASS.

The first whole-repository race run exposed a canary-only one-second deadline
that could time out after two authorized Frames and incorrectly satisfy the
intended failed-attempt path. The app-owned fixture deadline was raised to five
seconds and an exact `controlled_failure` Run-terminal assertion was added.

```text
go test -race ./internal/app \
  -run '^TestTeamDAGExecutionControlledCanary$' -count=10
```

Repair result: PASS.

## Repository matrix

```text
go test ./...
go test -race ./...
go vet ./...
git diff --check
```

Result after the canary repair: PASS.

## Post-repair verification

Implementation Review 1 returned `FAIL` on three bounded recovery defects.
Repair 1 added REDs, closed the complete binding/head/split-state rules, and
expanded restart fault injection.

```text
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
git diff --check
```

Result: PASS.

Fresh independent Implementation Repair 1 Review 2 returned `PASS` with no
findings.

## Repeated-race stability amendment

The frozen seven-package repeated-race command initially exhausted three
inherited non-timeout Pi Adapter fixture budgets under concurrent
race-instrumented package load. Independent Problem Analysis classified the
symptom as verification-harness scheduling exhaustion. Slice 3 Exit Contract
Amendment 6 and S3-W5 Contract Amendment 4 received independent Contract Review
`PASS` before edits.

Only these test budgets changed:

- grandchild PID readiness `5s -> 10s`;
- Supervisor integration Runtime profile `3s -> 10s`; and
- valid local-probe fixture `3s -> 10s`.

Dedicated timeout/cancellation semantics, assertions, product code, and the
original combined command were unchanged.

```text
go test -race ./internal/runtime/piadapter \
  -run '^(TestPiExecutionCancellationTimeoutAndProcessGroupCleanup|TestPiExecutionThroughSupervisorReopensExactTerminal|TestPiLocalRuntimeProbeFactory.*)$' \
  -count=20
```

Result: PASS (`101.391s`).

```text
go test -race ./internal/runtime/piadapter \
  -run 'TestPiMetadataProcessRunner.*(Timeout|Cancel)|TestPiExecution.*(Timeout|Cancellation)' \
  -count=10
```

Result: PASS (`30.640s`).

```text
go test -race ./internal/supervisor ./internal/runtime/piadapter \
  ./internal/teams ./internal/work ./internal/authorization \
  ./internal/projection ./internal/app -count=10
```

Result: PASS. Pi Adapter completed all ten repetitions in `189.837s`; all
other packages also passed.

The remaining frozen checks were rerun:

```text
go test ./internal/evidence ./internal/journal ./internal/teams \
  ./internal/work ./internal/authorization ./internal/projection \
  ./internal/supervisor ./internal/runtime/piadapter ./internal/app -count=1

go test -race ./internal/journal ./internal/work \
  ./internal/authorization ./internal/projection ./internal/teams \
  ./internal/app -count=10

go test ./internal/app \
  -run '^TestTeamDAGExecutionControlledCanary$' -count=10

go test -race ./internal/app \
  -run '^TestTeamDAGExecutionControlledCanary$' -count=3

go test -race ./internal/supervisor ./internal/runtime/piadapter -count=3
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
gofmt -d <all owned Go files>
git diff --check
```

Result: PASS after applying the one mechanical `gofmt` correction identified
by the first format check.

VERDICT: PASS
