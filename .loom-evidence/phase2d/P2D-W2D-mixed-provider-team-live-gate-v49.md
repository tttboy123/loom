# P2D-W2D Mixed-Provider Team Live Gate (V49)

Status: `RESOLVED BY V33 — INSTALLED-LIVE GATE PASSING`

Date: 2026-08-16

> RESOLUTION: the LocalModelCatalog over-constraint described below is fixed in
> V33 (`P2D-W2D-brokered-only-mission-execution-live-v33.md`). The installed
> App daemon now confirms + preflights + starts a real 4-Agent mixed-provider
> Team (`preflight nodes=4 ready=4`, `start status=running`).

Parent Goal: `Phase 2D - Multi-Provider, Multi-Model Agent Team Orchestration`

## What this slice did

1. **Drove the installed daemon through the full Team lifecycle live**:
   verified DeepSeek + MiniMax broker accounts, started a blank (form-first)
   Team draft, set name/purpose, selected a 4-Agent mixed-provider Team
   (DeepSeek main + MiniMax + DeepSeek reviewer + MiniMax researcher),
   confirmed it → `status=active` Team Definition persisted, and the Team
   appeared as confirmed + executable in the snapshot.

2. **Found the real G3 installed-live blocker**: `builder_confirm` returns
   `team_instance_created=false` because the installed App daemon builds no
   Mission execution runtime (`setupConfig.Execution == nil`), which follows
   from `missionExecutionConfigFromDaemonBuild` returning nil when
   `LocalModelCatalog == nil` (the installed runtime bundle ships the Pi CLI
   but no local GGUF model + llama-server). `buildProductMissionExecutionAPI`
   and `newProductMissionExecutor` both require `LocalModelCatalog != nil` as
   the primary (index 0) Pi supervisor adapter. Without it, Mission preflight
   returns `state_unavailable` and no TeamInstance is materialized.

3. **Added `TestLiveMixedProviderTeamE2E`** (`LOOM_LIVE_TEAM_E2E=1`) as the
   installed-live G3 gate: it re-verifies the broker accounts, builds + confirms
   the mixed Team, and runs preflight + start. It is skipped by default and
   fails fast with the precise blocking stage when the local model prerequisite
   is absent. The Team definitions created by the gate are archived after the
   run.

## Safety

No remote tool capability is published by default. No credential, prompt,
Provider body or user workspace entered source/logs/evidence. The gate reuses
the existing broker verified-account + Vault path.

## Verification

- Go build/vet clean; live gate compiles and skips by default.
- The installed daemon confirmed the mixed Team end-to-end (definition active,
  executable) — the remaining preflight step is blocked only by the missing
  local-model supervisor adapter.
- G3/G4 source proofs remain green: `TestFourProviderTeam*` and
  `TestPhase2DPerAgentFailureIsolationMatrix` in `internal/app/team_execution_test.go`.
