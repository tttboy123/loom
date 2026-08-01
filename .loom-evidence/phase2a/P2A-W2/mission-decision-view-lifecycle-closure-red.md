# P2A-W2 Final Mission Decision View Lifecycle Closure — Mandatory RED

**Date**: 2026-08-01

**Contract**: `mission-decision-view-lifecycle-closure-reopen.md`

**Production source changed**: no

## RED 1 — successful view advance

Command:

```text
go test -count=1 ./internal/app \
  -run TestPreparedMissionDecisionPublishesCurrentViewAfterJournalAdvance
```

Observed failure:

```text
prepared command view =
9020e40f9a94902c6c05442c851ad13e4cc7f7de444e0315f4e1b634d76b6b39

authoritative view =
55c0cc9b93febcfa4fd2ab957d673441fc9fadf2b711d8a1ddfc4a549bcd2885
```

The test uses a real Journal-backed approval fixture, appends a later Work
fact, rebuilds the real Projection and proves the registry still returns the
pre-advance command.

## RED 2 — projection-failure stale pair

Command:

```text
go test -count=1 ./internal/api \
  -run TestLocalProductReadServicePreservesPreparedCommandsWithStaleView
```

Observed failure:

```text
current prepared decision view =
40c875c9cafe94e521cc0652b9f777027c07d51df743f226885bed6d4139376e

stale prepared decision view =
bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
```

The service correctly retains the prior GlobalReadView on Projection failure,
but calls the command source again and combines that stale view with different
commands.

## Causal conclusion

The frozen contract is necessary and correctly scoped. The current Candidate
lacks both halves of one coherent lifecycle transaction:

- a successful snapshot cannot bind still-prepared commands to its exact
  authoritative view; and
- a failed refresh cannot preserve the previously published view/command pair.

Production implementation is now eligible only inside the exact owned files.
Live execution remains locked.
