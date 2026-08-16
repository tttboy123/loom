# Phase 2C Replacement Journey Attempt 021 - Failed

Attempt 021 is preserved and must not be promoted.

The Repair 18 source-locked binaries used main Journey UUID
`1703d05e-114a-45f3-a4f9-c72eec0d6104`. J1-J5 passed the authority boundary
and the explicit Mission reached `ready_for_review`: the primary Run succeeded,
the verifier terminated `cancelled/operator_cancelled`, and the Mission remained
quiescent for human review. The canonical J6 fixture used Journey UUID
`ce770eff-bf73-470a-8606-7b2594207d2c` and appended exactly one approved
`ApprovalDecided` plus one assigned `WorkItemApprovalResolved`. An earlier
rejected decision rehearsal is preserved separately and excluded from J6.

J7 then failed before the controlled offline Event could be appended. The
runtime-status fixture's `authoritative_time` had been frozen when the attempt
directories were created, before the daemon's first Runtime discovery. The
fixture correctly failed closed as stale and `loomd` returned
`daemon unavailable: build_state`. Read-only verification showed no status Event
was appended, SQLite integrity remained `ok`, and main Event identity remained
`84/84/84`.

This is a journey-harness timestamp error, not product-source drift. Attempt 022
must write the controlled offline authoritative time immediately before the
offline restart and after the projected online discovery time. J8-J10 were not
run. All attempt processes were stopped; the resident demo daemon was only
observed and not modified. No Phase or ADR acceptance claim is made.
