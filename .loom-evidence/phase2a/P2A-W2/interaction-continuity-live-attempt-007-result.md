# P2A-W2 Interaction Continuity Live Attempt 007 Result

**Date**: 2026-07-30
**Lineage**: `p2a-w2-live-20260730-007`
**Candidate commit**: `2225c9ff418252d344946ae612ab33d4868348c1`
**Verdict**: `FAIL — NATIVE_PROVIDER_MANAGEMENT_UNREACHABLE`

## Exact preflight

The Controller created the fresh private root:

```text
/Users/lune/Library/Application Support/Loom/phase2a-w2-live-20260730-007
```

All attempt directories were user-owned `0700`; ordinary files were regular
user-owned `0600`; executables were `0700`; the tree contained no symlink and
the isolation root was empty.

The copied SQLite database was byte-identical to the frozen attempt-002 source,
integrity `ok`, and contained exactly five Events:

```text
ProviderCredentialConfigured | 1
ProviderCredentialVerified   | 3
RuntimeInstanceDiscovered    | 1
```

The exact post-Review Candidate artifacts were:

```text
loomd =
d64a50ee2409e8f3961f7fef057bef339adee62572f1303c8f22b5c79e4d2507

LoomLocalApp =
acba820cce37cf3768f1252c8a26598731e48ba5b5f685ef2596cc033ec45a8e
```

The native bundle was arm64, strict ad-hoc signature verification passed, and
its identifier was `com.earendilworks.loom.local`. Pi 0.82.1, Node, Codex
0.144.1 native arm64, llama-server and GGUF identities matched the frozen
hashes.

The default product socket was absent. Its abandoned sibling lock remained the
exact regular private zero-byte inode `74706302` born
`2026-07-30T04:02:27+0800`. The Controller did not remove or modify it.

The complete private preflight manifest has SHA-256:

```text
834f81c3df8ba39d10735f662a5d70d22ea88a1339b1870ef155a769875d53cc
```

## One consumed daemon start

The exact Candidate daemon was started once. It atomically replaced the
abandoned lock with private inode `75594407`, created private product socket
inode `75594408`, and became the only listener at PID `82051`.

The first observation appended exactly one model-capable
`RuntimeInstanceDiscovered` Event:

```text
runtime = Pi 0.82.1 W2 Canary
status  = online
model   = loom-local/qwen2.5-coder-1.5b-instruct-q4-k-m
```

No retry, second start, alternate executable, argument mutation, manual lock
deletion or resident-service mutation occurred.

## Ordinary native journey

Computer Use opened the exact attempt-007 bundle once. The native app opened
directly into the chat/task-first three-column workspace:

```text
Tasks and recent work | New task conversation | Inspector
```

The user-visible state showed:

```text
Local service connected
Codex   Available
MiniMax Verified
```

The Controller entered the bounded task intent:

```text
Prepare a reviewed local code change
```

and completed the one-question-at-a-time Builder through the human-readable
preflight:

```text
name     = Local Coding Team
purpose  = Prepare a reviewed local code change
Main     = Coordinator
SubAgent = Bounded Worker
Provider = Codex
model    = loom-local/qwen2.5-coder-1.5b-instruct-q4-k-m
auth     = Native auth
Runtime  = Pi 0.82.1 W2 Canary
budget   = maximum 100 credits
```

The wide screenshot is:

```text
interaction-continuity-live-attempt-007-wide.jpeg
dimensions = 1100 x 720
sha256 = 10621e1a20a897fe2c9f535f13203f0aa34d20c0e08b4bc0f4c35621ee1b95a4
```

No secret, raw credential reference, OAuth material, hidden reasoning or
internal identifier is visible.

## Blocking live finding

The native task-first workspace exposes only read-only Provider status. It has
no reachable `Manage`, `Test` or Settings action.

The code explains the observed dead route:

```text
ContentView.swift:256-258
    mounts setupProviderPanel only when setupSnapshot == nil

ContentView.swift:1499-1500
    ProviderConnectionDirectory renders only when setupSnapshot != nil
```

The two conditions cannot be true in the same render. The accepted
`ProviderConnectionDirectory` contains the MiniMax `Test` control, but the new
ordinary-user workspace can never display it. The app menu also contains no
Settings item.

The frozen live contract requires exactly one MiniMax `Test` through the
ordinary product window before saving. The Controller therefore stopped at the
preflight. It did not invoke IPC directly, use the terminal as a UI bypass,
click `Confirm saved team`, append a verification fact or create a Team.

This is a product-path defect, not a credential, Provider, network or
environment failure.

## Authoritative stop state

Before shutdown the database contained:

```text
ProviderCredentialConfigured | 1
ProviderCredentialVerified   | 3
RuntimeInstanceDiscovered    | 2
TeamDefinitionSaved          | 0
total                        | 6
```

No new `ProviderCredentialVerified` fact was appended. There is no
TeamDefinition, TeamInstance, AgentInstance, WorkItem, Run, Grant, Evidence,
dispatch or execution fact.

The native app quit normally. One exact `SIGINT` was sent to PID `82051`.
The daemon exited `0` without SIGTERM or SIGKILL and reported:

```json
{
  "completed_cycles": 9,
  "discovery_events": 1,
  "status_events": 0,
  "no_write_cycles": 8
}
```

Postflight proves:

- the product socket and lock are both absent;
- the attempt app, daemon, Pi and llama-server are absent;
- the isolation root is empty;
- SQLite integrity is `ok`;
- the attempt database is regular private `0600`, SHA-256
  `6293390bda9e23211a00b597a335e1132bdaf6eb29368f6dcf715534dcf7137c`;
- the frozen source database remains byte-identical, integrity `ok`, and five
  Events;
- no credential-like secret marker occurs under non-database attempt files;
  and
- the separate resident Runtime-observer LaunchAgent remains running from its
  accepted executable and was not signalled or reconfigured.

## Result

The Candidate proves the new task-first workspace, exact Builder preflight,
model-capable Pi observation, abandoned product-lock reclamation and graceful
joined shutdown in a real native lineage. The complete W2 exit nevertheless
fails because the required native Provider management and MiniMax `Test` path
is unreachable.

The one attempt-007 allowance is consumed:

```text
P2A-W2 = HUMAN_REQUIRED / NOT ACCEPTED
P2A-W3 = LOCKED
P2A-W4 = DOES NOT EXIST
```

There is no retry, alternate live path or single-point Amendment. Fresh
independent read-only Result-Evidence Review is required for this failed
outcome.
