# P2A-W3 Authoritative Acceptance Time Terminal Contract Review 1

**Date**: 2026-08-03  
**Reviewer**: fresh independent read-only Contract Reviewer  
**Contract SHA-256**:
`d43ea00569669effa642a3cb8e80ee5e0f5957fedb5799f7e3cceb964c6ffc11`  
**Verdict**: `FAIL`

## P1 — required API shape conflicts with exact owned scope

The contract says `TeamNodeAcceptanceInput` must no longer carry a
caller-created final `AcceptanceDecision`, but current production
`internal/app/local_product_decision.go` constructs and supplies that field and
is outside the exact owned list.

A safe bounded implementation may keep the field only as deprecated ignored
compatibility input. Work Authority must not require it to be valid and must
derive exactly one decision from contract/result/verifier at its own operation
instant. Event identity, payload, time, outcome and recovery routing must use
only the authority-derived decision. Tests must prove stale/mismatched caller
values cannot influence any authoritative result or idempotent replay.

Without that clarification, implementation would either violate scope or leave
caller decision authority ambiguous.

## P2 — evidence event label drift

The bound Result Review says `VerificationDecisionCommitted 0`; the actual
authority event name is `WorkItemVerificationCommitted`. The absence of
`TeamNodeAcceptanceCommitted` and `TeamExecutionTerminal` remains valid, so this
is wording drift rather than a product blocker. Contract repair must use the
actual event name.

## Safe portions

The Reviewer accepted the core design: Work Authority owns one UTC operation
instant, calls existing `verification.DecideAcceptance`, uses one CAS batch,
preserves authority/schema boundaries, requires advancing-clock RED and permits
no retry or W4.

**FINAL VERDICT**: `FAIL / BOUNDED CONTRACT REPAIR REQUIRED`
