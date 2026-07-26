# S3-W5 Implementation Repair 2 Review 3

Reviewer: independent read-only implementation reviewer

Date: 2026-07-26

Scope: Slice 3 Exit Contract Amendment 6 / S3-W5 Contract Amendment 4
repeated-race test stability repair

## Findings

None.

## Evidence

- The Amendment 6-specific implementation is exactly three test-only changes:
  grandchild PID readiness `5s -> 10s`, Supervisor fixture Runtime profile
  `3s -> 10s`, and valid local-probe fixture `3s -> 10s`.
- Pre-existing S3-W5 Pi Adapter test changes for FrameSink authorization and
  identity-index initialization remain within the earlier frozen Candidate and
  are not part of this stability repair.
- Dedicated metadata timeout remains 100ms, cancellation grace remains 100ms,
  and the after-result timeout and process-death assertions remain intact.
- Production cancel-grace and metadata-process maxima are unchanged.
- Problem Analysis and Controller GREEN evidence record the same three edits
  and consistent harness/host-scheduling classification.
- Directed affected-family `-race -count=20`, dedicated timeout/cancel
  `-race -count=10`, the original seven-package `-race -count=10`, full
  repository/race, vet, format, and diff gates all passed.
- `AGENTS.md`, `PROGRESS.md`, `.codex/**`, `.loom-drafts/**`, and
  `POST-S3-W5-QUEUED-CONTRACT-INPUTS.md` remain outside the Candidate and must
  stay unstaged.

## Bounded reviewer checks

```text
git diff --check
gofmt -d internal/runtime/piadapter/execution_adapter_test.go \
  internal/runtime/piadapter/local_probe_test.go
go test ./internal/runtime/piadapter \
  -run '^(TestPiExecutionCancellationTimeoutAndProcessGroupCleanup|TestPiExecutionThroughSupervisorReopensExactTerminal|TestPiLocalRuntimeProbeFactory.*|TestPiMetadataProcessRunner.*(Timeout|Cancel)|TestPiExecution.*(Timeout|Cancellation))$' \
  -count=1
```

Result: PASS.

## Recommendation

Accept the bounded test-stability repair and stage only the complete S3-W5
Candidate/evidence scope.

VERDICT: PASS
