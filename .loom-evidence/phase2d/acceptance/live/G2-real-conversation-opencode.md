# G2 — Real conversation (OpenCode live)

Status: `PASS` (installed-live, real paid calls)

Date: 2026-08-16

## What was executed

1. Built and installed the App (`scripts/build-loom-local-app.sh`,
   `scripts/install-loom-local-app.sh`), started it, daemon online with
   OpenCode / Loom Native / Pi runtimes.
2. Credential Vault `unlocked`; DeepSeek account verified
   (`deepseek.primary`, revision 6, brokered).
3. `LOOM_LIVE_OPENCODE_E2E=1 go test ./cmd/loomd/ -run
   TestLiveOpenCodeConversationE2E -count=1 -v`
   - Sends `Reply with exactly: E2E-OK` on
     `conversation-opencode-default-v1` / `deepseek/deepseek-chat`
     (`ContextModeStartClean`, binding `{3, opencode}`) through the daemon UDS
     socket.
   - Asserts the final loom message is exactly `E2E-OK`.

## Result

```
vault status: unlocked
last message role=loom content="E2E-OK"
--- PASS: TestLiveOpenCodeConversationE2E (3.23s)
```

The conversation was dispatched through the OpenCode runtime, the DeepSeek
credential was leased from the Loom Credential Vault and injected into the
OpenCode process env, and a bounded real model reply was stored in the thread.

## Prior failures resolved (same gate)

- `state_unavailable / vault_encrypt` (openCode profile had no Context Capsule
  target) → fixed routing.
- `provider_unavailable / provider_connect` (OpenCode responder captured a nil
  vault lease access; model resolution failed) → responder bound after leases.
