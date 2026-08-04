# Phase 3A Bounded Entry Amendment

Date: `2026-08-03`

Status: `FROZEN — INDEPENDENT CONTRACT REVIEW REQUIRED`

Name:

```text
Phase 3A Exact Asset Lineage, Native Materialization and Cross-client Journey Boundary
```

Parent Goal: `Phase 3A Entry Audit and Versioned Evolution Assets`

Baseline: `6d380233b5b89309a1a7ce3919aa611654e0f4ee`

WorkItem impact: the single `P3A-W1` only. `P3A-W2` does not exist.

Authorization: the Product Owner explicitly authorized freezing and independent
review of this Amendment while preserving one P3A-W1 and prohibiting P3A-W2.

This Amendment is a boundary decision, not an implementation contract. A PASS
only permits Gate 1 to freeze the ADR, Phase 3A Exit Contract and exact P3A-W1
contract. Product code, RED, migration, Runtime materialization, daemon launch,
GUI/TUI journey and commit remain locked until those contracts independently
pass Review.

## 1. Evidence-led necessity

The accepted baseline has the Journal/CAS, immutable Evidence, terminal
acceptance, GlobalReadView and Credential Broker foundations needed by Phase
3A. The Entry Audit nevertheless proves three blocking gaps:

1. exact Skill ID/revision/digest stops at saved-Team configuration and is not
   copied into ExecutionPlan, dispatch, Run or Attempt authority;
2. Pi advertises no governed Skill materialization capability and all accepted
   Pi execution paths explicitly use `--no-skills`;
3. production GUI, TUI and local IPC exist, but there is no correlation-complete
   shared-root native-window plus PTY journey substrate.

Closing those gaps as separate WorkItems would violate the vertical Exit Gate.
This Amendment therefore reopens only the minimum shared seams inside P3A-W1.

## 2. Frozen authority decision

The P3A ADR, Exit Contract and P3A-W1 contract may extend the accepted authority
only as follows:

- create versioned evolution-asset streams whose only writer uses the existing
  Journal `ReadStreamSet` and `AppendBatchIfStreamHeads` transaction boundary;
- copy a canonical, sorted, bounded exact asset revision set and its manifest
  digest from accepted saved configuration into ExecutionPlan, Team dispatch,
  Run and every Attempt before execution;
- add one observed Runtime capability named
  `loom.skill-materialization.pi.v1` and a private per-Run Pi materializer;
- add strict correlation-only `journey_id` metadata to production IPC, P3A
  application commands, existing Journal `Event.CorrelationID`, P3A
  projections, structured daemon logs and Evidence manifests;
- expose the vertical asset lifecycle through the accepted chat/task-first
  native client and real Bubble Tea TUI over the same production daemon.

No new Journal, store, scheduler, queue, state writer, Projection authority,
credential store, Runtime launcher, Provider route or client cache is allowed.

## 3. Exact shared reopening allowlist

The later P3A-W1 contract may own a subset of the following existing files, but
may not name an existing shared file outside this list without a new reviewed
repair to this Amendment. Production edits remain prohibited until the child
contract passes.

### Saved configuration to immutable execution lineage

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
- `internal/projection/team_execution_test.go`
- `internal/projection/projection.go`
- `internal/projection/projection_test.go`
- `internal/projection/global_read_view.go`
- `internal/projection/global_read_view_test.go`

### Runtime capability and private Pi materialization

- `internal/runtime/catalog.go`
- `internal/runtime/catalog_test.go`
- `internal/runtime/pi_probe.go`
- `internal/runtime/pi_probe_test.go`
- `internal/runtime/piadapter/execution_adapter.go`
- `internal/runtime/piadapter/execution_adapter_test.go`
- `internal/runtime/piadapter/process_runner.go`
- `internal/runtime/piadapter/process_runner_test.go`
- `internal/runtime/piadapter/rpc_bridge_adapter.go`
- `internal/runtime/piadapter/rpc_bridge_adapter_test.go`

The child contract must prove which of the last six files actually require
native-path injection. Merely deleting `--no-skills` is forbidden.

### Production IPC, daemon and TUI

- `internal/localipc/protocol.go`
- `internal/localipc/protocol_test.go`
- `internal/localipc/client.go`
- `internal/localipc/client_test.go`
- `internal/localipc/server.go`
- `internal/localipc/server_test.go`
- `internal/localipc/swift_contract_test.go`
- `cmd/loomd/product_daemon.go`
- `cmd/loomd/product_daemon_test.go`
- `cmd/loom/tui.go`
- `cmd/loom/main_test.go`
- `internal/tui/model.go`
- `internal/tui/model_test.go`
- `internal/tui/program.go`
- `internal/tui/program_test.go`

### Production macOS client

- `apps/macos/Sources/LoomLocalApp/LoomLocalApp.swift`
- `apps/macos/Sources/LoomLocalAppCore/LocalIPCClient.swift`
- `apps/macos/Sources/LoomLocalAppCore/LocalProductExperience.swift`
- `apps/macos/Sources/LoomLocalAppCore/LocalProductStore.swift`
- `apps/macos/Sources/LoomLocalAppUI/ContentView.swift`
- `apps/macos/Sources/LoomLocalAppUI/MissionWorkbench.swift`
- `apps/macos/Tests/LoomLocalAppTests/LocalIPCClientTests.swift`
- `apps/macos/Tests/LoomLocalAppTests/LocalProductExperienceTests.swift`
- `apps/macos/Tests/LoomLocalAppTests/LocalProductExperienceViewTests.swift`
- `apps/macos/Tests/LoomLocalAppTests/LocalProductStoreTests.swift`

The child contract must retain the accepted chat/task-first interaction model.
It may not restore a dashboard-first flow or add a visual-only asset surface.

## 4. Exact new-file namespace

Only these new source/test/runbook paths may be created by the later P3A-W1
contract. The child contract must select and own exact files from this list; an
unused path remains absent.

- `internal/assets/model.go`
- `internal/assets/model_test.go`
- `internal/assets/canonical.go`
- `internal/assets/canonical_test.go`
- `internal/assets/authority.go`
- `internal/assets/authority_test.go`
- `internal/assets/replay.go`
- `internal/assets/replay_test.go`
- `internal/projection/evolution_assets.go`
- `internal/projection/evolution_assets_test.go`
- `internal/app/local_product_assets.go`
- `internal/app/local_product_assets_test.go`
- `internal/api/local_product_assets.go`
- `internal/api/local_product_assets_test.go`
- `internal/runtime/piadapter/skill_materialization.go`
- `internal/runtime/piadapter/skill_materialization_test.go`
- `apps/macos/Sources/LoomLocalAppCore/LocalProductAssetModels.swift`
- `apps/macos/Tests/LoomLocalAppTests/LocalProductAssetModelsTests.swift`
- `scripts/run-phase3a-cross-client-journey.sh`
- `scripts/verify-phase3a-cross-client-journey.sh`
- `docs/runbooks/phase3a-cross-client-journey.md`

Generated immutable result evidence belongs only under:

```text
.loom-evidence/phase3a/P3A-W1/journey-<journey_id>/
```

The generated evidence tree is not permission to add executable product code
or mutable state under `.loom-evidence`.

## 5. Frozen schema envelope

This Amendment permits the child contract to freeze only these schema classes.

### Exact asset revision set

Each entry contains exactly:

```text
asset_kind
definition_id
revision_id
sha256_digest
source_scope
```

The set is non-empty where required, bounded, sorted by the canonical tuple,
duplicate-free and digest-bound. Run and Attempt facts copy the values; they do
not resolve a mutable alias such as `active` during execution.

### Runtime materialization manifest

The immutable Artifact manifest contains exact revision entries, artifact
digests, target-relative Runtime-native paths, file modes, Runtime instance and
capability identity, Run/Attempt/generation binding and manifest digest. It
contains no raw Grant, credential, hidden reasoning, sensitive prompt, token
stream or unredacted external script.

### Journey metadata

The local IPC request and response may add:

```json
{"journey_id":"xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx"}
```

It is optional for pre-P3A protocol methods for wire compatibility and required
for every P3A mutation/read used by final journey evidence. Go and Swift use the
same canonical lowercase UUID validation. `request_id` remains per-call
identity. At the application boundary the validated journey ID is copied to the
existing Journal `Event.CorrelationID`; the Journal store schema is unchanged.

`journey_id` must never become an Event ID, stream ID, authority decision,
expected head, idempotency key, generation, Grant, digest input or retry key.
Repeated delivery is still governed by the frozen command identity and CAS.

## 6. Native materialization boundary

Materialization is permitted only after exact revision/digest validation,
active lifecycle state, Runtime capability compatibility, Run/Attempt/
generation binding and applicable Grant authorization.

The target is a fresh private directory beneath an isolated per-Run root. The
materializer must:

- create directories as `0700` and files as `0600` unless the Runtime contract
  requires a stricter executable mode for reviewed local bytes;
- write to a sibling temporary directory, fsync as supported, validate every
  byte and digest, then publish with a same-filesystem atomic rename;
- reject symlink, hard-link, traversal, case-fold collision, duplicate target,
  existing non-owned target, wrong owner/mode and identity drift;
- treat repository and user Skill paths as read-only collision inputs and never
  overwrite or delete them;
- bind the published manifest to one Run/Attempt/generation and never hot-update
  it;
- remove only the verified Loom-owned private root during cleanup and make
  interrupted cleanup/rebuild deterministic and idempotent;
- fail closed without selecting a fallback revision or modifying a real user
  Runtime configuration.

Pi execution may replace `--no-skills` only with an exact reviewed private-path
argument supported by the locked Pi compatibility fixture. Discovery must
advertise `loom.skill-materialization.pi.v1` only when the installed instance
truthfully supports that exact contract.

## 7. Cross-client journey boundary

The final P3A-W1 journey must build and operate:

```text
production loomd + one isolated root/socket
production native Loom app + real window input
production loom TUI + real PTY keystrokes
```

Both clients use the same daemon, application services, Journal, Projection,
GlobalReadView, Artifact Store and root. No script may call an application
service, writer, Projection reader or SQLite directly to manufacture a product
result. Scripts may only orchestrate production processes, record input/output,
inspect post-run evidence read-only and prove cleanup.

The child Exit Contract must preserve the full scenario matrix, `0700/0600`
source-locked bundle, GUI action/screenshots or recording, TUI transcript and
keystrokes, structured logs, IPC summary, Journal/heads/SQLite/digest checks and
pre/post process/socket/lock/lease cleanup required by `GOAL.md`.

Missing either real client, a shared root, correlation continuity, authority
evidence, cleanup evidence, Product Result PASS or Operational and Trace
Behavior PASS makes P3A-W1 `FAIL` or `PARTIAL`.

## 8. Paths and authority that remain closed

The following remain outside this Amendment:

- `internal/journal/store.go` and tests: existing CorrelationID and transaction
  APIs must be reused;
- `internal/evidence/store.go` and tests: existing immutable content-addressed
  API must be reused;
- Credential Broker, Keychain, Provider configuration and all secret stores;
- root policy, validators and accepted P2B-W1 source/evidence;
- Phase 3B, routing, Scheduling, sandbox, Provider fallback/checkpoint,
  Autopilot/webhook, Marketplace, shared/multi-user/Web and remote integration;
- `go.mod`, `go.sum`, `Package.swift` and external dependency changes;
- network, paid Provider, real user Runtime/configuration mutation, push, merge,
  install or live canary authority.

If the child contract proves an existing closed API insufficient, work stops
`HUMAN_REQUIRED`; it may not silently expand this list.

## 9. Independent Review acceptance

The Amendment passes only if a fresh read-only Reviewer proves all of the
following:

1. the three Entry Audit gaps can plausibly close inside one vertical P3A-W1;
2. the reopening is sufficient but does not create a second authority or
   silently modify Phase 2A/P2B-W1;
3. exact asset lineage is copied before execution and cannot resolve mutable
   state mid-Run;
4. Pi materialization is capability-gated, private, atomic, collision-safe and
   non-mutating toward repository/user Skills;
5. `journey_id` is strict correlation metadata and existing Journal schema is
   sufficient;
6. GUI and TUI remain production clients over one daemon/root and cannot be
   replaced by mocks, direct service calls or visual-only evidence;
7. no P3A-W2, dependency change, Provider/network action, live canary, staging
   or product edit is authorized by Review PASS.

Any blocking P0/P1/P2 finding returns `FAIL` and keeps Gate 1 closed.

VERDICT: `FROZEN — PENDING REVIEW`
