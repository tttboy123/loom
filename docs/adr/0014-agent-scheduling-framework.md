# ADR-0014 — Agent Scheduling Framework (v0.4.0)

Status: `ACCEPTED` — combined v0.4.0 Gate 1 Contract Re-review 2 PASS
(`.loom-evidence/agent-scheduling/GATE1-CONTRACT-REVIEW-2.md`, P0=P1=P2=0)
after `SF-CONTRACT-REPAIR-1.md` closed the seven P1 and ten P2 items of
`GATE1-CONTRACT-REVIEW-1.md`; no product code authority until RED is
established for SF-W1

Date: `2026-08-04`

## Context

Loom's next major version is the Agent Scheduling Framework / concurrent
development pipeline (Product Owner queued route input). After Phase 3A
(versioned Evolution Assets + Run-bound materialization, accepted at
`7d5f0b01`), the product must govern concurrent Agent development: a
Journal-authoritative development queue, event-driven Scheduler, ephemeral
Worker Pools, independent Candidates, deterministic Test, read-only Review,
single-writer Integration, and a controlled self-host canary. The queued
self-evolution closure (`2026-08-04-governed-self-evolution-pipeline-queued-input.md`)
folds a governed gap-to-successor feedback loop into the same three vertical
delivery boundaries.

The reference competitor is Kun Agent (`KunAgent`/`Kun`): architecture design
only, never copied code, never a model client, Provider route, key, or agent
loop. The Linux process-model draft
(`.loom-drafts/loom-scheduler-supplement-2026-07-27.md`) is reference material
for scheduling policy (fairness, priority, bounded retry), not an authority.

## Decision

Loom's development pipeline is composed of exactly one of each of the
following authorities, all rebuilt from the single Event Journal:

```text
single Event Journal (append-only, AppendBatchIfStreamHeads CAS)
  -> Queue Projection (admission-ready jobs, DAG-ready, claims, lanes)
  -> Event-driven Scheduler (dispatch only what is ready; never polls by an
     Agent)
  -> short-lived Worker Pools (Dev / Test / Repair / Review), one claim per
     worker, fresh context, exit after completion
  -> independent worktree Candidate per unit of work (own branch)
  -> single Integrator (the only writer of the target branch)
  -> controlled self-host canary (one Run, exact version bindings)
```

No second Scheduler, Journal, StateWriter, Projection, queue database, asset
authority, or integration writer exists anywhere in the system. Timeline /
Attention / streaming node output are projections and push presentations, never
an authority. Model context, session IDs, process memory, and uncommitted
worktrees are never state authority.

## Non-decisions (explicitly out of scope)

- Phase 3B / `v0.3.x` routing, Web/Marketplace/shared/multi-user, standing
  automation outside the framework, Provider fallback/checkpoint,
  session/model-context checkpoint as authority, auto-approval bypassing the
  human lane, hidden reasoning or per-token Journal writes, auto-activation,
  unreviewed scripts, network/paid/real-user configuration, push/merge remote,
  overwriting user Skills, or a second authority/queue database.
- The framework does not own Provider keys, model clients, or agent loops.

## Core invariants (frozen for the Exit Contract)

1. Ordinary conversation never creates a Team or dispatches a worker.
2. At-least-once dispatch + idempotent CAS + lease/generation fencing; late or
   stale results are rejected with no side effect.
3. A worker claims exactly one task, gets fresh context, and exits on
   completion; no Agent permanently busy-polls.
4. Parallel Development only when the DAG is ready and owned paths / mutex
   keys / resource claims do not conflict; each Candidate has an independent
   branch/worktree.
5. Test Executor is a deterministic process, not a Reviewer; the Reviewer is
   read-only and independent; the Integrator is the only target-branch writer.
6. Failure is classified into exactly seven classes (see Exit Contract) and
   routed truthfully; Repair is prioritized with aging/weighted fairness so
   new tasks cannot starve repairs.
7. Focused/impact checks run per Candidate; the full repository/race matrix
   runs at risk gates and integration/slice checkpoints.
8. Streaming node output is captured only with authorization, bound to the
   exact generation, and rejected when stale/unauthorized/malformed; retry and
   degraded semantics are decided by policy, never invented by the Scheduler.
9. Every WorkItem has a real GUI + TUI cross-client journey (per the accepted
   alternative-verification method: production native window + real PTY +
   production Swift client over the real socket).
10. A governed gap-to-successor loop closes the feedback boundary: observed
    gaps become successor Candidates only through Admission, and accepted
    integrations become versioned capabilities that only a later Run binds.

## Consequences

- Queue state, leases, decisions, and adoption state are all rebuildable from
  the Event Journal and immutable artifacts; crash/restart reconstructs them.
- A single Integrator and single-writer discipline prevent merge races and
  duplicate effects; competing/stale integrations lose via CAS/generation
  fencing.
- The three vertical WorkItems (SF-W1 Queue/Admission, SF-W2 Worker
  Pools/Recovery, SF-W3 Integration/Canary/Timeline) are the only delivery
  boundaries; no SF-W4 or thin wrapper WorkItem is created.
- Bounded recovery (retry/backoff, crash reclamation, stale-generation
  rejection, no hidden infinite retry) is frozen into SF-W2; streaming node
  output + Timeline/Attention is frozen into SF-W3.

VERDICT: `ACCEPTED` — GATE 1 CONTRACT RE-REVIEW 2 PASS
