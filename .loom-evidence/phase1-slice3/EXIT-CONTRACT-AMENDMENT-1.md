# Phase 1 Slice 3 Exit Contract Amendment 1 — S3-W5 State-Set and DAG Integration Boundary

- Date: `2026-07-26`
- Baseline: `cd1e594`
- Parent:
  `.loom-evidence/phase1-slice3/EXIT-CONTRACT.md`
- Trigger: explicitly authorized S3-W5 state/checkpoint/global-state and
  multi-Agent concurrency optimization

## Conflict

The accepted Exit Contract closes Slice 2 Journal and projection boundaries
and the accepted S3-W2 Run/WorkItem and S3-W3 AgentGrant authorities. The
authorized final S3-W5 vertical integration requires bounded changes inside
those accepted files:

- add one transaction-consistent related-stream read API;
- replace per-command whole-Journal replay in write authorities with replay of
  only their touched streams;
- publish a versioned immutable global read view from a successful projection
  rebuild; and
- perform one atomic ready-set dispatch across the coordination, WorkItem, Run,
  Runtime status, and Runtime capacity streams.

Implementing those changes without an amendment would reopen accepted
authorities outside owned scope. Omitting them would fail the authorized
concurrency and stale-view requirements.

## Amended S3-W5 boundary

S3-W5 remains the fifth and final Slice 3 WorkItem. No S3-W6 or thin
scheduler-, writer-, coordinator-, checkpoint-, or adapter-only WorkItem may
be created.

In addition to the existing S3-W5 vertical integration scope, its reviewed
contract may own bounded edits to:

- `internal/journal/store.go`
- `internal/journal/journal_test.go`
- `internal/work/run_authority.go`
- `internal/work/run_authority_test.go`
- `internal/authorization/authority.go`
- `internal/authorization/authority_test.go`
- `internal/projection/projection.go`
- `internal/projection/projection_test.go`

It may also own new S3-W5 Team execution, application integration, tests,
controlled fixtures, one architecture decision record, and its governance
evidence. The S3-W5 contract must enumerate every exact owned file before RED.

## Authority constraints

The amendment authorizes only these accepted-boundary changes:

1. `journal.Store.ReadStreamSet` reads a validated unique related-stream set
   within one SQLite read transaction and returns deeply copied Events plus the
   exact stream heads observed by that transaction.
2. Run/WorkItem and AgentGrant write commands replay only the stream set each
   command touches. Full snapshots may still deliberately replay the complete
   Journal for diagnostic/read compatibility, but no write command may call
   `ReadAll`.
3. `AppendBatchIfStreamHeads` remains the sole authoritative multi-stream
   writer and conflict detector. No second writer, sequence allocator, retry
   loop, lock authority, cache authority, or checkpoint authority may be
   introduced.
4. A successful full projection rebuild may atomically publish an immutable
   `GlobalReadView`. Its version is the canonical SHA-256 digest of sorted
   stream-head and head-Event-ID tuples. Typed accessors copy only requested
   records. Projection failure preserves the previous view and Snapshot.
5. S3-W5 may add `team-execution/<team_instance_id>` Events and one bounded
   authoritative dispatch method that CASes that stream together with all
   touched WorkItem, Run, Runtime status, and Runtime capacity streams.
6. AgentGrant raw material remains memory-only and hash-only in the Journal.
   Evidence bytes remain content-addressed in the accepted immutable Artifact
   Store; Evidence metadata remains an append-only Journal fact.

The existing Bridge, credential, filesystem/process, Runtime observation,
saved-Team, and Evidence Artifact Store trust boundaries remain closed except
for consumption through their accepted APIs.

## Required final vertical result

The S3-W5 contract and Candidate must close together:

- exactly one Main and at most two active SubAgents;
- a pure deterministic ready-set planner with dependency ordering;
- independent WorkItem, Run, Grant, and Evidence lineages;
- generation fencing, deterministic terminal aggregation, restart/recovery,
  and no unbounded hidden retry;
- concurrent schedulers where exactly one dispatch CAS wins;
- no Runtime capacity oversell;
- stale global-view and stale-generation rejection; and
- a controlled local SQLite/Bridge-compatible fixture canary proving two
  independent SubAgents overlap in execution, crash recovery creates no
  duplicate Run or Evidence lineage, and failed projection rebuild preserves
  the previous immutable view.

## Explicit exclusions

Phase 1 does not add:

- model-context checkpointing;
- process-memory or process-image checkpointing;
- an authoritative checkpoint or cache;
- incremental projection checkpoints;
- automatic user Runtime, Provider/model, credential, or resident-service
  activation.

Incremental projection checkpoints are only a post-Slice-3 hardening candidate
if measured rebuild cost later justifies a separately governed change.

## Review and exit

This amendment requires a fresh independent read-only review before the S3-W5
contract is frozen. The original whole-Slice exit gate remains unchanged:
S3-W5 must pass its implementation review and local commit, followed by the
fresh whole-Slice Reviewer. Slice 4 remains closed until that Reviewer passes.

VERDICT: FROZEN
