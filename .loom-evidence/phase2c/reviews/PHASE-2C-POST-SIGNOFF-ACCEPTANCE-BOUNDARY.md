# Phase 2C Post-signoff Acceptance Boundary

**Date**: 2026-08-09  
**Status**: `FROZEN FOR INDEPENDENT REVIEW`  
**Purpose**: reconcile accepted status without changing the reviewed product
Candidate, contracts, or Journey results

## Authority

`PHASE-2C-PRODUCT-OWNER-SIGNOFF.md` preserves the Product Owner's explicit
acceptance of Phase 2C, P2C-W1/W2/W3, and ADR-0015. The A4 evidence-lock review
already authorized the sign-off request and returned `P0=P1=P2=0`.

## Exact Write Boundary

After independent review of this boundary, the Controller may make only these
acceptance metadata changes:

1. Append one final acceptance section to `docs/CURRENT.md`.
2. Change ADR-0015's status token from `proposed` to `accepted` and add one
   short acceptance-evidence reference; its decision, alternatives,
   consequences, and date remain unchanged.
3. Change only ADR-0015's status cell from `proposed` to `accepted` in
   `docs/adr/README.md`.
4. Append a post-signoff final acceptance lock and its exact independent review
   under `.loom-evidence/phase2c/reviews/`.

The Product Owner sign-off and this boundary are the only pre-lock evidence
additions admitted after A4.

## Frozen Exclusions

- No product, test, fixture, script, build, app-bundle, daemon, IPC, Runtime,
  Provider, Journal, Projection, Scheduler, policy, Grant, Evidence, or
  authority byte may change.
- The P2C-W1/W2/W3 contracts, combined Exit Contract, Repair Amendment,
  Candidate Boundary, Journey Manifest, deterministic verification, source
  lock, Attempts 001-023, failed reviews, corrections, final results, A4 lock,
  and A4 review remain byte-identical.
- Historical `PARTIAL`, failed-attempt, pending-gate, and superseded-lock text
  remains historical evidence. The new `docs/CURRENT.md` EOF section is the
  current reconciliation record and must not rewrite those entries.
- No staging, commit, push, merge, publication, notarization, installation into
  a system directory, or activation is authorized by this boundary.

## Acceptance Lock Requirements

The post-signoff lock must:

- bind repository, branch, baseline HEAD, zero staged paths, Product Owner
  sign-off, and this reviewed boundary;
- bind the prior Repair 20 source lock and A4 final evidence lock/review;
- prove the 50 unchanged Repair 20 paths still match and isolate
  `docs/CURRENT.md` as the only changed Repair 20 source path;
- enumerate and hash the two ADR status files as newly admitted acceptance
  metadata;
- enumerate every Phase 2C acceptance evidence path, with explicit
  self/attestation handling;
- bind the durable local delivery executable and ZIP hashes without importing
  those artifacts into the repository; and
- receive an independent exact-byte review with `P0=P1=P2=0` before the goal
  is marked complete.

The final review must confirm that acceptance metadata truthfully reflects the
Product Owner decision and that no implementation or authority claim widened.

