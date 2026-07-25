# S2-W15 Deliverable

- WorkItem: `S2-W15 Saved-Team Instance StateWriter`
- Branch/base: `codex/loom-platform-slice2` at `f293a9f`
- Contract:
  `.loom-evidence/phase1-slice2/S2-W15/contract.md`
- Contract SHA-256:
  `01f476ffa03f392bd88bb31615005ceb547a54ced4485aa3779ee28b01a0bd74`
- Contract review:
  `.loom-evidence/phase1-slice2/S2-W15/contract-review.md`
- Contract Reviewer: `PASS`; blocking findings: none

## Mandatory RED

Command:

```text
go test ./internal/state -run 'TestCommitSavedTeamInstanceRecordSet' -count=1
```

Result: `FAIL` as required. The compiler reported only missing frozen S2-W15
symbols, beginning with `undefined: SavedTeamCommitInput`,
`undefined: EventBatchAppender`, and `undefined: SavedTeamCommitCandidate`.

## Candidate

- Product: `internal/state/saved_team_writer.go`
- Product SHA-256:
  `762bcf618dbbf0fc176513c02da868f54e82c60fe54125cc1af8ab65c7ecc7fa`
- Tests: `internal/state/saved_team_writer_test.go`
- Test SHA-256:
  `6a169a8fc0056224b0e88505bb0b3bf9986fa1ad8b03764273f4d8af21373684`

The Candidate revalidates the exact S2-W13 source chain, builds the complete
`TeamInstanceCreated` and Main `AgentInstanceCreated` facts before mutation,
calls accepted S2-W14 `AppendBatch` exactly once, verifies the complete returned
batch, and returns a copied digest-bound commit Candidate. It creates no dormant
AgentInstance, projection, WorkItem, Run, grant, workspace, process, or runtime
execution.

## Verification

```text
go test ./internal/state -run 'TestCommitSavedTeamInstanceRecordSet' -count=1
ok   loom-pi-rebuild/internal/state

go test ./internal/state -count=1
ok   loom-pi-rebuild/internal/state

go test -race ./internal/state -run 'TestCommitSavedTeamInstanceRecordSet' -count=50
ok   loom-pi-rebuild/internal/state

go test ./... -count=1
PASS: cmd/loom, internal/agents, internal/evidence, internal/journal,
internal/mode, internal/projection, internal/runtime, internal/state,
internal/teams; migrations has no test files

go test -race ./... -count=1
PASS: cmd/loom, internal/agents, internal/evidence, internal/journal,
internal/mode, internal/projection, internal/runtime, internal/state,
internal/teams; migrations has no test files

go vet ./...
exit 0

gofmt -d internal/state/saved_team_writer.go internal/state/saved_team_writer_test.go
no output

git diff --check
exit 0
```

The focused suite uses the real SQLite Journal for exact persisted facts,
whole-batch retry, row counts, partial-existing facts, idempotency and occupied
stream conflicts, and closed-store failure. It also covers main-only through
two-dormant cardinality, project/reusable shadowing, reusable-Agent fallback,
stale source rejection before append, typed-nil appender, cancellation, result
reorder/mutation, payload privacy, digest sensitivity, copy isolation, and the
frozen import boundary.

## Repair 1

The first implementation Reviewer required a bounded repair because the
same-ID shadow case contained only one TeamDefinition and the digest sensitivity
test did not exercise the complete frozen matrix. The findings are recorded in
`implementation-review-1.md`; Repair 1 is bounded by
`repair-1-contract.md` (SHA-256
`8c481534d7eedc2f621618b1657ca15f4d775c5a9000e98a5fec03c3e3c93292`).

Repair 1 now:

- builds simultaneous same-ID project and reusable TeamDefinitions, selects the
  reusable default, and verifies exact reusable Team/Main scope in the facts;
- tests every Team/Main Event immutable field plus source digest and count;
- builds valid semantic variants for dormant membership, Main definition
  version/scope, Runtime binding, identity, timestamp, and source record set;
  and
- binds exact payload bytes in the commit digest. The new payload-whitespace
  regression exposed that `json.RawMessage` canonicalized JSON semantics; the
  one-line production repair uses a copied `[]byte` digest field instead.

The complete focused, package, focused-race-50, repository, repository-race,
vet, format, and diff matrix was rerun after Repair 1 and passed.

## Review gate

Fresh independent Repair 1 review returned `PASS` with no blocking findings.
The accepted review is recorded in `implementation-review-2.md`. No external
action or runtime activation occurred.

VERDICT: PASS
