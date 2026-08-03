# P2B-W1 Side-task Handoff and Parent Decision Contract

**Date**: 2026-08-03  
**Status**: FROZEN — Contract Repair 2 Re-review pending  
**Authority**: Product Owner Phase 2A sign-off, active Goal and reviewed Phase
2B roadmap amendment  
**Baseline branch**: `codex/loom-platform-slice2`  
**Baseline commit**: `7a27149b31db7ffeefefb86c449ba448c3250fca`  
**Risk**: STRICT — new Journal facts, Work Authority methods, Artifact
publication, parent continuation/cancellation intent, strict IPC and native UI

The reviewed roadmap authority is byte-bound to Plan Repair 2 amendment
SHA-256
`fd37c198a5b63d64ee30084f8a458be6c0ef4eafce01d68f9e6c33e18a68fe0e`
and Plan Review 3 SHA-256
`595be75bccc531c418f3adeec09affb662e035507fddd5956dc7add79e9a2024`.
The amendment retains those exact reviewed bytes; its separate Review 3 is the
PASS authority. Earlier Plan Reviews remain preserved history.

## 1. One vertical WorkItem

This is the only Phase 2B WorkItem:

```text
P2B-W1 Side-task Handoff and Parent Decision
```

It creates no P2A-W4, P2B-W1a, P2B-W1b, P2B-W2, summary-only, writer-only,
adapter-only, projection-only, IPC-only or UI-only governance unit. A repair
stays inside this contract. A required Event/authority/schema expansion not
listed here stops `HUMAN_REQUIRED` before that change.

The Candidate preserves all unrelated modified/untracked paths and excludes
`internal/mcp/**`, `.codex/**`, `.loom-drafts/**`, Phase 1 live roots/evidence,
`AGENTS.md`, `README.md`, `PROGRESS.md`, generated `apps/macos/.build/**`,
credentials, network Providers, push and merge.

## 2. Product outcome

An ordinary user can perform this journey without entering an internal ID,
socket path, SQLite path, cursor, Grant, Runtime ID or Provider environment
variable:

```text
Mission
  -> New side task
  -> purpose + mode + bounded request
  -> zero-write proposal/preflight
  -> explicit Confirm
  -> independent child execution lineage
  -> authorized summary + Evidence
  -> Side-task Drawer
  -> report delivery or typed parent decision
  -> exact bounded ContextPacket / continuation / follow-up intent
  -> reconnect and restart preserve the same authority
```

Purposes are exactly `research`, `comparison`, `diagnosis`, `verification` and
`read_only_review`. Modes are exactly `report_only`, `decision_required` and
`merge_candidate`. Decisions are exactly `absorb`, `continue`,
`request_followup`, `pivot`, `discard`, `archive` and `cancel_parent`.

Side-task lifecycle status is exactly one of `admitted`, `running`,
`awaiting_review`, `report_delivered`, `decision_required`,
`merge_candidate_pending`, `merge_candidate_ready`, `decided`, `archived`,
`human_required`, `failed` or `cancelled`. Parent effect status is exactly
`none`, `pending`, `completed` or `human_required`.

The Native GUI and Bubble Tea TUI call the same Go application service through
the same strict local IPC. Neither client executes a Runtime, writes SQLite or
the Artifact store, creates IDs, reconstructs a Grant, retries a CAS loss,
decides policy or claims completion.

## 3. Current authority reuse

The only durable mutation path is:

```text
strict product command
  -> LocalProductHandoffService
  -> existing work.Authority + TeamCoordinator / DispatchTeamReadySet
  -> evidence.Store publish/read-verify
  -> Journal AppendBatchIfStreamHeads CAS
  -> existing Projection / immutable GlobalReadView
  -> LocalProductReadService
  -> strict IPC -> Native GUI / TUI
```

Rules:

1. Event Journal remains the sole lifecycle and decision authority.
2. `work.Authority` is extended by the exact P2B methods below; no second
   writer, StateWriter, Journal or transaction coordinator is introduced.
3. `TeamCoordinator` and `DispatchTeamReadySet` remain the only child execution
   coordinator/dispatch authority. No resident Scheduler, queue, new lease
   manager, worker pool or dispatch lane is added.
4. Existing WorkItem, Run/Attempt, generation, AgentGrant, Evidence, Runtime
   capacity and verification facts remain the child execution lineage.
5. Projection, GlobalReadView, UI state and daemon in-flight maps are
   replaceable caches, never authority.
6. Executor completion remains `ready_for_review`; accepted source plus
   independent Verifier Evidence closes a `merge_candidate` handoff.
7. No hidden retry exists. Exact idempotency returns the same result; a CAS loss
   returns a typed conflict and performs zero follow-up mutation.

## 4. Proposal and admission

The `propose` operation validates and canonicalizes the request and returns a
bounded proposal digest. It performs zero Journal, Artifact, WorkItem, Run,
Grant, Evidence, capacity, execution or daemon-registry write.

`create` requires the exact current proposal digest and one of:

- `confirmed=true`, representing the explicit user action; or
- a policy reference that passes the policy port described below.

P2B-W1 v1 intentionally exposes no standing-policy creation or revocation
surface. The accepted baseline Rules/Approval facts do not jointly encode a
revocable, expiring and budget-bearing standing `report_only` policy.
Therefore the production policy port is explicitly fail-closed and every
policy-reference admission returns `capability_gap` with zero write. Tests use
absent, stale, synthetically revoked, mismatched, over-budget and non-
`report_only` references only to prove rejection; they do not claim an
accepted-policy success path. A future success path requires a separately
reviewed Rules capability. Explicit confirmation is the only v1 admission
path.

Explicit admission records only safe bounded metadata and an input Artifact
digest. The child uses a new deterministic Side-task execution Team ID and the
parent's currently validated Main Agent/Runtime binding, but it receives a new
WorkItem, Run, Attempt, claim generation, least-privilege Grant, Evidence and
capacity reservation. It never copies the parent Grant, allowed operations,
credential, budget, claim ID/generation or broader resource scope.

`decision_required` creation is accepted only at a product safe gate with no
unjoined parent dispatch in flight. It gates future parent start/control/
continuation; it is not process suspension or a checkpoint.

## 5. Exact Artifact schemas and crash ordering

All P2B Artifact JSON uses UTF-8, RFC 3339 Nano UTC timestamps, sorted unique
collections, no unknown fields, no duplicate keys, no null required
collections, no floating-point numbers and canonical Go `encoding/json` bytes.

### 5.1 Side-task input Artifact v1

Exact fields:

```text
schema_version, side_task_id, parent_mission_id, parent_task_id,
parent_run_id, parent_claim_generation, purpose, mode, title,
authorized_request, permission_scopes, proposal_digest, created_at
```

### 5.2 Authorized summary Artifact v1

Exact fields:

```text
schema_version, side_task_id, parent_task_id, purpose, status,
source_generation, summary_version, what_happened, authorized_findings,
evidence_references, artifact_references, risk, uncertainties, scope_delta,
decision_options, recommended_option, recommendation_authority,
usage_observed, usage_microunits, usage_currency, created_at
```

`recommendation_authority` is exactly `proposal_only`. Usage is either an
accepted integer microunit fact plus ISO currency or `usage_observed=false`,
zero amount and empty currency. No cost is invented.

`authorized_findings`, `uncertainties`, `scope_delta`, `decision_options` and
`permission_scopes` are arrays of strings. `risk` is `low`, `medium` or
`high`. Each `evidence_references` entry is exactly
`{evidence_id,digest,kind}` and each `artifact_references` entry is exactly
`{digest,kind}`. Reference kind is one of `source`, `verifier`, `input`,
`summary` or `context_packet` as applicable.

### 5.3 ContextPacket Artifact v1

Exact fields:

```text
schema_version, context_packet_id, side_task_id, parent_task_id,
parent_run_id, parent_claim_generation, side_task_generation,
handoff_version, handoff_digest, summary_artifact_digest,
authorized_findings, risk, uncertainties, scope_delta, created_at
```

It is derived only from the summary allowlist. It contains no raw transcript,
hidden reasoning, credential, raw Grant, complete sensitive prompt or
unaccepted field.

### 5.4 Bounds

- input Artifact: at most 16 KiB;
- summary Artifact: at most 64 KiB;
- ContextPacket: at most 16 KiB;
- title/purpose/status/mode/IDs/digests: at most 128 UTF-8 bytes each;
- authorized request and `what_happened`: at most 4096 UTF-8 bytes each;
- at most 32 findings, 16 uncertainty/risk/scope entries and 32 references;
- each list string: at most 512 UTF-8 bytes; and
- every digest: lowercase SHA-256 hex over exact canonical bytes.

The only commit order is:

```text
canonicalize and bound
-> content-addressed publish
-> read back exact bytes and verify digest
-> Journal CAS references digest
```

A pre-CAS Artifact is a non-authoritative orphan. It may be reused by exact
digest; deletion requires separately governed bounded cleanup. A Journal fact
never references unverified bytes. Missing/corrupt referenced bytes fail closed
before ContextPacket, continuation or decision effects. Restart tests cover
crash before publish, after publish/before CAS, during CAS and after CAS/before
response.

## 6. Exact new Journal streams and Events

All Events below use schema version 1, UTC `emitted_at`, exact JSON decoding,
deterministic IDs/idempotency keys and the existing Journal validation.

### `side-task/<side_task_id>`

1. `SideTaskAdmitted` sequence 1:

```text
side_task_id, parent_mission_id, parent_team_instance_id, parent_task_id,
parent_run_id, parent_claim_generation, side_execution_team_instance_id,
purpose, mode, title, admission_kind, proposal_digest,
input_artifact_digest, expected_view_version, permission_scopes,
policy_stream_id, policy_version, policy_digest, budget_microunits,
budget_currency
```

Policy fields are empty/zero for explicit confirmation.

2. `SideTaskHandoffCommitted` next sequence:

```text
side_task_id, side_execution_team_instance_id, source_work_item_id,
source_run_id, source_generation, source_evidence_id,
source_evidence_digest, verifier_evidence_id, verifier_evidence_digest,
handoff_version, handoff_digest, summary_artifact_digest, mode, status,
usage_observed, usage_microunits, usage_currency,
decision_timeout_seconds, decision_deadline
```

`decision_timeout_seconds` is zero for `report_only` and otherwise between 60
and 2,592,000. Work Authority derives `decision_deadline` once from the
handoff-commit operation time. It is never caller time.

3. `SideTaskDecisionCommitted` next sequence:

```text
decision_id, side_task_id, parent_task_id, parent_run_id,
parent_claim_generation, side_task_generation, handoff_version,
handoff_digest, expected_view_version, decision,
context_packet_digest, effect_digest, resulting_status
```

4. `SideTaskDecisionTimedOut` next sequence when applicable:

```text
side_task_id, handoff_version, handoff_digest, source_handoff_event_id,
decision_deadline, timeout_observed_at, outcome
```

`outcome` is exactly `paused` or `human_required`.

### `parent-handoff/<parent_task_id>`

The same decision CAS appends exact schema-version-1 payloads. Required strings
are non-empty unless explicitly marked optional.

`ContextPacketCommitted`:

```text
context_packet_id, context_packet_version, context_packet_digest,
side_task_id, handoff_version, handoff_digest, summary_artifact_digest,
parent_task_id, parent_run_id, parent_claim_generation,
side_task_generation, continuation_execution_team_instance_id
```

`ParentContinuationAuthorized`:

```text
decision_id, side_task_id, handoff_version, handoff_digest,
parent_mission_id, parent_team_instance_id, parent_task_id, parent_run_id,
parent_claim_generation, side_task_generation, action,
context_packet_digest, continuation_execution_team_instance_id,
continuation_plan_digest, effect_digest
```

`action` is `absorb` or `continue`; `context_packet_digest` is required only
for `absorb` and empty for `continue`.

`ParentFollowupProposed`:

```text
decision_id, side_task_id, handoff_version, handoff_digest,
parent_task_id, parent_run_id, parent_claim_generation,
followup_proposal_digest, effect_digest
```

`ParentScopePivotProposed`:

```text
decision_id, side_task_id, handoff_version, handoff_digest,
parent_task_id, parent_run_id, parent_claim_generation,
scope_delta_digest, effect_digest
```

`ParentCancellationRequested`:

```text
decision_id, side_task_id, handoff_version, handoff_digest,
parent_mission_id, parent_team_instance_id, parent_task_id, parent_run_id,
parent_logical_node_id, parent_attempt_number, parent_claim_generation,
parent_execution_digest, cancellation_effect_digest, effect_digest
```

After consuming an authorized continuation or cancellation, the stream appends
`ParentHandoffEffectCompleted`:

```text
decision_id, side_task_id, effect_kind, effect_digest,
execution_team_instance_id, execution_plan_digest, terminal_status,
terminal_evidence_id, terminal_evidence_digest
```

For continuation, the execution identity/digest is the deterministic
continuation lineage and terminal Evidence is required. For cancellation it
identifies the parent execution and its existing canonical terminal Evidence.
Exact replay is idempotent.

`discard` and `archive` append no parent-handoff content Event. `report_only`
handoff delivery appends no parent mutation Event.

No existing Event payload is modified. No SQLite migration is required.
Pre-P2B Journal state projects an empty Side-task collection. Unknown future
Event versions fail rebuild while the read service preserves its previous
immutable view.

## 7. Exact Work Authority API

`internal/work/side_task_handoff.go` adds methods on the existing
`work.Authority` only:

```text
AdmitSideTask
CommitSideTaskHandoff
DecideSideTaskHandoff
ExpireSideTaskDecision
CompleteParentHandoffEffect
SideTaskHandoff
```

Each reads only the touched streams through `ReadStreamSet`, validates exact
heads/view/generations/lineage, replays before reading operation time, and
writes through one `AppendBatchIfStreamHeads`. Handoff commit proves the exact
child TeamExecution terminal, WorkItem/Run generation, source Evidence and
independent verifier Evidence required by mode. Decision CAS validates the
parent Run generation, parent execution digest and handoff Artifact digest.
`CompleteParentHandoffEffect` validates the authorized effect and the exact
existing TeamExecution/Evidence terminal before marking consumption. No retry
loop follows a conflict.

Decision effects are exact:

| Decision | Durable effect |
|---|---|
| `absorb` | one ContextPacket + one continuation authorization |
| `continue` | one continuation authorization, no packet |
| `request_followup` | one Proposal digest, no admission/dispatch |
| `pivot` | one scope-delta Candidate digest, no apply/dispatch |
| `discard` | Side-task disposition only; zero parent content |
| `archive` | Side-task disposition only; zero parent content |
| `cancel_parent` | one cancellation request; existing cancellation authority must later provide terminal Evidence |

Exact replay returns the same events. A different decision or stale binding
conflicts. Concurrent callers produce one CAS winner.

## 8. Execution and restart lifecycle

`LocalProductHandoffService` uses explicit Side-task adapters. It never asks the
existing Mission binding source to resolve a synthetic saved Team.

`ProjectionSideTaskExecutionBindingSource` accepts both the parent saved Team
ID and the distinct child execution Team ID. It resolves and revalidates only
the parent Team, its Main Agent, Runtime profile/instance, model, auth mode,
capacity and current GlobalReadView. It returns those authority references plus
the child ID. The child ID is a deterministic UUID derived from the
`side_task_id`, proposal digest and input Artifact digest; it is never looked
up or projected as a saved Team.

`BuiltInSideTaskExecutionCompiler`, defined in
`internal/app/local_product_handoff.go`, builds the child `ExecutionPlan` with
that deterministic child ID and the validated parent Main Agent/Runtime. It
generates new deterministic WorkItem/Run IDs and dispatch frames, read-only
permission scopes, a new least-privilege Grant and independent Evidence. It
does not call `ResolveMissionExecutionBinding(child_id)` and does not create a
fake TeamInstance.

Restart input is exactly the projected `SideTaskAdmitted` fact plus the
read-back verified input Artifact. The compiler re-derives the same child ID,
plan/node title, prompt, WorkItem/Run IDs, semantic contracts and plan digest.
Any parent Team/Agent/Runtime drift, missing Artifact or digest mismatch becomes
`human_required` before dispatch.

The service lifecycle is:

1. publish and read-verify the bounded input Artifact;
2. admit through existing Work Authority;
3. compile one child Team execution through the Side-task compiler;
4. run through existing TeamCoordinator;
5. derive only authorized output plus accepted Evidence into the summary;
6. publish and read-verify the summary Artifact; and
7. commit the handoff through Work Authority.

The daemon holds only a bounded in-memory map of at most 64 in-flight Side-task
operations. Startup first calls the accepted Mission execution recovery for
existing non-terminal Team executions. It then performs one bounded P2B
reconciliation pass over at most 64 admitted Side-tasks and 64 authorized-but-
uncompleted parent effects. It never guesses, loops or redispatches a terminal
lineage.

The existing Mission backend receives a read-only `ParentContinuationGate`.
A pending `decision_required` handoff rejects future parent start/recovery/
continuation for the exact generation. Losing/stale decisions cannot unlock it.

`ParentHandoffEffectReconciler` consumes only Journal-authorized effects:

- `absorb` read-verifies the ContextPacket, derives a bounded prompt only from
  its allowlist and runs one deterministic continuation Team execution through
  the same TeamCoordinator. The source parent generation is the authorization
  fence; the continuation has its own WorkItem/Run/generation/Grant/Evidence.
- `continue` runs the same deterministic continuation without a ContextPacket,
  using only the parent TeamExecution safe node title already in Journal.
- `cancel_parent` calls a new internal
  `CancelRecoveredOrActiveMissionExecution` method on the existing
  `AuthoritativeMissionExecutionBackend`. That method validates the exact
  execution digest/node/attempt/generation, uses the active or startup-
  recovered flight, cancels it and waits for existing canonical cancellation/
  terminal Evidence. It exposes no new IPC method and invents no terminal.

Continuation execution ID and plan digest are deterministically bound in the
decision CAS. A crash after dispatch reuses the same Coordinator lineage.
After exact continuation/cancellation terminal visibility, the reconciler calls
`CompleteParentHandoffEffect`. A crash after the effect but before completion
therefore observes and records the same terminal on restart, without a second
effect. ContextPacket consumption is explicit, bounded and restart-closed.

## 9. Strict product and wire surface

The one new IPC method is exactly:

```text
side_task_handoff
```

Operations are exactly `propose`, `create`, `read` and `decide`. No alias,
version-suffixed method or CLI-text forwarding is allowed. Every command binds
schema version, request/correlation ID, parent Mission/Team/task/Run identity,
parent generation, expected GlobalReadView version and operation-specific
proposal/handoff/effect digest. Authority-affecting unknown fields reject.

Exact request/result schemas follow. All listed fields are required. Optional
string values encode as `""`, never `null`; all collections are required JSON
arrays. Timestamps are RFC 3339 Nano UTC strings. Versions, counts,
generations, durations and microunits are JSON integers.

### `propose`

Request:

```text
schema_version=1, operation="propose", parent_mission_id,
parent_team_instance_id, parent_task_id, parent_run_id,
parent_claim_generation, purpose, mode, title, authorized_request,
permission_scopes, decision_timeout_seconds, expected_view_version,
correlation_id
```

Result:

```text
schema_version=1, status="proposal", proposal_digest, view_version,
purpose, mode, title, permission_scopes, decision_timeout_seconds,
requires_confirmation=true, policy_available=false
```

### `create`

Request contains every `propose` request field, changes `operation` to
`"create"`, and adds:

```text
proposal_digest, confirmed, policy_stream_id, policy_version, policy_digest
```

Explicit v1 requires `confirmed=true` and empty/zero policy fields. Result:

```text
schema_version=1, side_task_id, status, view_version,
side_execution_team_instance_id, proposal_digest,
handoff_version, handoff_digest
```

Before handoff commit, `handoff_version` is zero and `handoff_digest` empty.

### `read`

Request:

```text
schema_version=1, operation="read", side_task_id,
expected_view_version, correlation_id
```

Result:

```text
schema_version=1, side_task_id, parent_mission_id,
parent_team_instance_id, parent_task_id, parent_run_id,
parent_claim_generation, side_execution_team_instance_id,
purpose, mode, title, status, source_generation, handoff_version,
handoff_digest, summary_artifact_digest, what_happened,
authorized_findings, evidence_references, artifact_references,
risk, uncertainties, scope_delta, decision_options,
recommended_option, recommendation_authority, usage_observed,
usage_microunits, usage_currency, decision_deadline,
available_decisions, effect_status, view_version
```

### `decide`

Request:

```text
schema_version=1, operation="decide", side_task_id, parent_mission_id,
parent_team_instance_id, parent_task_id, parent_run_id,
parent_logical_node_id, parent_attempt_number, parent_claim_generation,
parent_execution_digest, side_task_generation, handoff_version,
handoff_digest, decision, effect_digest, expected_view_version,
correlation_id
```

Result:

```text
schema_version=1, side_task_id, decision, status, effect_status,
context_packet_digest, continuation_execution_team_instance_id,
view_version
```

Enums are only the purposes, modes, decisions and lifecycle/effect statuses
frozen by this contract. Any other value rejects.

The Local Product snapshot schema increments from 2 to 3 and adds an always-
array `side_tasks` collection. IPC envelope version remains 1. The Go daemon,
strict Swift client/contract probe and TUI ship atomically. Required collection
`null`, unknown/duplicate fields, oversized input, wrong identity/digest and
schema mismatch reject in Go and Swift.

Each snapshot `side_tasks` item has the exact `read` result fields except
`schema_version` and `view_version`; its collections are always arrays. The
snapshot's top-level schema version/view version remain authoritative. Page
cursor fields are not added in v1 because the collection is bounded to 64; a
65th projected Side-task makes the snapshot partial with reason exactly
`side_tasks_limit_reached` rather than silently truncating. If an existing
higher-priority stale/health reason is already present, that reason remains in
the single existing `reason` field while `partial=true`; the Side-task API
`read` remains available by exact ID and never silently drops an item from an
authority-affecting operation.

Method-specific failures use the existing strict IPC error envelope and closed
safe reason codes only: `invalid_request`, `capability_gap`, `not_found`,
`conflict`, `stale_view`, `stale_generation`, `digest_mismatch`,
`human_required` and `internal`. A failed response has no result body. Error
messages never include Artifact bytes, prompt/output text, credentials, raw
Grant material, filesystem paths or internal error strings.

The Side-task Drawer shows only bounded purpose, mode, status,
`what_happened`, authorized findings, Evidence/Artifact references, risk,
uncertainty, scope delta, usage/cost truth and currently allowed next actions.
Tentative child output is labelled tentative and is never parent context.

## 10. Exact owned files

Only these paths may change after Contract Review PASS.

### Authority, Artifact and Projection

- `internal/evidence/side_task_handoff.go`
- `internal/evidence/side_task_handoff_windows.go`
- `internal/evidence/side_task_handoff_test.go`
- `internal/work/side_task_handoff.go`
- `internal/work/side_task_handoff_test.go`
- `internal/projection/side_task_handoff.go`
- `internal/projection/side_task_handoff_test.go`
- `internal/projection/projection.go`
- `internal/projection/projection_test.go`
- `internal/projection/global_read_view.go`
- `internal/projection/global_read_view_test.go`

### Application, API and existing parent gate

- `internal/app/local_product_handoff.go`
- `internal/app/local_product_handoff_test.go`
- `internal/app/local_product_execution.go`
- `internal/app/local_product_execution_test.go`
- `internal/api/local_product_handoff.go`
- `internal/api/local_product_handoff_test.go`
- `internal/api/local_product_read.go`
- `internal/api/local_product_read_test.go`
- `internal/api/local_product_mission_test.go`

### Daemon, strict IPC and TUI

- `cmd/loomd/product_daemon.go`
- `cmd/loomd/product_daemon_test.go`
- `internal/localipc/protocol.go`
- `internal/localipc/protocol_test.go`
- `internal/localipc/swift_contract_test.go`
- `internal/tui/model.go`
- `internal/tui/model_test.go`

### Strict Swift native product

- `apps/macos/Sources/LoomLocalAppCore/LocalProductHandoffModels.swift`
- `apps/macos/Sources/LoomLocalAppCore/LocalIPCClient.swift`
- `apps/macos/Sources/LoomLocalAppCore/LocalProductModels.swift`
- `apps/macos/Sources/LoomLocalAppCore/LocalProductStore.swift`
- `apps/macos/Sources/LoomLocalAppCore/MissionOrchestration.swift`
- `apps/macos/Sources/LoomLocalAppUI/MissionWorkbench.swift`
- `apps/macos/Sources/LoomLocalAppContractProbe/main.swift`
- `apps/macos/Tests/LoomLocalAppTests/LocalProductHandoffModelsTests.swift`
- `apps/macos/Tests/LoomLocalAppTests/LocalIPCClientTests.swift`
- `apps/macos/Tests/LoomLocalAppTests/LocalProductModelsTests.swift`
- `apps/macos/Tests/LoomLocalAppTests/LocalProductStoreTests.swift`
- `apps/macos/Tests/LoomLocalAppTests/MissionOrchestrationTests.swift`
- `apps/macos/Tests/LoomLocalAppTests/LocalProductExperienceViewTests.swift`

### Governance and accepted roadmap inputs

- `PRODUCT-PLAN.md`
- `TECH-PLAN.md`
- `docs/CURRENT.md`
- `.loom-evidence/phase2a/PHASE2A-PRODUCT-OWNER-SIGNOFF.md`
- `.loom-evidence/phase2a/WHOLE-PHASE-EXIT-MATRIX.md`
- `.loom-evidence/phase2a/WHOLE-PHASE-SOURCE-EVIDENCE-LOCK.json`
- `.loom-evidence/phase2a/WHOLE-PHASE-REVIEW.md`
- `.loom-evidence/phase2a/WHOLE-PHASE-VERIFICATION.md`
- `.loom-evidence/phase2a/whole-phase-swift-timeline-fixture-race-repair.md`
- `.loom-evidence/plan-amendments/2026-08-03-phase2b-side-task-handoff-roadmap-amendment.md`
- `.loom-evidence/plan-amendments/2026-08-03-phase2b-side-task-handoff-roadmap-amendment-review-1.md`
- `.loom-evidence/plan-amendments/2026-08-03-phase2b-side-task-handoff-roadmap-amendment-review-2.md`
- `.loom-evidence/plan-amendments/2026-08-03-phase2b-side-task-handoff-roadmap-amendment-review-3.md`
- `.loom-evidence/phase2b/P2B-W1/**`

The pre-existing W3-only modified Swift Timeline test remains an accepted
Phase 2A sign-off Candidate input and may be staged only at final exact-path
inspection. No other pre-existing dirty path is owned.

## 11. Mandatory RED

Before product implementation, tests must compile only after adding test-local
references to the frozen absent symbols and must fail for missing behavior.
RED evidence must prove at least:

1. proposal is zero-write and create requires exact explicit confirmation;
2. every unavailable/stale/revoked/mismatched/over-budget policy reference is
   zero-write;
3. child WorkItem/Run/Attempt/generation/Grant/Evidence/capacity is independent
   and least privilege;
4. publish/read-verify/CAS crash points cannot create a missing reference,
   orphan authority or partial parent effect;
5. summary/handoff/packet/IPC unknown, duplicate, null, oversized,
   non-canonical and digest-mismatched inputs reject;
6. manual/report-only completion does not change the parent;
7. decision-required gates the parent and timeout remains paused or
   `human_required`;
8. every decision produces only its exact table effect;
9. absorb creates one packet/continuation for the exact generations;
10. discard/archive leak zero content; merge-candidate requires independent
    verification and never applies source;
11. two concurrent decisions have one winner;
12. the distinct child execution ID resolves only through the frozen parent-
    binding adapter, and restart re-derives the exact same child plan/IDs/
    digests without a synthetic saved Team;
13. startup resumes projected Mission flights before reconciling authorized
    parent effects; absorb/continue/cancel recovery completes exactly once,
    including a crash after effect terminal visibility but before the
    completion Event;
14. restart/rebuild reproduces handoff/pending decision without redispatch;
15. Projection failure preserves the prior immutable view;
16. real Go service/server to strict Swift probe and TUI traverse the same
    visible journey; and
17. no raw transcript, hidden reasoning, credential, raw Grant, complete
    sensitive prompt, per-token Journal fact or duplicate side effect exists.

RED may not be a syntax error, broken fixture or unrelated failure.

## 12. Verification and review gates

After RED, in order:

1. focused Go tests for evidence/work/projection/app/API/IPC/daemon/TUI;
2. repeated CAS, concurrency, crash/restart and Projection-failure tests;
3. real SQLite replay/rebuild and exact Event/payload/stream-head assertions;
4. deterministic loopback child component with Supervisor/Grant/frame/source
   Evidence/Verifier Evidence/terminal closure;
5. real `LocalProductReadService -> Go IPC Server -> Go/Swift/TUI` fixture;
6. whole-repository `go test`, `go test -race`, `go vet`, module/tidy, format,
   diff and non-disclosure checks;
7. full Swift tests, strict contract probe, Thread Sanitizer and Release build;
8. fresh independent Implementation Review PASS with P0/P1/P2 all zero;
9. one source-locked controlled offline canary;
10. independent Result Review PASS;
11. fresh whole-P2B Candidate Review PASS; and
12. exact staging plus one atomic local commit.

## 13. One controlled offline canary

Only after Implementation Review PASS, consume one fresh isolated canary with:

- a new 0700 root and 0600 SQLite/manifest/Artifact files;
- no network, Provider credential, OAuth, paid call or remote side effect;
- deterministic loopback child execution through existing TeamCoordinator,
  Supervisor, new least-privilege Grant, authorized frames, source Evidence,
  independent Verifier Evidence and terminal;
- one manually confirmed `decision_required` Side-task;
- one `absorb` decision, exactly one ContextPacket and parent continuation;
- restart/rebuild, stale/conflicting decision rejection and byte/digest checks;
- real private product socket, strict Swift contract probe, scripted TUI and
  one controlled native-window Side-task Drawer journey;
- process/socket/lock cleanup and no duplicate authority or external effect.

The canary is consumed after one run. Failure is preserved; there is no
replacement canary without a reviewed reopening inside this same contract.

## 14. Stop conditions and rollback

Stop `HUMAN_REQUIRED` before the affected action if:

- implementation needs an unlisted Event, Event field, schema, authority or
  owned file;
- a safe gate requires arbitrary process suspension or a checkpoint;
- exact restart needs a second durable store or guessed prompt/state;
- policy auto-admission would require a new policy authority;
- credentials, networking, remote Provider, push or merge becomes necessary;
- dirty-file overlap cannot be isolated; or
- the one WorkItem cannot close the journey safely.

Rollback is the one atomic P2B-W1 commit plus canary-local root. Rollback does
not delete Journal history, accepted Evidence, user credentials, Teams,
Phase 2A evidence or unrelated files.

VERDICT: FROZEN FOR CONTRACT RE-REVIEW
