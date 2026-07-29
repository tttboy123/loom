# P2A-W2 Credential Transaction and Shutdown Closure Live Attempt 006 Result

**Date**: 2026-07-30
**Lineage**: `p2a-w2-live-20260730-006`
**Candidate commit**: `47d5f56358d310fd561d2e5a089c79ac543a2cf8`
**Verdict**: `FAIL — PREEXISTING_PRODUCT_LOCK_BLOCKED_LOCAL_IPC`

## Preflight and materialization

The Controller revalidated the exact attempt-002 source database as a
user-owned regular `0600` file, SQLite integrity `ok`, SHA-256
`b0312739ba7b24961b9e8a698e4cbf436e9cace8758a1d2a058e050aec475e22`,
and exactly five Events:

```text
ProviderCredentialConfigured | 1
ProviderCredentialVerified   | 3
RuntimeInstanceDiscovered    | 1
```

The fresh private root was:

```text
/Users/lune/Library/Application Support/Loom/phase2a-w2-live-20260730-006
```

It and all child directories were user-owned `0700`, ordinary files were
`0600`, executables were `0700`, and there were no symlinks. The cloned
database remained byte-identical to the source. The exact post-Review
Candidate artifacts were:

```text
loomd:
cfb79fdd5f38a3cf1ba970edee17073ec7eb9d28901c6d5432211b8130071f02

LoomLocalApp:
75043fd285fd4c065f61a654851ab60f2f0b158e7e2be3a56e05e583e21013f2
```

The native bundle was arm64, strict ad-hoc signature verification passed, and
its bundle identifier was `com.earendilworks.loom.local`. The frozen Pi, Node,
Codex 0.144.1 native executable, llama-server and GGUF hashes matched.

The Controller proved the default product socket absent but failed to include
its sibling lock in the same preflight assertion. Read-only post-failure
inspection found:

```text
path  = /Users/lune/Library/Application Support/Loom/run/loomd.sock.lock
type  = regular file
mode  = 0600
owner = uid 501
size  = 0
inode = 74706302
birth = 2026-07-30T04:02:27+0800
```

The attempt root was created at `2026-07-30T05:08:05+0800`. The lock therefore
predated this attempt by more than one hour. `lsof` found no owner. The
Controller did not remove or modify the lock before or after the failed start.

The separate resident LaunchAgent remained
`com.earendilworks.loom.runtime-observer`, state `running`, with its configured
arguments unchanged and real daemon SHA-256
`e5ab283c75473619094442eb349324b2bf09b8f59250ca1b7fd8b85fffb0014a`.
It was not signalled or reconfigured.

## Consumed daemon start

The exact Candidate daemon was started once with the reviewed Runtime search,
Codex, local-model, state, isolation and default product-socket bindings. It
exited immediately with code `4`.

The complete process output was:

```text
stdout = empty
stderr = daemon failed: local_ipc
```

The strict local IPC server refused the pre-existing product lock before
listener readiness. The observer therefore did not begin its Runtime cycle and
the native app was not launched. No Computer Use action, Codex status process,
Pi metadata process, Keychain operation, MiniMax request, Provider observation,
Team Builder action or SIGINT was reached.

There was no second daemon start, alternate executable, argument change, lock
removal, Test action or retry.

## Authoritative postconditions

The attempt database is still SHA-256
`b0312739ba7b24961b9e8a698e4cbf436e9cace8758a1d2a058e050aec475e22`,
SQLite integrity `ok`, and exactly the inherited five Events. There is no
attempt Event delta and no TeamDefinition, TeamInstance, AgentInstance,
WorkItem, Run, Grant, Evidence, dispatch or execution fact.

Postflight also proves:

- the product socket is absent;
- the pre-existing product lock remains the same private zero-byte file;
- the attempt process, native app, Pi and llama-server are absent;
- the attempt isolation root is empty;
- the attempt database lock is a private zero-byte regular file;
- no credential-like marker occurs under the attempt root; and
- the resident LaunchAgent identity and configuration remain unchanged.

No credential value or raw Provider response was read, entered, logged or
captured.

## Result

The only replacement allowance is consumed before the ordinary native journey
because the preflight checked only the default socket and missed its
pre-existing sibling lock. The Candidate remained fail-closed and produced no
authority mutation, but attempt-006 did not prove any of the required live
journey outcomes.

Under section 7 of the frozen amendment:

```text
P2A-W2 = HUMAN_REQUIRED / NOT ACCEPTED
P2A-W3 = LOCKED
P2A-W4 = DOES NOT EXIST
```

No retry, second canary or further single-point amendment is authorized.
Fresh independent read-only Result-Evidence Review is required for this
recorded failed outcome.
