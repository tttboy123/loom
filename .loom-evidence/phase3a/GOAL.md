# Goal: Phase 3A Entry Audit and Versioned Evolution Assets

Date: 2026-08-03

Repository:
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-pi-rebuild`

Baseline: `6d380233b5b89309a1a7ce3919aa611654e0f4ee`

Status: `ACTIVE — P3A-W1 MANDATORY RED PARTIAL`

## Outcome

First complete the Phase 3A entry audit, ADR, Exit Contract and independent
Contract Review. Only after every gate passes may the single vertical WorkItem
be implemented:

```text
P3A-W1 Versioned Evolution Asset Lifecycle and Runtime Materialization
```

P3A-W1 must deliver one vertical capability containing:

- versioned Skill and Template libraries;
- controlled import and local creation;
- accepted Run/Evidence promotion;
- Candidate Evaluation and baseline comparison;
- explicit activate, reject, retain, archive, restore and rollback;
- exact revision/digest binding to Team, Agent, WorkPackage and Run lineage;
- Runtime-native materialization, cleanup and recovery;
- a real GUI and TUI cross-client user journey.

It must not recreate accepted Phase 2A or P2B-W1, create P2B-W2/P3A-W2, or
split writer, adapter, projection, materializer, evaluator, coordinator or
visual-only wrappers into separate WorkItems.

## Gate 0: Entry audit

Before product writes:

1. verify physical cwd, Git root, branch, HEAD and `docs/CURRENT.md`;
2. inventory and exclude unrelated dirty/untracked files;
3. classify each prerequisite as `DONE`, `PARTIAL` or `MISSING`:
   - Runtime capability and compatibility reads;
   - Credential/Provider boundary;
   - Agent/Team/WorkPackage binding;
   - exact Skill revision/digest entry into Run lineage;
   - Artifact, Evidence, Journal, Projection and GlobalReadView;
   - accepted terminal Run/Evidence authority;
   - Runtime-native Skill materialization capability;
   - archive/restore/rollback Journal and CAS support;
   - real GUI/TUI journey correlation and evidence substrate.

A critical missing prerequisite permits only an Entry Audit and bounded
Amendment proposal. Product implementation then stops `HUMAN_REQUIRED`.

## Gate 1: ADR and Exit Contract

Freeze and independently review:

1. Phase 3A Versioned Evolution Assets ADR;
2. Phase 3A Exit Contract;
3. the single exact P3A-W1 contract.

The contract must freeze exact owned files, Event/Artifact/IPC and journey
metadata schemas, CAS/idempotency/replay, migration compatibility, authority,
Runtime materialization, Mandatory RED, verification, rollback, isolated
journeys and atomic commit. Contract Review must pass before product code.

## Product contract

### Versioned assets

`SkillDefinition` and immutable `SkillRevision` record stable identity, source,
canonical digest, scope, dependencies, compatible Runtime capabilities, risk,
provenance and lifecycle:

```text
draft -> candidate -> active -> archived
```

Support local creation, controlled import, search, exact reads, revision diff,
archive, restore and rollback. Third-party imports are Candidates until reviewed
and explicitly activated.

Versioned `AgentTemplate`, `TeamTemplate`, `WorkPackageTemplate` and
`RecoveryStrategyTemplate` instantiate only Drafts/Candidates. They never
directly create TeamInstance, Run, Grant or expanded permission.

### Promotion and Evaluation

Only authorized terminal Runs with accepted Evidence and complete
Run/generation/Grant/Runtime/input lineage may produce Skill, Agent, Team,
Strategy or Recipe/Pattern Candidates. Each Candidate records source Run,
source Evidence/digest, provenance, redacted summary, scope difference,
expected benefit, risk, compatible Runtime and evaluation requirements.

Historical or synthetic Evaluation records baseline, quality, failure rate,
cost/usage, compatibility, applicable scope, regression and security findings.
Evaluation is Evidence, never activation authority. Activation, rejection,
archive, restore and rollback are explicit user decisions and Journal facts.

### Runtime materialization

Only an active exact Skill revision may be materialized through a Runtime-native
adapter. Each Run freezes revision and digest. Updates never affect an active
Run. Materialization must not overwrite repository/user-owned Skills or
unauthorized Runtime configuration. Incompatibility and partial failure fail
closed; cleanup and reconstruction are deterministic. No fallback revision is
selected silently.

## Authority and security

- Event Journal is the sole state authority.
- Projection, GlobalReadView, search index and materialized copies are
  rebuildable caches.
- Sidecar, Agent, Runtime and model output are Proposal/Candidate only.
- Users explicitly activate assets before any Run uses them.
- Concurrent activate/rollback has one CAS winner.
- stale view/revision/generation, wrong digest, identity drift and incompatible
  Runtime fail closed.
- restart/replay/rebuild does not duplicate assets, activation, materialization
  or Evaluation Evidence.
- raw Grant, credentials, hidden reasoning, sensitive full prompts and per-token
  output never enter assets, Journal, Evidence or logs.
- online evolution cannot modify root policy, validators, credential boundaries
  or authoritative writers.

## Mandatory RED

RED must cover unconfirmed import, non-accepted promotion, stale view, wrong
revision/digest, concurrent CAS, exact rollback, archived revisions, immutable
Run binding, incompatible Runtime, repository Skill protection, interrupted
materialization, restart/replay idempotency, Projection old-view preservation,
non-disclosure, Sidecar authority rejection and template Draft enforcement.

## Mandatory Cross-client User Journey Exit Gate

This gate applies to P3A-W1 and every later WorkItem. A capability that cannot
form a real GUI and TUI journey must be merged into its vertical parent; it
cannot be a standalone WorkItem. Missing either client means `PARTIAL`.

### Real clients

GUI E2E uses the real native window for launch/reconnect, navigation, click,
input, selection, confirm, reject, cancel, back, daemon restart and safe error
recovery. TUI E2E uses a real PTY for keyboard input, page changes, selection,
pagination, confirm, reject, cancel, cursor reconnect, restart and recovery.

Unit tests, ViewModel/service calls, Preview, snapshot, mocked Journal, IPC
bypass, visual-only fixture, launch-only proof and accessibility-tree-only
proof cannot replace the final journey.

Both clients use the same production client model, real local IPC socket,
Daemon handler, application service, authoritative writer, Journal,
Projection/GlobalReadView, Artifact Store and isolated state root.

### Journey correlation

Every journey has a unique `journey_id` carried through client actions, IPC
metadata, Daemon structured logs, application service, Journal metadata,
Projection reads, Artifact/Evidence manifests, results and cleanup. It is only
correlation metadata: never authority, generation, Grant or idempotency key,
and never sensitive.

### Scenario matrix

At minimum cover normal completion, rejection, cancellation, expiry, stale
view/generation, wrong digest/identity, concurrent single-CAS winner, crash
before write, crash after Journal commit but before response, Projection
rebuild/failure, GUI-to-TUI and TUI-to-GUI observation, reconnect, slow client,
duplicate delivery and restart after cleanup.

Each scenario checks visible state, Journal ordering/cardinality, Projection
consistency, SQLite integrity and uniqueness, Artifact/Evidence resolution, no
hidden retry/fallback/dispatch/side effect, and process/socket/lock/lease/temp
cleanup.

### Evidence bundle

Each `0700` journey root stores `0600` source-locked evidence:

```text
journey-<journey_id>/
  manifest.json
  result.md
  gui/actions.jsonl
  gui/screenshots-or-recording
  tui/transcript.txt
  tui/keystrokes.jsonl
  timeline.jsonl
  daemon/structured-log.jsonl
  ipc/request-response-summary.jsonl
  journal/event-summary.json
  journal/stream-heads.json
  journal/sqlite-integrity.txt
  artifacts/digest-verification.json
  processes/preflight.json
  processes/postflight.json
  processes/cleanup-proof.txt
```

Failed evidence is immutable. A replacement requires Implementation Re-review,
a new journey ID and a new root. Visual-only inspection never substitutes for
an authority journey.

Result and whole-Candidate Reviewers separately return:

```text
Product Result: PASS / FAIL
Operational and Trace Behavior: PASS / FAIL
```

Either missing client/evidence, IPC bypass, user/log mismatch, hidden retry or
incomplete cleanup makes the WorkItem `FAIL` or `PARTIAL`.

## Verification order

1. Entry Audit;
2. ADR/Exit Contract and independent Contract Review;
3. Mandatory RED;
4. focused tests;
5. Go full/race/vet/tidy/format;
6. Swift full/TSAN/Release;
7. schema/replay/CAS/restart/security;
8. Runtime materialization fixture;
9. real native GUI E2E;
10. real PTY TUI E2E;
11. shared-root cross-client evidence;
12. Implementation Review, P0/P1/P2 all zero;
13. Product Result Review PASS;
14. Operational and Trace Review PASS;
15. whole-Phase 3A Candidate Review PASS;
16. exact staging;
17. one atomic local commit.

## Exclusions and stop conditions

Exclude Phase 3B Sandbox, v0.3 routing, v0.4 Scheduling, Autopilot/webhooks,
Marketplace/shared assets/multi-user/Web, Provider fallback/session or model
checkpoint, automatic activation, unreviewed scripts, network/paid Provider,
real user Runtime mutation, push and merge.

Stop `HUMAN_REQUIRED` for identity mismatch, critical prerequisite gaps,
unreviewed authority/schema/credential expansion, inability to close one W1,
credentials/network/user-Skill overwrite, dirty-boundary collision or any
failed Contract/Implementation/Result/whole-Candidate Review.
