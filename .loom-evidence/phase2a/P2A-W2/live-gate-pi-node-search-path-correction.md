# P2A-W2 Live Gate Pi Node Search Path Correction

**Date**: 2026-07-29
**Status**: REVIEWED — fresh independent Contract Review and wording re-review PASS
**Parent**: reviewed P2A-W2 Live Gate and Deterministic Checkpoint Commit
Amendment

## 1. Pre-consumption startup result

The reviewed W2 binaries were built and copied into the frozen attempt root.
Their recorded identities are:

```text
loomd_sha256       = 1d06d2696272e7560fb86221516daa52d5b8ca0d0cc233c3d5966e3d08aa2180
LoomLocalApp_sha256 = 49ac7ea4a151269ae619acdb3d1716cb65770f207b2d50d1e110503264de3a92
```

The first daemon startup used the frozen invocation and a credential-clean
environment. It exited before the native app, Codex, Keychain, Provider
network, SecureField, or Team Builder was used:

```text
exit_code = 4
stderr    = daemon failed: observer
```

Post-exit evidence proves:

- the product socket is absent;
- the isolated SQLite file is a user-owned regular `0600` file;
- the Journal `events` table contains exactly zero rows;
- no credential was entered or stored;
- no network request was made;
- the resident accepted Runtime observer was not changed;
- the live allowance remains unconsumed.

This was a pre-consumption startup check, not the controlled W2 product
journey. It produced no authoritative product fact and cannot be treated as
live acceptance evidence.

## 2. Root cause

The frozen invocation supplied only:

```text
--runtime-dir /Users/lune/Library/Application Support/Loom/runtimes/pi/0.82.1/node_modules/.bin
```

The locked Pi executable at that path is a symlink to the reviewed Pi 0.82.1
JavaScript entry point. Production `PiMetadataProcessRunner` intentionally
constructs its child `PATH` solely from the ordered `--runtime-dir` values; it
does not inherit the daemon's ambient `PATH`. The Pi entry point uses
`#!/usr/bin/env node`, so the single frozen search directory cannot resolve
`node`.

An independent credential-clean diagnostic invocation with the exact Pi target
and an explicit reviewed Node directory returned:

```text
pi --version = 0.82.1
pi --offline ... --list-models =
  No models available. Use /login to log into a provider via OAuth or API key.
```

The result proves Pi itself and the frozen metadata arguments are compatible.
The missing ordered Node search directory is the complete startup defect.

## 3. Exact correction

The same unconsumed attempt may perform one corrected daemon startup by adding
exactly one ordered search path after the Pi candidate directory:

```text
--runtime-dir /Users/lune/Library/Application Support/Loom/runtimes/pi/0.82.1/node_modules/.bin
--runtime-dir /Users/lune/Documents/Codex/devtools/node/bin
```

The first directory remains the only Pi candidate source. The second directory
supplies only the interpreter required by the already reviewed Pi JavaScript
entry point. Production code continues to construct the child `PATH` from
these explicit directories and continues to reject ambient lookup.

Locked Node observations:

```text
path       = /Users/lune/Documents/Codex/devtools/node/bin/node
sha256     = 1ee75375e33b94fc34b3b19aede049e11dae90efb63b374dc96d6bdace70c4b8
file_type  = Mach-O 64-bit executable arm64
owner      = lune
mode       = 0755 regular file
bytes      = 120573328
device     = 16777229
inode      = 12984760
```

The Node path, device, inode, owner, type, mode, size and SHA-256 must match
immediately before corrected startup. Any mismatch stops without execution.

## 4. Attempt and authority preservation

- This correction remains inside P2A-W2; it creates neither W3 nor W4.
- It changes no product source, test, binary, StateWriter, Journal, Projection,
  Provider, credential, or authorization boundary.
- The same frozen attempt ID and root are retained. The existing zero-event
  SQLite file is retained for auditability rather than deleted or replaced.
- The corrected startup is not a second canary or a retry of a consumed
  canary. The sole allowance is still consumed only if the user enters a fresh
  secret and activates `Store securely`.
- All Codex native-binary corrections, source locks, one-request limit,
  TeamDefinition-only authority, no-hidden-retry rule, Result-Evidence Review,
  revocation, cleanup and stop rules remain unchanged.
- Immediately before corrected startup the Journal must still contain exactly
  zero Events.
- After corrected startup and before SecureField hand-off, only the expected
  isolated Runtime discovery/status observation Events may exist. Any
  credential, TeamDefinition, TeamInstance, AgentInstance, WorkItem, Run,
  Grant, Evidence, dispatch, generation, or other unexpected Event stops the
  W2 live result.
- If corrected startup fails, or if the socket/permission/identity checks fail,
  the W2 live result stops without another correction or daemon start.

Fresh independent Contract Review `PASS` is required before the corrected
daemon startup.
