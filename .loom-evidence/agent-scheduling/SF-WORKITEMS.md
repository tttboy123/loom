# v0.4.0 Agent Scheduling Framework — WorkItem Freeze (SF-W1/W2/W3)

Status: `ACCEPTED` — combined Gate 1 Contract Re-review 2 PASS
(`GATE1-CONTRACT-REVIEW-2.md`, P0=P1=P2=0); no product code until SF-W1 RED

Date: `2026-08-04`

Governed by `SF-EXIT-CONTRACT.md` and ADR-0014. Exactly three vertical
WorkItems; no SF-W4 and no thin WorkItem. Each WorkItem's final
`source-lock.json` must be a subset of its frozen allowlist below and exclude
`internal/projection/team_execution_test.go`, root docs, configuration,
drafts, generated builds, and other slices' evidence. All new IPC actions,
events, and projections use the schemas frozen in `SF-EXIT-CONTRACT.md` §5 and
the existing Journal CAS/dispatch/lease primitives; no second authority is
created.

Shared-path rules (frozen): `internal/localipc/protocol.go` and
`internal/localipc/protocol_test.go` are P3A-owned and reopened by each
WorkItem only additively (new action IDs/params; existing action semantics and
wire format unchanged). Parallel Candidates never both include `protocol.go`
— the Conflict Arbiter serializes it as a shared owned path, so each WorkItem
extends the protocol against the prior accepted commit. The queue projection
is a separate rebuildable read model in `internal/projection/queue.go`; the
accepted P3A core `internal/projection/projection.go` is not reopened.
The event-driven Scheduler has no separate package: it is implemented by
SF-W1 admission (`internal/queue/admission.go`,
`internal/queue/conflict_arbiter.go`, `internal/queue/gap_proposal.go`) and
SF-W2 dispatch (`internal/schedule/router.go`).

---

## SF-W1 — Queue State/Projection + Admission/Eligibility/Conflict Arbiter

Purpose: Journal-authoritative development queue with readiness, eligibility,
conflict arbitration, and gap-to-successor Admission (compilation and
deduplication only — no execution).

Owned-file allowlist:

- `internal/queue/model.go`, `internal/queue/model_test.go`
- `internal/queue/projection.go`, `internal/queue/projection_test.go`
- `internal/queue/admission.go`, `internal/queue/admission_test.go`
- `internal/queue/conflict_arbiter.go`, `internal/queue/conflict_arbiter_test.go`
- `internal/queue/gap_proposal.go`, `internal/queue/gap_proposal_test.go`
- `internal/projection/queue.go`, `internal/projection/queue_test.go`
- `internal/api/local_queue.go`, `internal/api/local_queue_test.go`
- `internal/app/local_queue.go`, `internal/app/local_queue_test.go`
- `internal/localipc/protocol.go` (additive actions only), `internal/localipc/protocol_test.go`
- `internal/tui/queue.go` (new queue/board view), `internal/tui/queue_test.go` (new)
- `apps/macos/Sources/LoomLocalAppCore/LocalQueueModels.swift` (new),
  `apps/macos/Tests/LoomLocalAppTests/LocalQueueModelsTests.swift` (new)

Owned-file Amendment 1 (frozen,
`SF-W1-OWNED-FILE-AMENDMENT-1.md`, independent Review 1 PASS) adds the
additive-only delivery wiring required by the frozen journey:

- `cmd/loomd/product_daemon.go` (additive: queue API build/wire +
  `queue_snapshot`/`queue_command` routing; existing methods unchanged),
  `cmd/loomd/product_daemon_test.go` (queue routing regression tests)
- `internal/tui/model.go` (additive: `ScreenQueue` + screens-list entry +
  Tab/refresh wiring + View dispatch; existing screens unchanged)
- `apps/macos/Sources/LoomLocalAppContractProbe/main.swift` (additive:
  queue action modes for the SF-W1 journey; existing modes unchanged)

No modification of Journal store internals, existing authority writers, or
accepted P2A/P2B/P3A files.

Deliverables:

1. `QueueJob` events appended only via Journal CAS; `queue` projection
   rebuildable from the Journal (restart/replay identical).
2. Admission validates: eligibility (user/previously accepted bounded-policy
   authority), DAG readiness, absence of duplicate active work, vertical
   capability (no thin decomposition), owned-path/mutex/resource conflict
   arbitration, exit conditions, verification/Review/integration strategy.
3. Gap Proposal: exactly one digest-bound `gap_id` per authorized source; stale
   or unauthorized evidence cannot create a successor; duplicates converge on
   one `gap_id`; disposition `observe|reject|merge_duplicate|human_required|
   propose_successor` recorded; no WorkItem side effect from discovery.
4. Queue/board IPC surface (GUI + TUI read the same projection; neither client
   is an authority).

RED (SF-W1): unmet eligibility not admitted; owned-path conflict not parallel;
same mutex not parallel; DAG cycle rejected; duplicate active work rejected;
unauthorized/stale evidence cannot create a successor; duplicate gap
observations converge on one `gap_id`; Journal rebuild reproduces the queue
projection; protected-authority path claim fails closed.

Journey (SF-W1): GUI+TUI real journey — user queues two jobs with a shared
owned path; both clients observe admission and the conflict arbiter
serializing the second; a third DAG cycle proposal is rejected with a visible
typed error; one authorized failed Run produces exactly one Gap Proposal with
no WorkItem side effect (visible in both clients). The journey includes an
explicit restart/reconnect checkpoint: the daemon restarts mid-journey and
both clients observe the rebuilt queue projection through their own
production reads with no duplicate facts.

Verification: focused + focused race per package; full Go matrix + Swift full
at the integration checkpoint; replay/restart; Decomposition Compiler matrix.

## SF-W2 — Ephemeral Worker Pools + Lease/Reconciler + routing

Purpose: short-lived worker pools with one-claim-per-worker, lease/generation
fencing, bounded recovery (retry/backoff, crash reclamation, stale-generation
rejection, no hidden infinite retry), and Dev/Test/Repair/Review routing with
repair aging/weighted fairness.

Owned-file allowlist:

- `internal/schedule/worker_pool.go`, `internal/schedule/worker_pool_test.go`
- `internal/schedule/lease.go`, `internal/schedule/lease_test.go`
- `internal/schedule/reconciler.go`, `internal/schedule/reconciler_test.go`
- `internal/schedule/router.go`, `internal/schedule/router_test.go`
- `internal/schedule/recovery.go`, `internal/schedule/recovery_test.go`
- `internal/work/worker_execution.go`, `internal/work/worker_execution_test.go`
- `internal/api/local_workers.go`, `internal/api/local_workers_test.go`
- `internal/app/local_workers.go`, `internal/app/local_workers_test.go`
- `internal/localipc/protocol.go` (additive), `internal/localipc/protocol_test.go`
- `internal/tui/workers.go` (new pool/workers view), `internal/tui/workers_test.go` (new)
- `apps/macos/Sources/LoomLocalAppCore/LocalWorkerModels.swift` (new),
  `apps/macos/Tests/LoomLocalAppTests/LocalWorkerModelsTests.swift` (new)

Existing substrate reused read-only: `internal/work/run_authority.go`
lease/generation primitives, `internal/runtime/piadapter` process runner, and
the seven-class failure routing in `SF-EXIT-CONTRACT.md` §2.

Deliverables:

1. A pool dispatches exactly one Job per worker; workers get fresh context and
   exit on completion; capacity never oversold.
2. `AttemptClaimed`/lease with generation; expiry ⇒ exactly one new
   generation; stale/late results rejected with zero side effects.
3. Crash reclamation (before/after CAS seams recorded truthfully); bounded
   retry/backoff; exhaustion routes to `repair` or `human_required`.
4. Deterministic Test Executor persists result Evidence; a classified failure
   enters Repair (aging/weighted fairness prevents starvation); the Developer
   cannot act as Reviewer; the Reviewer cannot write.

RED (SF-W2): lease expired/stale generation rejected; crashed worker reclaimed
with exactly one new generation; late result rejected; hidden infinite retry
impossible; test failure routed to Repair not Integration; Reviewer write
denied; Repair not starved by new Development; capacity oversell rejected;
same mutex not parallel; duplicate attempt idempotent.

Journey (SF-W2): GUI+TUI real journey — two independent workers run in parallel
on separate Candidates (both clients observe); one worker crashes at a frozen
seam (exit 93 pattern) and the Reconciler reclaims with one new generation; a
stale result from the old generation is rejected visibly; a failing test
enters Repair; the read-only Reviewer is denied a product write.
The journey includes an explicit restart/reconnect checkpoint: the daemon
restarts mid-journey, the Reconciler reclaims any open lease from the rebuilt
Journal with exactly one new generation, and both clients observe the rebuilt
worker/attempt state with no duplicate effects.

Verification: focused + focused race; full matrix at integration checkpoint;
crash/replay/restart; lease expiry and generation fencing tests; journey
frozen evidence.

## SF-W3 — Single-writer Integration + controlled self-host canary + Timeline/Attention

Purpose: the single target-branch Integrator with CAS/generation fencing,
versioned release handoff and later-Run binding, rollback proof, the
controlled self-host canary, and Timeline/Attention including authorized
streaming node output (projection + client push, never an authority).

Owned-file allowlist:

- `internal/integration/integrator.go`, `internal/integration/integrator_test.go`
- `internal/integration/canary.go`, `internal/integration/canary_test.go`
- `internal/integration/release.go`, `internal/integration/release_test.go`
- `internal/observability/timeline.go`, `internal/observability/timeline_test.go`
- `internal/observability/attention.go`, `internal/observability/attention_test.go`
- `internal/observability/streaming.go`, `internal/observability/streaming_test.go`
- `internal/api/local_integration.go`, `internal/api/local_integration_test.go`
- `internal/api/local_observability.go`, `internal/api/local_observability_test.go`
- `internal/app/local_integration.go`, `internal/app/local_integration_test.go`
- `internal/tui/attention.go`, `internal/tui/attention_test.go` (new views)
- `apps/macos/Sources/LoomLocalAppCore/LocalTimelineModels.swift` (new),
  `apps/macos/Sources/LoomLocalAppUI/TimelineView.swift` (new),
  `apps/macos/Tests/LoomLocalAppTests/LocalTimelineModelsTests.swift` (new)
- `internal/localipc/protocol.go` (additive push actions), `internal/localipc/protocol_test.go`

Deliverables:

1. Integrator applies one reviewed Candidate to the target branch; competing
   or stale integration loses via CAS/generation fencing with no duplicate
   effects; integration produces a versioned release Candidate (source +
   Evidence + dependency digests).
2. Controlled self-host canary: one offline Run, locked runtime fixture,
   exact model/skill bindings; no duplicate Run/Evidence/effect; no oversell;
   projection failure preserves the old view; crash recovery correct.
3. Timeline/Attention projection with streaming node output captured only
   when authorized and generation-bound; unauthorized/stale/malformed frames
   not published; Timeline never triggers Agent authority.
4. Later Run binds the exact new revision/digest; rollback restores the prior
   eligible version without rewriting Run/Evidence history; running
   Teams/Attempts keep their original bindings.

RED (SF-W3): two Integrators compete ⇒ one CAS winner; stale integration
rejected with zero effects; canary double-run blocked by idempotent CAS;
slow client gets one effect on reconnect (no hidden retry); projection failure
preserves the old view and rebuilds on restart; unauthorized/stale/malformed
streaming frames not published; rollback restores the prior version without
history rewrite; later Run binds the exact new revision.

Journey (SF-W3): GUI+TUI real journey — one reviewed Candidate is integrated
by the single Integrator (visible in both clients); a competing stale
integration loses; the canary runs once offline with exact bindings; streaming
node output appears in Timeline/Attention after authorization; a later Run
binds the new revision; rollback restores the prior version.
The journey includes an explicit restart/reconnect checkpoint: the daemon
restarts mid-integration and both clients observe the rebuilt release/adoption
state with no duplicate Event/Evidence/effect.

Verification: focused + focused race; full Go matrix + Swift full/TSAN/Release;
replay/CAS/restart/security; Decomposition Compiler; canary journey with the
frozen alternative evidence surface; dual Result (Product + Operational/Trace)
reviews; Whole-Candidate Review; exact staging; one atomic local commit per
WorkItem and a final whole-slice review.

VERDICT: `ACCEPTED` — GATE 1 CONTRACT RE-REVIEW 2 PASS
