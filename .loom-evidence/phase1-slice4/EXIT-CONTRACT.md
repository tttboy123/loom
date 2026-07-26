# Phase 1 Slice 4 Exit Contract

Status: FROZEN — independent Exit Contract Review 1 PASS.

- Branch: `codex/loom-platform-slice2`
- Frozen baseline candidate: `7bb9881`
- Date: `2026-07-26`
- Authority: `TECH-PLAN.md` sections 6, 8, 9, 10, 11, 14, 15, and 16
- Plan amendment:
  `.loom-evidence/plan-amendments/2026-07-26-post-slice3-capability-roadmap.md`
- Scope: customer rules, approval, semantic recovery, verification, and
  authoritative WorkItem acceptance

This contract freezes Slice 4's exit list before product work. It authorizes no
Provider/model traffic, installed Runtime, daemon activation, external
approval action, notification delivery, or autonomous execution.

## Accepted prerequisites

Slice 1 through Slice 3 provide accepted:

- append-only Journal, `AppendBatchIfStreamHeads`, immutable Evidence Artifact
  Store, and rebuildable Projection/`GlobalReadView`;
- Agent/Runtime/Team Draft/saved-Team records and explicit user confirmation;
- bounded Bridge v1, WorkItem/Run authority, AgentGrant, managed execution,
  deterministic Team DAG dispatch, generation fencing, authorized tentative
  Frames, durable attempt capture, restart recovery, and terminal aggregation.

Those authorities remain closed. Slice 4 consumes them through reviewed APIs
and may reopen an accepted file only through a frozen, independently reviewed
amendment.

## Exit capabilities

Slice 4 exits only when every capability is `DONE`:

| Capability | Baseline | Required Slice 4 result |
|---|---|---|
| Customer Rule authority | `MISSING` | Versioned bounded conditions/effects, deterministic matching/merge, explicit scope, immutable inputs, and no model-defined authority |
| Durable approval | `MISSING` | `require_approval` atomically pauses the exact WorkItem/action, survives reopen, records approve/reject/expire/cancel once, and resumes only the authorized continuation |
| Output contract | `MISSING` | Pure deterministic `valid_nonempty/valid_empty/transient_empty/invalid` classification over bounded authorized output/Evidence metadata |
| Bounded recovery policy | `MISSING` | Pure `retry/fallback/degraded/blocked/human_required` decision using contract/rule/budget/attempt inputs; explicit `retry_at`; no hidden or Provider/model fallback |
| Verification and completion authority | `MISSING` | Evidence/verification facts, risk-routed independent Verifier, Executor limited to `ready_for_review`, and one Journal CAS authority for accepted/rejected/Done transitions |
| Controlled integration proof | `MISSING` | Local SQLite/fixture canary proving approval restart, empty-output recovery, retry exhaustion, stale generation rejection, independent Verifier isolation, terminal-once Done, and projection failure preservation |

## Maximum WorkItems

At most three Slice 4 product WorkItems are permitted:

1. `S4-W1 Customer Rule and Durable Approval Authority`
   - one new customer-policy and approval persistence boundary;
   - versioned rule validation, deterministic match/effect merge,
     `require_approval`, exact paused continuation, decision CAS, reopen, and
     projection;
   - no output classifier, retry scheduler, Verifier execution, or Done.
2. `S4-W2 Output Contract and Bounded Recovery Integration`
   - one semantic recovery boundary;
   - pure output classification, pure recovery decision, explicit Node/Attempt
     state and `retry_at`, bounded budget/attempt enforcement, and execution by
     the accepted Team coordinator;
   - workflow fallback only; no Provider/model fallback, hidden retry,
     acceptance, or Done.
3. `S4-W3 Evidence Verification and WorkItem Acceptance Integration`
   - one user-observable vertical verification capability;
   - authorized Evidence submission, deterministic acceptance, risk-routed
     independent Verifier, `ready_for_review`, rejected/retry routing, and
     terminal-once WorkItem Done through Journal CAS;
   - controlled whole-Slice integration and restart proof.

No rule-parser-only, approval-writer-only, decision-wrapper, retry-scheduler,
projection-only, Evidence-adapter-only, Verifier-router-only, or coordinator
WorkItem may be added. Internal helpers belong inside the applicable vertical
WorkItem.

## WorkItem admission and sizing

Each WorkItem must introduce at least one:

- new authoritative persistence transaction;
- new customer approval or verification trust boundary;
- new semantic recovery decision that changes an end-to-end attempt; or
- new user-observable acceptance capability.

Each frozen WorkItem must enumerate exact owned files, typed errors, Event
payloads, deterministic IDs, CAS expectations, replay/projection rules,
acceptance, focused/impact/full/race/vet/format/scope checks, and controlled
fixtures. If one WorkItem cannot close coherently, freeze a reviewed amendment
or stop `HUMAN_REQUIRED`; do not create S4-W4.

## Frozen semantics

### Rule and approval

- Rule definitions are versioned, validated, bounded, and customer-authored.
  Model output may propose a Candidate but cannot activate a Rule.
- Matching is deterministic. Effect precedence and incompatible combinations
  are explicit; iteration order never changes the decision.
- `require_approval` writes an ApprovalRequest and pauses the exact WorkItem,
  Run/Attempt, action, claim generation, contract/rule versions, and
  continuation digest in one authoritative transaction.
- Approve/reject/expire/cancel is terminal-once and identity-bound. Client
  presence, notification, cursor, or UI state cannot approve or resume work.
- Approval may resume only the frozen continuation and must revalidate current
  generation, Rule/contract binding, budget, Runtime status, and applicable
  authority heads. Stale approval conflicts without partial mutation.

### Output and recovery

- Classifiers and policies are pure, deterministic, bounded, and side-effect
  free. They accept immutable value inputs and return immutable decisions.
- `valid_empty` may be an accepted result when the WorkItem contract permits
  it. `transient_empty` does not itself authorize a retry.
- Recovery policy consumes classification, logical node/attempt, prior
  outcomes, retry ceiling, `retry_at`, budget, approval requirements, and
  allowed workflow fallback. It never sees raw credentials or hidden
  reasoning.
- Every retry uses a distinct Run, generation, Grant, and Evidence lineage.
  The scheduler performs no implicit loop and never invents success,
  fallback, or retry.
- Retry exhaustion becomes `degraded`, `blocked`, or `human_required` as
  explicitly decided. A poisoned model context/session hint requires a new
  Attempt and is not a checkpoint.

### Verification and acceptance

- Executor may submit only `ready_for_review` plus Evidence references; it
  cannot write `done`, `accepted`, or a verification result.
- Evidence authority remains immutable artifact publication plus append-only
  digest metadata. No raw Grant, credential, hidden reasoning, or direct
  personal identifier enters Evidence.
- Deterministic verification runs before any independent Verifier. High-risk
  WorkItems always require a distinct AgentInstance/Run/Grant/Evidence lineage
  with no Executor write authority.
- One authoritative CAS transition records verification result, applicable
  Evidence and rule/contract versions, and the resulting WorkItem state.
  Accepted completion is terminal-once; rejection preserves Evidence and
  follows an explicit bounded policy.
- Projection, `GlobalReadView`, timeline, Attention, and notifications remain
  rebuildable/read-only surfaces, never acceptance authority.

## Mandatory RED and verification

Every WorkItem follows:

```text
reviewed frozen contract
→ mandatory behavioral RED
→ minimal GREEN
→ focused and impact tests
→ full repository and repository-race
→ vet / format / diff / scope / trust audit
→ fresh independent implementation Review
→ exact local atomic commit
```

Static marker tests may supplement but never replace behavioral RED.
Hermetic tests and controlled local SQLite fixtures do not prove live
Runtime/Provider readiness.

## Slice exit gate

Before Slice 5:

1. all three listed WorkItems are accepted and locally committed;
2. every exit capability is `DONE`;
3. all final deliverables end `VERDICT: PASS`;
4. full repository, repository-race, vet, format, scope, trust-boundary,
   authority, secret, and evidence audits pass;
5. the controlled integration proof covers approval/restart, bounded recovery,
   independent verification, terminal-once acceptance, stale fencing, and
   projection recovery; and
6. a fresh independent whole-Slice Reviewer returns `PASS`.

Slice 5 remains closed until all six conditions hold.

## Explicit exclusions

Slice 4 does not add or authorize:

- Provider/model automatic fallback or Credential Broker traffic;
- installed user Runtime, real Agent task, daemon/resident service, network,
  notification, WebSocket, TUI, or Web UI activation;
- model-context, session, process-image, or Projection checkpoint authority;
- standing orders/Autopilot, remote callback, shared marketplace, or
  multi-user authority;
- per-token Journal/Sidecar storage;
- root policy changes, a second Journal/StateWriter/acceptance authority, or
  direct Sidecar SQLite/Artifact Store writes;
- S4-W4, Phase 2, push, merge, rebase, reset, release, or publication.

VERDICT: PASS
