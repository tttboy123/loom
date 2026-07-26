# Slice 3 Exit Contract Amendment 6

Status: FROZEN — independent Contract Review 1 PASS.

Date: `2026-07-26`

## Name

S3-W5 Repeated-Race Test Stability Boundary

## Necessity

The frozen S3-W5 combined impact command:

```text
go test -race ./internal/supervisor ./internal/runtime/piadapter \
  ./internal/teams ./internal/work ./internal/authorization \
  ./internal/projection ./internal/app -count=10
```

passed every package except `internal/runtime/piadapter`. Three inherited
fixture deadlines expired under concurrent race-instrumented package load:

- five-second grandchild PID readiness;
- three-second local metadata probe; and
- three-second Supervisor integration Runtime profile.

No data race, Bridge mismatch, observer error, cleanup leak, or stable product
failure was reported. The three affected families passed isolated ten-run race
reproduction, and the complete Pi Adapter package passed isolated
`-race -count=10`. Fresh read-only Problem Analysis classified the failure as
verification-harness/host-scheduling budget exhaustion. Acceptance remains
open until the unchanged combined command passes.

## Exact reopened scope

Only these test files are reopened:

- `internal/runtime/piadapter/execution_adapter_test.go`
- `internal/runtime/piadapter/local_probe_test.go`

The first is already S3-W5 compatibility-owned. The second is an accepted
S2-W19 test file reopened only for the exact non-timeout fixture bound below.
No production, contract API, dependency, runtime behavior, or activation file
is reopened.

## Exact permitted edit

Only three non-timeout evidence budgets may increase, each to exactly ten
seconds:

1. grandchild PID readiness timer: `5s -> 10s`;
2. Supervisor success/failure integration Runtime profile: `3s -> 10s`;
3. valid local-probe fixture timeout: `3s -> 10s`.

The edit must preserve:

- the dedicated 100ms metadata timeout proof;
- 100ms cancellation grace and cancellation semantics;
- after-result timeout behavior;
- process-group death and cleanup assertions;
- production timeout maxima and validation;
- all fixture payloads, commands, environment isolation, and assertions; and
- the original seven-package `-race -count=10` acceptance command unchanged.

## Verification

```text
go test -race ./internal/runtime/piadapter \
  -run '^(TestPiExecutionCancellationTimeoutAndProcessGroupCleanup|TestPiExecutionThroughSupervisorReopensExactTerminal|TestPiLocalRuntimeProbeFactory.*)$' \
  -count=20

go test -race ./internal/runtime/piadapter \
  -run 'TestPiMetadataProcessRunner.*(Timeout|Cancel)|TestPiExecution.*(Timeout|Cancellation)' \
  -count=10

go test -race ./internal/supervisor ./internal/runtime/piadapter \
  ./internal/teams ./internal/work ./internal/authorization \
  ./internal/projection ./internal/app -count=10
```

Then rerun the existing full repository, race, vet, format, diff, scope,
security, canary, and fresh implementation review gates.

No S3-W6, product repair, timeout weakening, installed Runtime, daemon,
network, or activation is authorized.

VERDICT: FROZEN
