# P2D-W2D Codex Conversation Routing + Usage-Limit Message (V43)

Status: `SOURCE + INSTALLED-LIVE VERIFIED`

Date: 2026-08-16

Parent Goal: `Phase 2D - Multi-Provider, Multi-Model Agent Team Orchestration`

## Symptom

The Codex conversation profile (`conversation-openai-codex-default-v1`) failed
with the opaque message "Conversation Provider runtime is unavailable."

## Root causes

1. **Wrong responder**: with both OpenCode and Codex runtimes configured, the
   Codex profile was served by the OpenCode responder (single
   `defaultResponder` slot; the Codex client was only constructed when OpenCode
   was absent). `codex-default` was therefore sent to `opencode run` → generic
   failure.
2. **Official account usage limit**: the user's official OpenAI Codex account
   has exhausted its credits, so `codex-default` (gateway mode,
   `--ignore-user-config`) fails with "You've hit your usage limit". The Codex
   runner detected it but `RespondConfigured` collapsed the typed error into
   `ErrCodexConversationUnavailable`, losing the actionable signal.
3. The user's working Codex setup (CCswitch local proxy at
   `127.0.0.1:15721` → DeepSeek V4) was unreachable through Loom because the
   Codex profile defaulted to gateway mode.

## Fixes

- `cmd/loomd/product_conversation_profiles.go`: `productConversationProfileRouter`
  gains a `codexResponder`; `Respond` routes the Codex profile to it and the
  OpenCode profile to the default (OpenCode) responder.
- `cmd/loomd/product_composition_conversation.go`: the Codex client is built
  whenever `--codex-executable` is set (not only when OpenCode is absent), and
  the router receives the codex responder.
- `internal/provider/codex_conversation.go`:
  - `classifyCodexConversationFailure` inspects CLI stderr for
    usage-limit/credits/quota (→ `ErrCodexConversationUsageLimit`) and
    auth/401/connection markers (→ `ErrCodexConversationAuth`).
  - `RespondConfigured` propagates those typed errors instead of collapsing
    them to `ErrCodexConversationUnavailable`.
- `cmd/loomd/product_daemon.go`: `productCodexConversationFailure` maps
  usage-limit → `provider_insufficient_balance / provider_connect` with a
  Chinese message ("Codex 官方账号用量已达上限…切换 DeepSeek V4 使用你的
  Codex/cc-switch 配置"), and auth → `provider_auth`.
- Swift `LocalProductStore`: `(.providerInsufficientBalance, .providerConnect)`
  → "Codex account usage limit reached … DeepSeek V4 with your
  Codex/cc-switch configuration".

## Verification

- Live via installed daemon:
  - `codex-default` → clear usage-limit message.
  - `deepseek-v4-flash` (native, cc-switch proxy) → real Codex/DeepSeek reply.
- OpenCode live E2E still `E2E-OK`.
- Go `cmd/loomd` + `internal/provider` green (incl.
  `TestConversationProfileRouterRoutesCodexToCodexResponder`,
  `TestProductCodexConversationUsageLimitMapsToActionableError`,
  `TestProductCodexConversationAuthMapsToActionableError`).
- macOS package `236` tests, `0` failures (`1` visual-export skip).
