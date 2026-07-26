# Slice 3 Exit Contract Amendment 5

Status: FROZEN — independent Review 1 PASS.

## Name

Durable Attempt Receipt and Safe Restart Boundary

## Why this amendment is necessary

The frozen S3-W5 restart clauses require recovery across Run terminal, Artifact
publish, and Evidence metadata commit. The accepted API stores authorized Frames
only in an in-memory application collector and the content-addressed Artifact
Store exposes no durable `attempt -> digest` receipt. After process loss, the
Coordinator therefore cannot reconstruct the exact artifact or associate an
already-published artifact with the deterministic Evidence ID.

The same frozen clauses require a committed, not-started attempt to be reclaimed
after its prepare lease expires. The current Team stream has no event that binds
the same logical attempt to the higher accepted Run generation. Reusing the old
Team binding would weaken generation fencing.

These are contract feasibility defects, not permission to add checkpoints,
persist model context, or resume an indeterminate running process.

## Reopened owned files

Only these additional product files are reopened:

- `internal/evidence/attempt_capture.go`
- `internal/evidence/attempt_capture_test.go`
- `internal/evidence/attempt_capture_windows.go`

The following already-owned S3-W5 files remain open only for the exact repair:

- `internal/work/team_execution_authority.go`
- `internal/work/team_execution_authority_test.go`
- `internal/projection/team_execution.go`
- `internal/projection/team_execution_test.go`
- `internal/projection/global_read_view.go`
- `internal/projection/global_read_view_test.go`
- `internal/app/team_execution.go`
- `internal/app/team_execution_test.go`
- `.loom-evidence/phase1-slice3/S3-W5/contract-amendment-3.md`
- `.loom-evidence/phase1-slice3/S3-W5/contract-amendment-3-review-1.md`
- `.loom-evidence/phase1-slice3/S3-W5/restart-canary.md`
- `docs/adr/0009-authorized-node-output-and-attempt-recovery.md`

No other accepted file is reopened. A compatibility guard that rejects only the
new exact API/Event names requires a separately reviewed, name-only amendment.

## Authority boundary

The attempt capture and receipt are a bounded, private, rebuildable delivery
spool. They are not execution authority, terminal authority, acceptance
authority, a Projection checkpoint, or a second Journal. Only accepted Run
terminal Events and the atomic `EvidenceSubmitted` plus
`TeamNodeAttemptTerminal` transaction advance authoritative state.

Capture files may contain only already-authorized Bridge Frames and exact
attempt binding metadata. They never contain raw Grants, credentials, ambient
environment, hidden reasoning, model context, or process images.

## Safe restart boundary

- A terminal Run plus a complete capture can deterministically finalize or
  republish one content-addressed Artifact, recover its durable receipt, and
  complete missing Evidence metadata exactly once.
- A claimed, never-started Run may be reclaimed only after its accepted prepare
  lease expires. The old active Grant is explicitly revoked, the accepted Work
  Authority creates a higher generation, and one Team rebound Event fences the
  previous generation before execution.
- A Run already in `running` phase without an accepted terminal is
  indeterminate. Phase 1 returns a typed `HUMAN_REQUIRED` recovery error and
  performs no replay, duplicate execution, fabricated terminal, or automatic
  policy decision.
- A capture without an accepted terminal is never finalized as Evidence.
- Orphan staging files are non-authoritative and bounded; finalized receipts and
  Evidence metadata remain exact-once.

## Explicit exclusions

- no model-context, process-image, or provider checkpoint;
- no token-per-Event Journal persistence;
- no incremental Projection checkpoint;
- no automatic retry/fallback/degraded policy choice;
- no Provider fallback;
- no daemon, resident worker, timer, Web/TUI, SSE, or client delivery;
- no new WorkItem after S3-W5;
- no W6, second scheduler, second StateWriter, or second authority.

## Verification

Implementation may begin only after an independent PASS on this amendment and
S3-W5 Contract Amendment 3. Mandatory tests inject process loss at:

1. dispatch committed before execution;
2. capture begun;
3. authorized Frames captured;
4. Run terminal committed;
5. Artifact finalized and receipt committed;
6. Evidence metadata committed.

They must prove deterministic reopen, one Run identity, generation fencing, one
authoritative Evidence record, exact Artifact digest reuse, no output bytes in
Journal/Projection, and safe `HUMAN_REQUIRED` for indeterminate running state.

VERDICT: FROZEN
