# S3-W5 Contract Amendment 3 Review 1

Reviewer: independent read-only contract reviewer

Date: 2026-07-26

## Round 1 findings

1. `FinalizeAttemptCapture` incorrectly required a child terminal result Frame,
   but accepted Supervisor timeout, cancellation, workspace/source,
   adapter/protocol, observer, and cleanup failure paths can commit a failed or
   cancelled Run without such a Frame.
2. Recovery was not idempotent across a crash immediately after old-Grant
   revocation because only an active-Grant accessor was proposed.
3. `TeamNodeAttemptRebound` lacked an exact payload, deterministic Event ID,
   causation, and replay/Projection transition.

Round 1 verdict: FAIL.

## Candidate repair

The contract now:

- permits accepted Supervisor-generated failed/cancelled terminal finalization
  without a child result, while still requiring success/result reconciliation;
- exposes exact capture state and the latest active-or-revoked Grant per Run,
  including explicit absent/already-revoked rules;
- closes the post-Claim/pre-Team-rebound restart gap; and
- freezes the complete rebound payload, Run reference, Event ID inputs,
  causation, transition preconditions, and state update.

## Round 2 finding

The contract did not define the legitimate crash window after Journal dispatch
but before filesystem capture creation. Missing capture and present zero-Frame
capture were distinguishable, but recovery behavior for the missing state was
unspecified.

Round 2 verdict: FAIL.

## Candidate repair

The contract now fixes new-dispatch order as Journal dispatch, durable capture
begin, Grant issue, then execution. A missing capture is recoverable only for
the exact dispatch-committed claimed Run with no Grant or later Run activity;
recovery first creates the old-binding empty capture. Missing capture with any
Grant, running/terminal Run, authorization, or later generation conflicts.

## Round 3 findings

1. Team rebound and filesystem capture rebound were incorrectly described as
   atomic even though they cross the Journal/filesystem transaction boundary.
2. The absent-capture repair was a permitted non-authoritative filesystem
   mutation before lease expiry, but the following rule ambiguously prohibited
   all mutation.

Round 3 verdict: FAIL.

## Candidate repair

The exact cross-store order is now Run reclaim Journal CAS, empty capture
filesystem rebind, Team rebound Journal CAS, new Grant, execution. Capture
rebind is exact-retry idempotent, and the only accepted split state is
Run/capture at N+1 while Team remains at N. Team ahead of capture or any wider
divergence conflicts. The pre-expiry rule now explicitly permits only the
bounded absent-capture repair and forbids Journal/Grant/Run/Team/execution
mutation.

## Round 4 result

No remaining findings. The revision closes the dispatch/capture/Grant order,
pre-expiry filesystem-only repair, and cross-store rebound split-state
boundaries without adding a second authority or checkpoint.

Round 4 verdict: PASS.

VERDICT: PASS
