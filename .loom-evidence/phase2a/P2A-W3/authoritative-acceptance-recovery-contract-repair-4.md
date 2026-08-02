# P2A-W3 Authoritative Acceptance/Recovery Contract Repair 4

**Date**: 2026-08-03  
**Parent source lock SHA-256**: `29facd9ea623685795131202281afb8fa6cf9ee33676b930096ebc235f1c5098`  
**Implementation Review 1**: `FAIL`  
**Status**: `FROZEN — causal RED required before production repair`

## Purpose

Close the single P1 finding from Implementation Review 1 without expanding the
accepted P2A-W3 contract, owned-file set, Event schema or authority topology.
Recovery idempotency may return success before a new Authority clock read only
when the already-committed Journal history is the exact complete transaction
that the same raw recovery intent and the recorded decision time would produce.

## Exact owned boundary

Production repair is limited to:

- `internal/work/team_execution_authority.go`

Tests are limited to:

- `internal/work/team_execution_authority_test.go`

The other seven files from the accepted nine-file boundary remain locked unless
a test-only compilation adjustment is strictly necessary. No additional source
file is authorized.

## Required RED

Before production code changes, tests must prove that current replay incorrectly
accepts at least these incomplete or conflicting histories:

1. retry/fallback recovery Event without its exact
   `TeamNodeAttemptScheduled` downstream Event;
2. terminal recovery Event without its exact `TeamExecutionTerminal` downstream
   Event;
3. where practical in the same fixtures, a mismatched or duplicate downstream
   Event must be rejected rather than treated as idempotent success.

The RED must fail because `ScheduleTeamNodeRecovery` returns success for a
history that is not the complete expected transaction.

## Required implementation

For an existing recovery Event matching the logical node and attempt, Work
Authority must:

1. parse its committed UTC `decision_time`;
2. invoke the existing recovery policy with the replayed immutable semantics and
   that exact time;
3. require a genuine concrete `rules.RecoveryDecision` and validate all decision
   semantics exactly as Repair 3 already requires;
4. reconstruct the exact expected recovery Event, including Event ID, stream,
   sequence, type, schema version, time, correlation, causation and byte-exact
   canonical payload;
5. reconstruct exactly one required downstream Event:
   - `TeamNodeAttemptScheduled` for `retry` or `fallback`; or
   - `TeamExecutionTerminal` for `degraded`, `blocked` or `human_required` when
     the recovered Team is terminal;
6. compare downstream Event ID, stream, sequence, type, schema version, time,
   correlation, causation and byte-exact canonical payload;
7. reject missing, extra, duplicated, reordered or conflicting facts with the
   existing fail-closed recovery/conflict error;
8. only after this exact complete match return idempotent success without a new
   clock read or CAS.

The reconstruction must use the same production helpers as new recovery writes
so that the new-write and replay paths cannot drift. The single
`AppendBatchIfStreamHeads` write authority and one recovery transaction remain
unchanged.

## Invariants

- Proposal `Decision` remains ignored compatibility input.
- Rules stays the sole recovery-policy implementation; Work remains the sole
  recovery Event writer.
- No Event, IPC or Swift schema change.
- No hidden retry or auto-repair of partial Journal history.
- No second Journal, StateWriter, Projection, Scheduler or authority.
- No P2A-W4.
- No live canary until a new source lock and independent Implementation Review
  PASS.

## Exit gates

- Causal RED recorded.
- New retry and terminal incomplete-history tests GREEN.
- Existing exact replay remains clock-independent and idempotent.
- Focused normal and race tests pass.
- Full Go normal/race/vet and Swift test/build matrix passes.
- New source lock matches the exact nine-file candidate.
- Independent Implementation Review PASS.
