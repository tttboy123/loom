# P2D-W2D OpenCode Live Conversation + User-Visible Failure Codes (V39)

Status: `SOURCE + INSTALLED-LIVE VERIFIED`

Date: 2026-08-16

Parent Goal: `Phase 2D - Multi-Provider, Multi-Model Agent Team Orchestration`

Scope: make a real OpenCode conversation complete end-to-end through the
installed App daemon with a Loom-owned Credential Vault lease, and make every
failure the user can hit readable and actionable.

## Live acceptance (real OpenCode + real DeepSeek credential)

Gate: `TestLiveOpenCodeConversationE2E` in `cmd/loomd/opencode_live_e2e_test.go`
(gated by `LOOM_LIVE_OPENCODE_E2E=1`).

1. Rebuilt and reinstalled the App
   (`scripts/build-loom-local-app.sh --output …/Loom.app` +
   `scripts/install-loom-local-app.sh --destination /Users/lune/Applications/Loom.app`).
2. Started the App; daemon online: OpenCode / Loom Native / Pi.
3. Vault status: `unlocked`.
4. Sent `Reply with exactly: E2E-OK` on profile
   `conversation-opencode-default-v1`, model `deepseek/deepseek-chat`,
   binding `{SchemaVersion:3, ProviderID:"opencode"}`, `ContextModeStartClean`.
5. Result: thread's final loom message = `E2E-OK` (real OpenCode run, real
   DeepSeek call, credential leased from the Loom Credential Vault).

Repeated runs pass (`last message role=loom content="E2E-OK"`).

## Root causes fixed

1. **Nil lease access captured by the OpenCode responder** —
   `newProductConversationConstructionFactory` built the
   `productOpenCodeConversationResponder` before `leases` was assigned
   (`leases = setup.CredentialLeases` / `= vault`). The responder therefore fell
   back to native auth without an injected Provider key; OpenCode then failed
   model resolution (`ProviderModelNotFoundError: Model not found:
   deepseek/deepseek-chat`) and the router surfaced
   `provider_unavailable / provider_connect`. Responder binding now happens
   after `leases` is resolved; the bound model's Provider credential is leased
   from the vault and injected as `DEEPSEEK_API_KEY` into the OpenCode process
   environment.
   - Probe evidence: `OPENCODE-WITHCRED-DEBUG fallback known=true
     leasesNil=true …` → after fix the lease is used and the runner receives
     `CredentialEnvName=DEEPSEEK_API_KEY`.
2. **OpenCode profile not routed** — `ResolveConversationContextTarget` and
   `Respond` had no `provider.OpenCodeConversationProfileID` branch, so the
   profile fell through `resolveBrokeredProfile` → `state_unavailable /
   vault_encrypt`. Now both resolve a valid OpenCode target
   (`opencode / opencode-default / native_auth / context:loom-native:v1`) and
   route to the default responder.
3. **OpenCode event decoding** — `decodeOpenCodeConversation` accepts the real
   1.18.3 `text` / `step_finish` / `session.idle` event stream (plus legacy
   `message.part.updated`), verified against live CLI output.

## User-visible failure codes

`LocalProductStore.chatFailureDetail`/presentation now maps every chat
failure code+stage to a non-technical, actionable message. New explicit cases:

| code | stage | title (detail) |
|------|-------|----------------|
| `state_unavailable` | `vault_encrypt` | Conversation context could not be secured (Unlock the vault or re-verify the Provider credential; draft preserved) |
| `state_unavailable` | `vault_key_load` | Credential Vault locked (Unlock the vault, then retry) |
| `state_unavailable` | `vault_open` | Credential Vault unavailable (check storage permissions) |
| `state_unavailable` | `vault_commit` | Conversation context could not be committed |
| `provider_unavailable` | `provider_connect` | Conversation Provider could not start (Open Runtime & Providers, verify runtime + model credential) |
| `conversation_unavailable` | `conversation_dispatch` | Conversation could not start (check Runtime & Providers) |
| `invalid_request` | `conversation_dispatch` | Conversation Profile invalid (account or credential revision changed) |

The banner always shows title, stage label, detail, incident ID (copyable),
and Retry / Unlock Vault / View diagnostics actions where applicable.

## Tests

- Go: full `go test ./...` green (includes
  `TestConversationProfileRouterRoutesOpenCodeProfileToDefaultResponder` and
  `TestConversationProfileRouterResolvesOpenCodeBinding`).
- Swift: macOS package `231` tests executed, `0` failures (`1` visual-export
  skip by design); new
  `testConversationVaultEncryptFailureShowsActionableGuidance` and
  `testConversationProviderConnectFailureShowsActionableGuidance` pass.
- Live: `LOOM_LIVE_OPENCODE_E2E=1 go test ./cmd/loomd/ -run
  TestLiveOpenCodeConversationE2E -count=1 -v` → `E2E-OK`.
- `git diff --check` clean.

## Files

- `cmd/loomd/product_composition_conversation.go` (responder binding order)
- `cmd/loomd/product_conversation_profiles.go` (OpenCode routing + context
  target)
- `cmd/loomd/product_conversation_profiles_test.go` (routing tests)
- `cmd/loomd/opencode_live_e2e_test.go` (gated live E2E)
- `internal/provider/opencode_conversation.go` (real event-stream decoding)
- `apps/macos/Sources/LoomLocalAppCore/LocalProductStore.swift` (failure
  presentation)
- `apps/macos/Tests/LoomLocalAppTests/LocalProductStoreTests.swift`
