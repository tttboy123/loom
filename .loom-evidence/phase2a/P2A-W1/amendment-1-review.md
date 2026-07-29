# P2A-W1 Amendment 1 Independent Review

**Date**: 2026-07-28
**Reviewer**: fresh independent read-only Reviewer
**Verdict**: PASS
**Blocking findings**: none

## Findings

1. The amendment is necessary. The parent contract's ordinary client default
   is `$HOME/Library/Application Support/Loom/run/loomd.sock`, while its live
   gate names `$HOME/Library/Application Support/Loom/demo-resident/run/loomd.sock`.
   Those cannot satisfy the frozen no-configuration/no-terminal journey.
2. The amendment is minimal. It changes only live-gate steps 3 and 4 to use the
   already-frozen client default. Current code already resolves that default,
   daemon configuration already accepts one explicit `--socket`, and the
   installer already creates a no-argument launcher.
3. It is inside the parent governance ownership. The parent permits W1 evidence
   files and requires a reviewed amendment instead of a workaround.
4. It preserves one application, one authority, one private UDS, and one
   daemon. It adds no WorkItem, socket, listener, database, writer, or product
   code change.
5. It preserves all safety exclusions: omitted `--socket` remains
   observer-only; the live gate still prohibits Provider/model requests and
   Runtime execution.

## Bound live-gate note

Before live mutation, record absence or exact identity for the newly
authoritative default socket. Also record whether the historical
`demo-resident/run/loomd.sock` exists and prove it is not active or treated as a
second product socket.
