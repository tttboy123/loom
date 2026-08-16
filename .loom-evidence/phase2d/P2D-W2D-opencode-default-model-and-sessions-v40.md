# P2D-W2D OpenCode Default Model + Multi-Conversation Sessions (V40)

Status: `SOURCE + INSTALLED-LIVE VERIFIED`

Date: 2026-08-16

Parent Goal: `Phase 2D - Multi-Provider, Multi-Model Agent Team Orchestration`

## 1. Incident `loom-chat-81e0ea64-0b47-47d7-8f93-d78c11c3b50c` — root cause

`provider_auth / provider_connect / retryable:false` on
`conversation-opencode-default-v1` with `openai/gpt-5.5` (the catalog default
model). The Credential Vault only contained a verified DeepSeek account, so:

1. The OpenCode responder had no verified record for `openai` and (before V40)
   fell back to running OpenCode natively with NO injected key.
2. OpenCode's own stored native credential could then serve the request — the
   user saw an untracked "GPT55-OK" reply (a stale message from an earlier
   probe using the native path) that was impossible to account for.

## 2. Fixes

- **Dynamic OpenCode profile default** (`internal/app/local_product_setup.go`):
  `openCodeConversationDefaultModel(accounts)` returns the first catalog model
  whose Provider has a verified Loom account (`deepseek/deepseek-chat` when
  only DeepSeek is verified), falling back to
  `provider.OpenCodeConversationDefaultModel`.
- **Fail-fast actionable auth error** (`cmd/loomd/product_daemon.go`):
  `withCredential` now returns
  `NewLocalProductConversationDispatchErrorWithDetails(provider_auth,
  provider_connect, "The selected model requires a verified <provider>
  Provider credential…")` when the model's Provider has no verified Vault
  credential, instead of silently running OpenCode natively. The `Respond`
  method propagates this error.
- **Router preserves specific failures**
  (`cmd/loomd/product_conversation_profiles.go`): the default-responder error
  path reads `api.LocalProductConversationDispatchFailureDetails(err)` and keeps
  code/stage/message/retryable.
- **Swift presentation**: `(.providerAuth, .providerConnect)` →
  "Model Provider credential required — the selected model belongs to a
  Provider that has no verified Loom account…".

## 3. Multi-conversation sessions

- `LocalProductChatSession { threadID, title, createdAt, updatedAt }`
  (Codable/Identifiable).
- `LocalProductStore`: `chatSessions`, `selectedChatSessionID`,
  `newConversation()`, `selectChatSession(_:)`, `renameChatSession` /
  `touchChatSession`; `currentChatThreadID()` follows the active session; the
  workspace thread anchor is kept in sync so route-transition guards still
  work. A fresh session has no messages, so blank-profile selection does not
  force a route review.
- Auto-title from the first user message (first line, ≤40 chars).
- Persistence: `~/Library/Application Support/Loom/chat-sessions.json`
  (schema-versioned JSON; atomic write). The App core deliberately avoids
  UserDefaults/ProcessInfo per the native-app source gate.
- UI: the chat header shows the active conversation title, a switcher menu of
  all sessions, and a New Conversation button.

## 4. Verification

- Go: `go test ./cmd/loomd/ ./internal/app/ ./internal/provider/` green,
  including:
  - `TestSetupConversationProfilesOpenCodeDefaultFollowsVerifiedProvider`
  - `TestOpenCodeResponderMissingModelCredentialReturnsActionableError`
  - `TestConversationProfileRouterPreservesSpecificResponderFailure`
- Swift: macOS package `234` tests, `0` failures (`1` visual-export skip),
  including `testMultipleConversationSessionsSwitchAndAutoTitle` and
  `testNewConversationStartsWithEmptyThread`.
- Live: `LOOM_LIVE_OPENCODE_E2E=1 go test ./cmd/loomd/ -run
  TestLiveOpenCodeConversationE2E` → `last message role=loom content="E2E-OK"`.
- Installed App: header shows session switcher; registry file written with the
  existing thread as the selected session; `provider_auth` message surfaces in
  the failure banner for an unverified model Provider.

## Files

- `internal/app/local_product_setup.go` (+ test)
- `cmd/loomd/product_daemon.go` (+ test)
- `cmd/loomd/product_conversation_profiles.go` (+ test)
- `apps/macos/Sources/LoomLocalAppCore/LocalProductModels.swift`
- `apps/macos/Sources/LoomLocalAppCore/LocalProductStore.swift`
- `apps/macos/Sources/LoomLocalAppUI/LoomWorkspaceShell.swift`
- `apps/macos/Tests/LoomLocalAppTests/LocalProductStoreTests.swift`
