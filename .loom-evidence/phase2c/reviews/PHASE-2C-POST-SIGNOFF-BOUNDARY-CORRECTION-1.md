# Phase 2C Post-signoff Boundary Correction 1

**Date**: 2026-08-09  
**Status**: `READY FOR EXACT-BYTE RE-REVIEW`  
**Scope**: evidence-path closure only; no acceptance metadata write is yet
authorized

Boundary Review 1 correctly found that the original boundary did not name its
review and final lock paths exactly. The original boundary remains immutable
failed history. This correction replaces only its evidence-path rules.

## Exact Post-A4 Evidence Set

Exactly these seven evidence paths may exist in addition to the sealed 445-file
A4 set:

1. `.loom-evidence/phase2c/reviews/PHASE-2C-PRODUCT-OWNER-SIGNOFF.md`
2. `.loom-evidence/phase2c/reviews/PHASE-2C-POST-SIGNOFF-ACCEPTANCE-BOUNDARY.md`
3. `.loom-evidence/phase2c/reviews/PHASE-2C-POST-SIGNOFF-BOUNDARY-REVIEW-1.md`
4. `.loom-evidence/phase2c/reviews/PHASE-2C-POST-SIGNOFF-BOUNDARY-CORRECTION-1.md`
5. `.loom-evidence/phase2c/reviews/PHASE-2C-POST-SIGNOFF-BOUNDARY-REREVIEW-2.md`
6. `.loom-evidence/phase2c/reviews/PHASE-2C-POST-SIGNOFF-FINAL-ACCEPTANCE-LOCK.json`
7. `.loom-evidence/phase2c/reviews/PHASE-2C-POST-SIGNOFF-FINAL-ACCEPTANCE-LOCK-REVIEW-1.md`

No eighth post-A4 evidence path is allowed. The final Phase 2C evidence set
must therefore contain exactly 452 files.

## Sequence And Hash Rules

1. Boundary Review 1 and this correction preserve the failed finding.
2. Exact-byte Boundary Re-review 2 must pass before any status metadata write.
3. After the three admitted status files change, the final acceptance lock must
   hash all evidence except itself and its exact post-lock review path.
4. The final lock must bind the SHA-256 and `PASS P0=P1=P2=0` verdict of
   Boundary Re-review 2.
5. The lock records itself as a self-referential unhashed entry and the exact
   final-lock review as the sole post-lock unhashed attestation.
6. Final Lock Review 1 must recompute the lock, every hashed path, all admitted
   status metadata, the unchanged source subset, delivery hashes, final 452-file
   evidence set, and zero staged paths.

The attestation file remains intentionally unhashed by the lock to avoid
recursive mutation; its exact path is nevertheless enumerated. No file may be
appended after that attestation.

## Acceptance Metadata Boundary

The four writes described in the original boundary remain unchanged:

- append one current acceptance reconciliation to `docs/CURRENT.md`;
- change ADR-0015 status from `proposed` to `accepted` and add one short
  sign-off evidence reference;
- change only ADR-0015's README status cell from `proposed` to `accepted`; and
- generate the exact final lock and exact review named above.

Contracts, product, tests, Journeys, existing evidence, decision content, and
authority remain frozen. This correction authorizes only its own exact-byte
re-review; it does not yet authorize the status writes, lock, acceptance claim,
staging, commit, push, merge, or publication.

