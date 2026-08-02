# P2A-W3 Acceptance/Recovery Repair 4 Implementation Re-review 2

**Date**: 2026-08-03  
**Reviewer**: independent read-only Reviewer  
**Repair 4 source lock SHA-256**: `70e7b380f7eb40d1b978dee393c6f4ed21efe63e3df2afb091c7ee322f8c9954`  
**Verdict**: `FAIL`

## Findings

- P0: none
- P1: one
- P2: none

### P1 — exact matcher still accepts duplicated downstream facts

`matchExistingTeamRecovery` compares only the expected transaction prefix and
then returns success. It does not reject additional downstream facts after that
prefix.

Full Team replay does not close the gap:

- a second structurally plausible `TeamNodeAttemptScheduled` for the same node
  and attempt is appended again because replay has no attempt-identity
  uniqueness check;
- a second plausible `TeamExecutionTerminal` is accepted when it uses the first
  terminal Event as causation because replay does not reject an already-terminal
  Team.

Therefore these histories are incorrectly accepted as exact replay:

```text
recovery + exact scheduled + duplicate plausible scheduled
recovery + exact terminal + duplicate plausible terminal
```

Repair 4 already requires rejection of extra and duplicated downstream facts.
The same owned production/test pair is sufficient: add both regression tests,
reject duplicate attempt scheduling and repeated Team terminal facts during
replay, and preserve legitimate later progress after a retry.

## Independently verified sound

- Repair 4 lock, all nine file hashes, combined digest, contract, RED and
  verification hashes match.
- Only the authorized production/test pair changed from the parent lock.
- Shared transaction construction, committed-time decision reconstruction,
  exact field comparison, missing/mismatched downstream rejection and
  pre-clock replay ordering are correct.
- Focused/package normal and race tests pass.
- No schema, second authority, hidden retry, scope expansion or P2A-W4 exists.

No file was edited by the Reviewer. No live process, staging or commit occurred.
