# P2A-W3 Pi Terminal Aggregation Time Diagnosis

**Date**: 2026-08-03  
**Scope**: consumed Pi-005 result plus current locked source; read-only diagnosis  
**Live classification**:
`product_defect_terminal_aggregation_absent_after_verifier_evidence`  
**Status**: `STRONG CAUSAL HYPOTHESIS / RED REQUIRED BEFORE REPAIR`

## Observed authority boundary

Pi-005 successfully committed the source and independent Verifier terminal
Runs, Evidence and capacity/Grant closure. The last fact is Verifier
`EvidenceSubmitted`. No acceptance-decision, node-acceptance or Team-terminal
fact follows. The native product therefore remains `Ready For Review` rather
than canonical terminal.

## Source chain aligned with the stop point

The current product source has one live-time invariant that cannot generally
hold:

1. `BuiltInMissionExecutionCompiler.CompileMissionExecution` calls
   `compiler.now()` before execution and stores that instant in
   `TeamExecutionRequest.AuthoritativeTime`.
2. Source execution and independent verification run after that instant.
3. `TeamCoordinator.commitTeamAttemptReceipt` calls
   `verification.DecideAcceptance` using the original request instant as
   `DecisionTime`.
4. `work.Authority.CommitTeamNodeAcceptance` then calls its own
   `operationTime()` at acceptance commit and requires exact `time.Time.Equal`
   with the earlier decision instant.
5. On inequality it returns `ErrInvalidWorkItemAcceptance` before creating
   `WorkItemVerificationCommitted`, `TeamNodeAcceptanceCommitted` or
   `TeamExecutionTerminal`.

Production composition gives Compiler and Work Authority the same real UTC
clock function, but they call it at different wall-clock instants. Existing
deterministic tests commonly inject a clock that always returns one fixed
instant, so they do not exercise this elapsed-time boundary.

This chain exactly matches the first missing event boundary, but the retained
live artifacts do not record the background flight error value. Historical
result evidence must therefore keep the narrower accepted classification. A
causal RED using a monotonic advancing injected clock is required before any
behavior change.

## Required repair shape

The repair must not merely relax the equality check or trust an Agent-supplied
timestamp. Work Authority must remain the source of the acceptance decision's
authoritative commit time. A reviewed vertical contract should require:

- Work Authority derives or binds the acceptance decision at one authority
  operation instant and uses that same instant for decision digest and all
  acceptance/terminal Events;
- Coordinator provides deterministic result, independent Verifier Candidate,
  receipts and policy inputs, not independent time authority;
- idempotent replay and CAS conflict behavior remain exact;
- an advancing-clock RED crosses source Run, Verifier Run, Evidence,
  acceptance and Team terminal through the real Coordinator;
- a product-level background failure cannot remain silently presented as an
  indefinitely successful `Ready For Review` flight;
- no new Journal, StateWriter, Projection, scheduler, queue or P2A-W4 appears.

No production/test source, live process, retry, walkthrough, staging or commit
is authorized by this diagnosis.
