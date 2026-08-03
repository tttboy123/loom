# Phase 2B Side-task Handoff Roadmap Amendment

**Date**: 2026-08-03  
**Status**: FROZEN — Plan Repair 2 Re-review pending  
**Authority**: Phase 2A Product Owner sign-off and authorization to enter
Phase 2B contract governance  
**Baseline branch**: `codex/loom-platform-slice2`  
**Baseline commit**: `7a27149b31db7ffeefefb86c449ba448c3250fca`

## 1. Purpose and planning-only boundary

This amendment reconciles the queued Side-task Handoff input with the accepted
Phase 2, Phase 3A, Phase 3B and later scheduling roadmap. It is planning-only.
It authorizes no product code, Event/schema change, Journal write, Runtime or
Provider call, daemon activation, credential change, network action, live
canary, push or merge.

Owned planning paths are limited to:

- `PRODUCT-PLAN.md`;
- `TECH-PLAN.md`;
- `docs/CURRENT.md`;
- this amendment; and
- its independent Plan Review evidence.

## 2. Release placement

Phase 2B joins the planned `v0.2.0` release train as the single vertical
`Side-task Handoff and Parent Decision` boundary. It follows accepted Phase 2A
and may undergo contract governance independently of Phase 3A, but it does not
become a new Phase 3A entry prerequisite and does not replace or reduce Phase
3A's core delivery of the Versioned Evolution Asset Library and exact Runtime
materialization. Separate concurrent governance or implementation Candidates
must have independently reviewed owned paths and one writer per branch; P2B
and Phase 3A must not implement concurrently against shared files.

The version target remains a planning label, not a delivery claim:

| Target | Boundary | Planned outcome |
|---|---|---|
| `v0.1.x` | Phase 1 + accepted Phase 2A | Local execution kernel and ordinary-user controlled product chain |
| `v0.2.0` | Phase 2B + Phase 3A | Side-task handoff/parent decision plus Versioned Evolution Assets and exact Runtime materialization |
| `v0.2.1 experimental` | Phase 3B | One default-off governed sandbox backend |
| `v0.3+` | Phase 4 and separately governed work | Interoperability, optional Web, more Runtimes and shared capabilities |

Phase 2B does not consume the Phase 3A Skill/Template Library, Phase 3B
SandboxBackend, v0.3 Plan Compiler/economic routing, or v0.4 Agent Scheduling
Framework boundary.

## 3. One WorkItem exit rule

Phase 2B may contain exactly one vertical WorkItem:

```text
P2B-W1 Side-task Handoff and Parent Decision
```

There is no P2A-W4 or P2B-W2. Summary, Artifact publication, ContextPacket,
Archive, Drawer, application service, IPC, TUI, Swift client, daemon wiring,
Projection or restart recovery must not be split into wrapper-only WorkItems.

The P2B-W1 contract must freeze exact owned files, all new Event names and
payload schemas, authority/CAS boundaries, replay/migration compatibility,
Mandatory RED, verification matrix, rollback, one default-offline controlled
canary and one atomic local commit.

## 4. Product boundary

The single vertical journey is:

```text
parent Mission
  -> user explicitly creates or confirms a bounded Side-task
  -> independent WorkItem / Run / Attempt / generation / Grant / Evidence
  -> immutable authorized summary Artifact
  -> versioned SideTaskHandoff projection
  -> report-only delivery or exact typed parent decision
  -> at most one bounded ContextPacket / continuation
  -> restart and Projection rebuild preserve the same outcome
```

Supported Side-task purposes are research, comparison, diagnosis,
verification and read-only review. A planner may submit only a Proposal. A
Proposal is zero-write with respect to Side-task lifecycle, WorkItem, Run,
Grant, Evidence, capacity and dispatch facts. It may not create or dispatch a
Side-task without explicit user confirmation. Phase 2B v1 has no policy
auto-admission success path: the accepted baseline Rules/Approval facts do not
jointly encode a revocable, expiring and budget-bearing standing `report_only`
policy. Every supplied policy reference therefore returns `capability_gap`
with zero write. Adding an accepted policy path requires a separately reviewed
Rules capability that freezes exact version, digest, scope, budget, risk
ceiling, expiry and revocation semantics; P2B-W1 must not invent that authority.

Every admitted Side-task has a separate WorkItem, Run, Attempt, claim
generation, least-privilege AgentGrant, Evidence and capacity lineage. It
cannot inherit the parent's Grant, credential, allowed operations, budget,
generation or broader resource scope. The current reusable dispatch boundary
is `TeamCoordinator` plus `work.Authority.DispatchTeamReadySet` and the existing
generation/CAS ports. P2B adds no resident scheduler, queue, lease manager,
worker pool or dispatch lane.

Supported modes are `report_only`, `decision_required` and `merge_candidate`.
Supported decisions are `absorb`, `continue`, `request_followup`, `pivot`,
`discard`, `archive` and `cancel_parent`. `merge_candidate` enables absorption
only after accepted source Evidence and independent verification.

`decision_required` is a durable gate on future parent dispatch or
continuation, not arbitrary process suspension and not a checkpoint. Timeout
leaves the parent durably paused or appends the accepted `human_required`
outcome; it never guesses a decision. `merge_candidate` creates only a verified
Candidate or bounded parent input. It never applies, merges or modifies source,
artifacts or originals by itself.

### Decision effects

| Decision | Sole permitted durable effect |
|---|---|
| `absorb` | CAS-create at most one bounded ContextPacket for the exact handoff and permit at most one continuation of the same parent generation. |
| `continue` | Record the decision and permit at most one continuation without a ContextPacket. |
| `request_followup` | Record the decision and create at most one new explicitly bounded follow-up Proposal/lineage; it does not dispatch without admission. |
| `pivot` | Record a bounded scope-delta Candidate and leave execution gated until the normal confirmation/admission authority accepts it. |
| `discard` | Record terminal disposition with zero ContextPacket, continuation content or parent-content disclosure. |
| `archive` | Record archival disposition with zero ContextPacket or parent-content disclosure; the immutable Artifact remains governed evidence. |
| `cancel_parent` | Invoke the existing parent cancellation authority for the exact current generation and require its terminal/Evidence closure; it never fabricates cancellation state. |

Every row uses the exact reviewed writer/Event set and one multi-stream CAS.
Two concurrent or replayed decisions have one winner or the same idempotent
result. A losing CAS performs zero ContextPacket, follow-up, continuation,
cancellation or parent mutation.

The Native GUI and Bubble Tea TUI must use the same Go application service and
strict local IPC. The Side-task Drawer presents bounded purpose, status,
authorized findings, Evidence/Artifact digest, uncertainty, risk, scope delta,
usage/cost and available next actions. It never manufactures authority from a
visible control.

## 5. Authority and non-disclosure

The Event Journal remains the sole durable state authority. P2B-W1 must reuse:

- the existing Journal and `AppendBatchIfStreamHeads` CAS writer;
- the existing WorkItem, Run/Attempt, generation, AgentGrant and Evidence
  lineage;
- the existing Team execution coordinator and Scheduler boundary;
- the existing Projection and immutable versioned `GlobalReadView`; and
- the existing content-addressed Evidence Artifact store.

It must not introduce a second Journal, StateWriter, Projection, Scheduler,
queue database, client-side writer, completion authority or checkpoint.

The full authorized summary is an immutable content-addressed Artifact. The
exact versioned Artifact schema contains Side-task and parent IDs, purpose,
status, source generation, summary version, `what_happened`, authorized
findings, Evidence and Artifact references/digests, risk, uncertainty, scope
delta, closed decision options, proposal-only recommendation, observed-or-
unavailable usage/cost and creation time. Journal facts store lifecycle,
lineage, version, exact canonical digest and parent decision, not the full
summary.

Artifact publication and Journal commit are deliberately not presented as one
filesystem/SQLite transaction. The required order is: canonicalize and bound
the authorized summary; publish content-addressed bytes; read back and verify
the digest; then reference that digest through the sole Journal CAS authority.
A published but unreferenced Artifact is a safe non-authoritative orphan that
may be reused by digest or removed only by separately governed bounded
cleanup. A Journal fact may never reference an unverified Artifact. Missing or
corrupt referenced bytes fail closed and cannot create a ContextPacket or
continue a parent. Restart reconciliation must cover crashes before publish,
after publish/before CAS, during CAS, and after CAS/before response.

The parent receives only one bounded versioned `ContextPacket` derived from an
allowlist of authorized Artifact fields. It never receives raw transcript
concatenation, hidden reasoning, credentials, raw Grant material, complete
sensitive prompts or per-token output.

The P2B-W1 contract must freeze exact strict schemas for the summary Artifact,
`SideTaskHandoff`, `ContextPacket`, command/result and read/IPC surfaces. It
must define canonical digest encoding and provenance plus byte, count, string,
collection and field bounds. Unknown/duplicate fields, null in required
collections, oversized values, non-canonical bytes and digest mismatch fail
closed in Go and Swift.

Every authority-changing command is bound to the exact GlobalReadView version,
parent generation, Side-task generation, handoff version/digest and applicable
stream heads. Stale view/generation, wrong digest, unauthorized output,
identity drift and concurrent losing decisions fail closed. There is no hidden
retry or automatic Provider fallback.

## 6. Minimum acceptance

The P2B-W1 contract and Candidate must prove:

1. one manually created Side-task completes without changing the parent;
2. a planner Proposal is zero-write; explicit confirmation succeeds; every
   policy reference fails `capability_gap` with zero write until a separately
   reviewed Rules capability can express exact accepted/unexpired/unrevoked
   `report_only` policy version, digest, scope, budget and risk semantics;
3. the Side-task has independent least-privilege WorkItem/Run/Attempt/
   generation/Grant/Evidence/capacity lineage and inherits no parent authority;
4. `report_only` delivers only the authorized summary;
5. `decision_required` durably gates future parent continuation until a typed
   decision, and timeout remains paused or becomes `human_required`;
6. `absorb` creates exactly one bounded ContextPacket and resumes only the
   correct parent generation;
7. `continue`, `request_followup`, `pivot` and `cancel_parent` each produce
   only the effect frozen in the decision table, with zero duplicate lineage,
   continuation or cancellation side effect;
8. `discard` and `archive` do not disclose Side-task content to the parent;
9. `merge_candidate` is unavailable until source Evidence and independent
   verification pass;
10. `merge_candidate` produces only a verified Candidate/bounded input and
    never applies or merges source or originals;
11. unauthorized output, stale view/generation, wrong digest and identity drift
   fail closed with zero parent mutation;
12. two concurrent decisions have exactly one CAS winner;
13. canonical schema bounds reject unknown/duplicate/null/oversized/non-
    canonical/digest-mismatched summary, handoff, packet and IPC input;
14. allowlist packet derivation and negative scans prove no raw transcript,
    hidden reasoning, credential, raw Grant or complete sensitive prompt;
15. publish/read-verify/CAS crash-point tests prove no referenced missing
    Artifact, orphan authority, partial parent mutation or duplicate response;
16. daemon restart and Projection rebuild reproduce the same handoff and pending
   decision without redispatch;
17. Projection refresh failure preserves the previous immutable view;
18. Native GUI and real-socket TUI complete the same visible journey through
    the shared Go service/IPC; and
19. no duplicate Run, Evidence, ContextPacket, parent continuation or external
    side effect occurs.

## 7. Planned authoritative plan edits

After independent Plan Review PASS only:

- `PRODUCT-PLAN.md` will add Phase 2B before Phase 3A, state the one-WorkItem
  exit rule, add the product journey and update the `v0.2.0` mapping;
- `TECH-PLAN.md` will add Phase 2B entry conditions, single-writer/CAS,
  Artifact/Journal split, SideTaskHandoff/ContextPacket lifecycle, replay and
  minimum acceptance while preserving Phase 3A/3B conditions; and
- `docs/CURRENT.md` will record Plan Review status and keep P2B product writes
  locked until independent Contract Review PASS and Mandatory RED.

## 8. Explicit exclusions

- no Phase 3A asset library or Runtime materialization implementation;
- no Phase 3B sandbox implementation;
- no v0.3 Plan Compiler or economic routing;
- no v0.4 scheduling framework, second scheduler or worker pool;
- no Autopilot, standing order, webhook or remote callback;
- no multi-user, Marketplace or shared asset authority;
- no Provider/session-context checkpoint;
- no credential, network, remote Provider or production activation;
- no automatic Provider fallback or unbounded retry; and
- no P2A-W4, P2B-W2 or wrapper-only governance unit.

VERDICT: FROZEN FOR PLAN RE-REVIEW
