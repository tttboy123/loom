# S3-W2 Amendment 4 Focused RED

- Baseline: `c21a8f1`
- Date: `2026-07-26`
- Product behavior changed before RED: no

Command:

```text
go test ./internal/work -run '^TestClaimRuntimeStatusCapacityAtomicity/claim_writes_matched_run_and_runtime_facts$' -count=1
```

Result: exit `1`.

The test reached the existing authority behavior and failed only because the
claim still appended `RuntimeCapacityReserved` to
`runtime_instance:runtime-1`, left `runtime_capacity:runtime-1` empty, and
omitted the frozen exact Runtime status-head reference from the paired
`RunClaimed` and capacity payloads. The accepted discovery fact remained at
status-stream sequence 1. There was no syntax, dependency, environment, or
unrelated-test failure.

VERDICT: PASS
