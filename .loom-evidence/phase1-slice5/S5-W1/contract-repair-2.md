# S5-W1 Contract Repair 2 — External-Package Integration Test Ownership

- Date: `2026-07-26`
- Baseline: `006db8c`
- Scope: one test-file ownership substitution; no product edit

## Trigger

Mandatory RED preparation confirmed the accepted production dependency:

```text
internal/api -> internal/app
```

The original ownership attempted to place the reverse integration in the
existing `internal/app/team_execution_test.go`, whose package is `app`.
Importing `internal/api` there would make the package-under-test import its
consumer and create a test import cycle.

## Bounded repair

Repair 2 substitutes exactly one test ownership entry:

```text
remove reopened internal/app/team_execution_test.go
add new internal/app/team_execution_stream_test.go
    with package app_test
```

The external test may import both `internal/app` and `internal/api`, while
`internal/api` continues to import `internal/app` only in the accepted
production direction. It proves the same frozen coordinator/attempt-capture/
observer behavior. No production app file, existing test file, API, schema,
bound, behavior, authority, package direction, dependency, migration,
WorkItem, S5-W2, or S5-W3 changes.

Already-added RED tests in other frozen files remain test-only and must not
advance to product GREEN until fresh Contract Repair Review 3 passes.

VERDICT: PASS
