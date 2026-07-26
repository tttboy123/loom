# Slice 5 Exit Contract Amendment 1 Review 1

- Reviewer: fresh independent read-only Contract Reviewer
- Baseline: `006db8c`
- Date: `2026-07-26`

## Findings

None.

## Review

The stale Projection diagnosis is valid on both executable paths:

- first dispatch commits Team/Work/Run/capacity facts and then prepares and
  executes tasks without another Projection rebuild; and
- recovery can commit a generation rebound and return an executable task that
  is likewise executed before the next rebuild.

The S5-W1 observer reads `GlobalReadView` directly and checks exact
Team/attempt/Run/generation/Runtime/Agent binding. Exactly one rebuild after
each accepted write path and before execution is therefore necessary and
sufficient. Rebuilding inside the observer would violate its non-blocking
delivery boundary.

Projection constructs a complete candidate before publishing; rebuild failure
retains the old view. The amended call sites therefore fail closed before any
task/Frame execution and add no retry or write.

Ownership is minimal: only `internal/app/team_execution.go` and its existing
same-package test are reopened for the refresh behavior. The separate
`package app_test` consumer integration remains cycle-free.

No authority, retry, write, schema, API, dependency, migration, WorkItem,
S5-W2, or S5-W3 scope is added.

No tests were required for this read-only contract review.

VERDICT: PASS
