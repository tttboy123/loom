# P2A-W1 Reopen 4 RED Evidence

**Date**: 2026-07-28
**Status**: `RED CAPTURED`
**Live action**: none

## TUI empty-Team RED

Command:

```text
go test ./internal/tui \
  -run '^TestModelTruthfullyHandlesJournalWithNoTeams$' -count=1
```

Result: exit `1`.

The new test reached the Teams screen with a valid snapshot containing the
actual resident-state shape: one Runtime and zero Teams/Runs/Evidence/
Attention. The current model returned:

```text
Confirmed and historical Teams
No records in this bounded page.
```

It did not yet provide the frozen truthful Journal-specific empty state.

## Target-process helper RED

Command:

```text
scripts/test-p2a-w1-live-evidence.sh
```

Result: exit `1`.

Exact failure:

```text
missing reviewed live-evidence helper
```

The test itself is executable and already freezes fake-process argument,
one-row, closed-status, marker non-disclosure, invalid-PID, and former
`-eww`-absence expectations.

Both failures are caused only by missing Reopen 4 behavior. No product source,
installed service, process, socket, SQLite, credential, Provider, Runtime,
staging area, or external resource was changed by RED.
