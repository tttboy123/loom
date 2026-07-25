# S2-W16 Deliverable

- WorkItem: `S2-W16 Team and Agent Instance Read-Model Projection`
- Branch/base: `codex/loom-platform-slice2` at `560834a`
- Contract:
  `.loom-evidence/phase1-slice2/S2-W16/contract.md`
- Contract SHA-256:
  `2cc5a6a6354dba52228625467f849b4c5ac5dbd58fec2c0aeedd5e4a1a559fb1`
- Contract Review 1: bounded ownership-wording `REPAIR`
- Repaired contract review:
  `.loom-evidence/phase1-slice2/S2-W16/contract-review-2.md`
- Repaired contract Reviewer: `PASS`; blocking findings: none

## Mandatory RED

Command:

```text
go test ./internal/projection -run 'TestRebuildSavedTeamInstanceFacts' -count=1
```

Result: `FAIL` as required. The compiler reported only missing frozen S2-W16
read-model fields, beginning with `Snapshot.Teams undefined` and
`Snapshot.AgentInstances undefined`.

## Candidate

- Product: `internal/projection/projection.go`
- Product SHA-256:
  `d372873a14b3a64fe7fc9fdc6b87e7913f38c1cfcdc6a3983bfff1b492c00afb`
- Tests: `internal/projection/projection_test.go`
- Test SHA-256:
  `cfcc51134e44ac13ce0c02f021f4c05448a13d32a85f5022320925853ddea447`

The Candidate extends only the accepted S1-W4 in-memory projection. It strictly
decodes the two S2-W15 schema-v1 facts, creates immutable Team/Main Agent views,
validates every Team has exactly one linked Main after all streams replay, and
deep-copies all maps and dormant slices. It adds no Journal write, table,
migration, CLI, WorkItem, resource, process, Runtime execution, or Slice 3
behavior.

## Verification

```text
go test ./internal/projection -run 'TestRebuildSavedTeamInstanceFacts' -count=1
ok   loom-pi-rebuild/internal/projection

go test ./internal/projection -count=1
ok   loom-pi-rebuild/internal/projection

go test -race ./internal/projection -run 'TestRebuildSavedTeamInstanceFacts' -count=50
ok   loom-pi-rebuild/internal/projection

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

gofmt -d internal/projection/projection.go internal/projection/projection_test.go
no output

git diff --check
exit 0
```

The focused suite covers real SQLite replay and restart equivalence,
main-only/one/two dormant cardinality, project/reusable shadow, reusable-Agent
fallback, reversed input and agent-stream-before-team replay, strict Team and
Agent payload/envelope matrices, missing/orphan/duplicate Main and link
mismatches, failed/canceled/concurrent/closed-Journal preservation, deep-copy
isolation, and the frozen production import boundary.

## Repair 1

The first implementation Reviewer found that ordinary JSON decoding could not
distinguish omitted fields from present legal zero values. The finding is
recorded in `implementation-review-1.md`; Repair 1 is bounded by
`repair-1-contract.md` (SHA-256
`2777be8211f09e688dee8819d6d79c9bb38003da8fe0d241877fac3d5b5beeff`).

Repair 1 adds RED cases for missing dormant, count, and Team/Main scope-identity
fields, then uses private pointer-backed fact structs to require field presence
before copying values into the unchanged public read-model shape. All envelope,
link, scope, digest, clone, concurrency, import, and no-side-effect behavior
remains unchanged.

The complete focused, package, focused-race-50, repository, repository-race,
vet, format, and diff matrix was rerun after Repair 1 and passed.

## Review gate

Fresh independent Repair 1 review returned `PASS` with no blocking findings.
The accepted review is recorded in `implementation-review-2.md`. No external
action or runtime activation occurred.

VERDICT: PASS
