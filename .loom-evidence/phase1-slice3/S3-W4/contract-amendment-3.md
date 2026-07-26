# S3-W4 Contract Amendment 3 — Impacted Accepted-Test Stability

- Date: `2026-07-26`
- Baseline: `47b4b50`
- Trigger: Repair 1 exact impact race matrix

## Problem

The required command:

```text
go test -race ./internal/supervisor ./internal/runtime/piadapter -count=30
```

passed the S3-W4 Supervisor package for all 30 runs, then failed after
528.388s in the accepted S2-W18 metadata-runner test
`interpreter_bytes_inside_stable_directory_are_residual`.

The failure was `ErrPiMetadataProcessTimeout` after 3.65s with no fixture
diagnostic file, meaning the shell fixture had not reached its first diagnostic
branch before the shared default 3s test timeout. The S3-W4 execution adapter
does not call the metadata runner. A focused independent command:

```text
go test -race ./internal/runtime/piadapter \
  -run '^TestPiMetadataProcessRunnerBindingAndSymlinkBehavior/interpreter_bytes_inside_stable_directory_are_residual$' \
  -count=100
```

passed in 47.699s. This supports a repeated-package race-load fixture deadline
gap, not an S2-W18 product regression.

## Bounded owned-file amendment

Add exactly one accepted-prerequisite test file to S3-W4 Repair 1 ownership:

- `internal/runtime/piadapter/process_runner_test.go`

The only permitted edit is to increase the shared non-timeout fixture default
from 3s to a bounded value no greater than 10s. The dedicated product timeout
proof remains explicitly fixed at 100ms. Caller-cancellation readiness,
process-group cleanup, production maximum 30s, product code, API, dependency,
Event, authority, and capability remain unchanged.

## Required proof

- the impacted subtest passes `-race -count=100`;
- the dedicated internal-timeout and caller-cancellation tests still pass;
- the exact S3-W4 dual-package `-race -count=30` command passes;
- full repository, race, vet, format, diff, dependency, and scope checks pass.

No product behavior is repaired or reopened. This is a test-only impact-matrix
stability amendment for one already-required package.

VERDICT: FROZEN
