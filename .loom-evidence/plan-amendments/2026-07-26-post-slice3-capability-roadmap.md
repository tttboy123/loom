# Post-Slice-3 Capability Roadmap Amendment

Status: FROZEN — independent Plan Review 1 PASS.

Date: 2026-07-26

Authority input: explicit user queue after accepted Phase 1 Slice 3

Owned documentation:

- `PRODUCT-PLAN.md`
- `TECH-PLAN.md`
- this amendment and its independent review

This is a roadmap and boundary clarification only. It authorizes no product
code, schema migration, Runtime/Provider use, daemon activation, network
action, external publication, or additional Phase 1 WorkItem.

## Reconciliation with accepted Slice 3

Accepted S3-W5 already provides the execution mechanisms needed by later
policy and delivery surfaces:

- logical node and attempt lineage;
- deterministic DAG dispatch and bounded concurrency;
- Journal-authoritative milestones and generation fencing;
- authorized tentative Frame observation;
- durable non-authoritative attempt capture and terminal recovery; and
- immutable `GlobalReadView`.

The amendment must not reopen S3-W5 or create S3-W6. It must preserve attempt
capture, Projection, notifications, streams, and client cursors as
non-authoritative read/delivery surfaces.

## Phase 1 remaining vertical boundaries

### Slice 4 — Output contract and bounded recovery policy

One policy/verification vertical boundary may define:

- `internal/verification/output_contract.go` classifications:
  `valid_nonempty`, `valid_empty`, `transient_empty`, and `invalid`;
- `internal/rules/recovery_policy.go` decisions:
  `retry`, `fallback`, `degraded`, `blocked`, and `human_required`;
- explicit Node/Attempt status, `retry_at`, retry ceilings, budget/approval
  checks, and one independent Run/generation/Grant/Evidence lineage per retry;
- deterministic Journal milestones and crash/restart recovery; and
- scheduler consumption of a policy decision without inventing acceptance.

This is workflow data-source degradation/fallback, not automatic Provider or
model fallback. Empty output never causes a hidden or infinite retry.

### Slice 5 — Local observation and attention delivery

One client-delivery vertical boundary may define a versioned local event
interface and CLI timeline:

- authorized text deltas are bounded, memory-only, and explicitly tentative;
- started/retry/warning/degraded/blocked/human-required/terminal authority
  comes from Journal/Projection;
- reconnect uses cursor or `Last-Event-ID` plus `GlobalReadView`;
- slow consumers may coalesce text deltas but never drop warning, retry,
  degraded, blocked, human-required, or terminal facts;
- overflow emits `stream_gap` plus an artifact digest; and
- raw Grants, credentials, hidden reasoning, direct personal identifiers, and
  per-token Journal Events are forbidden.

Phase 1 may claim only the local event contract and CLI timeline. TUI, Web UI,
remote callback, WebSocket authority, and external notification delivery remain
excluded.

## Phase 2 product surfaces

Phase 2 may govern cohesive query/UX surfaces over Journal, Projection, and
`GlobalReadView`, never a second authority:

1. Run History and Compare by Issue/WorkItem/Team, including Attempts,
   authorized output, Evidence, terminal/failure, usage/cost, Runtime/model, and
   exact Skill revisions.
2. Attention Inbox for approval, blocked, human-required, retry exhausted,
   Runtime offline, and verification failed items. Attention is a projection
   and never an Agent trigger.
3. Runtime Capability Matrix for version, models, health, capacity, session
   resume, MCP, Skill materialization, and streaming. Policy still validates
   every binding.
4. Project Resources Catalog of versioned repository/document/dataset/tool
   pointers. Team Draft receives only a bounded catalog; unavailable resources
   become explicit `partial` or `capability_gap`.
5. Conversational Agent/Team Builder starting from blank, template, or
   historical Candidate while preserving one-question Draft revision and
   explicit confirmation before instantiation.
6. Asset binding UX/CLI for attaching or detaching an exact Skill revision to
   AgentDefinition, TeamDefinition, or WorkPackage with permission and Runtime
   compatibility preview. Running instances never hot-update.

## Phase 3 Evolution Asset Library

Phase 3 strengthens the accepted Candidate-only Sidecar boundary.

### Versioned assets

- `SkillDefinition` and immutable `SkillRevision` record source, digest, scope,
  dependency constraints, compatible Runtime capabilities, risk, and lifecycle
  `draft/candidate/active/archived`.
- Local creation, controlled import, search, diff, archive/restore, rollback,
  and provenance are supported.
- Third-party imports enter `candidate`; script and bundled-asset boundaries
  require validation before activation.
- `AgentTemplate`, `TeamTemplate`, `WorkPackageTemplate`, and
  `RecoveryStrategyTemplate` are versioned. Instantiation produces only a Team
  Draft or WorkPackage Candidate, never a TeamInstance, Run, or expanded grant.

### Runtime materialization

- Runtime adapters materialize only activated exact Skill revisions into
  Runtime-native paths.
- Every Run freezes exact revision IDs and digests.
- Materialization never overwrites a repository-owned Skill on conflict.
- Asset updates do not affect an in-flight Run.

### Promotion and evaluation

- Only a permitted terminal Run with accepted Evidence may propose promotion.
- Promotion creates Skill/Agent/Team/Strategy Candidates with provenance,
  source Run/Evidence digests, redacted summary, scope diff, expected benefit,
  and risk.
- Historical or synthetic Eval compares quality, cost, failure rate, and
  applicability against a baseline.
- Users explicitly activate, reject, or retain Candidates. Activation and
  rollback append Journal facts; Sidecar never writes SQLite/Artifact Store
  directly or changes a running Team.
- Recipe/pattern extraction may retain reusable steps from repeated successes
  and failures, but never raw Grants, credentials, hidden reasoning, or complete
  sensitive prompts.

## Later opt-in capabilities

The following remain post-Phase-1 later opt-in capabilities and require
separate contracts:

- explicit standing orders/Autopilot with manual/schedule/webhook triggers,
  default off, persistent authorization, budget, concurrency, scope,
  stop/revoke, and audit;
- team-shared asset catalogs/marketplace, Lark/Slack/GitHub notifications, and
  multi-user roles/visibility, all local/private by default;
- Provider/session resume as a Runtime capability and recovery hint only.

Ordinary chat/comment never creates automation. A model session/context ID is
not a checkpoint or state authority; poisoned context requires a new Attempt.
Automatic retry, resume, notification, callback, or Autopilot never bypasses
generation, Grant, Evidence, approval, budget, or terminal rules.

## Explicit non-goals and copied-risk exclusions

- No unreviewed third-party Skill execution.
- No plaintext long-lived `custom_env` credentials; OS Secret Store/Broker
  remains the credential boundary.
- No raw Grant material enters Journal, Evidence, output streams, Sidecar
  assets, logs, errors, or exported history.
- No Agent self-completion; Executors stop at `ready_for_review`.
- No squad-leader mention surface masquerades as a real DAG.
- No WebSocket, notification, callback, Projection, cursor, or checkpoint
  becomes state authority.
- No hidden reasoning or per-token Journal/Sidecar storage.
- No Phase 1 model-context/process-image checkpoint or incremental Projection
  checkpoint. Later hardening requires measured need and an independent
  contract.

## Planned authority-plan edits

`PRODUCT-PLAN.md` will:

- clarify local timeline/Attention and Run comparison surfaces;
- expand the Phase 3 Sidecar roadmap into the versioned Evolution Asset
  Library;
- add explicit opt-in later automation/session/shared-catalog boundaries; and
- record the copied-risk exclusions as durable product decisions.

`TECH-PLAN.md` will:

- add the versioned asset/template entities and candidate/materialization
  invariants;
- freeze the authorized tentative-output delivery order;
- add Slice 4 semantic recovery and Slice 5 local delivery acceptance;
- expand Sidecar evaluation/promotion and non-authority rules; and
- keep Phase 1 checkpoint, Web/TUI, Broker, Provider fallback, marketplace,
  multi-user, and Autopilot exclusions explicit.

The edits must be planning-level and must not claim any queued capability is
implemented.

VERDICT: FROZEN
