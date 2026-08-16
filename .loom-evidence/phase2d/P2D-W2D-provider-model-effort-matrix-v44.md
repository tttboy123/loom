# P2D-W2D Provider/Model/Effort Matrix + Official Catalog Verification (V44)

Status: `SOURCE + INSTALLED-LIVE VERIFIED`

Date: 2026-08-16

Parent Goal: `Phase 2D - Multi-Provider, Multi-Model Agent Team Orchestration`

## What was verified

### 1. Official model catalogs (each Provider's official docs/API)

| Provider | Official source | Current models | Reasoning efforts |
|---|---|---|---|
| OpenAI/Codex | installed Codex CLI 0.144.1 `~/.codex/models_cache.json` | gpt-5.6-sol/terra/luna, gpt-5.5, gpt-5.4, gpt-5.4-mini, gpt-5.3-codex-spark, codex-auto-review (+ native cc-switch deepseek-v4-flash/pro) | low/medium/high/xhigh/max(/ultra) per model |
| DeepSeek | api-docs.deepseek.com + live API | deepseek-v4-flash, deepseek-v4-pro (legacy deepseek-chat/reasoner alias) | low/high/max (medium/xhigh map to high) |
| OpenCode | installed CLI 1.18.3 models.dev cache | deepseek/*, minimax/*, zai/*, opencode/* | v4-flash low/high/max; v4-pro high/max; glm-5.2 high/max; toggle-only none |
| Kimi | platform.kimi.com | kimi-k3 (low/high/max), kimi-k2.6 (toggle) | k3 low/high/max |
| MiniMax | platform.minimax.io + live API | MiniMax-M3 | toggle (effort values accepted but don't tune depth) |
| Anthropic | platform.claude.com | claude-sonnet-5 | adaptive-only always-on |
| Zhipu GLM | docs.z.ai | glm-5.2 (high/max), glm-4.5 (toggle) | per-model |

### 2. Misalignments fixed (Provider/Model/effort no longer drift)

- `openai` catalog: replaced invented models (gpt-5.5-pro, gpt-5.2, gpt-5.1-codex-max, o3) with the installed Codex CLI 0.144.1 catalog; every GPT model now declares its real reasoning levels.
- `deepseek` catalog: added deepseek-v4-flash / deepseek-v4-pro with low/high/max (official current models); legacy aliases kept.
- `kimi` catalog: added kimi-k3 with low/high/max.
- `opencode` catalog: corrected deepseek/deepseek-v4-pro to high/max (was low/high/max); glm-5.2 high/max; toggle-only models declare none.
- Swift mirror updated to match; reasoning label handles none/minimal/low/medium/high/xhigh/max/ultra.

### 3. Live matrix E2E (real paid calls through installed daemon)

`TestLiveProviderModelEffortMatrixE2E` (gated by `LOOM_LIVE_MATRIX_E2E=1`) drives the installed
App daemon UDS socket across every selectable Provider/Model/Reasoning-effort combination, plus
mid-conversation switching and multi-session isolation.

Passed cells (real replies):
- Codex profile: deepseek-v4-flash (default/none/high), deepseek-v4-pro (default/none/high); codex-default → clear usage-limit message (probe).
- OpenCode profile: deepseek/deepseek-chat, deepseek/deepseek-v4-flash (default/low/high/max), deepseek/deepseek-v4-pro (default/high/max), minimax/MiniMax-M3 (classified rate-limit on repeat runs).
- DeepSeek brokered: deepseek-v4-flash (default/low/high/max), deepseek-v4-pro (default/low/high/max), deepseek-chat (legacy).
- MiniMax brokered: MiniMax-M3.
- Mid-conversation switch: same thread, DeepSeek brokered v4-flash/low → OpenCode v4-flash/high returns a real loom reply (segment moved to OpenCode).
- Multi-session: two interleaved threads (OpenCode + DeepSeek) stay isolated; resuming session A returns A's reply.

## Bugs found and fixed by the matrix

1. **128-thread store cap returned an opaque error with no recovery path.**
   - `SendMessage` thread-limit now returns a typed `conversation_limit` dispatch error; Swift shows
     "Conversation limit reached" with an actionable message.
   - Added a bounded, encrypted delete-conversation capability end-to-end:
     `chat_thread_delete` daemon route → `LocalProductChatAPI.DeleteThread` →
     `VaultStore.DeleteConversationDocument` + `DeleteContextConversation`, plus a Swift
     client/store method and a Delete Conversation context menu. The store is back to the user's
     original 3 threads after cleaning test threads.
2. **Continuing a long thread failed with an opaque "conversation unavailable" after ~5 turns.**
   - Root cause: the `context:loom-native:v1` context capsule dispatch payload is allowed up to
     32 KiB and grows with history, but OpenAI-compatible provider clients rejected any single
     message > 4096 bytes (`maxConversationContentBytes`). Raised the per-message wire cap to
     32 KiB (matching the adapter allowance; the 60 KiB total budget still bounds the turn) and
     added a regression test.

## Verification

- `go test ./...` green (only `internal/runtime/harnessadapter` child-process tests are flaky
  under full parallel load and pass 3/3 in isolation; unrelated to this slice).
- macOS package: `236` tests, `0` failures (`1` visual-export skip by design).
- Live: matrix E2E PASS, OpenCode E2E `E2E-OK`, `git diff --check` clean.
- No key, prompt, conversation content, Provider body or user workspace entered source, logs or
  evidence.
