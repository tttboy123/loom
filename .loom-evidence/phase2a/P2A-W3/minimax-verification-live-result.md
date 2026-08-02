# P2A-W3 MiniMax Verification Live Canary Result

**Date**: 2026-08-02  
**Attempt**: `phase2a-w3-live-20260802-minimax-001`  
**Initial manifest SHA-256**: `f42d2d8c...109ef`  
**Active manifest SHA-256**: `a96860080c32c91e21bd399165a956d2164646a98ebff0e6ec52c307b5d20d36`  
**Allowance**: consumed once; no retry  
**Verdict**: `FAIL — VERIFICATION FACT NOT COMMITTED`

## Preflight and daemon

The unconsumed manifest was completed with the reviewed explicit `30s` Pi
metadata process budget; its prior bytes remain under the attempt manifest
directory. Source lock, Implementation Review, private modes, no-symlink tree,
attempt binaries, native bundle and external Codex/Pi/Node/llama/GGUF identities
all matched.

The private initial SQLite was byte-identical to the frozen W2 metadata source,
integrity `ok`, with one `ProviderCredentialConfigured`, three
`ProviderCredentialVerified` and one legacy Runtime discovery fact. It contains
no raw credential.

The single daemon start succeeded. It appended one current model-capable online
Pi Runtime discovery and remained available for three observation cycles. The
exact native app displayed:

```text
Codex   Available
MiniMax Verified
```

The raw credential was never read, focused, typed, displayed, copied, logged or
placed in the daemon environment.

## One verification action

Computer Use clicked the ordinary product-sheet `Test` button exactly once.
The UI continued to display the inherited `Verified` state, but a bounded
12-second Journal observation found no fourth `ProviderCredentialVerified`
fact. The Controller did not click again, use direct IPC, replace the
credential, issue another network request or claim a successful new
verification.

Because the mandatory new terminal verification fact is absent, exact product
preflight was not attempted and this one-shot canary fails.

## Shutdown and authoritative postflight

The native sheet was dismissed and the app quit. One normal interrupt stopped
the daemon cleanly with exit `0` and:

```json
{"completed_cycles":3,"discovery_events":1,"status_events":0,"no_write_cycles":2}
```

The final DB is private, integrity-valid and SHA-256:

```text
946871393f5689b1d1eb10280e3d71dc80af581b220e6778abb9c0a4b0cc43f9
```

Complete facts:

```text
AgentGrantIdentityIndexInitialized | 1
ProviderCredentialConfigured       | 1
ProviderCredentialVerified         | 3
RuntimeInstanceDiscovered          | 2
WorkRunIdentityIndexInitialized    | 1
```

There is zero TeamDefinition, TeamInstance, AgentInstance, WorkItem, Run,
Grant, Evidence, dispatch or execution fact. Product socket/lock are absent,
isolation is empty, all attempt processes are absent, and retained non-binary
files contain no raw credential/token/API-key value.

**VERDICT**: `FAIL — VERIFICATION FACT NOT COMMITTED`
