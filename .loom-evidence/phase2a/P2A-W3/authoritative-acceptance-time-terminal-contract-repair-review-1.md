# P2A-W3 Authoritative Acceptance Time Terminal Contract Repair Review 1

**Date**: 2026-08-03  
**Reviewer**: independent read-only Contract Reviewer  
**Verdict**: `FAIL`  
**Parent contract SHA-256**:
`d43ea00569669effa642a3cb8e80ee5e0f5957fedb5799f7e3cceb964c6ffc11`  
**Contract Review 1 SHA-256**:
`8ad39aa99ea6617c491d90a7ab78b76736f43e4532b65ea962d9ce6c8b2d3bf9`  
**Repair 1 SHA-256**:
`b7281445dc10e9b8c26850b6c329f68fe152e727eaeb09044a7bee38794de7f2`

The Reviewer found no P0 issue and confirmed that Repair 1 corrected the
historical Event label to `WorkItemVerificationCommitted`. Implementation is
not authorized because one P1 contract ambiguity remains.

## P1 — prepared Review Gate still binds the caller decision digest

Repair 1 says `internal/app/local_product_decision.go` may remain locked while
Work Authority ignores the caller-created `TeamNodeAcceptanceInput.Decision`.
That is not sufficient. The locked prepared Review Gate currently:

- places `reviewInput.Decision.Digest()` in the user-visible and command-bound
  `MissionDecisionSheet.DecisionDigest`;
- requires the prepared input decision digest to equal that sheet digest; and
- requires the submitted command digest to equal the sheet digest.

If Work Authority later derives a fresh decision at its operation instant, its
authoritative digest necessarily differs because decision time participates in
the digest. The frozen scope therefore permits the user/command to authorize
one digest while the Journal records another.

The repair must either prove that binding is safely non-authoritative or reopen
the exact prepared Review Gate source/tests and replace the timestamp-bearing
binding with an explicit, time-independent acceptance-intent binding. Silent
digest divergence is not acceptable.

## Gate result

- source implementation: not authorized;
- live action: not authorized;
- staging or commit: not authorized;
- new WorkItem or `P2A-W4`: forbidden.

No source was edited, no test or process was run, and no live lineage was
started by the Reviewer.
