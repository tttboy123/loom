# P2A-W1 Local Product Vertical Live Closure Contract Review

**Date**: 2026-07-29  
**Review mode**: fresh, independent, read-only  
**Verdict**: `FAIL`  
**Live authority**: none

## Contract Review 1 findings

1. `P1`: the contract required canonicalizing
   `observed_capabilities`, but Mandatory RED forced only the historical
   `model_ids:null` path. Projection can preserve nil for either collection,
   so the sibling failure class was not executable through the authoritative
   Go path.
2. `P1`: Candidate materialization referred to current reviewed P2A-W1/UI
   source without binding the broad dirty, uncommitted worktree inputs by exact
   path and hash. Output hashes alone could not prove which source entered the
   Candidate.
3. `P1`: deterministic verification required mutating `go mod tidy` while the
   replacement explicitly did not own the already-modified module lock.

The Reviewer confirmed that the causal fix belongs at the shared
`LocalProductReadService` wire boundary, strict Swift should remain strict,
historical `model_ids:null` can be supported without Journal mutation, and the
one-bootstrap/rollback/P2A-W2 gates are otherwise directionally correct.

`VERDICT: FAIL`

## Same-contract repair

The frozen contract now:

- adds exact Go-path RED/GREEN for nil `observed_capabilities`;
- adds a base-commit plus exact path/SHA source lock, four-file closure delta
  allowlist, generated/private/unrelated exclusions, and Candidate manifest
  derivation rule;
- changes module verification to read-only `go mod tidy -diff` and stops on
  any lock drift.

Fresh independent Contract Re-review is required before RED or product edits.

## Contract Re-review 2

**Review mode**: fresh, independent, read-only  
**Verdict**: `PASS`

No findings.

The Reviewer independently verified:

- the authoritative Go-path nil/null coverage now includes both sibling
  Runtime collections;
- source-lock SHA-256 matches the contract, base commit exists, all 53 locked
  inputs match, and the exact four delta paths do not overlap the lock;
- Candidate derivation is limited to source lock plus final delta hashes;
- `go mod tidy -diff` is read-only and stops on module-lock drift;
- strict Swift, Journal, Projection, StateWriter, one-bootstrap/no-retry,
  rollback, activation, commit, and P2A-W2 boundaries remain closed.

`VERDICT: PASS`
