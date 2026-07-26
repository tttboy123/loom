# S4-W1 Bounded Product Repair 1

Status: FROZEN

Date: 2026-07-26

Parent: `contract.md`

Trigger: fresh Implementation Review 1 `FAIL`

This is the first bounded product repair in the existing S4-W1 Candidate
lineage. It adds no WorkItem and changes no owned file list.

## Required repairs

1. Derive all four exact RuleSet streams from the immutable ActionContext.
   Request and resolution must transaction-consistently read and CAS those
   four streams plus WorkItem, Run, and Approval. Re-evaluation over every
   current non-empty scope stream must equal the submitted/stored Decision.
2. Exact retry must bind the complete committed command:
   - RuleSet digest/revision, correlation, command digest, actor, and
     authorization digest;
   - Approval request action/continuation/decision, requested time, and
     correlation; and
   - terminal decision, correlation, actor, and authorization digest.
   A presentation change is visible through the injected authorizer's
   authorization digest. Exact committed retry remains idempotent even after
   the original authorization window expires; divergent retry is a typed
   conflict.
3. Rebuild the approved resume Candidate after restart from immutable Journal
   facts: exact Approval terminal head, exact WorkItem resolution head, frozen
   empty Run head, and the exact revision/Event head or zero head for every
   derived RuleSet stream.
4. Make the injected port usable outside package `rules` with immutable
   request accessors and validated authorized-response constructors. Ordinary
   Authority callers still cannot submit an authorized response; only the
   injected CustomerAuthorizer return path is consumed.

## Regression proof

- omitted current higher-scope reject produces zero approval Events;
- divergent correlation and presentation retries conflict;
- exact retry after authorization expiry remains idempotent;
- reopened approved retry returns heads identical to the initial resolution;
- Authorizer request slices are copied and response constructors reproduce the
  exact binding; and
- concurrent request/claim and decision races remain one-winner.

All original S4-W1 exclusions and required gates remain unchanged.

VERDICT: FROZEN
