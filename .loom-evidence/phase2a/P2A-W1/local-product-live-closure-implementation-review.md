# P2A-W1 Local Product Vertical Live Closure Implementation Review 1

**Date**: 2026-07-29  
**Reviewer**: fresh independent read-only Reviewer  
**Verdict**: `FAIL`  
**Live action**: none

## Finding

### P1 - activation record path was outside the frozen create allowlist

The frozen contract permits only:

```text
.loom-evidence/phase2a/P2A-W1/local-product-live-closure-activation-audit.md
```

The transaction instead required:

```text
.loom-evidence/phase2a/P2A-W1/local-product-live-closure-post-review-activation-audit.md
```

Therefore a contract-compliant activation record could not unlock the
transaction, while creating the transaction-expected record would exceed the
frozen evidence allowlist. The deterministic fixture did not bind the
activation path.

## Passing checks

The Reviewer independently reproduced:

- all 53 source-lock hashes and all four closure delta hashes;
- all 89 local Go production inputs and all 11 Swift/build inputs were bound;
- Candidate manifest, artifacts, modes, owner, strict signature, arm64
  architecture, non-zero UUID, canonical bundle digest, privacy, and
  toolchain;
- focused API/daemon/real-Go-to-Swift tests;
- full repository, full race, vet, module, format, transaction fixture, diff,
  and empty-staging gates;
- missing activation remained fail-closed at zero bootstrap/restart.

No file was edited by the Reviewer. No activation record, install, App launch,
service restart, Journal mutation, or live action occurred.

## Required repair

Within the already owned transaction/test/evidence boundary:

1. bind the transaction to the exact contract-allowed activation record;
2. add a deterministic assertion for that exact path;
3. rerun syntax, transaction fixture, production lock, source/Candidate,
   diff, staging, and relevant complete deterministic gates;
4. obtain fresh independent Implementation Re-review.

## Fresh independent Re-review 2

**Verdict**: `PASS`  
**Findings**: none

The Reviewer independently confirmed:

- the transaction now binds the exact contract-owned activation filename;
- the fixture asserts that path and rejects the obsolete unowned spelling;
- reversing only the path repair reproduces the Review 1 transaction bytes;
- removing only the assertion reproduces the Review 1 fixture bytes;
- current transaction and fixture hashes are
  `a23b3d3ca04040c42d8fdb2b891e945e44cf3ff7b2966cca354faf35da6c528a`
  and
  `6f7b8252d861561bc2beed3616fdbd3e8343bb5316497f8f03580796438d2e1d`;
- all source-lock, delta, production-input, Candidate, complete deterministic,
  privacy, diff, and staging gates remain valid;
- missing activation remains fail-closed with zero bootstrap, consumption,
  rollback, and restart.

The Reviewer performed no edit or live action. Re-review PASS does not itself
grant live authority.
