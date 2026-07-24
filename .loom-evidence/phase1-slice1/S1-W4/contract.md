# S1-W4 Frozen WorkItem Contract

- WorkItem: `S1-W4`
- Risk: Strict
- Depends on: accepted `S1-W2`, accepted `S1-W3-L2`
- Corresponds to: `TECH-PLAN.md §14 Slice 1.3 "状态投影", §6;
  ADR-0002`

## Owned files

- `internal/projection/*.go`
- `internal/projection/*_test.go`

Do not modify Journal, Evidence, migrations, mode, CLI, dependencies, docs,
S1-W5, or Slice 2 behavior.

## Objective and authority boundary

Build a minimal read-only mode, WorkItem, and Evidence projection exclusively
from committed Event Journal facts.

- The Event Journal remains the only fact authority.
- The projection has no public domain mutation methods.
- Rebuild constructs a complete candidate snapshot off to the side and exposes
  it only after successful replay.
- A failed or canceled rebuild leaves the previous valid snapshot unchanged.
- Deleting/recreating the projection and replaying the same committed facts
  produces an equal snapshot.

## Frozen event surface

Schema version `1` projection handlers:

- `ModeSelected`: payload `{"mode":"conversation|agent"}`; project mode by
  stream ID.
- `WorkItemCreated`: payload
  `{"work_item_id":"...","title":"...","status":"..."}`; create one projected
  WorkItem.
- `WorkItemTerminal`: payload
  `{"work_item_id":"...","status":"..."}`; update an existing projected
  WorkItem.
- `EvidenceSubmitted`: payload
  `{"evidence_id":"...","work_item_id":"...","digest":"sha256:..."}`; add one
  digest-only Evidence record.

Event and status strings are contract data, not new enums. Do not modify
existing Journal errors or validation.

## Frozen replay strategy

- The production source reads only committed rows from the Journal `events`
  table and materializes `journal.Event` values.
- Canonical replay order is `(stream_id ASC, seq ASC, id ASC)`, independent of
  source delivery order.
- Per-stream sequence must start at `1` and remain contiguous.
- An exact duplicate immutable Event is ignored idempotently.
- Reuse of an Event ID or `(stream_id, seq)` for different immutable content
  returns typed `ErrConflictingEvent`.
- A sequence gap returns typed `ErrSequenceGap`.
- An unknown schema version returns typed `ErrUnsupportedEventVersion`.
- An unknown event type at supported schema version is ignored because this
  bounded projection consumes only four of the core Event types.
- Malformed relevant payload, invalid projected mode, missing required fields,
  terminal-before-create, conflicting create, or Evidence referencing an
  unknown WorkItem returns typed `ErrInvalidProjectionEvent`.
- Repeated identical WorkItem/Evidence facts are idempotent; conflicting
  repetitions fail closed.

## Read model

- A `Snapshot` exposes modes keyed by stream ID, WorkItems keyed by WorkItem ID,
  and Evidence keyed by Evidence ID.
- Returned snapshots are deep copies so callers cannot mutate current state.
- Evidence stores only stable ID, WorkItem ID, and digest; no mutable artifact
  path becomes identity.
- Snapshot equality is deterministic and does not depend on map iteration.

## Mandatory RED tests

1. `TestRebuildFromCommittedJournalIsDeterministic`
   appends projection and irrelevant facts through `journal.Store`, rebuilds,
   recreates the projection, and proves equal mode/WorkItem/Evidence results.
2. `TestRebuildCanonicalizesOutOfOrderAndDuplicateEvents`
   supplies out-of-order exact duplicate Events and proves one deterministic
   result.
3. `TestRebuildRejectsConflictGapAndUnknownVersion`
   proves each frozen typed failure policy.
4. `TestFailedRebuildPreservesPreviousSnapshot`
   starts from a valid snapshot, then injects invalid/canceled replay and proves
   the prior snapshot remains exactly unchanged.
5. `TestSnapshotIsReadOnlyCopy`
   mutates a returned value and proves current projection state is unchanged.

## Deterministic checks

- RED/focused:
  `go test ./internal/projection -run 'Test(Rebuild|Snapshot)' -count=1`
- Repeated focused race:
  `go test -race ./internal/projection -run 'Test(Rebuild|Snapshot)' -count=50`
- Projection full: `go test ./internal/projection -count=1`
- Projection race: `go test -race ./internal/projection -count=1`
- Impact: `go test ./... -count=1`
- Repository race: `go test -race ./... -count=1`
- Static analysis: `go vet ./...`
- Formatting/diff: `gofmt`, trailing-whitespace check, `git diff --check`

## Gate and non-goals

A new read-only strict Reviewer must return `VERDICT: PASS` against this
contract and the complete Candidate before S1-W5 may open.

No persisted second authority, projection migration, daemon, API, Team Draft,
Run state machine, Artifact mutation, network access, background process,
FastContext work, or Slice 2 behavior is authorized.
