# P2A-W2 Isolated Pi Catalog Live Attempt 003 Result

**Date**: 2026-07-30
**Lineage**: `p2a-w2-live-20260730-003`
**Status**: `FAIL — OBSERVER_AFTER_IPC_READY — NO RETRY UNDER FROZEN LINEAGE`

## Verdict

The single daemon start authorized by the reviewed Isolated Pi Catalog and Live
Closure Amendment exited with code `4` and the sanitized terminal result:

```text
daemon failed: observer
```

Production lifecycle inspection proves that the product IPC server reached its
internal `Ready()` boundary before the observer was started. The observer then
failed, the daemon closed the IPC server, and the product socket was absent by
the time the Controller observed the failed exit. The polling evidence did not
capture the transient ready socket and must not be represented as proof that
the socket was never created.

The native app was not launched. The native Team Builder journey therefore did
not start, and none of its live acceptance claims were earned.

No replacement start or alternate executable/configuration path was attempted.
The frozen attempt-003 live allowance is consumed by this failed start.

## Frozen inputs and preflight

The exact inherited source was:

```text
/Users/lune/Library/Application Support/Loom/phase2a-w2-live-20260730-002/state/loom.db
```

Immediately before cloning it was a same-owner regular `0600` file of `24576`
bytes with SHA-256
`b0312739ba7b24961b9e8a698e4cbf436e9cace8758a1d2a058e050aec475e22`,
`PRAGMA integrity_check=ok`, schema version `1`, and exactly five Events:

```text
ProviderCredentialConfigured = 1
ProviderCredentialVerified   = 3
RuntimeInstanceDiscovered    = 1
total                        = 5
```

The credential stream was sequence `1..4`; the Runtime stream was sequence
`1`. The credential reference was checked only for one stable value, accepted
shape and length and was not printed or read into evidence.

The fresh attempt root and its `state`, `isolation`, `bin`, `native`, and
`evidence` directories were private `0700`. The destination database was a
same-owner regular `0600` byte-for-byte copy with the same SHA before daemon
startup, `integrity_check=ok`, and no WAL/SHM.

The reviewed amendment and Implementation Review were copied into the attempt
as regular `0600` files. Their SHA-256 values are:

```text
contract.md              b2a9e78a135190ea5c6e7ca08569fb9012327a2a25524d5c3e1f5df9a04f210a
implementation-review.md cf7bcfa704bd4fe75467c5e5340efbce85f24ed69f51f98365084de671431720
```

The newly built exact Candidate artifacts were:

```text
daemon  2eeb5095a8bbec2269d438968a86ccd101f16e9d55ebabc8ea02e7b89bf88add
app     8b12670b30aeef25959f4acd024967655b3bf688ccb456d21004ca31d4eddbc7
```

Both executables were regular `0700` files owned by uid `501`. The arm64 native
bundle had identifier `com.earendilworks.loom.local`, passed strict code-sign
verification, and contained no symlink. It was materialized but never launched.

The exact Pi 0.82.1 CLI, Node, Codex native binary, llama-server, and frozen
GGUF identities matched the reviewed preflight. The model was the regular
`0600`, uid `501`, `1117320768`-byte file with SHA-256
`cc324af070c2ecbfd324a30884d2f951a7ff756aba85cb811a6ec436933bb046`.

## Post-failure authoritative state

Read-only inspection after the failed start reproduced:

```text
attempt database mode/owner = regular 0600, uid 501, gid 20
attempt database size       = 24576
attempt database SHA-256    = b0312739ba7b24961b9e8a698e4cbf436e9cace8758a1d2a058e050aec475e22
SQLite integrity_check      = ok
total Events                = 5
new attempt-003 Events       = 0
WAL/SHM                     = absent
product socket after exit   = absent
attempt processes           = absent
Pi/llama child processes    = absent
native app process          = absent
```

The five Events remain only the inherited invalid/diagnostic attempt-002
baseline. Attempt 003 appended no Runtime, Provider, Team, Agent, WorkItem, Run,
Grant, Evidence, dispatch, or execution fact.

The post-exit socket absence proves successful cleanup, not pre-socket failure.
The separate accepted resident Runtime observer remained PID `44887` with its
original state and arguments and no product socket. It was not stopped,
reconfigured, signaled, or otherwise touched.

The source attempt-002 database remained byte-identical after the clone and
failed start: regular `0600`, uid `501`, `24576` bytes,
`integrity_check=ok`, five Events, and the same frozen SHA-256.

## Diagnostic boundary

Read-only source inspection found no safe basis to identify one exact root
cause from the daemon's deliberately sanitized `observer` failure:

- the isolated catalog serializer matches Pi 0.82.1's documented provider/model
  shape;
- the strict Pi table parser accepts the expected real table spacing;
- the observer intentionally rejects any Pi diagnostic stderr;
- the large model binding is revalidated before metadata calls.

The failed process left no invocation directory or unsanitized child output.
Therefore this evidence does not speculate whether the precise cause was Pi
stderr, process timeout, model-binding revalidation, or another observer
failure. Isolating that cause would require a newly reviewed diagnostic
lineage; it is not silently inferred and no second canary is consumed here.

## Acceptance impact

- The deterministic isolated Pi catalog Candidate and its independent
  Implementation Review remain `PASS`.
- The required W2 native journey and live Result-Evidence acceptance proof are
  `FAIL — OBSERVER_AFTER_IPC_READY`.
- P2A-W2 remains unaccepted.
- P2A-W3 remains locked.
- No P2A-W4 exists.
- This result awaits fresh independent read-only Result-Evidence Review.
