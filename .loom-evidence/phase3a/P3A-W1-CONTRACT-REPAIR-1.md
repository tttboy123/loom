# P3A-W1 Contract Repair 1: Exact Authority and Schema Closure

Date: `2026-08-03`

Status: `FROZEN — INDEPENDENT CONTRACT RE-REVIEW REQUIRED`

Parent Candidate:

- ADR-0013 SHA-256
  `b16234afbe8df5c114e7fd881932ab2ceeb8c4b6ad9d24646dd8ccd243a44931`;
- Exit Contract SHA-256
  `ddba05f803ce33eb18a8ae384e799b6c1c1d7d7311a6c01cc4b55de1492c3b8a`;
- P3A-W1 Contract SHA-256
  `12b8c1f2595c16cb7faabe889982182635b45d3d836f0ccc32e139357629fc9c`;
- Gate 1 Contract Review 1: `P0=0`, `P1=4`, `P2=0`, dual `FAIL`.

This Repair supersedes only the contradictory/incomplete parent clauses named
below. Every other parent requirement remains frozen. It creates no P3A-W2 and
grants no product-code, RED, live, staging or commit authority.

## 1. Repair P1-1: product ownership excludes governance

The entire parent section `2. Exact owned files / Governance and decision` is
deleted from the P3A-W1 product owned-path manifest.

ADR-0013, `docs/adr/README.md`, `docs/CURRENT.md` and
`.loom-evidence/phase3a/**` are Gate 0/Gate 1 governance or generated acceptance
evidence authorized by the Goal. They are not P3A-W1 product implementation
ownership and cannot justify a product edit. They remain eligible for the final
single atomic commit only after whole-Candidate Review binds their exact bytes.

The product owned-path manifest begins at `internal/assets/model.go` and remains
the exact set in the parent contract. The excluded pre-existing modified
`internal/projection/team_execution_test.go` remains unowned and unstaged.

## 2. Repair P1-2: authority-owned time

The parent mutation-common-field block is replaced by:

```text
operation_id
journey_id
expected_view_version
expected_stream_heads{}
```

`authoritative_time_utc` is not a command, IPC, GUI, TUI or canonical intent
field. Each authority receives an injected trusted clock at construction and
samples it exactly once after replay/validation and immediately before Event
construction. Every Event in one CAS batch uses that same UTC value in the
existing Journal `EmittedAt` envelope.

Production clocks use system UTC. Deterministic tests may inject a fixed clock
through an internal constructor/configuration only. The sampled time is:

- excluded from IPC request/response schemas;
- excluded from canonical command intent bytes/digest;
- excluded from operation-ID equivalence;
- never accepted from a client or Runtime;
- persisted only as Journal `EmittedAt` and server-derived domain deadlines
  where a separately frozen rule requires one.

Redelivery returns committed Events/times; it never resamples time for an
already committed operation.

## 3. Repair P1-3: exact subject binding authority

### Binding subject schema

Closed subject kinds:

```text
agent_definition | team_definition | work_package
```

`EvolutionAssetBindingRecord` contains exactly:

```text
schema_version                 integer = 1
subject_kind                   closed enum
subject_id                     required identifier
subject_version                integer 1..1000000
subject_digest                 required sha256
subject_scope                  required bounded identifier
asset_revision_bindings        array, 0..32, canonical sorted
asset_revision_set_digest      required sha256
binding_revision               integer >= 1
last_event_id                  required identifier
last_journey_id                required UUIDv4
```

An empty binding array means an explicit detach and has the SHA-256 of canonical
`[]`; it does not delete history. Each non-empty element is the exact
`ExactAssetRevisionBinding` schema from the parent contract. Sorting key is
`asset_kind`, `definition_id`, `revision_id`, `sha256_digest`, `source_scope`.

### Subject resolution

The asset authority owns an internal `BindingSubjectResolver`, configured by
the daemon, that returns a freshly validated immutable subject identity and,
where Journal-backed, its current stream head:

```text
ResolveAgentDefinition(id, version, digest)
ResolveTeamDefinition(id, version, digest)
ResolveWorkPackage(id, version, digest)
```

The resolver uses existing typed constructors/catalogs and the current accepted
Team-definition fact. It never trusts a client-provided subject digest. Immutable
catalog AgentDefinitions and WorkPackages must byte-rebuild to the supplied
digest; Journal-backed TeamDefinitions add their stream head to the CAS set.
Missing/ambiguous/stale/mismatched subjects have zero writes.

No change to `internal/agents/**`, `internal/teams/team_definition.go` or
`internal/work/work_package.go` is required or authorized. The new binding is a
separate Journal fact in the already owned `internal/assets/authority.go` and
is projected by the already owned P3A projection files.

### Binding Event

Add the exact Event name:

```text
EvolutionAssetBindingSetCommitted
```

Its payload contains exactly:

```json
{
  "schema_version": 1,
  "operation_id": "...",
  "subject_kind": "agent_definition|team_definition|work_package",
  "subject_id": "...",
  "subject_version": 1,
  "subject_digest": "<sha256>",
  "subject_scope": "...",
  "asset_revision_bindings": [],
  "asset_revision_set_digest": "<sha256>",
  "binding_revision": 1
}
```

No field is optional or nullable. The binding stream is included in later
planning and dispatch CAS. Projection/GlobalReadView expose the exact binding
record and defensive copies.

### Binding command and execution merge

Add closed action `set_binding`. Its strict action input is:

```json
{
  "subject_kind": "agent_definition|team_definition|work_package",
  "subject_id": "...",
  "subject_version": 1,
  "subject_digest": "<sha256>",
  "subject_scope": "...",
  "asset_revision_bindings": [],
  "asset_revision_set_digest": "<sha256>"
}
```

The authority resolves the subject, replays its binding stream and every exact
asset definition/revision/activation stream, validates `active` and digest,
then commits one binding Event by CAS. GUI/TUI show the subject identity,
current exact set, compatibility/permission delta and detach warning before
explicit confirmation.

Execution planning canonical-merges the currently committed TeamDefinition,
selected AgentDefinition and WorkPackage binding sets plus the existing exact
saved-Team Skill configuration. Identical entries deduplicate; the same
definition with different revisions/digests is `conflict` with zero dispatch.
The merged set and every source binding/activation head enter planning and
dispatch CAS. The exact merged set is copied into each node, Run and Attempt.

## 4. Repair P1-4: canonical encoding and identities

### Canonical JSON

All P3A canonical bytes use typed Go structs marshalled by `encoding/json` with
fields in the exact order frozen below, UTF-8, no insignificant whitespace,
HTML escaping disabled, no maps in canonical payloads, no floats, no `null`,
and non-nil arrays encoded as `[]`. UTC values exist only in the Journal
envelope and use the existing store representation. Decoders reject duplicate
and unknown keys and require one complete JSON value.

`canonical_intent` contains exactly:

```json
{"schema_version":1,"operation_id":"...","action":"...","input":{}}
```

`input` is the exact action schema in section 6. `journey_id`, `request_id`,
expected view/heads and authority time are excluded. `intent_digest` is the
lowercase SHA-256 of these bytes.

For `create_skill`, `import_skill` and `create_template`, canonical intent input
omits ephemeral `source_path` and contains the server-verified
`supplied_artifact_digest` and `supplied_content_digest`. All other request
fields copy exactly. Thus a path spelling cannot change authority identity and
unverified client digests cannot enter canonical intent.

### Exact immutable asset Artifact

Source files are normalized without executing or decompressing them into one
canonical JSON Artifact with exact ordered fields:

```text
schema_version = 1
asset_kind
definition_id
revision_id
entries[]
content_digest
```

Each entry is exactly:

```text
relative_path, file_mode, file_size, file_sha256, content_base64
```

Entries sort by UTF-8 relative path. Paths use `/`, contain no empty/dot/dot-dot
segment and are unique under exact and Unicode case-fold comparison.
`file_mode` is integer `384` (`0600`). `file_size` is the decoded byte length;
`file_sha256` is its lowercase digest; `content_base64` is RFC 4648 standard
base64 with padding and no whitespace. Raw content totals at most 1 MiB per
revision and 128 entries; the canonical Artifact is at most 1.5 MiB. A Run's
complete expanded asset set is at most 16 MiB.

`content_digest` is SHA-256 of canonical JSON `entries[]` with
`content_base64` retained. `artifact_digest` is SHA-256 of the complete Artifact
including `content_digest`. The Evidence Store publishes only when both
server-derived digests match the supplied request digests. No archive format,
compression, executable mode, symlink or filesystem metadata is preserved.

### Stream IDs

After identifier validation, exact stream formulas are:

```text
evolution-asset-definition/<asset_kind>/<definition_id>
evolution-asset-revision/<asset_kind>/<definition_id>/<revision_id>
evolution-asset-candidate/<candidate_id>
evolution-asset-evaluation/<evaluation_id>
evolution-asset-activation/<asset_kind>/<definition_id>
evolution-asset-binding/<subject_kind>/<subject_id>
runtime-skill-materialization/<run_id>/<attempt_number>/<generation>
```

IDs cannot contain `/`, so concatenation is injective. Attempt number and
generation are canonical base-10 without leading zero.

### Event IDs and idempotency keys

Each action freezes an ordered Event plan. For zero-based `event_index`:

```text
event_seed = schema_version + "\n" + event_type + "\n" + stream_id +
             "\n" + operation_id + "\n" + intent_digest + "\n" + event_index
event_id = "p3a-" + event_code + "-" + sha256(event_seed)[0:32]
idempotency_key = "p3a/" + operation_id + "/" + event_code + "/" + event_index
```

`event_code` is the exact lower snake-case Event name without the
`EvolutionAsset` or `RuntimeSkill` prefix. Event plan order is definition,
revision, import, candidate, evaluation, binding, decision/activation,
template/promotion, materialization, cleanup. A reused operation ID with a
different `intent_digest` is `conflict`; identical intent returns the original
Events even if journey/view/head metadata differs on redelivery.

The exact Event-code mapping is:

```text
EvolutionAssetDefinitionCreated          definition_created
EvolutionAssetRevisionCreated            revision_created
EvolutionAssetImportProposed             import_proposed
EvolutionAssetCandidateCreated           candidate_created
EvolutionAssetEvaluationRecorded         evaluation_recorded
EvolutionAssetBindingSetCommitted         binding_set_committed
EvolutionAssetCandidateActivated         candidate_activated
EvolutionAssetCandidateRejected          candidate_rejected
EvolutionAssetCandidateRetained          candidate_retained
EvolutionAssetRevisionArchived           revision_archived
EvolutionAssetRevisionRestored           revision_restored
EvolutionAssetActivationRolledBack       activation_rolled_back
EvolutionTemplateInstantiated            template_instantiated
EvolutionRunPromotionProposed            run_promotion_proposed
RuntimeSkillMaterializationPublished     materialization_published
RuntimeSkillMaterializationCleaned       materialization_cleaned
```

Exact ordered Event plans are:

```text
create_skill:
  DefinitionCreated only when definition stream is empty,
  RevisionCreated, CandidateCreated
import_skill:
  DefinitionCreated only when definition stream is empty,
  RevisionCreated, ImportProposed, CandidateCreated
create_template:
  DefinitionCreated only when definition stream is empty,
  RevisionCreated, CandidateCreated
instantiate_template: TemplateInstantiated
promote_run:
  DefinitionCreated only when definition stream is empty,
  RevisionCreated, CandidateCreated, RunPromotionProposed
record_evaluation: EvaluationRecorded
set_binding: BindingSetCommitted
activate: CandidateActivated
reject: CandidateRejected
retain: CandidateRetained
archive: RevisionArchived
restore: RevisionRestored
rollback: ActivationRolledBack
dispatch materialization: MaterializationPublished in dispatch CAS
terminal cleanup: MaterializationCleaned after verified removal
```

If a definition exists, its immutable kind/name/scope must match the request;
otherwise the action is `conflict`. There is no no-op placeholder Event, so
indices are assigned over the actual deterministic plan above.

## 5. Exact Event JSON schemas

All payloads start with fields in this order:

```text
schema_version, operation_id
```

The remaining exact ordered fields are below. No additional, optional or
nullable payload field exists. Fields described as arrays encode `[]` when
empty.

```text
EvolutionAssetDefinitionCreated:
  asset_kind, definition_id, name, description, subject_scope

EvolutionAssetRevisionCreated:
  asset_kind, definition_id, revision_id, artifact_digest, content_digest,
  source_scope, source_reference_digest, provenance_digest, dependencies[],
  compatible_runtime_capabilities[], risk, lifecycle

EvolutionAssetImportProposed:
  candidate_id, asset_kind, definition_id, revision_id,
  external_source_digest, provenance_digest, risk

EvolutionAssetCandidateCreated:
  candidate_id, asset_kind, definition_id, revision_id, source_scope,
  source_run_id, source_run_generation, source_run_digest,
  source_evidence_ids[], source_evidence_digests[], redacted_summary,
  redacted_summary_digest, scope_difference, expected_benefit, risk,
  required_evaluation_ids[]

EvolutionAssetEvaluationRecorded:
  evaluation_id, candidate_id, fixture_kind, fixture_digest,
  baseline_revision_id, baseline_digest, candidate_revision_id,
  candidate_digest, quality_result, failure_count, case_count,
  usage_observed, usage_microunits, cost_observed, cost_microunits,
  cost_currency, compatibility_result, applicable_scope,
  regression_result, security_result, evidence_id, evidence_digest

EvolutionAssetBindingSetCommitted:
  subject_kind, subject_id, subject_version, subject_digest, subject_scope,
  asset_revision_bindings[], asset_revision_set_digest, binding_revision

EvolutionAssetCandidateActivated:
  candidate_id, asset_kind, definition_id, revision_id, revision_digest,
  expected_previous_revision_id, evaluation_ids[],
  decision_source

EvolutionAssetCandidateRejected:
  candidate_id, asset_kind, definition_id, revision_id, revision_digest,
  reason_code, decision_source

EvolutionAssetCandidateRetained:
  candidate_id, asset_kind, definition_id, revision_id, revision_digest,
  reason_code, decision_source

EvolutionAssetRevisionArchived:
  asset_kind, definition_id, revision_id, revision_digest,
  previous_lifecycle, reason_code

EvolutionAssetRevisionRestored:
  asset_kind, definition_id, revision_id, revision_digest,
  restored_lifecycle, reason_code

EvolutionAssetActivationRolledBack:
  asset_kind, definition_id, from_revision_id, to_revision_id,
  from_digest, to_digest, evaluation_ids[], reason_code,
  decision_source

EvolutionTemplateInstantiated:
  asset_kind, definition_id, revision_id, revision_digest, template_output,
  parameter_digest, output_candidate_id, output_digest

EvolutionRunPromotionProposed:
  candidate_id, asset_kind, definition_id, revision_id,
  source_run_id, source_run_generation, source_run_digest,
  source_evidence_ids[], source_evidence_digests[],
  redacted_summary_digest

RuntimeSkillMaterializationPublished:
  team_execution_id, logical_node_id, run_id, attempt_number, generation,
  runtime_instance_id, runtime_identity_digest, capability,
  asset_revision_bindings[], asset_revision_set_digest,
  manifest_artifact_digest, materialization_root_digest

RuntimeSkillMaterializationCleaned:
  run_id, attempt_number, generation, manifest_artifact_digest,
  materialization_root_digest, cleanup_result
```

Closed values:

- `decision_source` is exactly `user_explicit`;
- `cleanup_result` is exactly `removed`;
- `fixture_kind` is `historical|synthetic`;
- observed=false requires microunits=0 and currency empty; observed=true
  requires non-negative microunits and, for cost, a three-letter uppercase
  currency;
- result strings are bounded closed values frozen by Evaluation test fixtures,
  never free-form model output;
- promoted-only fields in Candidate Event are required; local/import Candidate
  Events encode source Run strings as empty, generation as `0` and source
  Evidence arrays as `[]`.

Additional exact enums are:

```text
quality_result       pass | fail | partial
compatibility_result compatible | incompatible | partial
regression_result    improved | equivalent | regressed | unknown
security_result      pass | fail | partial
reason_code          user_rejected | keep_for_later | superseded |
                     risk_unacceptable | evaluation_failed |
                     archive_requested | restore_requested |
                     rollback_requested
```

`restored_lifecycle` is exactly `candidate`; restore never activates.
`previous_lifecycle` is `candidate|active`. Archiving an active revision touches
the activation stream in the same CAS and leaves no implicit replacement.
`expected_previous_revision_id` is encoded as an empty string only for first
activation. All other ID/digest fields are non-empty.

## 6. Exact IPC action schemas

P3A Request envelope field order is exactly:

```json
{"version":1,"request_id":"...","journey_id":"...","method":"...","params":{}}
```

P3A Response envelope field order is exactly:

```json
{"version":1,"request_id":"...","journey_id":"...","ok":true,"result":{},"error":null}
```

or the existing strict error form with `ok:false`, absent/empty result as
required by the existing protocol contract and exact echoed identities. For
pre-P3A methods `journey_id` is omitted, not empty or null. For P3A methods it
is required canonical lowercase UUIDv4 in both directions.

### Read params

```text
evolution_asset_snapshot params, exact order:
  cursor, limit, asset_kind, lifecycle, search_text

evolution_asset_diff params, exact order:
  definition_id, left_revision_id, left_digest,
  right_revision_id, right_digest
```

Empty filter/cursor strings are permitted; all other empty identifiers/digests
are rejected. Snapshot result exact fields:

```text
view_version, next_cursor, definitions[], revisions[], candidates[],
evaluations[], bindings[], materializations[]
```

Every record uses its exact domain/projection schema; arrays are sorted and
encode `[]`. Diff result exact fields:

```text
definition_id, left_revision_id, left_digest, right_revision_id,
right_digest, changes[]
```

Each change is exactly `kind`, `relative_path`, `left_digest`, `right_digest`;
`kind` is `added|removed|changed|unchanged`.

### Command params

Common exact order:

```text
operation_id, action, expected_view_version, expected_stream_heads[], input
```

Each head entry is exactly `stream_id`, `sequence`, `event_id`, sorted by
stream ID. `input` uses one exact schema:

```text
create_skill:
  definition_id, revision_id, name, description, subject_scope, source_path,
  supplied_artifact_digest, supplied_content_digest, dependencies[],
  compatible_runtime_capabilities[], risk

import_skill:
  candidate_id, definition_id, revision_id, name, description, subject_scope,
  source_path, supplied_artifact_digest, supplied_content_digest,
  external_source_digest, provenance_digest, dependencies[],
  compatible_runtime_capabilities[], risk

create_template:
  asset_kind, definition_id, revision_id, name, description, subject_scope,
  source_path, supplied_artifact_digest, supplied_content_digest,
  template_output, parameter_schema_digest, permission_ceiling_digest,
  scope_ceiling_digest, compatible_runtime_capabilities[], risk

instantiate_template:
  asset_kind, definition_id, revision_id, revision_digest,
  parameter_values[], parameter_digest

promote_run:
  candidate_id, asset_kind, definition_id, revision_id,
  source_run_id, source_run_generation, source_run_digest,
  source_evidence_ids[], source_evidence_digests[], redacted_summary,
  redacted_summary_digest, scope_difference, expected_benefit, risk

record_evaluation:
  evaluation_id, candidate_id, fixture_kind, fixture_digest,
  baseline_revision_id, baseline_digest, candidate_revision_id,
  candidate_digest, requested_case_ids[]

set_binding:
  subject_kind, subject_id, subject_version, subject_digest, subject_scope,
  asset_revision_bindings[], asset_revision_set_digest

activate:
  candidate_id, asset_kind, definition_id, revision_id, revision_digest,
  expected_previous_revision_id, evaluation_ids[]

reject | retain:
  candidate_id, asset_kind, definition_id, revision_id, revision_digest,
  reason_code

archive | restore:
  asset_kind, definition_id, revision_id, revision_digest, reason_code

rollback:
  asset_kind, definition_id, from_revision_id, from_digest,
  to_revision_id, to_digest, evaluation_ids[], reason_code
```

`source_path` is an ephemeral absolute clean local input used only by the
daemon's no-follow bounded reader. It is absent from canonical intent, Events,
Evidence, logs and responses; canonical intent replaces it with the supplied
Artifact/content digests after server-side byte verification. This is the only
exception to the literal request-to-intent field copy.

Parameter values are exact sorted `{name,value}` pairs with bounded safe text;
they cannot contain secrets or paths. `record_evaluation` requests a
server-owned deterministic evaluator; result metrics and Evidence IDs/digests
are server-produced and never trusted from IPC.

Command result exact fields:

```text
operation_id, action, view_version, event_ids[], definition_id, revision_id,
candidate_id, evaluation_id, subject_kind, subject_id,
asset_revision_set_digest, materialization_manifest_digest, status
```

Non-applicable identity strings are encoded empty, Event arrays as `[]`; status
is the canonical resulting lifecycle/decision/materialization state.

## 7. Exact materialization manifest schema

The stored manifest is exact ordered JSON:

```text
schema_version = 1
run_id
attempt_number
generation
runtime_instance_id
runtime_identity_digest
capability = loom.skill-materialization.pi.v1
asset_revision_bindings[]
asset_revision_set_digest
entries[]
manifest_digest
```

Each entry exact order:

```text
asset_kind, definition_id, revision_id, source_artifact_digest,
content_digest, target_relative_path, file_mode, file_size, file_sha256
```

Entries sort by target path then canonical asset tuple. `target_relative_path`
uses `/`, contains no empty/dot/dot-dot segment and is relative. `file_mode` is
integer `384` (`0600`); file size is non-negative and bounded. Directory modes
are not entries and are always `448` (`0700`).

`manifest_digest` is SHA-256 of the same canonical object with the final field
omitted. `materialization_root_digest` is SHA-256 of canonical JSON containing
exactly `schema_version`, run ID, attempt, generation, Runtime ID,
Runtime identity digest and manifest digest. Neither digest includes an
absolute filesystem path.

## 8. Exact journey manifest and evidence schema

Each scenario has its own immutable journey root and UUID. The final
cross-client result index references all scenario roots; it does not reuse one
ID for unrelated scenarios.

`manifest.json` exact ordered fields:

```text
schema_version = 1
journey_id
scenario_id
created_at_utc
baseline_commit
entry_amendment_digest
gate1_contract_set_digest
source_lock_digest
daemon_binary_digest
gui_binary_digest
tui_binary_digest
runtime_fixture_digest
state_root_path_digest
socket_path_digest
network_allowed = false
provider_credentials_present = false
production_components[]
user_actions_digest
ipc_summary_digest
daemon_log_digest
journal_summary_digest
projection_summary_digest
sqlite_summary_digest
artifact_verification_digest
preflight_process_digest
postflight_process_digest
cleanup_proof_digest
product_result
operational_trace_result
evidence_files[]
manifest_digest
```

Each production component is exactly `kind`, `path_digest`, `sha256`, `build_id`.
Each evidence file is exactly `relative_path`, `sha256`, `size`, `mode`, sorted
by path. Result enums are `PASS|FAIL|PARTIAL`. All required Goal evidence paths
must appear. Absolute paths are represented only by salted path digests; no
credential/environment value is recorded.

`manifest_digest` is SHA-256 of the canonical object with that final field
omitted. `source-lock.json` contains the exact sorted owned source paths and
SHA-256 values plus the four Gate 1 governance Candidate digests. It excludes
untracked build outputs and every unrelated dirty file.

`ipc/request-response-summary.jsonl` exact fields per line are:

```text
sequence, monotonic_offset_micros, client_kind, request_id, journey_id,
method, action, request_digest, response_ok, error_code, response_digest
```

`daemon/structured-log.jsonl` exact fields per line are:

```text
sequence, monotonic_offset_micros, journey_id, request_id, component,
operation, phase, outcome, error_code, authority_event_ids[]
```

Logs contain digests/IDs only, never params, paths, asset bytes, prompts,
credentials or Runtime output.

The remaining exact evidence schemas are frozen here:

```text
gui/actions.jsonl:
  sequence, monotonic_offset_micros, action, control_id, input_digest,
  expected_visible_state, observed_visible_state, screenshot_relative_path

tui/keystrokes.jsonl:
  sequence, monotonic_offset_micros, key, screen_id,
  expected_visible_state, observed_visible_state

timeline.jsonl:
  sequence, monotonic_offset_micros, source, kind, subject_id, status,
  journey_id, request_id, authority_event_ids[]

journal/event-summary.json:
  schema_version, journey_id, event_count, events[]
  event item = event_id, event_type, stream_id, sequence, correlation_id,
               causation_id, emitted_at_utc, payload_digest

journal/stream-heads.json:
  schema_version, journey_id, streams[]
  stream item = stream_id, sequence, event_id

journal/sqlite-summary.json:
  schema_version, journey_id, integrity_check, foreign_key_violation_count,
  event_count, duplicate_event_id_count, duplicate_idempotency_key_count,
  stream_gap_count, journey_event_count

artifacts/digest-verification.json:
  schema_version, journey_id, artifacts[]
  artifact item = expected_digest, actual_digest, size, available, match

processes/preflight.json and processes/postflight.json:
  schema_version, journey_id, processes[], sockets[], locks[], leases[], temps[]
  process item = pid, executable_digest, role, state
  resource item = kind, path_digest, owner_uid, mode, present
```

All arrays are sorted by their primary identity and encode `[]`. GUI action and
TUI keystroke enums are the closed actions/keys frozen in the runbook source
lock; they cannot change after Implementation Review. `tui/transcript.txt`,
GUI PNG/recording, `result.md`, raw integrity command output and
`processes/cleanup-proof.txt` are byte evidence referenced by digest, not JSON
authority. Cleanup PASS requires postflight arrays empty except the explicitly
authorized resident daemon, which must match its preflight identity.

## 9. Re-review acceptance

Fresh independent Re-review must prove:

1. governance is no longer presented as W1 product ownership;
2. authority time is internal and non-client/non-idempotency input;
3. Agent/Team/WorkPackage binding has exact resolver, Event, command,
   projection, CAS and execution-merge semantics inside existing allowed files;
4. every Event/action/manifest/journey schema and identity formula is frozen
   before RED with no authoritative placeholder deferred to tests;
5. all parent authority, security, Cross-client Exit Gate, verification,
   exclusion and single-W1 rules remain intact.

Re-review must return `P0=0`, `P1=0`, `P2=0`, Product/Authority `PASS` and
Operational/Trace Governance `PASS`. Otherwise product code remains locked.

VERDICT: `FROZEN — PENDING RE-REVIEW`
