# P2A-W3 Authoritative Acceptance Time Terminal Contract Repair 2

**Date**: 2026-08-03  
**Status**: `FROZEN / PENDING INDEPENDENT REPAIR REVIEW`  
**Parent contract SHA-256**:
`d43ea00569669effa642a3cb8e80ee5e0f5957fedb5799f7e3cceb964c6ffc11`  
**Repair 1 SHA-256**:
`b7281445dc10e9b8c26850b6c329f68fe152e727eaeb09044a7bee38794de7f2`  
**Repair Review 1**: bound by the file hash recorded after this freeze  
**New WorkItem**: none; `P2A-W4` does not exist

This Repair 2 resolves only Repair Review 1's prepared Review Gate ambiguity.
The parent contract and Repair 1 remain binding except where this document
explicitly replaces their exact owned-file list.

## 1. Stable acceptance-intent binding

For `kind=review`, the existing JSON field
`MissionDecisionSheet.DecisionDigest` is a command replay/binding digest for a
prepared acceptance **intent**. It is not the final
`AcceptanceDecision.Digest`, because that authoritative digest includes the
later Work Authority operation time.

The prepared Review Gate must derive one canonical, versioned,
time-independent acceptance-intent digest from the immutable semantics the
user is authorizing:

- Team instance, plan, logical node, attempt and current claim generation;
- source WorkItem/Run/Evidence identity and source receipt/output-summary
  digests;
- acceptance contract and deterministic verification-result digests;
- optional independent Verifier Candidate and receipt digests;
- expected acceptance kind (`accepted` or `rejected`);
- recovery policy digest, max-attempt and remaining-credit boundary; and
- the prepared action bound to that expected kind.

It must exclude wall-clock time and the timestamp-bearing final acceptance
decision digest. Existing mission/view/decision IDs, command digest equality,
stale-view rejection and one-shot command behavior remain unchanged.

`validReviewAcceptanceBinding` must validate all of the above immutable
semantics and compare the sheet digest with the recomputed acceptance-intent
digest. It may validate the deprecated caller decision only as a prepared UI
proposal whose kind and immutable semantic digests agree; Work Authority must
still neither require nor read that field. A malformed, stale-time or
different-time proposal cannot alter the intent digest, and any kind or
semantic mismatch remains fail-closed before authority dispatch.

The final authoritative `AcceptanceDecision.Digest` recorded in Journal facts
is derived later by Work Authority from the same contract/result/Verifier
semantics plus its single operation instant. Tests must prove the prepared
intent and final authority decision have identical immutable semantics and
expected kind while intentionally having distinct digest roles. UI/command
code must never present the intent digest as the final Journal decision digest.

This is a semantic normalization of the existing strict field, not an IPC
schema version change, decoder relaxation or second authority.

## 2. Direct Coordinator and prepared Review paths

- `internal/app/team_execution.go` sends zero compatibility `Decision`; the
  Work Authority path does not depend on prepared UI state.
- `internal/app/local_product_decision.go` may retain a proposal decision only
  to prepare and validate the visible review action. Its operation time is not
  forwarded as authority time and its digest is not the sheet binding.
- both paths converge on the same Work Authority method and therefore on the
  same one-clock, one-CAS acceptance/terminal transaction.
- exact replay is recognized from committed authority facts and immutable
  lineage/semantic inputs before obtaining a new operation time.

## 3. Mandatory tests added by this repair

In addition to the parent RED and matrices, focused prepared Review tests must
prove:

1. the same immutable acceptance intent yields the same sheet/command digest
   across different proposal times;
2. a different kind, contract/result/Verifier/source receipt, claim generation,
   recovery policy or action is rejected before authority dispatch;
3. a valid prepared command executed later commits the Authority-derived time
   and final digest, closes the Team, and refreshes the view exactly once;
4. the sheet/command intent digest and final Journal decision digest are not
   conflated, while their immutable semantics and kind agree;
5. stale-view replay and concurrent command one-winner behavior remain intact;
6. no failure appends partial acceptance or terminal facts.

The direct Work Authority compatibility-mutation tests from Repair 1 remain
mandatory and prove that caller `Decision` values cannot affect authoritative
events.

## 4. Repaired exact owned files

The exact owned set is now seven files:

```text
internal/work/verification_authority.go
internal/work/verification_authority_test.go
internal/app/team_execution.go
internal/app/team_execution_test.go
internal/app/local_product_decision.go
internal/app/local_product_decision_test.go
cmd/loomd/product_daemon_test.go
```

No other source file is reopened. In particular, verification policy, Event
schemas, Journal, Projection, IPC structs/decoders, Swift, daemon assembly,
Supervisor, Runtime/Provider and native UI remain locked.

## 5. Gates

Repair Review 1 remains a historical FAIL. RED or source implementation may
begin only after a fresh independent Reviewer confirms that the parent
contract plus Repairs 1 and 2 form one complete, unambiguous boundary. Contract
freeze/review does not authorize staging, commit, live execution or P2A-W4.
