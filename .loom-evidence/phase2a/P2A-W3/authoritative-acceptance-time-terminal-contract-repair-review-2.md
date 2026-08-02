# P2A-W3 Authoritative Acceptance Time Terminal Contract Repair Review 2

**Date**: 2026-08-03  
**Reviewer**: independent read-only Contract Reviewer  
**Verdict**: `FAIL`  
**Repair 2 SHA-256**:
`277befe1c672714a6b19d31bdfff2d4244a0ee445689fed2786f873292cb82e5`

No P0 or P2 finding was reported. Repair 2 resolves the prepared Review
acceptance-intent ambiguity, but implementation remains unauthorized because a
P1 advancing-clock defect remains on the rejected path.

## P1 — recovery scheduling still uses caller time

After an authoritative rejected acceptance, the Coordinator currently creates
a Recovery Decision using the frozen mission-start
`TeamExecutionRequest.AuthoritativeTime`. `ScheduleTeamNodeRecovery` then reads
a fresh Work Authority clock value and requires exact equality with the caller
decision time.

With the required advancing clock, rejected acceptance can therefore commit
`WorkItemRejected` and `TeamNodeAcceptanceCommitted` and then fail recovery
scheduling with `ErrInvalidTeamRecovery`. The seven-file Repair 2 scope locks
`internal/work/team_execution_authority.go` out, so it cannot close the
contract's required rejected recovery routing.

The same P2A-W3 contract must either reopen Recovery Authority and make it own
the recovery decision time, or explicitly abandon automatic rejected recovery.
The latter contradicts the frozen vertical behavior and is not accepted.

## Gate result

No RED, implementation, live action, staging, commit or P2A-W4 is authorized.
The Reviewer edited no file and ran no test or process.
