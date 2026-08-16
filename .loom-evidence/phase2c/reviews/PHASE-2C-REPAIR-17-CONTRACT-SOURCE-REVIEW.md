# Phase 2C Repair 17 Contract + Source Review

**Result**: `FAIL`  
**Findings**: `P0=0`, `P1=0`, `P2=2`

## Findings

### P2 - Amendment header still reports the Repair 16 gate

`.loom-evidence/phase2c/contracts/PHASE-2C-REPAIR-AMENDMENT.md:3` still said
`Repair 16 status re-review 2 passed; replacement source lock pending`, while
section 25 made Repair 17 the active gate. This contradiction blocks replacement
source-lock authorization.

### P2 - Candidate Purpose still claims Repair 16 clean J1-J10 remains required

`.loom-evidence/phase2c/repair-candidate-boundary.md:39` still listed Repair 16
review, lock, matrix, signed Release, and clean J1-J10 as pending, while the
current Candidate tail recorded that Repair 16 passed lock/matrix/Release and
Repair 17 review was the gate. This contradiction blocks replacement source-lock
authorization.

## Source Review

No source findings. `ResumeProjectedMissions` skips reconstruction only for
terminal executions and the new narrow human-review predicate. Aggregate status
must be `running`, nodes non-empty, at least one node `ready_for_review`, and all
nodes either `succeeded` or `ready_for_review`. Active, pending, recovery, empty,
and unknown nonterminal states retain the existing resume or fail-closed path.
The causal restart test, table-driven predicate coverage, and existing exact
resume/unknown-status tests are adequate for this repair.

## Verdict

`FAIL`. Correct both stale status statements and obtain an exact-byte re-review
before generating a replacement source lock.
