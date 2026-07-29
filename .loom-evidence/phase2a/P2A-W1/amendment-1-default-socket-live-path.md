# P2A-W1 Amendment 1: Default Socket Live-Path Reconciliation

**Date**: 2026-07-28
**Status**: FROZEN — fresh independent Amendment Review PASS
**Parent**: frozen P2A-W1 Local App Shell and Read Experience Contract

## Conflict

The frozen contract defines the ordinary client default as:

```text
$HOME/Library/Application Support/Loom/run/loomd.sock
```

Section 16 separately tells the live gate to configure the resident daemon at:

```text
$HOME/Library/Application Support/Loom/demo-resident/run/loomd.sock
```

Those cannot satisfy the same no-argument installed-TUI canary. Adding a
symlink, environment override, second socket, hidden CLI argument, or duplicated
server would violate other frozen boundaries.

## Amendment

Only section 16 steps 3 and 4 are reconciled:

1. The live gate creates
   `$HOME/Library/Application Support/Loom/run` as an owned non-symlink
   directory with mode `0700`.
2. The existing private LaunchAgent receives exactly:

   ```text
   --socket
   /Users/lune/Library/Application Support/Loom/run/loomd.sock
   ```

3. The installed `loom` and `Loom.command` continue to use the exact default in
   section 11. No socket override or environment variable is needed.

All other section 16 gates remain unchanged, including one controlled
replacement, pre-state hashes, credential-clean service-manager context,
secret-negative process metadata, no Provider/Runtime execution, one restart,
same-view recovery, exact rollback on failure, and preservation only on full
PASS.

Before live mutation, the gate also records absence or exact identity for the
authoritative default socket and records whether the historical
`demo-resident/run/loomd.sock` exists. A historical socket must not be active or
treated as a second product socket.

## Boundary impact

- No product code, authority, method, IPC schema, installer behavior, or owned
  file changes.
- No additional WorkItem, socket, listener, daemon, database, or writer.
- No current LaunchAgent mutation before independent Amendment and
  Implementation Reviews pass.
- `demo-resident` may remain an installed binary/state location; it is no
  longer the product socket location.
