# S4-W1 Repair 1 GREEN

Date: 2026-07-26

Baseline: `7bb9881`

Repair 1 closes all four findings from Fresh Implementation Review 1:

1. approval authority derives and transaction-consistently reads all four
   current RuleSet scopes plus Approval, WorkItem, and Run;
2. exact retries bind the complete committed command while remaining
   idempotent after the original authorization window;
3. approved resume Candidates reconstruct the exact seven stream heads after
   restart; and
4. the injected CustomerAuthorizer port exposes immutable request accessors
   and validated authorized-response constructors.

## Focused and regression checks

```text
go test ./internal/rules ./internal/work ./internal/projection -count=1
ok loom-pi-rebuild/internal/rules
ok loom-pi-rebuild/internal/work
ok loom-pi-rebuild/internal/projection

go test -race ./internal/rules -run 'TestApprovalLifecycle|TestApprovalRequestCannotOmit|TestApprovalDecisionRejectsRuleSetChanged|TestConcurrentApproval|TestCustomerAuthorizerPort|TestApprovalExpiry' -count=20
ok loom-pi-rebuild/internal/rules

go test -race ./internal/rules ./internal/work ./internal/projection -count=10
ok loom-pi-rebuild/internal/rules
ok loom-pi-rebuild/internal/work
ok loom-pi-rebuild/internal/projection
```

## Impact and whole-repository checks

```text
go test ./internal/app ./internal/supervisor ./internal/runtime/piadapter -count=1
```

The first impact run had one pre-existing fixture-level failure:

```text
TestLocalRuntimeObservationDaemonRealSQLiteRestart
pi metadata command failed: version
```

The same run passed `internal/supervisor` and `internal/runtime/piadapter`.
The exact failing test then passed five consecutive runs:

```text
go test ./internal/app -run '^TestLocalRuntimeObservationDaemonRealSQLiteRestart$' -count=5
ok loom-pi-rebuild/internal/app
```

The independent whole-repository run, which includes the same package and
test, passed:

```text
go test ./... -count=1
ok all packages

go test -race ./... -count=1
ok all packages

go vet ./...
exit 0
```

This classifies the isolated first result as an existing transient fixture
failure rather than an S4-W1 regression.

## Format, scope, and trust-boundary checks

```text
gofmt -d <all owned Go files>
no output

git diff --check
no output

git diff -- go.mod go.sum
no output
```

Static inspection found no new network listener, process execution, daemon
activation, credential or environment mutation, or dependency change. Normal
Run-authority command paths use `ReadStreamSet`; the remaining `ReadAll` calls
are the pre-existing identity-index initialization and read-only `Snapshot`.
All authoritative mutations continue through
`AppendBatchIfStreamHeads`.

Unrelated user-owned `AGENTS.md`, `PROGRESS.md`, `.codex`, `.loom-drafts`, and
post-S3 scratch queue changes remain outside this Candidate and will not be
staged.

VERDICT: PASS
