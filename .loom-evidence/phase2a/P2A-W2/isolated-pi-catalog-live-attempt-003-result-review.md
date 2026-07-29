# P2A-W2 Live Attempt 003 Result-Evidence Review

**Date**: 2026-07-30
**Review type**: fresh independent read-only Result-Evidence Review
**Result reviewed**: superseded `FAIL — OBSERVER_PRE_SOCKET`
**Verdict**: `REPAIR 1 REQUIRED`

```text
P0 = none
P1 = socket lifecycle chronology is contradicted by product code
P2 = none
VERDICT = REPAIR 1 REQUIRED
```

## Evidence verdict

The initial read-only review confirmed the post-exit filesystem and Journal
facts but missed a lifecycle contradiction:

- the frozen contract permitted exactly one
  `p2a-w2-live-20260730-003` lineage after Implementation Review `PASS`;
- the single daemon start exited code `4` with sanitized
  `daemon failed: observer`, and the native app was not launched;
- the deterministic Implementation Review remains `PASS` and is not
  contradicted by the live failure;
- the failed live result does not accept P2A-W2, unlock P2A-W3, or create
  P2A-W4.

## Read-only reproduction

The Reviewer independently reproduced:

```text
repo = /Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-pi-rebuild
HEAD = 6b155191fe3cbd112d65f4840fa9b4d389a2b0bc

attempt root mode/owner = directory 0700, uid 501, gid 20
attempt DB              = regular 0600, uid 501, gid 20, 24576 bytes
attempt DB SHA-256      = b0312739ba7b24961b9e8a698e4cbf436e9cace8758a1d2a058e050aec475e22
source DB SHA-256       = b0312739ba7b24961b9e8a698e4cbf436e9cace8758a1d2a058e050aec475e22
integrity_check         = ok for both
WAL/SHM/journal         = absent for both
```

Without reading `payload_json` or printing a credential reference, both
databases had the same Event metadata:

```text
ProviderCredentialConfigured = 1
ProviderCredentialVerified   = 3
RuntimeInstanceDiscovered    = 1
total                        = 5
schema_version               = 1

provider-credential/minimax                           seq 1..4, count 4
runtime_instance:runtime.p2a-w2-live.pi.0.82.1        seq 1..1, count 1
```

Byte identity with the inherited baseline proves that attempt 003 appended zero
new Events. The Reviewer also reproduced these post-exit facts:

```text
product socket candidates after exit = absent
attempt processes          = absent
isolation directory        = empty
native executable SHA-256  = 8b12670b30aeef25959f4acd024967655b3bf688ccb456d21004ca31d4eddbc7
daemon SHA-256             = 2eeb5095a8bbec2269d438968a86ccd101f16e9d55ebabc8ea02e7b89bf88add
contract copy SHA-256      = b2a9e78a135190ea5c6e7ca08569fb9012327a2a25524d5c3e1f5df9a04f210a
implementation review SHA  = cf7bcfa704bd4fe75467c5e5340efbce85f24ed69f51f98365084de671431720
```

Resident observer PID `44887` remained a separate
`demo-resident/bin/loomd` process using its original demo-resident state and
isolation paths, not an attempt-003 process.

## Repair 1

Fresh Controller source-path inspection established that
`productDaemonRunner.Run` starts the local IPC server, waits for
`runner.server.Ready()`, and only then starts `runner.observer.Run`. An
`observer` failure classification therefore proves that IPC reached the ready
boundary before the observer failed. The later absent socket proves cleanup;
it cannot prove that the socket was never created.

The original `PASS` verdict is withdrawn. The result record is repaired to
`FAIL — OBSERVER_AFTER_IPC_READY`, retaining the independently reproduced
zero-new-Event, no-app-launch, no-retry, cleanup and resident-preservation
facts. A fresh independent Repair 1 Result-Evidence Review is required.

## Fresh independent Repair 1 review

**Verdict**: `PASS`

```text
P0 = none
P1 = none
P2 = none
VERDICT = PASS
```

The fresh Reviewer directly inspected `productDaemonRunner.Run` and confirmed
the exact chronology: `Serve`, internal `Ready`, observer start, observer
failure, cancellation, server close and sanitized `observer` classification.
It independently reproduced both database hashes and integrity, the inherited
five-Event metadata set, zero new Events, empty isolation directory, absent
post-exit socket/processes, and the separate resident PID `44887`.

The Reviewer read no `payload_json` or credential reference and ran no product,
Pi, llama-server, Codex, native app, Keychain or network action. It modified
nothing. Repair 1 Result-Evidence Review therefore accepts the corrected
failure record, not the live closure.

## Independence and non-actions

The Reviewer used only read-only file/stat/hash/process-list and SQLite
metadata/count queries. It did not read or print credential payloads or
references; edit, format, stage, commit, delete, restore, or clean up files;
run daemon, Pi, Codex, llama-server, or the native app; use Keychain,
MiniMax/network, or GUI; mutate the resident observer; or execute a live
canary.
