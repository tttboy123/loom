# P2A-W3 Independent Implementation Review 1

**Date**: 2026-08-01  
**Reviewer**: fresh independent read-only Reviewer  
**Source lock**: `source-lock.json` (all 29 digests independently matched)  
**Verdict**: `FAIL`  
**Live gate**: `LOCKED`

## P0

None.

## P1

1. The frozen `mission_execution/control` surface lists prepared decision and
   recovery actions, but the authoritative execution backend accepts only
   `cancel`. Product claims about prepared-decision routing therefore do not
   satisfy the single execution control contract.
2. The production `LoomLocalAppContractProbe` does not call
   `mission_execution`; existing Go fixture coverage does not close the required
   strict real-Swift-probe lifecycle.
3. Completed terminal mission flights remain in the bounded 64-entry in-memory
   registry forever, eventually causing false `busy` for unrelated missions.

## P2

1. Preflight has no issued/expiry field or bounded server-side lease, so an
   arbitrarily old digest can Start if the authoritative view is unchanged.

## Independently verified

- physical repository, branch and baseline identity match the contract;
- every source-lock digest matches;
- deterministic `internal/app`, `internal/api` and `internal/localipc` tests
  pass;
- no Provider, credential, daemon live attempt or live canary ran.

## Gate decision

The Candidate is not accepted. Codex, MiniMax and Pi live canaries remain
locked. All repairs must remain inside the unique W3 owned boundary and receive
a new source lock plus fresh independent Re-review.
