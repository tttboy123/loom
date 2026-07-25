# S2-W33 Candidate Deliverable

- WorkItem: `S2-W33`
- Title: One-Shot Discovery-Priority Runtime Observation Write Coordination
- Risk: Strict
- Candidate state: `ACCEPTED_PENDING_LOCAL_COMMIT`
- Branch/head: `codex/loom-platform-slice2` at `26bf981`
- Contract SHA-256:
  `c813473829000854624e0930925d1f38a70f67442b62f90b03e85d1083654b82`
- Contract review SHA-256:
  `86b65634e7630b92bf597c83f00508d61b40b46fbe1423877c3ea9656c3d61d9`

## Contract and mandatory RED

Fresh independent contract review returned `PASS` with no findings.

Only `internal/app/runtime_observation_write_test.go` was added before product
code. The required focused command exited nonzero because the frozen S2-W33
symbols did not exist:

```text
go test ./internal/app -run 'TestRunRuntimeObservationWriteOnce' -count=1
# loom-pi-rebuild/internal/app [loom-pi-rebuild/internal/app.test]
internal/app/runtime_observation_write_test.go:39:4: undefined: RunRuntimeObservationWriteOnce
internal/app/runtime_observation_write_test.go:42:22: undefined: ErrInvalidRuntimeObservationWriteRun
...
FAIL loom-pi-rebuild/internal/app [build failed]
```

No product file existed during RED, and the failure named only the frozen
coordinator and error symbols.

## Minimal Candidate

The Candidate validates context, computes accepted S2-W32 exactly once, checks
context, and branches on the immutable plan:

- `none` returns only the exact plan;
- `discovery` validates and calls only the accepted discovery committer once,
  validates its exact S2-W27 result, and returns only plan plus discovery
  commit; and
- `status` validates and calls only accepted S2-W31 once and returns only plan,
  reconciliation, and status commit.

Every error returns all four Candidates as zero. Only the selected dependency
is inspected. There is no discovery execution, projection query/rebuild,
metadata preparation, retry, dual write, Journal/SQLite access, scheduler,
daemon, Runtime activation, external action, or Slice 3 authority.

```text
951f454694a220009de87cb9da72e8ccd2d8af8d4ae2516859b21e8e6d966773  internal/app/runtime_observation_write.go
a3703902c07d82ef4fe79e9e1ad07cbdcd4028d893ed7accfd752ba13bd32947  internal/app/runtime_observation_write_test.go
```

## Controller verification

All frozen checks passed:

```text
go test ./internal/app -run 'TestRunRuntimeObservationWriteOnce' -count=1
ok loom-pi-rebuild/internal/app

go test ./internal/app -count=1
ok loom-pi-rebuild/internal/app

go test ./internal/app ./internal/runtime ./internal/state \
  ./internal/projection ./internal/journal -count=1
ok loom-pi-rebuild/internal/app
ok loom-pi-rebuild/internal/runtime
ok loom-pi-rebuild/internal/state
ok loom-pi-rebuild/internal/projection
ok loom-pi-rebuild/internal/journal

go test -race ./internal/app \
  -run 'TestRunRuntimeObservationWriteOnce' -count=50
ok loom-pi-rebuild/internal/app

go test ./... -count=1
PASS

go test -race ./... -count=1
PASS

go vet ./...
PASS

gofmt -d internal/app/runtime_observation_write.go \
  internal/app/runtime_observation_write_test.go
no output

git diff --check
no output
```

Focused proof covers nil/canceled/deadline context, invalid projection/current,
stable identity drift, nil and typed-nil path-scoped dependencies, empty,
unchanged and absent-only no-write cycles, current-only/inventory/mixed
discovery priority, exact status-only S2-W31 delegation, selected source
errors/result mismatches/cancellation, opposite-path non-use, exact caller-owned
retry, input/Candidate/Event accessor mutation isolation, and static imports,
call counts, and excluded authority.

The temporary SQLite integration appends prior discovery sequence 1, rebuilds
projection, appends mixed discovery-priority sequence 2 twice idempotently with
zero status calls, rebuilds, then appends status-only sequence 3 twice
idempotently with zero discovery calls. Exactly three rows remain with Event
types `RuntimeInstanceDiscovered`, `RuntimeInstanceDiscovered`, and
`RuntimeInstanceStatusChanged`; the final rebuilt Runtime is online at
discovery sequence 2 and status sequence 3.

## Review gate

Fresh independent implementation review returned `PASS` with no findings after
independently rerunning the complete strict matrix:

```text
implementation-review.md
VERDICT: PASS
```

The Candidate is accepted and may receive its one exact-scope local atomic
commit after a fresh pre-commit matrix and staged-scope audit.

VERDICT: PASS
