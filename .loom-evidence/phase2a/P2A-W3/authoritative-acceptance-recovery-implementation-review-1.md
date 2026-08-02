# P2A-W3 Authoritative Acceptance/Recovery Implementation Review 1

**Date**: 2026-08-03  
**Reviewer**: independent read-only Reviewer  
**Source lock SHA-256**: `29facd9ea623685795131202281afb8fa6cf9ee33676b930096ebc235f1c5098`  
**Verdict**: `FAIL`

## Findings

- P0: none
- P1: one
- P2: none

### P1 — recovery exact replay does not validate the complete committed transaction

`matchExistingTeamRecovery` reconstructs a genuine
`rules.RecoveryDecision`, but only compares its digest, correlation and selected
input semantics. It does not verify:

- recovery Event ID or causation ID;
- full payload equality for action, `retry_at`, next-attempt identities,
  credits-after, fallback, approval and dependency state;
- the required downstream `TeamNodeAttemptScheduled` fact for retry/fallback;
- the required downstream `TeamExecutionTerminal` fact for terminal recovery;
- duplicate or partial recovery transactions.

`replayTeamExecution` validates structural plausibility, but does not bind all
those payload fields to `RecoveryDecisionDigest` or require the downstream fact.
A structurally valid partial history can therefore be accepted as an exact replay
and returned without a new clock read or CAS. Examples are a terminal recovery
Event without its terminal Event, or a retry recovery Event without its scheduled
attempt Event.

This violates Repair 3 section 2's full Event ID, payload and downstream-fact
requirement and the fail-closed Authority boundary. The repair must reconstruct
the entire expected recovery transaction and compare the exact recovery Event
plus exactly one expected downstream Event, including IDs, payloads, order,
correlation and causation.

## Independently verified sound

- Source-lock SHA, all nine file hashes and combined digest match.
- Contract chain and Contract Review 3 hashes match.
- Acceptance Authority timing and one-CAS batch are correct.
- Coordinator routing uses returned Authority state.
- The Rules-decision reflection check, zero-delay logical advancement,
  positive-delay behavior, prepared-intent separation and ignored proposal
  fields are correct.
- Focused normal and race tests pass.
- No schema expansion, second authority, hidden retry or P2A-W4 exists.

No source or evidence files were edited by the Reviewer. No live process,
staging or commit occurred.
