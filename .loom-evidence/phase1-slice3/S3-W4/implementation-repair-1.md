# S3-W4 Implementation Repair 1

- Date: `2026-07-26`
- Trigger: fresh independent Implementation Review 1 `FAIL`
- Baseline: current S3-W4 Candidate on `47b4b50`

## Findings

1. Raw Grant substring rejection covered file bytes, stderr, Bridge payloads,
   and returned values, but not source manifest or returned workspace-change
   path text.
2. The fuzz harness treated platform filename-creation rejection as a product
   failure before product code was invoked, making the required fuzz command
   non-reproducible on Darwin.

## Bounded repair

- Add RED for a raw token substring in a source filename and in a
  child-created changed filename.
- Reject the raw token substring in source manifest paths before adapter
  execution and in collected change paths before returning an Outcome.
- Let the fuzz harness return when the platform rejects fixture directory/file
  creation; once setup succeeds, all product failures remain exercised.

Only already-owned files change:

- `internal/supervisor/managed_execution.go`
- `internal/supervisor/managed_execution_test.go`
- `internal/supervisor/managed_workspace.go`
- `internal/supervisor/managed_workspace_test.go`

No API, Event, authority, dependency, owned scope, execution, or capability
change.

VERDICT: FROZEN
