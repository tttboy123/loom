# P3A-W1 Amendment: Exact Asset Lineage, Native Materialization and Cross-client Journey Boundary

Date: `2026-08-03`

Status: `FROZEN — INDEPENDENT AMENDMENT REVIEW REQUIRED`

WorkItem: the existing and only `P3A-W1`. This Amendment does not create
`P3A-W2`, a new authority, a new database or a thin adapter/coordinator
WorkItem.

Reviewed parent set:

- P3A-W1 Contract SHA-256:
  `12b8c1f2595c16cb7faabe889982182635b45d3d836f0ccc32e139357629fc9c`;
- Contract Repair 1 SHA-256:
  `5a2b16778b5cccb989496f1918d0d81a59b140feb38e55716dbd7574d6a25a00`;
- Contract Repair 2 SHA-256:
  `b59b178b80bb1146a0e0e803846d097cf9db648ee7b7b9dbda0e102325127796`;
- Gate 1 PASS Review SHA-256:
  `626fc0702f252e35b8c9177bd4328fb9eaea67a198f13dac757b36786fe24890`.

This Amendment supersedes only the clauses named below. Every unchanged
parent/Repair security, authority, exact-schema, owned-path, RED, verification,
cross-client, review, exclusion and single-commit requirement remains frozen.
No product or live action may rely on this Amendment until a fresh independent
read-only Reviewer returns `P0=P1=P2=0` and dual PASS.

## 1. Discovery that requires the Amendment

The parent Repair placed `materialization_manifest_digest` in a node-level
semantic binding while the exact manifest includes `run_id`,
`attempt_number` and `generation`. A retry or recovery therefore must produce a
different manifest digest, but a node semantic binding is immutable for the
whole Team execution. A fixed node digest cannot truthfully bind more than one
Attempt.

The parent also requires the published materialization fact to be committed in
the dispatch CAS before Grant/Pi launch. That requires an exact attempt to be
selected and materialized before the CAS, while still guaranteeing a losing
scheduler leaves no authoritative or executable residue.

The resolution below preserves one Journal, one dispatch writer, one
Projection and one P3A-W1.

## 2. Exact lineage levels

### 2.1 Definition binding and ExecutionPlan

`EvolutionAssetBindingSetCommitted`, the saved Team/Agent/WorkPackage binding
merge and `ExecutionNode` retain exactly:

```text
asset_revision_bindings[]
asset_revision_set_digest
```

The canonical merge precedence is saved-Team base, TeamDefinition,
AgentDefinition, then WorkPackage. Identical tuples deduplicate. A later level
may replace an earlier tuple only for the same `asset_kind + definition_id`;
two different revisions at the same precedence level are `conflict`. Every
source binding stream, active-revision stream and subject source stream is in
the dispatch head set.

`TeamExecutionPlanned.nodes[]` and `semantic_bindings[]` both carry the same
two fields. The node-semantic payload no longer carries
`materialization_manifest_digest`. The plan digest and semantic digest include
the exact canonical binding array and set digest.

### 2.2 Attempt-specific lineage

Only the selected Attempt can carry:

```text
asset_revision_bindings[]
asset_revision_set_digest
materialization_manifest_digest
materialization_root_digest
```

Those four fields are copied byte-for-byte into:

- `RuntimeSkillMaterializationPublished`;
- the corresponding `TeamReadySetDispatched.attempts[]` item;
- the corresponding `RunClaimed` payload;
- Team Attempt and Run Projection records;
- the immutable execution request used after authoritative re-read.

For an execution with no bound assets, all three arrays are `[]`, the set,
manifest and root digest strings are empty and
`asset_lineage_available=false`. For a new execution with one or more bound
assets, all digests are lowercase SHA-256 and
`asset_lineage_available=true`. Partial presence is malformed. Existing
pre-P3A facts remain readable as empty/false and are never backfilled.

Retry/recovery creates a new Attempt and generation-specific manifest. It does
not mutate the prior Attempt, Run, manifest or materialized root.

## 3. Prepared native materialization and the single dispatch CAS

For each selected ready Attempt, the application performs exactly this order:

1. read one immutable `GlobalReadView` and correlated stream-set snapshot;
2. deterministically derive WorkItem, Run, Attempt and generation identities;
3. re-read every exact Artifact by digest and validate active revision,
   Runtime identity and `loom.skill-materialization.pi.v1` capability;
4. build and fsync a sibling temporary private materialization tree;
5. publish the final no-replace attempt root and its exact manifest Artifact;
6. ask the evolution authority for a prepared, immutable
   `RuntimeSkillMaterializationPublished` Event plus its exact zero/current
   stream-head expectation;
7. call the existing Team dispatch authority once; it validates the prepared
   Event and performs one `AppendBatchIfStreamHeads` containing the
   materialization Event, Team dispatch, WorkItem, Run, Runtime capacity and
   status-reference facts;
8. rebuild/re-read Projection and require all four Attempt lineage values to
   equal the filesystem manifest before issuing a Grant or invoking Pi.

The Team dispatch authority remains the sole writer for this composite
transaction. The evolution authority may prepare and validate its Event but may
not append it separately. No second writer, transaction, retry loop or
best-effort reconciliation is introduced.

The materialization operation ID is the deterministic ASCII ID
`materialize:<run_id>:<attempt_number>:<generation>`. Its canonical intent
binds Team execution ID, logical node ID, Run, Attempt, generation, Runtime
identity, capability, exact bindings, set digest, manifest Artifact digest and
root digest. Redelivery with identical bytes returns the committed Event;
different bytes conflict.

If publication fails, no Journal write or launch occurs and the verified temp
root is removed. If the dispatch CAS loses, the losing scheduler verifies the
manifest and removes only its own uncommitted root; it never removes an
authoritative winner's root. A crash after filesystem publication but before
CAS leaves an uncommitted root that startup classifies and removes only after
exact manifest/ownership verification. A crash after CAS reconstructs or
reuses only the committed current-generation root. Corrupt, foreign or
ambiguous roots become `human_required`; Pi is not launched.

Grant issue, `Evidence.BeginAttemptCapture` and Pi invocation occur only after
step 8. Terminal cleanup verifies generation plus manifest/root digests,
removes the root, then appends exactly one
`RuntimeSkillMaterializationCleaned` fact. Cleanup failure is visible and never
rewrites terminal Evidence.

## 4. Artifact and template closure

The common immutable Artifact envelope remains exactly:

```text
schema_version, asset_kind, definition_id, revision_id, entries[],
content_digest
```

Each entry remains exactly:

```text
relative_path, file_mode, file_size, file_sha256, content_base64
```

For `skill`, create/import accepts one bounded regular local source file. For a
template kind the server constructs a mandatory first entry named
`template-contract.json` with exact ordered fields:

```text
schema_version, asset_kind, template_output, parameter_schema_digest,
permission_ceiling_digest, scope_ceiling_digest, source_file_sha256
```

The optional second entry is the bounded regular source file. Entry sorting and
the outer content digest bind both. `instantiate_template` re-reads this
Artifact, verifies every digest and exact metadata value, validates sorted
parameter values against the frozen schema/ceilings, then calls the existing
typed Draft/Candidate-only output boundary. It cannot create a TeamInstance,
Run, Grant or expanded permission.

Promotion accepts only a terminal succeeded Run whose WorkItem has an
authoritative `accepted` decision and whose required source/verifier Evidence
IDs and digests all re-read successfully from the Evidence Store. The server
derives the source Run digest from canonical Run/Attempt/acceptance/Evidence
lineage and creates a new immutable `SUMMARY.md` Artifact solely from the
bounded user-visible redacted summary. Raw Evidence, Grant, prompt, hidden
reasoning and token output are never copied into the promoted Artifact.

Artifact publication is content-addressed and may precede the Journal CAS only
as inert immutable staging. A rejected command may leave a deduplicated inert
Artifact, but never a binding, activation, materialized Runtime path or
execution side effect. Journey cleanup reports such unreferenced fixture
Artifacts explicitly.

## 5. Runtime-native publication boundary

Every ancestor below the isolated materialization root is lstat/openat
validated without following links, owned by the current uid and mode `0700`.
Source Artifacts and output files are regular, single-link, mode `0600`, bounded
and digest-verified on the same descriptor. Duplicate/case-fold-colliding,
absolute, dot, dot-dot, symlink, hard-link, device, FIFO and socket entries fail
closed.

Final publication is same-filesystem and no-replace. A prior target of any
kind is a collision; an empty directory is not replaceable. Unsupported
platform no-replace semantics are `capability_gap`, not a fallback to
`os.Rename` replacement. Repository and user Runtime Skill paths are never
targets and are never scanned.

Pi receives only the exact committed private materialization root through its
reviewed native option. The conformance probe advertises the capability only
after a deterministic fixture proves option, path, collision and cleanup
semantics for the installed component identity.

## 6. Cross-client interaction closure

The native window and production PTY TUI must expose the same authoritative
concepts and action availability in this single W1:

- search/filter/page asset definitions and revisions;
- exact revision detail and two-revision diff;
- create/import and template Candidate preview;
- promotion provenance and accepted Evidence identity;
- Evaluation result with truthful unknown usage/cost;
- explicit activate/reject/retain/archive/restore/rollback confirmation;
- Team/Agent/WorkPackage exact binding preview and confirmation;
- current Run/Attempt pin, Runtime compatibility, materialization, cleanup and
  recovery state.

Every mutation uses an explicit confirmation surface. Back, Escape, Cancel and
dialog dismissal produce zero IPC mutation calls and zero Journal facts.
Reconnect and daemon restart issue reads only. Neither client may synthesize a
success state before the authoritative response and Projection refresh.

The real shared-root journey matrix is frozen to these scenario IDs, each with
a fresh UUIDv4 `journey_id` and immutable subroot:

```text
happy-create-evaluate-activate-bind-execute-clean
cancel-reject-retain
stale-view-digest-generation
concurrent-single-winner
crash-before-cas
crash-after-cas-before-response
projection-failure-rebuild-reconnect
slow-client-redelivery-clean-restart
```

The happy scenario must cross-observe a GUI mutation in TUI and a TUI mutation
in GUI, then execute through the exact committed private materialization root.
The crash/reconnect scenarios must include both clients across the matrix; a
visual-only fixture, service direct call or in-memory IPC substitution is not
evidence.

Each result separately records `product_result` and
`operational_trace_result`. Missing GUI, PTY, screenshots/transcript, IPC
summary, daemon log, Journal/Projection/SQLite/digest assertions or cleanup
proof is `PARTIAL`/`FAIL`, never PASS.

## 7. Tests and review additions

Mandatory RED and implementation evidence must additionally prove:

- node semantic lineage cannot contain an Attempt manifest digest;
- attempt 1 and retry Attempt 2 have distinct manifest/root digests while the
  same exact revision set remains pinned;
- materialization publication plus dispatch is one Journal transaction and a
  losing CAS leaves zero authoritative facts and zero executable root;
- no Grant, capture or Pi invocation is observable before authoritative
  re-read;
- restart classifies pre-CAS, post-CAS, missing, corrupt, foreign and stale
  generation roots exactly;
- template metadata survives replay solely through immutable Artifact bytes
  and all four template kinds remain Draft/Candidate-only;
- promotion re-reads accepted source/verifier Evidence and discloses only the
  redacted summary Artifact;
- Projection deep-copies exact Team/Attempt/Run lineage and explicitly marks
  legacy records unavailable;
- native and TUI confirmation cancellation makes zero writes;
- all eight real shared-root journey scenario packages pass both result axes.

The existing deterministic Go/Swift/race/vet/TSAN/Release/security matrix and
all five final independent review gates remain mandatory.

## 8. Ownership and exit

No product owned path is added. Implementation must stay within the exact
parent P3A-W1 allowlist; `internal/projection/team_execution_test.go` remains
excluded and unstaged. The Amendment and its review evidence live only under
the already-authorized `.loom-evidence/phase3a/**` governance path.

If this exact boundary cannot be implemented and verified inside the single
P3A-W1, Phase 3A stops `HUMAN_REQUIRED`; it does not create P3A-W2 or split a
writer/materializer/coordinator/journey WorkItem.

Fresh independent Amendment Review must report finding counts, separately
judge Product/Authority and Operational/Trace Governance, verify no scope or
owned-path expansion, and return PASS only with `P0=P1=P2=0`.

VERDICT: `FROZEN — PENDING INDEPENDENT AMENDMENT REVIEW`
