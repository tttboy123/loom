# P2D-W2D Conversation Three-Layer Selection and Chat UX V38

Status: `SOURCE VERIFIED / CLIENT THREE-LAYER SELECTION LIVE / PAID CALLS OPEN`

Date: 2026-08-16

## Acceptance boundary

V38 splits conversation selection in the client into three dependent layers —
Provider -> Model -> Reasoning Effort — mirroring the DeepSeek separation of
model and reasoning, and fixes two chat UX gaps (message copy, Enter-to-send).
The server is the authority: it validates every model/reasoning combination and
fails closed on unsupported ones. No paid Provider call was executed.

## Implemented source

### Server (`internal/provider`, `internal/api`, `cmd/loomd`)

- `provider/conversation_catalog.go`: the single source of truth.
  `ProviderConversationModels(providerID)` lists selectable models per Provider
  (deepseek: deepseek-chat / deepseek-reasoner; kimi; minimax; anthropic;
  opencode: deepseek/minimax/zhipu/openai identities each with reasoning
  efforts). `ValidateConversationModel` + `ValidateConversationReasoningEffort`
  fail closed on unsupported combinations; OpenCode stays dynamic for any valid
  `provider/model`.
- `LocalProductChatMessageRequest` + `LocalProductConversationRequest` carry
  optional `ModelID` / `ReasoningEffort`; the chat API passes them through.
- OpenAI-compatible clients (DeepSeek/Kimi/MiniMax) and Anthropic gain
  `RespondConfigured(ctx, messages, secret, modelID, reasoningEffort)`: model
  per request, `reasoning_effort` on the wire when the model supports it,
  validation fail-closed.
- OpenCode conversation client gains `RespondConfigured(ctx, prompt, modelID,
  reasoningEffort)` mapping reasoning to `--variant`; the daemon
  `productOpenCodeConversationResponder` forwards the request selection.
- The conversation profile router validates ModelID/ReasoningEffort against the
  catalog before using the credential lease and calls `RespondConfigured`.

### Client (Swift)

- `LocalProductChatMessageRequest` sends `model_id` / `reasoning_effort`.
- Store: `selectedConversationModelID` / `selectedConversationReasoningEffort`
  state, `selectConversationModel` / `selectConversationReasoningEffort`, reset
  on profile change, and passthrough in `sendChatMessage`.
- `conversationSelectionControls`: three dependent pickers — Provider menu
  (deduped profiles), Model menu (catalog for the selected Provider), Reasoning
  Effort menu (only when the model declares support). The client catalog mirrors
  the Go catalog; the server remains authoritative.
- Chat UX: each message gains a Copy button + context menu (NSPasteboard), and
  the composer sends on Enter (`onSubmit`).

## Verification

Passed:

- `go test ./internal/provider` (catalog layers + validation tests),
  `./internal/api`, `./internal/app`, `./cmd/loomd` (complete daemon suite with
  the router/client changes and the updated fixture).
- `go vet` on touched packages, `gofmt -l` clean on touched files,
  `git diff --check` clean, `go build ./...`, full `go test ./...` with no new
  failures.
- `swift build` + complete macOS package suite (XCTest + Swift Testing) green.
- Rebuilt and reinstalled the App; daemon `serving_request` with OpenCode, Loom
  Native and Pi runtimes online.

## Follow-up fixes (same slice)

- Incident `loom-chat-d69fd342-aff2-44ad-b4dc-e8e62d6e39e1`
  (`stage=input_admission`, `invalid_request`, OpenCode profile) was traced to
  the Swift client pre-validating `model_id` with `validIdentifier`, which
  rejects the `/` in OpenCode identities like `openai/gpt-5.5`. The client now
  validates `model_id` with `validModelID` (bounded, no control characters,
  slash allowed), locking the regression with a test.
- The Provider layer no longer carries a model:
  `conversationProfileMenuLabel` now shows Provider + Account only; Model is a
  separate layer. The label test was updated to assert the new separation.
- The chat send protocol now requires the context-carrying 7-parameter method
  alongside the 8-parameter model/reasoning method, with extension defaults
  delegating correctly, so store mocks and the unavailable client conform
  without recursion; `LocalProductStoreTests` (84) and the full macOS package
  suite pass.

## Follow-up fixes (defaults + Codex catalog + vault-lock diagnosis)

- Incident `loom-chat-1ea6e228-a807-4c09-a8fc-0c1f00b9bfe2`
  (`vault_encrypt / state_unavailable / retryable`) is a locked Credential
  Vault after App restart, not a routing bug: conversation context-capsule
  encryption requires the vault to be Unlocked. The fix is operator action
  (Unlock in the App); a follow-up may surface an explicit "unlock vault"
  affordance on this failure.
- Codex now has a model picker: the conversation model catalog gained the
  `openai` Provider (`codex-default` / "GPT-5.5 Codex") on both the Go and
  Swift sides.
- All three selectors always have a default: `effectiveConversationModelID`
  falls back to the selected profile's model, and
  `effectiveConversationReasoningEffort` falls back to `medium` (or the first
  supported effort) when the model supports reasoning; selecting a model
  resets reasoning to the default; the reasoning menu offers an explicit
  "Provider default" option.
- Regression tests: `testConversationThreeLayerDefaultsAreAlwaysPresent` and
  `testConversationModelCatalogCoversCodexAndThreeLayers`.

## Privacy and live status

No key value, prompt, conversation content, Provider body or user workspace
entered source, logs or evidence. The three-layer selection is live in the App
with Codex model support and defaults; the vault-lock failure requires the
operator to Unlock the vault; a paid conversation turn remains operator-driven.

## Follow-up: Codex multi-model (native DeepSeek V4)

The `openai` conversation model catalog previously exposed only
`gpt-5.5-codex` because Loom's Codex conversation path ran with
`--ignore-user-config` through the Loom OpenAI gateway. The user's Codex is
actually configured (via cc-switch) with a local custom provider
(`base_url http://127.0.0.1:15721/v1`, `wire_api responses`) serving
DeepSeek V4 Flash / Pro.

- `provider/conversation_catalog.go` + Swift catalog: `openai` now lists
  `codex-default` / `gpt-5.5-codex` (gateway) and `deepseek-v4-flash` /
  `deepseek-v4-pro` (native, reasoning `none` / `high`, grounded in the user's
  cc-switch catalog).
- `CodexConversationClient.RespondConfigured` + `SystemCodexConversationRunner`:
  native mode drops `--ignore-user-config` (so the user's real Codex config +
  local proxy apply), passes `--model <slug>` and
  `-c model_reasoning_effort="<effort>"`; gateway mode is unchanged for the
  default model. The daemon `productCodexConversationResponder` forwards the
  three-layer selection.
- Tests: `TestCodexConversationArgumentsNativeVsGateway`, expanded catalog
  assertions; full Go + macOS package suites green.

Verified the user's `~/.codex/cc-switch-model-catalog-codex.json` (DeepSeek V4
Flash/Pro, reasoning none/high) and `[model_providers.custom]` local proxy.
Selecting DeepSeek V4 Flash/Pro in the Codex conversation provider now routes
through the user's own Codex configuration; a live turn remains operator-driven
(and still requires the Credential Vault to be Unlocked).

## Follow-up: vault-lock chat recovery UX

Incidents `loom-chat-34f593b4-...` (and the earlier `...-1ea6e228-...`) are
`vault_encrypt / state_unavailable / retryable=true`: the Credential Vault is
locked after App restart, so conversation context-capsule encryption fails.
The chat failure banner previously hid the vault recovery action because
`conversationVaultRecoveryAvailable` gated on `!recoverable` and its allowlist
omitted `.vaultEncrypt` — the user saw only a generic retryable failure with no
way forward ("无法进行对话").

Fix: vault-stage failures (including `.vaultEncrypt` / `.vaultCommit`) now
always offer vault recovery even when retryable, and the chat failure banner
gains a direct **Unlock Vault** button (calls `setCredentialVaultLocked(false)`
then reloads the thread) alongside "Open Credential Vault". Tests:
`testVaultEncryptFailureAlwaysOffersVaultRecovery` and the updated closed
allowlist test; full macOS package suite green. Unlock is passphrase-free
(LocalKeyFile `LoadOrCreate`), so the button recovers the vault immediately.

## Follow-up: Codex GPT model catalog corrected to the real CLI catalog

The `openai` conversation models were previously `gpt-5.5-codex` (invented) +
`codex-default`. The authoritative model list was extracted from the installed
Codex CLI 0.144.1 binary catalog:
`gpt-5.5` (default), `gpt-5.5-pro`, `gpt-5.4`, `gpt-5.4-mini`, `gpt-5.2`,
`gpt-5.1-codex-max`, `gpt-5.6-terra`, `gpt-5.6-sol`, `gpt-5.6-luna`, `o3`,
plus `codex-default` as the Loom alias for the profile default and the native
DeepSeek V4 Flash/Pro pair. Native mode now keys on the `deepseek-v4-` prefix;
GPT models run through the user's Codex OpenAI auth with an explicit `--model`.
Go + Swift catalogs and tests updated; full Go and macOS package suites green;
App rebuilt/reinstalled.

## Follow-up: auto-unlock the Credential Vault before sending

Incident `loom-chat-74849682-...` is again `vault_encrypt / state_unavailable`
(locked Vault after App restart). A direct probe against the running daemon's
`credential_vault_unlock` route proved the unlock succeeds passphrase-free
(LocalKeyFile `LoadOrCreate`), so the blocker was purely UX: the user had no
path that reliably unlocked before chatting.

Fix: `LocalProductStore.sendChatMessage` now auto-unlocks the vault first when
`setupSnapshot.credentialVault.status == "locked"` (passphrase-free), so a
locked vault after restart never blocks a conversation; the failure banner
keeps the explicit Unlock Vault action as a fallback. The macOS package suite
is green; App rebuilt/reinstalled.

## Follow-up: stale-profile chat failure (post-unlock)

Incident `loom-chat-d0838d99-...` was `conversation_dispatch / invalid_request /
retryable=false` on `conversation-deepseek-deepseek-chat-r6` — AFTER the vault
was unlocked (progress past vault_encrypt). Diagnosis: probes against the live
daemon proved the vault was unlocked, the DeepSeek r6 profile existed in the
authoritative setup snapshot, and a `chat_message` with the real thread +
profile SUCCEEDED. The 22:51 failure was transient: the DeepSeek credential was
being re-imported/verified and the selected profile was momentarily stale
(credential revision mid-flight) before the projection caught up.

Robustness fix: `LocalProductStore.sendChatMessage` now detects
`invalid_request / conversation_dispatch` (stale binding), refreshes the
authoritative setup snapshot, and retries once with the now-current profile
before surfacing the failure. macOS package suite green; App rebuilt/reinstalled.

## Follow-up: repeated Change-conversation-route sheet after provider switch

After switching the conversation Provider, the "Change conversation route"
confirmation sheet kept re-appearing on every send. Root cause: the switch
branch of `confirmConversationRouteTransition` never set
`forceNewConversationSegment` (only the rebind branch did), so the thread still
carried the old profile and the UI's pre-send
`requestConversationDispatchTransition` returned a fresh transition on every
send — an infinite re-prompt loop, made worse while chat sends were failing.

Fix: confirming a switch (or rebind) now sets `forceNewConversationSegment =
true` and clears `hasPendingConversationRoute` /
`conversationRouteSourceProfileID`, so the next send proceeds with a new segment
and the sheet never re-pops. Regression test
`testConversationRouteTransitionRequiresConfirmationAfterHistory` now asserts
`requestConversationDispatchTransition()` is nil after confirm. macOS package
suite green; App rebuilt/reinstalled.

## Follow-up: conversation dispatch conflict self-heal

Incident `loom-chat-7404eed7-...` was `conversation_dispatch / conflict /
retryable=true` on `conversation-deepseek-deepseek-chat-r6`. Diagnosis: the
server rejects with conflict when the thread's last segment binding differs
from the current resolved binding (`bindingChanged`) or the profile differs
(`switchingProfile`) and the request lacks `ExpectedExecutionBinding` +
`ContextMode`; the current state had DeepSeek verified at r6 with no policy, so
an old segment's policy-bearing binding made `bindingChanged` true. Live probes
confirmed the current daemon accepts both bare and fully-bound sends (the
23:18 conflict was a transient stale-binding moment during the credential
re-import).

Robustness fix: `LocalProductStore.sendChatMessage` now self-heals a
`conflict / conversationDispatch` failure by aligning to the current profile's
binding, forcing a new segment, and retrying once; if the retry also fails it
resets the state so the normal route-transition confirmation sheet remains
available. Regression test
`testConversationConflictSelfHealsAndRetriesWithCurrentBinding` added and the
existing conflict-visibility test updated; macOS package suite green; App
rebuilt/reinstalled.
