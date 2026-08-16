# P2D-W2D OpenCode Model Catalog Correction (V42)

Status: `SOURCE + LIVE VERIFIED`

Date: 2026-08-16

Parent Goal: `Phase 2D - Multi-Provider, Multi-Model Agent Team Orchestration`

## Problem

Loom's OpenCode conversation model catalog did not match the installed OpenCode
CLI 1.18.3:

| Loom catalog (wrong) | OpenCode CLI reality |
|----------------------|----------------------|
| `zhipu/glm-4.5` | `zai/glm-4.5` (GLM is exposed under the `zai` provider; key `ZHIPU_API_KEY`) |
| `openai/gpt-5.5` | not usable via the OpenCode CLI in this environment (no openai provider/key) |
| only `deepseek/deepseek-chat` | also `deepseek-reasoner`, `deepseek-v4-flash`, `deepseek-v4-pro` |
| only `minimax/MiniMax-M3` | also `MiniMax-M2.7` |
| — | free hosted tier `opencode/deepseek-v4-flash-free`, … |
| every model: `low/medium/high/minimal` | per-model: v4-flash→`low/high/max`, glm-5.2→`high/max`, toggle-only models→none |

## Reference

- OpenCode CLI: `opencode models` (authoritative for this environment).
- models.dev cache `~/.cache/opencode/models.json`: provider env vars
  (`zai`→`ZHIPU_API_KEY`, `opencode`→`OPENCODE_API_KEY`, `deepseek`→
  `DEEPSEEK_API_KEY`, `minimax`→`MINIMAX_API_KEY`) and per-model
  `reasoning_options`.
- CCswitch (`~/.cc-switch/cc-switch.db`) OpenCode Go integration: hosted
  `https://opencode.ai/zen/go/v1` (wire_api responses) with models
  glm-5.2/glm-5.1/kimi-k2.7-code/deepseek-v4-pro/deepseek-v4-flash/
  mimo-v2.5-pro — confirming the deepseek-v4-flash/pro + glm model identities
  the CLI exposes.

## Fix

- `internal/provider/conversation_catalog.go` + Swift
  `localProductConversationModels("opencode")`: corrected list above, with
  real per-model reasoning efforts.
- `internal/provider/opencode_conversation.go`:
  `OpenCodeConversationDefaultModel = "deepseek/deepseek-chat"`.
- `internal/provider/opencode_adapt.go`: `OpenCodeCredentialEnv` now maps
  `zai` → `ZHIPU_API_KEY` and `opencode` → `OPENCODE_API_KEY`.

## Verification

- `deepseek/deepseek-v4-flash` live CLI call → `FLASH-OK`.
- OpenCode live E2E through the installed App daemon → `E2E-OK`.
- OpenCode profile default = `deepseek/deepseek-chat`.
- Go `internal/provider` + `internal/app` tests green; macOS package `235`
  tests, `0` failures (`1` visual-export skip).
