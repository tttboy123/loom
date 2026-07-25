# S3-W2 Deliverable

- Baseline: `c21a8f1`
- Risk: `STRICT`
- Date: `2026-07-26`
- Candidate state: `accepted`

## Delivered boundary

- Journal exact multi-stream-head compare-and-append with atomic exact retry,
  conflict, rollback, cancellation, mutation-isolation, and deterministic
  read-all behavior.
- One Run authority for atomic WorkItem create/assignment, capacity-aware
  claim/reclaim, prepare-lease extension, start, and terminal commit.
- Accepted Runtime discovery/status remains exclusively in
  `runtime_instance:<id>`; capacity facts use `runtime_capacity:<id>`.
- Claim/reclaim/start/terminal CAS the exact current Runtime status head.
  Relevant Run and capacity facts persist the exact status stream, sequence,
  and Event ID used by that CAS.
- Successful Run completion stops at WorkItem `ready_for_review`; the executor
  never marks its own WorkItem done.
- Rebuildable projection uses dependency-aware replay across WorkItem, Run,
  Runtime status, and capacity streams, including historical status/capacity
  references, offline-before/after, exact pairing, and failure isolation.

No process, filesystem workspace, network, credential, Grant, Provider/model,
Bridge payload, scheduler, retry loop, Runtime activation, heartbeat,
acceptance authority, or Slice 4 behavior was added.

## TDD evidence

- Complete parent RED:
  `.loom-evidence/phase1-slice3/S3-W2/red.md`
- Amendment 2 `ReadAll` RED:
  `.loom-evidence/phase1-slice3/S3-W2/red-amendment-2.md`
- Amendment 4 separate-capacity/status-reference RED:
  `.loom-evidence/phase1-slice3/S3-W2/red-amendment-4.md`
- All seven frozen RED markers occur exactly once.

## Controller verification

The following commands returned exit `0` on the complete Candidate and again
in the fresh pre-commit matrix after Implementation Review 2:

```text
go test ./internal/journal ./internal/work ./internal/projection -count=1
go test -race ./internal/journal ./internal/work ./internal/projection -count=30
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go test ./internal/work -run '^$' -fuzz '^FuzzRunAuthorityReplayNeverPanics$' -fuzztime=5s
gofmt -d internal/journal/store.go internal/journal/journal_test.go internal/work/run_authority.go internal/work/run_authority_test.go internal/projection/projection.go internal/projection/projection_test.go internal/projection/run_authority.go internal/projection/run_authority_test.go
git diff --check
```

The focused race matrix passed during stabilization and again in the final
pre-commit matrix. The final pre-commit fuzz run executed 214,985 inputs after
loading 230 baseline cases and found no panic.
Tests use deterministic private temporary SQLite databases only. Direct proof
includes capacity-one concurrent claim/status races, exact operation payloads
and causation, offline terminal cleanup, historical capacity-head auditing,
real Journal rebuild after database reopen, snapshot preservation, mutation
isolation, and concurrent reads.

Fresh independent Implementation Review 2 returned `PASS` with no findings.
The final pre-commit matrix also passed.

VERDICT: PASS
