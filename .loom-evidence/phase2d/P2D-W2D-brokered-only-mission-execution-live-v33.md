# P2D-W2D Brokered-Only Mission Execution + G3 Installed-Live PASS (V33)

Status: `INSTALLED-LIVE PASS (G3) / SOURCE VERIFIED`

Date: 2026-08-16

Parent Goal: `Phase 2D - Multi-Provider, Multi-Model Agent Team Orchestration`

## What this slice fixed

The installed App daemon ships the Pi runtime bundle with the Pi CLI but no
local GGUF model + llama-server, so `missionExecutionConfigFromDaemonBuild`
returned nil, `setupConfig.Execution == nil`, and no Mission execution runtime
existed. `builder_confirm` could not materialize a TeamInstance and preflight
returned `state_unavailable`. This slice made brokered-only (Provider-native)
Mission execution work on the installed bundle:

1. **No `LocalModelCatalog` hard-requirement**: `missionExecutionConfigFromDaemonBuild`
   emits a brokered-only Execution config; `newProductMissionExecutor` and
   `buildProductMissionExecutionAPI` accept a nil catalog; the `pi-cli`
   deferred adapter is only appended when a local model exists, and legacy
   `pi-cli` single-role bindings fail closed without one.
2. **Per-instance runtime discovery digest**: `MaterializeConfirmedTeam` now
   records the main Agent's runtime instance `DiscoveryDigest`
   (`WithMainRuntimeDiscoveryDigest` / `MainRuntimeDiscoveryDigest`) instead of
   the full-catalog composite; the state writer persists it and binding
   validation compares canonical fields only. Preflight binding resolution now
   matches the projected runtime instance.
3. **Governed Context retrieval pairing**: `productMissionExecutor` no longer
   injects a bare `ContextRetriever`; the governed attempt-loop adapter
   materializes the scoped retriever and its paired `DeliveryBroker`
   downstream (the supervisor rejects a retriever without its delivery).
4. **Native prompt bound**: `deepSeekAgentMaxPromptBytes` raised 4 KiB → 64 KiB
   (loom-native Context Capsule cap is 32 KiB; conversation adapters use
   64 KiB) so real mission prompts dispatch to DeepSeek/MiniMax.

## Live gate result (G3)

`LOOM_LIVE_TEAM_E2E=1 go test ./cmd/loomd -run TestLiveMixedProviderTeamE2E -count=1 -v`

- 4-Agent mixed Team confirmed + materialized (`team_instance_created=true`).
- Preflight: `nodes=4 ready=4`.
- `mission_execution start`: `status=running` with real paid Provider calls.
- Accounting rows verified for deepseek/minimax/zhipu.
- Evidence: `.loom-evidence/phase2d/acceptance/live/G3-mixed-team-atl9.md`.

## Regression proof

- New RED tests: brokered-only executor/API construction with nil catalog
  (`TestProductMissionExecutorConstructsBrokeredOnlyWithoutLocalModel`,
  `TestProductMissionExecutionCompositionBuildsBrokeredOnlyWithoutLocalModel`,
  `TestMissionExecutionConfigFromDaemonBuildAllowsBrokeredOnlyWithoutLocalModel`),
  writer per-instance digest
  (`TestSavedTeamCommitMainAgentRecordsPerInstanceRuntimeDiscoveryDigest`),
  large mission prompt dispatch
  (`TestLoomNativeDispatchAcceptsLargeMissionRoleContextPrompt`).
- Full Go suite green except the known pre-existing
  `internal/runtime/harnessadapter` child-process flake (passes 3/3 in
  isolation); `swift test` all suites pass; focused race checks pass;
  `git diff --check` clean.

## Safety

No remote tool capability is published by default. No credential, Prompt,
Provider body or user workspace entered source/logs/evidence. Evidence files
are owner-only (`0600`).
