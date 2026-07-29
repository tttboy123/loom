# P2A-W1 Reopen 4 Review Repair 1 RED

**Date**: 2026-07-28
**Status**: `RED CAPTURED`
**Live action**: none

## Stale-selection RED

```text
go test ./internal/tui \
  -run '^TestModelTruthfullyHandlesJournalWithNoTeams$' -count=1
```

Exit `1`. After loading `team-previous`, a zero-Team snapshot retained:

```text
currentTeam="team-previous"
timeline.TeamInstanceID="team-previous"
```

and continued rendering its terminal record instead of the Team-selection
instruction.

## All-zero PID RED

```text
scripts/test-p2a-w1-live-evidence.sh
```

Exit `1`. The added `00`/`000000` cases returned:

```text
target_unavailable
```

instead of closed status:

```text
invalid_pid
```

No product implementation, installed state, service, SQLite, credential,
Provider, Runtime, staging, or live authority changed during RED.
