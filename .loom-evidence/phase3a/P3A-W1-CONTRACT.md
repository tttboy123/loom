# P3A-W1 Contract: Versioned Evolution Asset Lifecycle and Runtime Materialization

Date: `2026-08-03`

Status: `FROZEN — INDEPENDENT CONTRACT REVIEW REQUIRED`

Parent: Phase 3A Versioned Evolution Assets Exit Contract

Baseline: `6d380233b5b89309a1a7ce3919aa611654e0f4ee`

WorkItem: the only `P3A-W1`. `P3A-W2` does not exist.

## 1. Vertical acceptance boundary

One Candidate must deliver the complete lifecycle:

```text
create/import
-> Candidate
-> inspect/search/diff
-> promote/evaluate
-> explicit decide/archive/restore/rollback
-> exact Team/Agent/WorkPackage binding
-> immutable ExecutionPlan/Run/Attempt lineage
-> private Pi materialization
-> terminal Evidence and cleanup/rebuild
-> real shared-root GUI + TUI journey
```

No backend-only, adapter-only, writer-only, materializer-only, projection-only,
screen-only or journey-only completion is accepted.

## 2. Exact owned files

The Candidate may modify/create only the paths below. Existing paths not listed
remain read-only. An owned path is permission to make only contract-required
changes, not unrelated refactoring.

### Governance and decision

- `docs/adr/0013-versioned-evolution-assets-and-run-bound-materialization.md`
- `docs/adr/README.md`
- `docs/CURRENT.md`
- `.loom-evidence/phase3a/GOAL.md`
- `.loom-evidence/phase3a/ENTRY-AUDIT.md`
- `.loom-evidence/phase3a/ENTRY-AMENDMENT-DISCOVERY.md`
- `.loom-evidence/phase3a/BOUNDED-ENTRY-AMENDMENT-PROPOSAL.md`
- `.loom-evidence/phase3a/ENTRY-AMENDMENT.md`
- `.loom-evidence/phase3a/ENTRY-AMENDMENT-REVIEW-1.md`
- `.loom-evidence/phase3a/EXIT-CONTRACT.md`
- `.loom-evidence/phase3a/P3A-W1-CONTRACT.md`
- `.loom-evidence/phase3a/P3A-W1/**` generated RED, verification, review and
  immutable journey evidence only

### Evolution asset domain and authority

- `internal/assets/model.go`
- `internal/assets/model_test.go`
- `internal/assets/canonical.go`
- `internal/assets/canonical_test.go`
- `internal/assets/authority.go`
- `internal/assets/authority_test.go`
- `internal/assets/replay.go`
- `internal/assets/replay_test.go`

### Saved binding, execution authority and Projection

- `internal/teams/saved_team_binding.go`
- `internal/teams/saved_team_binding_test.go`
- `internal/teams/saved_team_instantiation.go`
- `internal/teams/saved_team_instantiation_test.go`
- `internal/teams/execution_plan.go`
- `internal/teams/execution_plan_test.go`
- `internal/app/local_product_execution.go`
- `internal/app/local_product_execution_test.go`
- `internal/app/team_execution.go`
- `internal/app/team_execution_test.go`
- `internal/work/team_execution_authority.go`
- `internal/work/team_execution_authority_test.go`
- `internal/work/run_authority.go`
- `internal/work/run_authority_test.go`
- `internal/projection/team_execution.go`
- `internal/projection/projection.go`
- `internal/projection/projection_test.go`
- `internal/projection/global_read_view.go`
- `internal/projection/global_read_view_test.go`
- `internal/projection/evolution_assets.go`
- `internal/projection/evolution_assets_test.go`

`internal/projection/team_execution_test.go` is explicitly excluded because it
contains pre-existing user work. New P3A projection/lineage tests must live in
`evolution_assets_test.go`, `projection_test.go` or other listed clean tests.

### Runtime and private Pi materialization

- `internal/runtime/catalog.go`
- `internal/runtime/catalog_test.go`
- `internal/runtime/pi_probe.go`
- `internal/runtime/pi_probe_test.go`
- `internal/runtime/piadapter/skill_materialization.go`
- `internal/runtime/piadapter/skill_materialization_test.go`
- `internal/runtime/piadapter/execution_adapter.go`
- `internal/runtime/piadapter/execution_adapter_test.go`
- `internal/runtime/piadapter/process_runner.go`
- `internal/runtime/piadapter/process_runner_test.go`
- `internal/runtime/piadapter/rpc_bridge_adapter.go`
- `internal/runtime/piadapter/rpc_bridge_adapter_test.go`

### Application, API, IPC and daemon

- `internal/app/local_product_assets.go`
- `internal/app/local_product_assets_test.go`
- `internal/api/local_product_assets.go`
- `internal/api/local_product_assets_test.go`
- `internal/localipc/protocol.go`
- `internal/localipc/protocol_test.go`
- `internal/localipc/client.go`
- `internal/localipc/client_test.go`
- `internal/localipc/server.go`
- `internal/localipc/server_test.go`
- `internal/localipc/swift_contract_test.go`
- `cmd/loomd/product_daemon.go`
- `cmd/loomd/product_daemon_test.go`

### Production TUI

- `cmd/loom/tui.go`
- `cmd/loom/main_test.go`
- `internal/tui/model.go`
- `internal/tui/model_test.go`
- `internal/tui/program.go`
- `internal/tui/program_test.go`

### Production native client

- `apps/macos/Sources/LoomLocalApp/LoomLocalApp.swift`
- `apps/macos/Sources/LoomLocalAppCore/LocalIPCClient.swift`
- `apps/macos/Sources/LoomLocalAppCore/LocalProductAssetModels.swift`
- `apps/macos/Sources/LoomLocalAppCore/LocalProductExperience.swift`
- `apps/macos/Sources/LoomLocalAppCore/LocalProductStore.swift`
- `apps/macos/Sources/LoomLocalAppUI/ContentView.swift`
- `apps/macos/Sources/LoomLocalAppUI/MissionWorkbench.swift`
- `apps/macos/Tests/LoomLocalAppTests/LocalIPCClientTests.swift`
- `apps/macos/Tests/LoomLocalAppTests/LocalProductAssetModelsTests.swift`
- `apps/macos/Tests/LoomLocalAppTests/LocalProductExperienceTests.swift`
- `apps/macos/Tests/LoomLocalAppTests/LocalProductExperienceViewTests.swift`
- `apps/macos/Tests/LoomLocalAppTests/LocalProductStoreTests.swift`

### Controlled journey tooling

- `scripts/run-phase3a-cross-client-journey.sh`
- `scripts/verify-phase3a-cross-client-journey.sh`
- `docs/runbooks/phase3a-cross-client-journey.md`

No `go.mod`, `go.sum`, `Package.swift`, Journal/Evidence/Credential store,
Provider, policy, root validator or accepted P2B-W1 edit is permitted.

## 3. Canonical values and bounds

Closed enums:

```text
asset_kind = skill | agent_template | team_template |
             work_package_template | recovery_strategy_template
lifecycle = draft | candidate | active | archived
source_scope = local | imported | promoted
risk = low | medium | high | critical
candidate_decision = activate | reject | retain
template_output = agent_candidate | team_draft |
                  work_package_candidate | recovery_strategy_candidate
```

IDs are 1-64 ASCII characters from `[A-Za-z0-9._:-]`, never paths. Digests are
lowercase 64-byte hexadecimal SHA-256. Names are 1-128 UTF-8 bytes; bounded
descriptions/redacted summaries are at most 4096 UTF-8 bytes after control,
bidi and terminal-escape rejection. A revision has at most 32 dependencies and
32 compatible capabilities. A definition has at most 64 revisions in one view
page. Search pages contain 1-64 records and use canonical cursors.

One revision Artifact is at most 1 MiB, 128 files and 16 MiB total expanded
materialization. Absolute/unclean/traversing paths, empty path segments,
duplicate/case-fold-colliding targets, symlink/hard-link/device/FIFO/socket
input and decompression are rejected. P3A does not execute imported scripts.

## 4. Immutable domain schemas

### SkillDefinition

```text
definition_id
name
description
scope
created_event_id
latest_revision_id
lifecycle
active_revision_id (empty unless active)
head
```

### SkillRevision

```text
definition_id
revision_id
artifact_digest
content_digest
source_scope
source_reference_digest
provenance_digest
dependencies[]
compatible_runtime_capabilities[]
risk
lifecycle
created_event_id
```

Template revisions use the same immutable identity fields plus exact
`asset_kind`, `template_output`, parameter schema digest and bounded permission/
scope ceiling. Parameter substitution is canonical and cannot add permissions,
resources, Runtime capabilities or scope absent from the revision.

### EvolutionCandidate

```text
candidate_id
asset_kind
definition_id
revision_id
source_scope
source_run_id
source_run_generation
source_evidence_ids[]
source_evidence_digests[]
redacted_summary
scope_difference
expected_benefit
risk
required_evaluation_ids[]
decision
decision_event_id
```

Run fields are empty for local/import candidates and required for promoted
Candidates. Decisions are empty until the explicit user command.

### EvaluationRecord

```text
evaluation_id
candidate_id
fixture_kind (historical | synthetic)
fixture_digest
baseline_revision_id
baseline_digest
candidate_revision_id
candidate_digest
quality_result
failure_count
case_count
usage_observed
usage_value
cost_observed
cost_value
compatibility_result
applicable_scope
regression_result
security_result
evidence_id
evidence_digest
```

Unknown usage/cost uses `observed=false` with no value. It is never encoded as
zero. Evaluation records are immutable and replacement uses a new ID.

### ExactAssetRevisionBinding

```text
asset_kind
definition_id
revision_id
sha256_digest
source_scope
```

Bindings are canonical-sorted, duplicate-free and copied into every
ExecutionNode, Team semantic dispatch binding, Run and Attempt payload. The
canonical JSON array digest is `asset_revision_set_digest` and becomes an input
to the ExecutionPlan and dispatch decision digests.

## 5. Exact Event payloads

Every payload has `schema_version:1`, `operation_id` and its domain IDs/digests.
The Event envelope supplies Event ID, stream ID, sequence, UTC time,
`CorrelationID` and causation. Fields not named below are rejected.

| Event | Required payload beyond common fields |
|---|---|
| `EvolutionAssetDefinitionCreated` | asset_kind, definition_id, name, description, scope |
| `EvolutionAssetRevisionCreated` | asset_kind, definition_id, revision_id, artifact_digest, content_digest, source_scope, source_reference_digest, provenance_digest, dependencies, compatible_runtime_capabilities, risk, lifecycle |
| `EvolutionAssetImportProposed` | candidate_id, definition_id, revision_id, external_source_digest, provenance_digest, risk |
| `EvolutionAssetCandidateCreated` | full Candidate schema excluding later decision fields |
| `EvolutionAssetEvaluationRecorded` | full Evaluation schema |
| `EvolutionAssetCandidateActivated` | candidate_id, definition_id, revision_id, expected_previous_revision_id, evaluation_ids, decision_source=`user_explicit` |
| `EvolutionAssetCandidateRejected` | candidate_id, reason_code, decision_source=`user_explicit` |
| `EvolutionAssetCandidateRetained` | candidate_id, reason_code, decision_source=`user_explicit` |
| `EvolutionAssetRevisionArchived` | definition_id, revision_id, previous_lifecycle, reason_code |
| `EvolutionAssetRevisionRestored` | definition_id, revision_id, restored_lifecycle, reason_code |
| `EvolutionAssetActivationRolledBack` | definition_id, from_revision_id, to_revision_id, from_digest, to_digest, reason_code |
| `EvolutionTemplateInstantiated` | asset_kind, definition_id, revision_id, template_output, parameter_digest, output_candidate_id, output_digest |
| `EvolutionRunPromotionProposed` | candidate_id, source_run_id, source_run_generation, source_run_digest, source_evidence_ids, source_evidence_digests, redacted_summary_digest |
| `RuntimeSkillMaterializationPublished` | team_execution_id, logical_node_id, run_id, attempt_number, generation, runtime_instance_id, capability, asset_revision_set_digest, manifest_artifact_digest, materialization_root_digest |
| `RuntimeSkillMaterializationCleaned` | run_id, attempt_number, generation, manifest_artifact_digest, cleanup_result=`removed` |

Definition/revision/Candidate/Evaluation/activation/materialization stream IDs
and Event/idempotency IDs are deterministic functions of validated domain
identity and operation ID. Exact formulas and canonical JSON golden bytes must
be recorded in Mandatory RED evidence before implementation.

## 6. Command/CAS rules

Every mutation includes:

```text
operation_id
journey_id
expected_view_version
expected_stream_heads{}
authoritative_time_utc
```

The application layer may validate presentation completeness but the domain
authority replays every touched stream with `ReadStreamSet`, recomputes the
command result, and commits once with `AppendBatchIfStreamHeads`. It does not
trust client lifecycle, digest, Evaluation acceptance or active revision.

Activation and rollback CAS touch definition, revision, Candidate, Evaluation
and activation streams. Dispatch CAS additionally touches the exact active
revision streams, materialization, Team execution, WorkItem, Run and relevant
Runtime status/capacity streams. A changed head is a conflict with zero writes.

Identical redelivery of a committed operation returns the same Event IDs and
result. A reused operation ID with different canonical command bytes is
`conflict`. There is no automatic mutation retry in app, IPC, TUI or GUI.

## 7. Promotion and template authority

Promotion replays source Run, Attempt, Grant, accepted terminal Team execution
and every referenced Evidence stream. It rejects nonterminal, unaccepted,
missing verifier/source receipt, generation drift, digest mismatch, rejected
Evidence or disallowed disclosure. The created Candidate references immutable
digests and an allowlisted redacted summary Artifact.

Template instantiation verifies exact revision/digest, lifecycle, parameter
schema and scope/permission ceilings. It calls the existing typed Draft or
Candidate construction boundary and records only `EvolutionTemplateInstantiated`.
No TeamInstance, Run, Grant, dispatch or expanded permission Event may occur in
the same transaction.

## 8. Runtime materialization and execution ordering

`loom.skill-materialization.pi.v1` is the only new Runtime capability. The Pi
probe reports it only after a deterministic installed-component conformance
probe proves the reviewed private Skill-path option, exact path semantics and
cleanup behavior. Model/version discovery remains unchanged.

The materializer accepts only a validated `MaterializationPlan` containing the
exact bindings, immutable Artifact readers, private attempt root, Runtime
identity, Run/Attempt/generation and expected manifest digest. It performs
no network access and cannot resolve credentials.

Target layout:

```text
<isolated-root>/materialized/run-<run_id>/attempt-<n>/generation-<g>/
  manifest.json
  skills/<definition_id>/<revision_id>/...
```

Every ancestor is opened/validated without following links, owned by the
current uid and `0700`; materialized files and manifest are `0600`. Publication
uses a sibling temp root and same-filesystem atomic rename. Existing target,
foreign ownership, wrong mode, identity drift or byte mismatch fails closed.

`ExecutionPlan` and Team dispatch are prepared before materialization. Pi is
not launched until the published manifest digest is committed in the same
dispatch CAS and re-read from the authoritative result. Process/RPC adapters
receive only the exact private path. They do not scan repository/user paths.

Restart rules:

- uncommitted temp/published roots are verified then removed;
- committed current-generation roots are verified and reused;
- missing committed roots are reconstructed from immutable bytes;
- corrupt/foreign roots produce `human_required`, no launch and no deletion of
  unknown files;
- stale generation results and cleanup are rejected;
- cleanup removes only a manifest-matching Loom root and is idempotent.

## 9. Projection and migration

No SQLite schema migration is authorized. New facts use existing Journal Event
rows and `CorrelationID`. Existing Events/Runs remain readable and project with
empty `asset_revision_bindings` plus explicit `asset_lineage_available=false`.
No historic fact is rewritten or backfilled.

The Projection rebuilds immutable definition, revision, Candidate, Evaluation,
activation, exact Run binding and materialization records. GlobalReadView typed
accessors return defensive copies and bounded sorted pages. Search is a derived
bounded scan of the published view, not a database or mutable index.

Unknown/malformed/duplicate/gapped facts reject the rebuild and preserve the
previous published view. Replay from identical Journal heads produces the same
view version and canonical records.

## 10. IPC schema and strict errors

Protocol v1 remains. Request and Response add optional wire field
`journey_id`; it is mandatory and echoed for the three P3A methods. Generic
`Call` remains compatible; new `CallJourney` requires canonical lowercase
UUIDv4 and checks exact response equality.

Methods:

```text
evolution_asset_snapshot
evolution_asset_diff
evolution_asset_command
```

Snapshot request: journey_id, cursor, limit, optional exact asset_kind and
lifecycle filters, bounded search text. Diff request: journey_id,
definition_id, left_revision_id/digest and right_revision_id/digest. Command
request: journey_id, operation_id, action, expected_view_version,
expected_stream_heads and an action-specific strict object.

Closed actions are those in the parent Exit Contract. Required action data:

- create/import: kind, stable identity, bounded metadata, exact source and
  supplied digest;
- create_template: template kind/output, parameter schema, ceilings and bytes;
- instantiate_template: exact revision/digest, bounded parameters;
- promote_run: source Run/generation and Evidence IDs/digests;
- record_evaluation: Candidate/revision/baseline/fixture/Evidence identities;
- activate/reject/retain: Candidate, revision/digest and reason/evaluation IDs;
- archive/restore: exact revision/digest and reason;
- rollback: definition, current and target exact revisions/digests and reason.

Closed errors:

```text
invalid_request
not_found
conflict
stale_view
stale_generation
digest_mismatch
incompatible
denied
capability_gap
human_required
state_unavailable
timeout
busy
internal
```

Unknown/duplicate fields, invalid enums/UUIDs/digests/counts/cursors, mismatched
response journey/request identity and non-object params/results fail closed.
Structured messages contain no local source path or sensitive content.

## 11. Product interaction contract

The native app and TUI expose the same user concepts and action availability:

- searchable asset list and lifecycle/risk/compatibility filters;
- exact revision detail and two-revision diff;
- create/import with source/risk/permission preview;
- Candidate provenance and accepted Run/Evidence links;
- Evaluation baseline/results with unknown cost/usage shown truthfully;
- explicit activate/reject/retain/archive/restore/rollback confirmation;
- template instantiation preview that says Draft/Candidate-only;
- exact Team/Agent/WorkPackage binding and current Run pin;
- Runtime compatibility/materialization/cleanup/recovery state;
- canonical conflict/error explanation and next safe action.

The native surface stays inside `MissionWorkbench`; the TUI stays inside the
production Bubble Tea program. Back/cancel never writes. Reconnect refreshes the
authoritative view and never replays a mutation. Slow clients may refresh or
show an explicit gap; they may not hide lifecycle/decision/materialization/
terminal milestones.

## 12. Mandatory RED manifest

Before implementation, create
`.loom-evidence/phase3a/P3A-W1/red.md` recording exact failing tests and output
for every parent RED category. Required test names must include identifiable
cases for:

- unconfirmed import and Sidecar/model activation rejection;
- nonaccepted Run/Evidence promotion rejection;
- stale view/head/revision/generation and digest/identity mismatch zero-write;
- concurrent activate/rollback/dispatch single winner;
- archived/restore/exact rollback semantics;
- immutable Run binding across asset updates;
- Runtime capability incompatibility;
- repo/user Skill collision and no overwrite;
- atomic partial/crash materialization and restart recovery;
- replay/redelivery exact-once facts;
- Projection old-view preservation;
- disclosure-negative scans;
- all template kinds remain Draft/Candidate-only;
- strict Go/Swift journey wire identity;
- GUI and TUI state/action/error parity over production IPC fixtures.

RED must fail for the intended missing behavior, not compilation accidents or
test harness errors. Contract Review PASS is required before RED is captured.

## 13. Deterministic verification commands

Focused and impact:

```text
go test ./internal/assets ./internal/teams ./internal/work \
  ./internal/projection ./internal/runtime ./internal/runtime/piadapter \
  ./internal/app ./internal/api ./internal/localipc ./internal/tui \
  ./cmd/loom ./cmd/loomd

go test -race ./internal/assets ./internal/work ./internal/projection \
  ./internal/runtime/piadapter ./internal/app ./internal/localipc ./internal/tui
```

Whole Go repository:

```text
go test ./...
go test -race ./...
go vet ./...
test -z "$(gofmt -l $(rg --files -g '*.go'))"
go mod tidy
git diff --exit-code -- go.mod go.sum
```

Swift:

```text
swift test --package-path apps/macos
swift test --package-path apps/macos --sanitize=thread
swift build --package-path apps/macos -c release
```

Additional deterministic gates:

- canonical JSON/Event/manifest golden bytes and replay permutations;
- SQLite `PRAGMA integrity_check`, duplicate Event IDs/idempotency keys and
  stream-sequence gaps;
- repeated CAS/race/restart/redelivery matrix;
- Pi conformance fixture with repository/user collision sources;
- permissions, symlink/hard-link/traversal/case-fold and cleanup matrix;
- secret/raw-Grant/hidden-reasoning/prompt/token negative scan;
- `git diff --check`, exact owned-path and dependency checks.

`go mod tidy` may format module metadata only if bytes remain unchanged; any
dependency delta stops `HUMAN_REQUIRED`.

## 14. Real cross-client journey manifest

The final controlled root is fresh, private and not a real user Runtime root.
Before launch, freeze source hashes, binaries, commands, environment allowlist,
installed Pi identity/capability fixture, state/socket paths, journey ID,
process inventory and no-network assertion.

The real native window must exercise launch/reconnect, navigation, clicks,
input, selection, create/import, confirm, reject, retain/cancel, diff,
Evaluation, activate/archive/restore/rollback, daemon restart and error recovery.
The real PTY TUI must exercise equivalent keyboard navigation, selection,
pagination, confirm/reject/cancel/back, cursor reconnect, daemon restart and
error recovery.

The scenario matrix is exactly the parent Exit Contract matrix. Cross-observe
at least one GUI mutation in TUI and one TUI mutation in GUI without direct
state injection. Each action and IPC call carries the same scenario journey ID.

Evidence root layout is the Goal layout and permissions are `0700/0600`.
Failed results are immutable. Replacement requires Implementation Re-review and
a new root/journey ID. The scripts may start/stop production binaries, feed the
real PTY and collect read-only evidence; they may not call services/writers/
Projection/SQLite to cause product behavior.

## 15. Reviews, rollback, staging and commit

Required independent gates:

1. combined ADR/Exit/P3A-W1 Contract Review: `P0=P1=P2=0`, `PASS`;
2. Implementation Review: `P0=P1=P2=0`, `PASS`;
3. Product Result Review: `PASS`;
4. Operational and Trace Behavior Review: `PASS`;
5. whole-Phase 3A Candidate Review: `P0=P1=P2=0`, `PASS`.

Rollback before commit removes only P3A-created private fixture roots and
restores no pre-existing user file. Because unrelated dirty files are excluded,
rollback uses the exact owned-path manifest and never `git reset --hard` or
broad checkout. Failed journey evidence is retained immutable.

Only after all gates pass may exact reviewed paths be staged. The index must
contain no excluded/unrelated file, particularly
`internal/projection/team_execution_test.go`. One local atomic commit closes
Phase 3A. No push or merge.

VERDICT: `FROZEN — PENDING REVIEW`
