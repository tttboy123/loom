# ADR-0008: Stream-set views and Team dispatch CAS

**Date**: 2026-07-26
**Status**: accepted
**Deciders**: lune, Codex

**Phase 2D amendment (2026-08-10)**: the three-node product ceiling in this ADR
is superseded by the accepted multi-Provider Team contract. The current shared
limit is nine Agent nodes, enforced consistently by Team definition, planning,
dispatch, authoritative replay, and projection replay. The Journal CAS,
one-main-Agent, bounded Attempt, independent lineage, and fail-closed scheduler
decisions below remain accepted. See
`../../.loom-evidence/phase2d/contracts/P2D-W2B-per-agent-provider-account.md`.

## Context

The Event Journal is Loom's state authority, but the accepted Run/WorkItem and
AgentGrant write paths replay the entire Journal before each command. That is
correct but scales with unrelated history and makes scheduling depend on a
mutable full-map copy.

Slice 3 also needs two schedulers to compete safely for one ready set, up to
two independent SubAgents to execute concurrently without Runtime capacity
oversell, and restart recovery without duplicate Run, Grant, or Evidence
lineage. A Projection can make reads efficient, but it cannot become a second
write authority or silently validate stale dispatch state.

AgentGrant adds a separate global constraint: random Grant IDs and token hashes
must remain unique across every Run. A per-Run touched-stream replay alone
cannot preserve that accepted collision authority.

## Decision

The Journal exposes one transaction-consistent related-stream-set read. It
returns deeply copied Events and the exact sequence and head Event ID for every
requested stream, including empty streams. Write commands replay only the
streams they touch and continue to commit solely through
`AppendBatchIfStreamHeads`.

AgentGrant identity uniqueness is represented by append-only
`agent-grant-identity/v1` reservation Events written in the same transaction as
each Grant issue Event. An explicit idempotent compatibility rebuild indexes
pre-S3-W5 Grant history before normal Grant commands are enabled. Raw token
material is never persisted.

Run identity uniqueness is represented the same way through
`work-run-identity/v1`. Every assignment reserves its Run ID in the same
transaction, so touched-stream create/assign and Team dispatch preserve
cross-WorkItem uniqueness. A separate idempotent compatibility rebuild indexes
accepted pre-S3-W5 assignments before normal Work write commands are enabled.

A successful full Projection rebuild atomically publishes a versioned
immutable `GlobalReadView`. Its version is a SHA-256 digest of canonical sorted
`stream ID / head sequence / head Event ID` tuples. Typed accessors copy only
the requested record. Rebuild failure preserves both the previous Snapshot and
the previous view.

Team scheduling uses:

- a pure deterministic ready-set planner over an immutable execution plan and
  GlobalReadView;
- one append-only `team-execution/<team_instance_id>` coordination stream; and
- one dispatch transaction that CASes the coordination stream together with
  every touched WorkItem, Run, Runtime status, and Runtime capacity stream.

The planner admits exactly one Main and at most two SubAgents. A dispatch
contains at most three nodes and has no automatic conflict retry. Competing
schedulers therefore produce one committed dispatch and one typed conflict.
Each node owns an independent WorkItem, Run, AgentGrant, and Evidence lineage.
Generation-bound Evidence and terminal facts drive deterministic dependency
release and final Team aggregation after restart.

The existing stream-head expectation limit rises from eight to sixteen; the
Event batch limit remains 32. `AppendBatchIfStreamHeads` remains the sole
multi-stream writer and conflict detector.

## Alternatives Considered

### Alternative 1: Keep whole-Journal replay for every command

- **Pros**: Simple global validation and no new read API.
- **Cons**: Every unrelated Event increases command cost; scheduler concurrency
  still lacks one ready-set transaction and immutable read provenance.
- **Why not**: It does not close the authorized state and concurrency
  optimization.

### Alternative 2: Treat Projection or a checkpoint as dispatch authority

- **Pros**: Fast reads and a compact scheduler implementation.
- **Cons**: A stale or failed cache could authorize execution and become a
  second state authority.
- **Why not**: Event Journal facts and CAS expectations, not cache freshness,
  must permit mutation.

### Alternative 3: Dispatch each ready node in its own transaction

- **Pros**: Fewer touched streams per transaction.
- **Cons**: Competing schedulers can split a ready set, oversell shared
  capacity, or leave partial WorkItem/Run creation.
- **Why not**: Slice 3 requires one atomic ready-set decision and deterministic
  recovery.

### Alternative 4: Global database table for Grant identity

- **Pros**: Straightforward unique constraints and lookups.
- **Cons**: The table would become an additional persistence authority and
  would need separate crash reconciliation with the Journal.
- **Why not**: Same-transaction Journal identity Events preserve one writer and
  remain replayable.

## Consequences

### Positive

- Command replay cost follows touched history rather than total history.
- Immutable read versions make scheduler provenance explicit.
- Two schedulers linearize through existing Journal CAS.
- Runtime capacity, Run generation, Grant identity, Evidence lineage, and Team
  terminal state remain recoverable from append-only facts.
- Projection failure cannot replace the last known-good read view.

### Negative

- Grant identity needs a one-time compatibility rebuild.
- Run identity needs a one-time compatibility rebuild.
- Dispatch Event construction and replay validate several related streams in
  one vertical authority.
- Full Projection rebuild remains intentionally non-incremental in Phase 1.

### Risks

- A missing head in the dispatch expectation set could weaken stale-view
  rejection. Exact touched-stream enumeration and distinct-Runtime concurrency
  tests mitigate this.
- Large ready sets could exceed bounded Journal transactions. Phase 1 limits
  the Team to three nodes and rejects larger plans.
- A compatibility rebuild could conflict with another writer. It returns the
  conflict without hidden retry and is explicitly resumable.
- Checkpoint pressure may appear as history grows. Metrics may justify a later
  hardening contract, but no model-context, process-image, authoritative, or
  incremental Projection checkpoint is introduced here.
