# S3-W4 Verification Repair 1 RED

- Date: `2026-07-26`
- Trigger: exact required 30-run race matrix

## Command

```text
go test -race ./internal/supervisor ./internal/runtime/piadapter -count=30
```

## Result

`RED` — Supervisor passed 30 race runs. Pi adapter failed after 510.116s:

```text
TestPiExecutionCancellationTimeoutAndProcessGroupCleanup:
grandchild PID proof missing: .../workspace/grandchild.pid: no such file or directory
```

## Diagnosis

The test used one fixed 750ms deadline both to wait for the race-instrumented
helper to start and write `grandchild.pid` and to trigger cancellation. Under
the repeated race load, cancellation could occur before the helper emitted the
readiness proof. The failure therefore did not establish a process-group
cleanup product defect; it established that the test did not deterministically
reach the state it claimed to verify.

Repair is test-only inside the frozen owned file: wait for the PID file as an
explicit bounded readiness signal, then cancel, await adapter completion, and
retain the exact grandchild-death assertion. No product API, behavior,
authority, or capability changes.

VERDICT: RED
