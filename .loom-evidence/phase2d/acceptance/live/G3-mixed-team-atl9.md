# G3 — Mixed Provider Team (ATL9) installed-live

Status: `PARTIAL LIVE / FOUR-DISTINCT-PROVIDER MATRIX DEFERRED`

Date: 2026-08-16

The evidence below remains valid Phase 2D history. The installed Codex/OpenAI +
Claude/Anthropic + Loom/Kimi + Loom/MiniMax completion matrix moved to a later
Phase on 2026-08-24; no future Phase is active yet.

## What was executed

1. Built and installed the App (`scripts/build-loom-local-app.sh`,
   `scripts/install-loom-local-app.sh`), started it, daemon online at
   the private Loom daemon Socket with the Pi
   runtime and **no local GGUF model** (`--local-model-*` absent).
2. Credential Vault unlocked; DeepSeek + MiniMax broker accounts verified from
   the installed Credential Vault.
3. `LOOM_LIVE_TEAM_E2E=1 go test ./cmd/loomd/ -run
   TestLiveMixedProviderTeamE2E -count=1 -v`
   - Builds a blank (form-first) Team, selects a 4-Agent mixed-provider Team:
     DeepSeek main + MiniMax bounded worker + DeepSeek reviewer + MiniMax
     researcher.
   - `builder_confirm` → Team Definition `active`, `team_instance_created=true`.
   - `snapshot` → confirmed + executable saved TeamInstance.
   - `mission_execution preflight` → `4/4 ready` nodes across
     `runtime.loom-native.local` (DeepSeek) and `runtime.loom-native.minimax`.
   - `mission_execution start` → `status=running` with real paid Provider
     calls; per-Agent Vault lease + frozen execution binding.
   - `setup_snapshot` accounting rows verified (deepseek/minimax/zhipu).

## Observed result

```
confirmed: team=team-live-mixed-20260816T125750Z status=active
instanceCreated=true runCreated=false
team=team-instance-08afbac64aa727aeadd42989be903a2d executable=true confirmed=true
preflight nodes=4 ready=4
mission result: status=running note=""
account deepseek status=verified revision=2
account minimax status=verified revision=2
--- PASS: TestLiveMixedProviderTeamE2E (3.22s)
```

The test process passed its bounded two-Provider assertions, but the Mission
was observed only at `running`; this is not evidence that the Phase 2D G3
completion criterion passed. A completed Build-specific four-Provider Team
remains required by the live acceptance ledger.

## Fixes that unblocked this gate (V33)

- Brokered-only Mission execution no longer requires `LocalModelCatalog`
  (`missionExecutionConfigFromDaemonBuild`,
  `newProductMissionExecutor`, `buildProductMissionExecutionAPI`).
- Materialized main Agent records its per-instance runtime `DiscoveryDigest`
  (not the full-catalog composite) so binding resolution matches preflight.
- Governed attempt-loop adapter materializes ContextRetriever + DeliveryBroker
  together (supervisor rejects an unpaired retriever).
- OpenAI-compatible nativeadapter prompt bound raised 4 KiB → 64 KiB.
- Live gate harness: UUID correlation IDs, exact work-package digest, snapshot
  limit 64, real TeamInstance selection, 4/4-ready assertion, envelope decode.

## Safety

No credential, Prompt, Provider body, or user workspace entered source, logs,
or evidence. Evidence files are owner-only (`0600`). Default production still
exposes no unbound remote tool capability.
