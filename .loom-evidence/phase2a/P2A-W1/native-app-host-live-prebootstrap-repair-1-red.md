# P2A-W1 Native App Host Live Pre-Bootstrap Repair 1 RED

Date: 2026-07-28  
Status: `RED — LIVE ALLOWANCE UNCONSUMED`

## Trigger

After the exact Native App Host activation was received, the controlled live
gate reproduced every original pre-state invariant and built the reviewed
Candidate in a fresh private root. The daemon installer dry-run then stopped
before any install, bootout, bootstrap, socket creation, app launch, or
Computer Use action:

```text
incomplete current installation
```

The accepted resident installation contains the exact signed-off `loom` and
`loomd` binaries but predates the optional `Loom.command` compatibility
launcher. The installer accepted only zero or three current files, so it
rejected this valid two-binary legacy state.

## Mandatory failing proof

`scripts/test-install-loom-local-product.sh` now constructs a private
user-owned legacy resident fixture containing exactly:

```text
bin/loom
bin/loomd
```

It requires:

1. dry-run acceptance with byte-for-byte no mutation;
2. atomic Candidate install that creates the current compatibility launcher;
3. exact prior binary bytes retained without fabricating a prior launcher;
4. rollback to the exact launcher-absent legacy state;
5. the Candidate triple retained as the new rollback state.

Against the pre-repair installer, the focused fixture fails at the first
dry-run with `incomplete current installation`. This is the expected RED.

## Authority and live state

The live failure occurred before Candidate bootstrap, so the native allowance
remains `1`, unconsumed. Fresh rollback audit reproduced:

- exact original binary, wrapper, plist, and SQLite hashes;
- running original observer;
- SQLite integrity `ok`, one Event, unchanged canonical digest;
- absent product run directory, sockets, launcher, and native app;
- empty staging and absent Swift build cache.

No Provider value, Runtime, Team, Journal write, authority transition, app
installation, daemon installation, service mutation, or UI action occurred.
