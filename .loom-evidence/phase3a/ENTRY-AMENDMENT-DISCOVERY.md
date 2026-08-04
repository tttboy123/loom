# Phase 3A Entry Amendment Contract Discovery

Date: 2026-08-03

Status: `DISCOVERY COMPLETE — PROPOSAL REMAINS UNFROZEN`

This is read-only contract discovery performed while the Product Owner
authorization gate remains closed. It changes no product code and does not
authorize ADR, Exit Contract, RED, client journey or implementation.

## Existing data path and loss point

The accepted setup path already stores exact Skill references:

```text
SetupSkillRevision
-> TeamConfigurationSkillRevision
-> TeamDefinitionSaved.configuration.role_bindings[].skill_revisions[]
-> Projection TeamDefinitionRecord.Configuration
```

The executable path then loses that set:

```text
projected saved Team configuration
-> productSavedTeamMaterializationInputs
-> BuildSavedTeamRuntimeBinding
-> SavedTeamInstance plan
-> ExecutionNodeInput
-> TeamDispatchInput
-> Run/Attempt Events
```

`ExecutionNodeInput`, `TeamNodeSemanticBinding`, `RunRecord` and the Team
dispatch/attempt payloads have no asset revision set or manifest digest.

## Minimum shared source boundary to reopen

The Amendment should permit the P3A-W1 contract to own only the following
shared paths when their exact diffs are frozen.

### Saved configuration to executable lineage

Required candidates:

- `internal/teams/saved_team_binding.go` and test;
- `internal/teams/saved_team_instantiation.go` and test;
- `internal/teams/execution_plan.go` and test;
- `internal/app/local_product_execution.go` and test;
- `internal/app/team_execution.go` and test;
- `internal/work/team_execution_authority.go` and test;
- `internal/work/run_authority.go` and test;
- `internal/projection/team_execution.go` and test;
- `internal/projection/global_read_view.go` and test;
- `cmd/loomd/product_daemon.go` and test.

Existing setup writer/projection files should remain read-only unless Contract
Review proves that their current exact Skill ID/revision/digest record is
insufficient. They already preserve the required source fact.

### New asset authority

New P3A-owned paths may be introduced under:

```text
internal/assets/
```

The package may contain domain values, canonical revision manifests, authority
commands, replay and tests. It must use the existing Journal Store,
`ReadStreamSet`, `AppendBatchIfStreamHeads` and Evidence Store. It must not add
a database, writer service, scheduler, index authority or mutable catalog.

Asset projection integration requires the existing projection entry points and
typed GlobalReadView accessors. Search remains a derived bounded read over the
published immutable view unless profiling later justifies a rebuildable index.

### Runtime capability and private materialization

Required candidates:

- `internal/runtime/catalog.go` and test for an exact governed capability;
- `internal/runtime/pi_probe.go` and test for observed capability truth;
- new Pi adapter materialization file(s) and tests;
- `internal/runtime/piadapter/process_runner.go` and/or
  `rpc_bridge_adapter.go` only if reviewed native-path injection requires it.

The current `--no-skills` flags must not simply be removed. The Contract must
first bind a private per-Run agent/Skill root, canonical manifest digest,
collision check, permissions, cleanup and generation. Repository and user
Skill paths stay read-only and outside the materialization target.

### Product/API/client surface

The vertical asset user journey requires new application/API methods and
strict models, likely under:

- `internal/app/local_product_assets.go` and test;
- `internal/api/local_product_assets.go` and test;
- `internal/localipc/protocol.go`, client/server tests;
- `cmd/loomd/product_daemon.go` and test;
- `internal/tui/model.go` and test;
- macOS Core client/models/store, production UI and tests.

The exact client files must be named by the P3A-W1 Contract after the product
interaction is specified. This discovery does not freeze a dashboard or a new
top-level navigation model; it requires a chat/task-first path consistent with
the accepted Phase 2A workbench.

## Journey metadata boundary

The current local IPC `Request` contains `version`, `request_id`, `method` and
`params`; `request_id` is per-call identity and cannot be reinterpreted as a
journey. The bounded Amendment must therefore permit a new strict optional-at-
protocol-entry but mandatory-for-P3A-operation `journey_id` metadata field in
both Go and Swift production clients.

`journey_id` propagation may touch:

- Go/Swift strict IPC request encoders and decoders;
- P3A application commands;
- Journal Event correlation metadata;
- Daemon structured journey logger;
- Artifact/Evidence manifest generation.

It must not be added to Event IDs, stream IDs, idempotency keys, authorization,
generation fencing, asset digest or domain decision digest.

## Real client execution boundary

No mock or in-process E2E package is required to claim the final journey.
Instead, the controlled result must build and launch production binaries:

```text
production loomd + isolated root/socket
production native app + real window automation
production loom TUI + real PTY keystrokes
```

Both clients use the same daemon and root. The journey runbook and immutable
evidence bundle live under `.loom-evidence/phase3a/P3A-W1/**`; any reusable
local orchestration script must be explicitly frozen by the Contract and may
not call application services directly.

## Paths that remain closed

- `internal/journal/store.go`: existing transaction APIs are sufficient;
- `internal/evidence/store.go`: existing immutable content-addressed store is
  sufficient unless a focused test proves otherwise;
- Credential Broker and Keychain implementation;
- root policy and validator authority;
- P2B-W1 source/evidence;
- Phase 3B, scheduling, sandbox, Provider and external integration paths.

## Discovery verdict

The missing capabilities can plausibly be closed inside one vertical P3A-W1,
but only by a reviewed Amendment that explicitly reopens the shared execution,
Runtime and client correlation paths above. No justification exists for a
P3A-W2 or a separate journey-infrastructure WorkItem.

