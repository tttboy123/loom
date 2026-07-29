# P2A-W1 Implementation Review 1

**Date**: 2026-07-28
**Reviewer**: fresh independent read-only Reviewer
**Live gate executed**: no
**Verdict**: `FAIL`

## Blocking findings

1. The stale fallback stored one response instead of the last immutable
   `GlobalReadView`, so a request using different cursors could receive the
   wrong page.
2. Compare was a placeholder and the snapshot Attention list was hard-coded
   empty.
3. The TUI did not render stream gaps, board/Attention, or partial state;
   protocol mismatch was misclassified as daemon offline; in-flight reads did
   not use a cancelable model context.
4. IPC socket deadlines did not bound the handler context, and `Close` could
   race `Accept` before connection WaitGroup registration.
5. Stale-socket cleanup accepted ambiguous dial errors, and lock cleanup had
   path-only TOCTOU branches.
6. Installer replacement was not transactionally restored after an
   intermediate failure, and its fixture did not run the real Bubble Tea
   binary or compare all rollback files.
7. The claimed real SQLite/UDS/TUI E2E used an empty Journal and did not prove
   CLI transport, cursor reconnect, daemon restart, or exact stream-head
   preservation.

## Required gate

The resident-daemon live gate remains closed. Repair must stay inside the
frozen P2A-W1 owned files, add direct regression evidence for every finding,
rerun the complete deterministic matrix, and receive a fresh independent
Implementation Re-review `PASS`.

