# Product Interaction, Handoff, Auth, Routing, and Scheduling Queued Input

Status: `QUEUED` — non-authoritative and non-executing.

Date: 2026-08-01

Authority input: the Product Owner accepted the proposed release placement and
authorized recording all discussed product capabilities, features, and
optimizations in the future work queue.

This file is future contract input only. It does not modify or reopen the
current Phase 2A Candidate, create P2A-W4, amend an accepted authority boundary,
authorize product code or schema changes, start a daemon or Runtime, call a
Provider, change credentials, use a live-canary allowance, or change the
delivery status of any capability.

## Reconciliation with the accepted roadmap

The existing roadmap already assigns:

- Phase 2A and `v0.1.x` to the ordinary-user controlled product chain;
- Phase 3A and `v0.2.0` to Versioned Evolution Assets and exact Runtime
  materialization;
- Phase 3B and `v0.2.1 experimental` to one optional governed sandbox; and
- `v0.3+` to interoperability and separately governed later capabilities.

This queue entry does not replace those assignments. Before any implementation,
a reviewed roadmap amendment must reconcile the additions below with
`PRODUCT-PLAN.md`, `TECH-PLAN.md`, and
`2026-08-01-v0.2.0-phase3a-3b-queued-input.md`. In particular, Side-task
Handoff may join the `v0.2.0` release train without displacing Phase 3A assets;
credential hardening must not silently consume the reserved `v0.2.1
experimental` sandbox label; and the `v0.3.0` planning/routing workstream must
coexist with, rather than erase, the existing Phase 4 interoperability target.

## Product north star

Loom Code is the software-development profile of Loom, not a replacement for
the wider Loom product boundary. It is task allocation, governed execution,
verification, evidence, and recovery rather than a provider-switching product.

The ordinary-user experience remains chat/task first:

```text
goal
-> mode, budget, privacy, and permission preview
-> explicit Start
-> governed plan and execution
-> evidence-backed result or decision request
-> acceptance, follow-up, rollback, or archive
```

Agent/DAG/Grant/generation/Journal details use progressive disclosure. Ordinary
conversation never creates a Team or authorizes execution.

## Release placement

| Release target | Planned boundary | Queued outcome |
|---|---|---|
| Phase 2A / `v0.1.x` | Interaction Continuity | Complete the already-governed Mission Room, Timeline, Attention, Provider/Runtime connection presentation, explicit Start/Stop/Resume, Evidence, and recovery journey. Do not duplicate accepted work. |
| Phase 2B / `v0.2.0` release train | Side-task Handoff and Parent Decision | Manual Side-task, structured summary Artifact, typed handoff decision, bounded parent ContextPacket, Side-task Drawer, restart recovery, and Projection rebuild. |
| `v0.2.x` hardening | Credential and Device Reliability | OS Secret Store persistence, scoped/revocable device identity, native Provider-auth observation, expiry/reconnect/logout/revocation, and fail-closed daemon identity. Exact patch version requires roadmap reconciliation. |
| `v0.3.0` workstream | Intelligent Planning and Economic Execution | Plan Compiler, TaskContract, Runtime capability registry, evaluated routing, privacy/budget policy, and accepted-task economics. |
| `v0.4.0` / next major | Agent Scheduling Framework | Journal-authoritative queue, event-driven Scheduler, ephemeral Worker Pools, independent Candidates, deterministic Test, read-only Review, single-writer Integration, and controlled self-host canary. |

Version targets are planning labels, not delivery claims. Every boundary remains
`TARGET` until its own contract, RED, implementation, verification, independent
Review, and applicable live evidence pass.

## Phase 2B — Side-task Handoff, Summary, and Parent Decision

Freeze at most one vertical product contract. Do not split Summary, Archive,
Drawer, ContextPacket, and Resume into thin WorkItems.

### Product behavior

1. A user may explicitly branch a Mission into a Side-task for research,
   comparison, diagnosis, verification, or read-only review.
2. A planner may propose a Side-task, but cannot create or dispatch it without
   user confirmation or a previously accepted bounded policy.
3. The Side-task has independent WorkItem/Run/Attempt/generation/Grant/Evidence
   lineage and cannot inherit broader authority from the parent.
4. Completion produces a structured `SideTaskHandoff`; the complete authorized
   summary is an Artifact/Evidence object, while Journal facts record lifecycle,
   digests, lineage, and decisions.
5. The parent receives only a bounded, versioned `ContextPacket`, never raw
   transcript concatenation, hidden reasoning, credentials, raw Grants, or
   complete sensitive prompts.
6. Mission Room exposes a Side-task Drawer with purpose, source, status,
   compact findings, Evidence, uncertainty, scope delta, usage/cost, and next
   actions.

### Required summary fields

- `side_task_id` and `parent_task_id`;
- purpose, status, source generation, and summary version;
- `what_happened` and authorized findings;
- Evidence/Artifact references and digest;
- risks, uncertainty, and scope delta;
- decision options and a recommended option marked only as a Proposal;
- usage/cost and creation time.

### Handoff modes

- `report_only`: asynchronous summary delivery; it cannot mutate the parent
  plan or authorize another side effect;
- `decision_required`: the parent pauses until an authorized decision; and
- `merge_candidate`: Evidence and verification must pass before absorption is
  available.

Typed decisions are `absorb`, `continue`, `request_followup`, `pivot`,
`discard`, `archive`, and `cancel_parent`. Every decision is bound to the exact
view version and generation. Timeout defaults to `human_required` or a paused
parent; only a preauthorized low-risk `report_only` policy may return without a
human decision.

### Minimum acceptance

- one manually created Side-task completes without changing the parent;
- authorized output becomes an immutable summary Artifact with Journal digest;
- absorb creates exactly one bounded ContextPacket and resumes the correct
  parent generation;
- discard/archive does not leak content into the parent;
- stale view/generation and unauthorized output are rejected;
- crash/restart rebuilds the same handoff and pending decision;
- Projection failure preserves the previous visible view; and
- no duplicate Run, Evidence, ContextPacket, or parent continuation occurs.

## `v0.2.x` — Credential and Device Reliability

This is Loom identity hardening, not Provider-token capture.

### Required boundary split

1. Loom user identity: local identity by default; remote/team identity only
   under a later explicit account contract.
2. Loom device/daemon identity: scoped, revocable, expiring, least-privilege
   credential with auditable issuance and revocation.
3. Provider identity: use Codex, Claude, Pi, or another Runtime's native auth;
   Loom records only validated capability/auth availability and never copies or
   parses private OAuth material.

### Product behavior

- a `Connect` action opens the approved browser/native flow and returns to Loom;
- transient authorization material is exchanged for a Loom-scoped device
  credential;
- secrets persist only in macOS Keychain or the applicable OS Secret Store;
- daemon configuration stores only non-secret references and presentation
  metadata;
- expiry, reauthorization, offline state, device listing, and explicit revoke
  are visible;
- logout clears local state and revokes the server-side Loom credential when a
  server credential exists; and
- unavailable or ambiguous identity fails closed without falling back to a
  different user, profile, Provider, or plaintext configuration.

Do not copy Multica's long-lived stateless browser JWT or plaintext PAT-in-JSON
storage as Loom's security model. Provider native OAuth and Loom account/device
auth remain separate trust boundaries.

## `v0.3.0` — Plan Compiler and Economic Execution

Freeze one cohesive planning/routing contract or a very small number of
vertical contracts. Do not create separate writer, selector, registry,
coordinator, or forwarding WorkItems.

### Plan Compiler

Compile a confirmed goal into an immutable TaskContract and executable
Candidate plan. Admission must validate:

- DAG cycles and dependency readiness;
- owned-path, authority, mutex, and resource conflicts;
- vertical capability and explicit exit conditions;
- integration and rollback strategy;
- budget, privacy, permission, and network constraints; and
- wrapper-only decomposition and unbounded retry rejection.

The compiler reuses TeamDefinition, Team Draft, WorkItem, Run/Attempt,
AgentGrant, Evidence, Journal, and Projection. It cannot introduce a parallel
planning or execution authority.

### Runtime capability registry

Every routing Candidate is backed by validated capability evidence for:

- Runtime/version/model and health;
- governed execution level and capacity;
- streaming, tool calling, MCP, Skill materialization, and session-resume hints;
- context/output limits;
- network, privacy, and local/cloud properties; and
- measured cost, latency, and acceptance performance.

Discovery alone never claims governed execution support.

### Economic routing

- advanced models may plan, diagnose high-risk failure, or independently
  review;
- low-cost/local models may execute only bounded patches, tests, documentation,
  or extraction for which their capability has passed an Eval;
- failure escalation is explicit and bounded, with no hidden Provider fallback
  or infinite retry;
- user-facing modes may include `Fast`, `Balanced`, `Private`, and
  `Economical`, each compiled into inspectable policy; and
- the primary economic metric is `Accepted Task Cost = total attempt cost /
  accepted tasks`, accompanied by first-pass acceptance, retry count, human
  intervention, latency, local execution share, and rollback rate.

### Minimum acceptance

- the same goal compiles deterministically under the same versioned inputs;
- invalid/conflicting plans fail before dispatch;
- privacy and budget constraints change eligibility rather than merely labels;
- a bounded local-model task and a high-risk cloud-model task route according
  to accepted capability evidence;
- stale capability views and forged cost data are rejected;
- explicit escalation creates independent lineage and preserves prior Evidence;
  and
- routing decisions and accepted-task economics rebuild from authoritative
  facts without a second queue or metrics authority.

## `v0.4.0` — Agent Scheduling Framework

The next-major scheduling capability must extend the existing Event Journal,
StateWriter, Projection, Team DAG, and generation fencing. It must not build a
second Scheduler, Journal, queue database, state writer, or projection.

### Required lanes

1. Planning/Admission;
2. New Development;
3. Repair Development;
4. Deterministic Test;
5. read-only Review;
6. single-writer Integration; and
7. HumanRequired/dead-letter.

### Required behavior

- Scheduler is resident; workers claim one task with fresh context and exit;
- at-least-once dispatch uses idempotent CAS, leases, and generation fencing;
- stale or late results are rejected;
- parallel Developers require DAG readiness and non-conflicting owned paths,
  mutexes, authorities, and resources;
- every Candidate uses an independent branch/worktree;
- deterministic Test is a process lane distinct from a read-only Reviewer;
- failure classification distinguishes product, test, infrastructure,
  contract, integration, cross-layer, and human-required outcomes;
- Repair has aging/weighted fairness so new work cannot starve;
- focused/impact checks run per Candidate, while full repository/race gates run
  at risk, integration, and Slice checkpoints; and
- only the Integrator writes the target branch.

At most three vertical WorkItems may implement this release:

A. Queue State/Projection plus Admission, Eligibility, and Conflict Arbiter.
B. Ephemeral Worker Pools plus Lease/Reconciler and Dev/Test/Repair/Review
   routing.
C. Single-writer Integration plus controlled self-host development canary,
   Timeline, and Attention.

Minimum acceptance proves two independent WorkItems develop in parallel,
development overlaps deterministic testing, failure routes to the correct
Repair lane, crashed workers are reclaimed, stale generations are rejected,
mutex conflicts never overlap, Reviewer cannot write product state, Integrator
is the sole target-branch writer, all queue state rebuilds from Journal, and the
self-host canary has no duplicate side effects.

## Cross-cutting safety and UX constraints

- Journal remains the only state authority; Projection, Timeline, Attention,
  Side-task Drawer, capability matrix, metrics, and queue views are rebuildable.
- Models, Sidecars, planners, reviewers, and workers submit Proposal/Candidate
  material only.
- Explicit Start and applicable approval remain required; ordinary chat never
  creates automation.
- An executor may reach `ready_for_review` but cannot mark its own WorkItem
  accepted or done.
- No raw Grant, credential, hidden reasoning, or per-token output enters
  Journal, Sidecar, logs, exported history, or Side-task summaries.
- Automatic retry, resume, handoff, routing, or scheduling never bypasses
  generation, Grant, Evidence, acceptance, approval, budget, or stop/revoke.
- Progressive disclosure keeps ordinary user surfaces focused on goal, work,
  result, cost, and required decisions; control-plane details remain available
  for inspection.

## Required governance sequence

For each future release boundary:

1. reconcile version placement with accepted product and technical plans;
2. freeze one Exit Contract with exact owned files, authority boundaries,
   migration/replay compatibility, acceptance, live limits, and rollback;
3. obtain an independent Contract Review `PASS`;
4. establish RED before product behavior changes;
5. implement without expanding the frozen scope or creating wrapper-only
   WorkItems;
6. run focused, impact, race, security, replay/restart, and appropriate live
   verification;
7. obtain a fresh independent Implementation Review `PASS`; and
8. update status only from accepted evidence.

No item in this queue is implementation authorization.

VERDICT: `QUEUED`
