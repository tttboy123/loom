# P2A-W1 Implementation Re-review 2

**Date**: 2026-07-28
**Reviewer**: fresh independent read-only Reviewer
**Live gate executed**: no
**Verdict**: `FAIL`

## Blocking findings

1. The IPC server treated a five-millisecond read timeout after the first frame
   as request completion. A half-open peer could therefore dispatch a request
   without the frozen one-frame-followed-by-EOF boundary.
2. The server detached handler work into a goroutine that was not part of the
   server lifecycle. `Serve` and `Close` could return while a non-cooperative
   handler remained alive; the regression test explicitly unblocked that
   handler only after shutdown.
3. The production CLI real-UDS test used a synthetic hard-coded handler. It did
   not prove that the real `loom` binary used the same non-empty SQLite-backed
   product daemon and immutable view already exercised by the TUI.

## Required gate

The resident-daemon live gate remains closed. Repair 2 must stay inside the
frozen P2A-W1 owned files, require exact EOF before handler dispatch, join all
trusted context-aware handler work, and exercise the real CLI binary against
the same non-empty SQLite product daemon used by the TUI. The complete
deterministic matrix and a fresh independent Implementation Review must pass
before any live mutation.
