# P2A-W3 Authoritative Acceptance and Recovery Time Contract Repair 3

**Date**: 2026-08-03  
**Status**: `FROZEN / PENDING INDEPENDENT REPAIR REVIEW`  
**Parent contract SHA-256**:
`d43ea00569669effa642a3cb8e80ee5e0f5957fedb5799f7e3cceb964c6ffc11`  
**Repair 1 SHA-256**:
`b7281445dc10e9b8c26850b6c329f68fe152e727eaeb09044a7bee38794de7f2`  
**Repair 2 SHA-256**:
`277befe1c672714a6b19d31bdfff2d4244a0ee445689fed2786f873292cb82e5`  
**New WorkItem**: none; this remains the unique P2A-W3

This Repair 3 closes Repair Review 2's rejected-recovery finding. It extends
the same authority-owned operation-time rule from acceptance to recovery; it
does not introduce a scheduler, retry loop, new policy or second authority.

## 1. Recovery Authority owns the decision time

`ScheduleTeamNodeRecovery` must no longer treat a caller-created
`rules.RecoveryDecision` as time authority. Its input must carry the immutable
request semantics required by existing `rules.DecideRecovery`:

- Team/plan/node/attempt and source agent/runtime/Evidence/output-summary
  lineage;
- the already verified `verification.Classification`;
- the exact versioned `rules.RecoveryPolicy`;
- the command correlation ID; and
- optionally a deprecated proposal decision used only by the prepared UI
  compatibility seam.

Work Authority derives from the replayed Team record, rather than trusting the
caller, the exact prior classifications, remaining credits, fallback-consumed
state, trigger and authoritative acceptance-decision digest. For a new recovery
transaction it must:

1. validate the immutable request, Team/node/attempt/receipt/classification and
   policy binding before mutation;
2. obtain exactly one UTC operation instant from its injected clock;
3. call the existing pure `rules.DecideRecovery` itself with the reconstructed
   complete input and that instant;
4. use only that derived decision and instant for recovery Event ID/payload,
   retry time, next-attempt scheduling, recovery/terminal aggregation and CAS;
5. preserve the existing no-hidden-retry, attempt-credit, fallback,
   approval-required, generation and max-attempt rules.

Deleting only the time equality check, trusting caller retry time/action, using
mission-start time, tolerating clock drift or duplicating recovery policy in
Work is forbidden.

## 2. Recovery exact replay before clock

An exact previously committed recovery must be matched before obtaining a new
clock value. Matching reconstructs the existing decision using its committed
decision time plus the same immutable request and replayed Team semantics. It
must validate the full Event ID/payload and downstream scheduled-attempt or
terminal facts. A semantic, lineage, policy, classification, generation,
acceptance-digest or action mismatch remains a typed conflict.

The deprecated caller proposal is ignored by Work Authority. Zero, stale-time,
different-time, malformed or mismatched proposal values must not alter direct
authority output under identical immutable input and authority clock.

## 3. Coordinator routing and prepared Recovery intent

The Coordinator passes raw immutable recovery semantics and does not call
`rules.DecideRecovery` for final authority. It routes from the Work Authority
returned Team record. `RecoveryNone`/human-required behavior remains explicit;
there is no infinite or implicit resubmission.

The prepared Recovery Gate mirrors Repair 2's Review Gate rule:

- the existing sheet/command `DecisionDigest` binds a canonical, versioned,
  time-independent recovery-intent digest;
- the intent covers immutable request semantics, expected action, policy,
  trigger, authoritative acceptance digest, credits/fallback boundary and
  prepared action;
- it excludes wall-clock, retry-at and final timestamp-bearing Recovery
  Decision digest;
- a proposal may be used to display/validate the expected action, but is not
  sent as time authority and cannot affect Authority output;
- final Journal facts record the Authority-derived Recovery Decision digest and
  time. The UI/command intent digest must never be presented as that final
  digest.

No JSON field, schema version, strict decoder or action vocabulary changes.

## 4. Mandatory rejected-path RED and proofs

With a successive-call advancing UTC clock, the pre-repair rejected path must
causally prove the current failure after node acceptance and before recovery.
After repair the same path must prove:

- one rejected WorkItem verification/outcome and node acceptance;
- one Authority-derived recovery decision with a later operation instant;
- correct retry/degraded/blocked/human-required outcome from the existing
  Recovery Policy, with no invented fallback;
- exact replay adds zero Events even after the clock advances;
- concurrent identical recovery has one CAS winner and one idempotent result;
- incompatible input conflicts, and injected clock/CAS failures append no
  partial recovery or next-attempt facts;
- stale/malformed proposal mutation cannot change Event bytes;
- prepared Recovery intent is stable across proposal times and binds exact
  semantics/action while the final Journal digest has its distinct authority
  role.

The accepted terminal advancing-clock vertical proof and all Repair 1/2 tests
remain mandatory.

## 5. Final exact owned files

The bounded owned set is now nine files:

```text
internal/work/verification_authority.go
internal/work/verification_authority_test.go
internal/work/team_execution_authority.go
internal/work/team_execution_authority_test.go
internal/app/team_execution.go
internal/app/team_execution_test.go
internal/app/local_product_decision.go
internal/app/local_product_decision_test.go
cmd/loomd/product_daemon_test.go
```

No Rules implementation/test, Event schema, Journal, Projection, IPC/Swift,
daemon construction, Supervisor, Runtime/Provider or native UI file is reopened.

## 6. Gates

Repair Review 2 remains a historical FAIL. RED and implementation may begin
only after a fresh independent Reviewer confirms that the parent and all three
Repairs form one authority-safe, exact and testable contract. This freeze does
not authorize live execution, staging, commit or a new WorkItem.
