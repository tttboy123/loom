# P2A-W2 Final Mission Decision View Lifecycle — Repair 1

**Date**: 2026-08-01

**Status**: FROZEN inside the existing complete P2A-W2 reopen

**Trigger**: Implementation Review 1 P1/P2

## Exact repair

Repair 1 may modify only:

- `cmd/loomd/product_daemon_test.go` to replace direct handler calls in the
  lifecycle regression with one private real `localipc.Server` and
  `localipc.Client` transaction; and
- lifecycle evidence/source-lock text and `docs/CURRENT.md`.

The test must perform:

```text
real client snapshot before discovery
-> real Journal Runtime discovery commit
-> real client snapshot after discovery
-> old mission_decision read conflicts
-> rebound mission_decision read succeeds
-> Event count remains unchanged by reads
```

The production files remain byte-identical. No Swift, IPC protocol, server,
daemon assembly or authority source is reopened. The combined source digest
method must be corrected to source-lock file order when hashes are regenerated.

After the focused test passes, the complete Go/race/vet/strict-Swift/Swift
matrix, five-file source lock and fresh independent Implementation Re-review
remain mandatory. Live stays locked.
