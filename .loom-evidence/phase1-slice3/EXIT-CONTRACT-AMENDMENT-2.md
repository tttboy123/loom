# Phase 1 Slice 3 Exit Contract Amendment 2 — Global Grant Identity and Bounded Dispatch CAS

- Date: `2026-07-26`
- Baseline: `cd1e594`
- Parent:
  `.loom-evidence/phase1-slice3/EXIT-CONTRACT-AMENDMENT-1.md`
- Repairs:
  `.loom-evidence/phase1-slice3/EXIT-CONTRACT-AMENDMENT-1-REVIEW-1.md`

This amendment preserves Amendment 1 except where the two P1 findings are
replaced by the exact constraints below.

## AgentGrant global collision authority

S3-W5 must add one Journal-authoritative global identity stream:

```text
agent-grant-identity/v1
```

Every new `AgentGrantIssued` Event must be committed in the same
`AppendBatchIfStreamHeads` transaction as exactly one
`AgentGrantIdentityReserved` Event. The reservation contains the Grant ID,
token SHA-256 hash, Run ID, Grant stream ID/sequence, and Grant issue Event ID.
Replaying this one touched identity stream must preserve the accepted S3-W3
global `ErrGrantIDCollision` and `ErrGrantTokenCollision` behavior across all
Runs. Raw token material never enters the identity Event.

Before normal Grant write commands are enabled against a pre-S3-W5 Journal, an
explicit bounded maintenance operation must initialize the identity stream:

- it may deliberately call `ReadAll` because it is a separately invoked
  compatibility rebuild, not an Issue/Authorize/Revoke write command;
- it validates accepted historical Grant facts, appends deterministic missing
  reservations in batches no larger than the Journal Event and head limits,
  and finally appends an initialization marker bound to the canonical digest
  of all indexed historical Grant issue Event IDs;
- each batch uses `AppendBatchIfStreamHeads`; committed partial progress is
  idempotently resumable, but a conflict is returned to the caller without an
  automatic retry loop;
- initialization fails closed on malformed, conflicting, or unindexable Grant
  history;
- a fresh empty Journal still receives the initialization marker before the
  first Grant can be issued.

After initialization:

- `Issue`, `Authorize`, and `Revoke` may replay only their touched WorkItem,
  Run, per-Run Grant, and global identity streams;
- no normal Grant write command may call `ReadAll`;
- `Issue` must CAS the Run, per-Run Grant, and identity heads atomically;
- Snapshot/diagnostic reads may still deliberately perform a full replay.

The identity stream is an append-only Journal fact written by the accepted
AgentGrant Authority. It is not a Projection, cache, table, random retry,
second writer, or second state authority.

## Bounded CAS head expansion

S3-W5 may raise the accepted
`AppendBatchIfStreamHeads` expectation limit from eight to exactly sixteen.
The Event batch limit remains 32.

Sixteen covers the maximum authorized dispatch:

- one Team execution stream;
- up to three WorkItem streams;
- up to three Run streams;
- up to three Runtime status streams; and
- up to three Runtime capacity streams.

The product planner remains limited to exactly one Main plus at most two
SubAgents, and one ready-set dispatch may contain at most three nodes. Duplicate
head expectations, empty stream IDs, negative heads, invalid Events, and
seventeen or more expectations remain rejected before any transaction starts.

The S3-W5 contract must test both:

- two SubAgents on distinct Runtime instances in one atomic dispatch; and
- rejection of seventeen expectations without partial mutation.

## Superseding rule

Where Amendment 1 says only that AgentGrant commands replay touched streams,
this amendment supplies the required global identity stream and compatibility
rebuild. Where the accepted S3-W2 contract fixes eight expectations, this
amendment replaces that number with sixteen only for the existing
`AppendBatchIfStreamHeads` authority.

All other Amendment 1 authority constraints, vertical acceptance, explicit
exclusions, no-W6 rule, and whole-Slice review gate remain unchanged.

VERDICT: FROZEN
