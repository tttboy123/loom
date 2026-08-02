# P2A-W3 Authoritative Acceptance/Recovery Repair 4 RED

**Date**: 2026-08-03  
**Repair contract SHA-256**: `4d54af39b1e08b6b499775107cda4d8c88e74fe929cd4c08d6b81ff3885798d4`  
**Status**: `CAUSAL RED CONFIRMED`

Before changing production code, two real Work Authority fixtures appended a
structurally valid `TeamNodeRecoveryRecorded` Event without the downstream fact
that the same recovery decision requires. They then resubmitted the identical
raw recovery intent after advancing the Authority clock.

Command:

```text
go test -count=1 ./internal/work -run '^TestTeamRecoveryExactReplayRejectsMissing(ScheduledAttempt|TeamTerminal)$'
```

Observed failure:

```text
--- FAIL: TestTeamRecoveryExactReplayRejectsMissingScheduledAttempt
    partial retry replay error = <nil>, want conflict
--- FAIL: TestTeamRecoveryExactReplayRejectsMissingTeamTerminal
    partial terminal replay error = <nil>, want conflict
FAIL
```

The failures are causal: `ScheduleTeamNodeRecovery` accepted both incomplete
histories as exact idempotent replay, returned without a new write and did not
fail closed. No production file had been changed when this RED was captured.
