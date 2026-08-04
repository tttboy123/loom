# Phase 3A Versioned Evolution Assets Exit Contract

Date: `2026-08-03`

Status: `FROZEN — INDEPENDENT CONTRACT REVIEW REQUIRED`

Authority: active Phase 3A Goal, Product Owner authorization and accepted Entry
Amendment Review 1.

Decision: proposed ADR-0013.

## 1. Purpose and single vertical boundary

Phase 3A delivers one user-operable evolution-asset lifecycle that creates,
imports, evaluates, explicitly decides, binds, materializes, observes and
recovers exact immutable revisions through both production clients.

Exactly one WorkItem exists:

```text
P3A-W1 Versioned Evolution Asset Lifecycle and Runtime Materialization
```

There is no P3A-W2. Asset model, authority, projection, IPC, materializer,
client screen, journey harness, repair or wrapper is not an independent
WorkItem. If a required capability cannot close inside P3A-W1, work stops
`HUMAN_REQUIRED` rather than splitting it.

## 2. Baseline and accepted prerequisites

- repository:
  `/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-pi-rebuild`;
- branch: `codex/loom-platform-slice2`;
- baseline: `6d380233b5b89309a1a7ce3919aa611654e0f4ee`;
- Phase 2A and P2B-W1 are accepted and immutable for this Candidate;
- Entry Amendment SHA-256:
  `e0346dc41dc69629b3fa3598e8a5cec85b244cadfb9a7538a8698bad7bea545f`;
- Entry Amendment Review 1: `P0=0`, `P1=0`, `P2=0`, Product/Authority
  `PASS`, Operational/Trace Governance `PASS`.

The existing Journal, `ReadStreamSet`, `AppendBatchIfStreamHeads`, immutable
Evidence Store, accepted terminal Run/Evidence rules, versioned GlobalReadView,
Credential Broker and private local IPC are reused. They are not recreated.

Pre-existing dirty/untracked files remain user-owned. In particular,
`internal/projection/team_execution_test.go` is excluded and may not be edited,
staged or reformatted by P3A-W1.

## 3. Exit capability matrix

Every row must be `DONE` with source, deterministic test and real journey
evidence before acceptance.

| ID | Required capability | Initial state |
|---|---|---|
| EA-01 | Immutable `SkillDefinition` and `SkillRevision` with source, digest, scope, dependencies, Runtime compatibility, risk and provenance | MISSING |
| EA-02 | Immutable Agent, Team, WorkPackage and RecoveryStrategy Template revisions | MISSING |
| EA-03 | Local create and controlled third-party import; import always enters Candidate | MISSING |
| EA-04 | Bounded search, exact read and canonical revision diff through GlobalReadView | MISSING |
| EA-05 | Explicit activate, reject, retain, archive, restore and exact rollback with one CAS winner | MISSING |
| EA-06 | Accepted terminal Run+Evidence promotion with complete lineage and redacted provenance | MISSING |
| EA-07 | Historical/synthetic Evaluation with baseline, quality, cost, failure, scope, compatibility, regression and security Evidence | MISSING |
| EA-08 | Template instantiation creates only Draft/Candidate and never TeamInstance/Run/Grant/permission expansion | MISSING |
| EA-09 | Exact canonical revision set and manifest digest copied into ExecutionPlan, dispatch, Run and every Attempt | MISSING |
| EA-10 | Capability-gated private Pi materialization, collision protection, atomic publish, cleanup and rebuild | MISSING |
| EA-11 | Restart/replay/projection failure preserve authority, old view and exact-once facts | MISSING |
| EA-12 | Production native app provides the full asset lifecycle inside the accepted chat/task-first workbench | MISSING |
| EA-13 | Production TUI provides the same lifecycle through real keyboard/PTY interaction | MISSING |
| EA-14 | One shared-root cross-client journey correlates every layer and passes dual Result Review | MISSING |

## 4. Domain and authority invariants

1. Event Journal facts are the only lifecycle and activation authority.
2. Immutable asset bytes and Evaluation payloads live in the existing Artifact/
   Evidence Store. Journal payloads contain bounded metadata and digests.
3. Projection, GlobalReadView, search results, client state, materialized files
   and Runtime discovery are rebuildable copies.
4. Asset IDs, revision IDs, digests, source scope, dependencies, capability
   requirements and risk are canonical and bounded before any write.
5. Revision content is immutable. A change creates a new revision.
6. Lifecycle transitions are explicit Journal facts. `active` is not inferred
   from file presence, Evaluation score, accepted Run or Runtime discovery.
7. Sidecar, Agent, Runtime, model, imported package and Evaluation may propose
   a Candidate only. They cannot activate, reject, retain, archive, restore or
   rollback.
8. Activation/rejection/retention/archive/restore/rollback use expected stream
   heads and expected GlobalReadView version. Stale state has zero writes.
9. Concurrent conflicting decisions have exactly one CAS winner; the loser
   receives a canonical conflict without hidden retry.
10. Templates instantiate only typed Drafts/Candidates. Existing confirmation,
    policy, Grant and execution gates remain mandatory.
11. New Runs resolve active bindings once during preflight and copy exact
    revisions/digests into immutable planning and authority facts. Attempts
    inherit the same set unless an explicit new generation/new Run is created.
12. An asset update, archive or rollback cannot mutate an active/nonterminal
    Run, its private materialization or its Evidence lineage.
13. Existing legacy Runs remain readable. They are labelled as lacking P3A
    asset lineage and are never backfilled with invented revisions.

## 5. Lifecycle and Event contract

The P3A-W1 child contract freezes exact JSON schemas for these Event families:

```text
EvolutionAssetDefinitionCreated
EvolutionAssetRevisionCreated
EvolutionAssetImportProposed
EvolutionAssetCandidateCreated
EvolutionAssetEvaluationRecorded
EvolutionAssetCandidateActivated
EvolutionAssetCandidateRejected
EvolutionAssetCandidateRetained
EvolutionAssetRevisionArchived
EvolutionAssetRevisionRestored
EvolutionAssetActivationRolledBack
EvolutionTemplateInstantiated
EvolutionRunPromotionProposed
RuntimeSkillMaterializationPublished
RuntimeSkillMaterializationCleaned
```

All Events use existing Journal envelope identity, stream sequence, UTC time,
`CorrelationID` and causation rules. P3A payloads must be strict, versioned and
duplicate-key safe. Unknown versions, kinds, lifecycle states, template kinds,
risk levels, source scopes, capability names or fields fail Projection rebuild
without replacing the last published view.

No lifecycle Event contains raw asset bytes, raw Grant, credential, hidden
reasoning, sensitive prompt, per-token output, external authorization header or
unredacted Runtime environment.

### Stream and idempotency rules

The child contract must freeze deterministic stream IDs and command keys for:

- definition identity;
- immutable revision identity;
- Candidate lineage;
- Evaluation identity;
- activation scope;
- Run/Attempt materialization lineage.

Create/import/promote/evaluate/decide/archive/restore/rollback/materialize/
cleanup commands each carry a bounded operation ID distinct from `journey_id`.
Byte-identical redelivery returns the committed result. Same operation ID with
different canonical input fails closed. No command loops indefinitely.

## 6. Import, promotion, Evaluation and template rules

### Local creation and import

- Local creation accepts bounded text/files from an explicitly selected local
  source and stores canonical immutable bytes before proposing a revision.
- Third-party import never executes imported scripts and never activates the
  result. It records source, source digest, license/provenance when supplied,
  compatibility claim and risk as a Candidate.
- Path traversal, symlink/hard-link input, device/FIFO/socket input, excessive
  size/count, wrong digest, duplicate identity and unsupported encoding fail
  before authority write.

### Accepted Run promotion

Promotion requires an authoritative terminal Run, accepted source and verifier
Evidence, matching generation/Grant/Runtime/input digests and an allowlisted
redacted summary. Missing, rejected, stale or inconsistent lineage has zero
writes. Promotion creates a Candidate, never an active revision.

### Evaluation

Historical and synthetic Evaluation references immutable fixture/source
digests and records baseline and Candidate measures separately. Unknown cost or
usage remains unknown; zero is not invented. A failing or partial Evaluation is
preserved as Evidence and cannot be overwritten by a replacement.

### Templates

`AgentTemplate`, `TeamTemplate`, `WorkPackageTemplate` and
`RecoveryStrategyTemplate` revisions are immutable. Instantiation accepts exact
revision/digest and bounded parameters, then produces only the existing typed
Draft/Candidate boundary. It cannot bypass user confirmation, policy, approval,
Grant, generation or Runtime compatibility.

## 7. Execution lineage and Runtime materialization

Each execution node freezes a canonical asset revision set containing:

```text
asset_kind
definition_id
revision_id
sha256_digest
source_scope
```

The set is sorted, duplicate-free, bounded and included in ExecutionPlan digest,
Team dispatch semantic binding, Run creation Event, Attempt Event and material
manifest digest. Dispatch CAS includes every touched asset activation head plus
Team execution, WorkItem, Run, Runtime status/capacity and materialization
streams. A stale asset head therefore rejects dispatch before execution.

Materialization order is:

1. validate lifecycle, exact bytes/digests, dependencies, Runtime capability,
   installed Runtime identity, Grant, Run/Attempt/generation and target plan;
2. write a sibling private temporary tree with `0700` directories and `0600`
   files, rejecting traversal/link/case-fold/duplicate/existing-target issues;
3. fsync where supported, re-read and verify every byte, create a canonical
   immutable manifest Artifact, then atomically rename on one filesystem;
4. commit `RuntimeSkillMaterializationPublished` in the same dispatch CAS that
   records the exact manifest digest in Run/Attempt lineage;
5. launch Pi only with the reviewed private native path and locked capability;
6. after terminal/revocation, verify ownership and manifest, remove only the
   Loom-owned root and record one cleanup fact.

Crash before Journal commit leaves no dispatch and recovery removes any
uncommitted private tree. Crash after commit but before response is recovered
from Journal and the same operation ID without duplicate Run, Attempt,
materialization or Evidence. Missing/corrupt committed materialization is
rebuilt only from exact immutable Artifact bytes for the same current
generation. Repository/user Skill locations are never targets.

## 8. IPC and product interaction contract

The child contract adds strict production methods:

```text
evolution_asset_snapshot
evolution_asset_diff
evolution_asset_command
```

`evolution_asset_command` uses an exact closed action enum:

```text
create_skill
import_skill
create_template
instantiate_template
promote_run
record_evaluation
activate
reject
retain
archive
restore
rollback
```

Every P3A request and response carries one canonical lowercase UUIDv4
`journey_id`; existing pre-P3A methods remain wire-compatible when it is absent.
The daemon echoes the exact value, application commands validate it, Events copy
it to existing `CorrelationID`, P3A projections expose it for trace inspection,
and Evidence/cleanup manifests record it.

`request_id`, command operation ID, expected view version, expected heads,
generation, Grant and digests remain separate. A mismatched/unknown/duplicate
journey field fails closed; reconnect does not resubmit a mutation.

The native app and TUI must expose create/import/search/diff, Candidate and
Evaluation review, explicit decision, archive/restore/rollback, compatibility,
exact Run binding and materialization/recovery status in the accepted
chat/task-first workbench. Disabled actions and canonical errors must explain
the next safe step. Neither client may call CLI text, shell, SQLite, Artifact
files or application services as its product protocol.

## 9. Mandatory RED and deterministic verification

Before product implementation, RED must fail for every requirement below:

- unconfirmed import cannot activate or execute;
- nonterminal/nonaccepted Run or rejected/missing Evidence cannot promote;
- stale view/head/revision/generation and wrong digest/identity have zero writes;
- concurrent activate/rollback/dispatch has one CAS winner;
- archived revision cannot bind; restore and rollback select exact history;
- a bound Run remains byte-identical across later activation changes;
- incompatible Runtime or absent capability prevents materialization/dispatch;
- repository/user Skill collision is never overwritten;
- partial/crashed materialization is atomic and recoverable;
- restart/replay/redelivery does not duplicate lifecycle, Run, Attempt,
  materialization, cleanup, Evaluation or Evidence;
- failed Projection rebuild preserves the previous GlobalReadView;
- secrets, raw Grants, hidden reasoning, sensitive prompts and per-token output
  never enter source, logs, Events, Evidence, manifests or clients;
- Sidecar/Agent/Runtime/model cannot decide lifecycle;
- every template kind remains Draft/Candidate-only;
- unknown/duplicate wire fields and journey/request identity drift fail closed;
- GUI/TUI user-visible state matches authoritative facts and structured logs.

Required non-live verification includes focused Go/Swift tests, full Go tests,
repository race, vet, tidy/format, Swift full tests, TSAN, Release build,
schema/replay/CAS/restart/security matrices and a locked deterministic Pi
materialization fixture. Exact commands are frozen in the child contract.

## 10. Mandatory Cross-client User Journey Exit Gate

P3A-W1 and every later WorkItem must have both a real native-window GUI journey
and real PTY TUI journey. Either missing means `PARTIAL`; no mock, Preview,
snapshot, ViewModel/service direct call, IPC bypass, visual-only fixture,
launch-only proof or accessibility-tree-only proof substitutes.

Both clients use one production daemon, one real IPC socket, the same
application services, writer, Journal, Projection/GlobalReadView, Artifact Store
and isolated state root. A unique journey ID spans client actions, IPC, daemon
structured logs, application calls, Journal correlation, Projection reads,
Artifacts/Evidence and cleanup.

The final matrix covers:

- normal create/import/review/Evaluate/activate/bind/materialize/terminal flow;
- reject, retain, cancel and expiry;
- stale view, stale generation, wrong digest and identity drift;
- concurrent conflicting clients with one winner;
- crash before write and crash after commit before response;
- Projection failure and rebuild;
- GUI action observed in TUI and TUI action observed in GUI;
- cursor reconnect, daemon restart, slow client and duplicate delivery;
- cleanup followed by restart/rebuild without residue or duplication.

Every scenario jointly asserts visible status/actions/errors/next step, Event
order/cardinality, Projection consistency, SQLite integrity/uniqueness,
Artifact/Evidence digest resolution, no hidden retry/fallback/side effect and
process/socket/lock/lease/temp cleanup.

Each failed or passing journey is an immutable `0700` root with `0600`
source-locked manifest/result, GUI actions/screenshots or recording, TUI
transcript/keystrokes, timeline, daemon structured logs, IPC summary,
Journal/heads/SQLite summary, digest verification and pre/post cleanup proof.
A replacement requires Implementation Re-review, a new journey ID and root.

## 11. Review, staging and completion gates

Order is mandatory:

1. ADR-0013, this Exit Contract and exact P3A-W1 contract freeze;
2. fresh independent combined Contract Review `PASS` with `P0=P1=P2=0`;
3. ADR-0013 becomes accepted; only then capture Mandatory RED;
4. focused implementation and verification;
5. full Go/race/vet/tidy/format and Swift full/TSAN/Release;
6. replay/CAS/restart/security and deterministic materialization fixture;
7. fresh independent Implementation Review `PASS`, `P0=P1=P2=0`;
8. real GUI journey and immutable evidence;
9. real PTY TUI journey over the same root and immutable evidence;
10. cross-client Product Result Review `PASS`;
11. cross-client Operational and Trace Behavior Review `PASS`;
12. whole-Phase 3A Candidate Review `PASS`, `P0=P1=P2=0`;
13. exact staging of only reviewed P3A files;
14. one atomic local commit.

No product code changes before step 2. No staging or commit before step 12.
No push or merge is authorized.

## 12. Exclusions and stop conditions

Excluded: Phase 3B, v0.3 routing, v0.4 Scheduling, Autopilot/webhook,
Marketplace/shared/multi-user/Web, Provider fallback/checkpoint, automatic
activation, unreviewed executable imports, external install, network/paid
Provider, real user configuration/Skill mutation, dependency changes, public
API/TCP, push and merge.

Stop `HUMAN_REQUIRED` for identity/baseline mismatch, dirty overlap, need to
expand the reviewed Amendment, closed authority/schema/credential/dependency
change, inability to close one W1, need for network/credential/user overwrite,
or any failed Contract/Implementation/Result/whole-Candidate Review.

VERDICT: `FROZEN — PENDING REVIEW`
