# S2-W38 Fresh Read-Only Problem Analysis 1

- WorkItem: `S2-W38`
- Trigger: second same-class stale-projection contract failure
- Frozen head: `9175f9426f1e153b05fbc6af6e2fbf7b27103b6e`
- Analyst: fresh independent read-only Problem Analyst
- Files changed by analyst: none

## Root cause

The defect is in the application-composition/read-model lifecycle boundary,
not Journal, projection replay, planning, trigger, or writer behavior.

S2-W35 reads `readModel.Snapshot()` and accepted committers append authoritative
Events. S2-W36 stores the exact projection pointer privately. S2-W37 awaits one
trigger and calls that observer, but no accepted layer refreshes the projection
after a successful write. Earlier real-chain tests rebuilt explicitly between
discovery and status phases, masking the missing product synchronization.

Repair 1 found the lifecycle position but introduced a second projection
parameter. That could refresh projection A while the observer read projection
B, preserving the same stale-view failure.

## Minimum Repair 2

Remove the separate projection parameter. S2-W38 must:

1. reject nil context, nil/typed-nil trigger, nil observer, and a non-nil
   observer with `observer.readModel == nil`;
2. capture the actual immutable binding once as
   `readModel := observer.readModel`;
3. use one stack-local unexported trigger decorator ordered exactly as
   underlying await → context check → captured projection rebuild;
4. call accepted S2-W37 exactly once;
5. return five zero outputs for every S2-W37 error;
6. only after S2-W37 success, rebuild the same captured projection once; and
7. return the exact successful path-specific tuple plus the exact error only
   when that post-success rebuild fails.

No separate context check should follow a successful post-refresh. Rebuild
already checks cancellation before source access, during replay, and before
swap; a later check would turn a completed observation and refresh into an
ambiguous error without preventing a cancellation race.

## Required discriminating proof

- Zero-value observer rejection without panic.
- Underlying trigger error/cancellation, deterministic pre-refresh failure,
  complete downstream failure propagation, and no post-refresh/retry.
- Post-success refresh failure retains the exact successful tuple, committed
  Event, and previous projection snapshot.
- One same-observer real SQLite chain with no test-side inter-call rebuild:
  seeded discovery sequence 1, mixed discovery sequence 2, then status-only
  sequence 3.
- Static one-await, one-S2-W37, two-Rebuild, no-loop/no-lower-layer proof.

## Remaining explicit limits

An error returned inside accepted S2-W37 does not prove that no Event committed:
accepted lower layers may commit and then observe cancellation while returning
five zero outputs. S2-W38 cannot recover discarded values and must never imply
blind retry is safe.

The repaired one-shot provides sequential successful-call freshness, not
linearizability across concurrent calls. Later scheduling must prevent overlap
or obtain a separate serialization contract.

No new ADR, StateWriter, Event, exported accessor, retry, scheduler, daemon,
recurrence, or activation authority is needed.

VERDICT: ANALYSIS_COMPLETE
