# P2A-W3 Acceptance/Recovery Repair 4 Duplicate-Fact RED

**Date**: 2026-08-03  
**Parent Repair 4 source lock SHA-256**: `70e7b380f7eb40d1b978dee393c6f4ed21efe63e3df2afb091c7ee322f8c9954`  
**Implementation Re-review 2 SHA-256**: `cd1a03869c9fb93975aa152247bbbc642976e7f54101840780b751f10b321178`  
**Status**: `CAUSAL RED CONFIRMED`

Before the second production repair, two Work Authority fixtures appended the
exact recovery transaction followed by one additional structurally plausible
duplicate downstream fact:

```text
recovery + exact TeamNodeAttemptScheduled + duplicate TeamNodeAttemptScheduled
recovery + exact TeamExecutionTerminal + duplicate TeamExecutionTerminal
```

Command:

```text
go test -count=1 ./internal/work -run '^TestTeamRecoveryExactReplayRejectsDuplicate(ScheduledAttempt|TeamTerminal)$'
```

Observed failure:

```text
--- FAIL: TestTeamRecoveryExactReplayRejectsDuplicateScheduledAttempt
    duplicate retry replay error = <nil>, want conflict
--- FAIL: TestTeamRecoveryExactReplayRejectsDuplicateTeamTerminal
    duplicate terminal replay error = <nil>, want conflict
FAIL
```

The failures prove that both full Team replay and exact recovery replay accepted
the duplicated facts. No production change had been made when this RED was
captured.
