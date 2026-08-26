# G2 — Real conversation (OpenCode live)

Status: `PASS` (installed-live conversation and restart recovery)

Date: 2026-08-23

## What was executed

1. Built and installed the App (`scripts/build-loom-local-app.sh`,
   `scripts/install-loom-local-app.sh`), started it, daemon online with
   OpenCode / Loom Native / Pi runtimes.
2. Credential Vault `unlocked`; DeepSeek account verified
   (`deepseek.primary`, revision 6, brokered).
3. `LOOM_LIVE_OPENCODE_E2E=1 go test ./cmd/loomd/ -run
   TestLiveOpenCodeConversationE2E -count=1 -v`
   - Sends one bounded acceptance turn on the OpenCode profile and model
     (`ContextModeStartClean`, binding `{3, opencode}`) through the daemon UDS
     socket.
   - Asserts an exact bounded Loom reply without retaining its content here.
4. Build 112 created one fixed-ID four-Segment DeepSeek/OpenCode Conversation,
   terminated the App and managed daemon, cold-launched the installed bundle,
   and performed a verify-only read of the same Conversation.
5. Build 113 added `chat_context_disclosure`, inspected every Segment using an
   exact thread/Segment/Capsule/receipt binding, and repeated the fixed-ID
   restart gate with an asserted old/new daemon PID transition. The returned
   shape contains metadata only and rejects unknown Swift wire fields.
6. Build 115 moved typed product route dispatch into its bounded route module,
   then returned an exact real OpenCode reply before and after a cold App and
   daemon restart. The post-restart catalog remained 25 Providers, 7 Runtimes,
   5 safe import candidates and 4 Conversation Profiles.
7. The same installed build completed both a brokered OpenCode + DeepSeek
   2-Agent Team and a native-auth OpenCode 2-Agent Team.
8. Build 117 fixed strict Mission decoding for slash-qualified native model IDs,
   visually inspected the installed conversation-first shell, Mission Team
   Pulse and Team configuration directory, then returned an exact real OpenCode
   reply before and after a second cold App/daemon restart.

## Result

The conversation was dispatched through the OpenCode runtime, the DeepSeek
credential was leased from the Loom Credential Vault and injected into the
OpenCode process env, and a bounded real model reply was stored in the thread.
After cold restart, all four Segment/Profile/context-mode pairs, Context Capsule
digests, distinct disclosure receipts and explicit omission counts were
recovered. The verify-only phase passed in 0.23 seconds and issued no Provider
request.

Build 113 repeated the installed gate with thread
`thread-build113-restart-20260823t005000z`; the new inspector passed before and
after daemon PID `56670` was replaced by `57224`. Native click/visual inspection
remains open because macOS was locked; source interaction and VoiceOver tests
pass.

Build 115 retains the same privacy and restart behavior after the COMP2-E route
module migration. Real installed Team execution confirms OpenCode is an Agent
Harness in both brokered and native-auth modes, not only a conversation option.

Build 117 closes the installed visual gate. The App opens into the Conversation
surface with local service ready; Mission shows per-Agent Harness, Provider
Account/native auth, Model, Attempt, limits, Context and Incident; Teams shows
one current configuration with per-Agent route rows. Before and after cold
restart, setup remained 25 Providers, 7 Runtimes and 4 Conversation Profiles,
the Vault was unlocked, and the live reply was exact `E2E-OK`.

## Prior failures resolved (same gate)

- `state_unavailable / vault_encrypt` (openCode profile had no Context Capsule
  target) → fixed routing.
- `provider_unavailable / provider_connect` (OpenCode responder captured a nil
  vault lease access; model resolution failed) → responder bound after leases.

## Build 127 active-gate closure (2026-08-24)

Status: `PASS / NO NEW EXTERNAL PROVIDER REQUEST`.

Build 127 preserves the previously accepted real Provider conversation and
closes the remaining local governance cells around it. The authenticated
private-UDS acceptance matrix proves all three Context modes, exact frozen
target binding/Capsule/receipt/Incident identity, and zero mutation for missing
or stale expected bindings. Capacity authority persists across dispatch and
restart as exact, estimated or unavailable without exposing Context content.

Installed accessibility inspection verifies that a real DeepSeek-to-MiniMax
review names trust domain, retention and data-region changes, disables Confirm
until acknowledgement, and retains all three Context choices. The signed App
and managed daemon shut down together and restart into `Local service ready`.

Bundle identity and full verification are recorded in
`../../P2D-W2A-W2D-trust-capacity-route-build127.md`. No credential was read or
changed and no external Provider request was made for this Build 127 closure.
