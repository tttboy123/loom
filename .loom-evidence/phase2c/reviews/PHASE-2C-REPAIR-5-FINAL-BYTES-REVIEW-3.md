# Phase 2C Repair 5 Final-bytes Review 3

**Date**: 2026-08-08  
**Reviewer**: Leibniz (`019fe10a-5a92-74a2-973c-54af0a87b6de`)  
**Mode**: independent final-byte read-only review  
**Verdict**: `FAIL`  
**Counts**: `P0=0`, `P1=1`, `P2=0`

## Finding

`P1`: Contract Re-review 2 binds the provisional lock and pre-status source
bytes, while the final lock points to that record as its PASS binding and says
reviewed contract bytes did not change after re-review. Current bytes differ:

| Path | Re-review 2 | Current at Review 3 |
|---|---|---|
| `repair-source-lock.json` | `ad3ff6c7...b486a` | `d7f22092...4bcd0` |
| `PHASE-2C-REPAIR-AMENDMENT.md` | `8323b6ae...8eee7` | `fb66dc6b...110ad` |
| `repair-candidate-boundary.md` | `957d07ca...83e74` | `13ab7eee...73be6` |
| `docs/CURRENT.md` | `2a5e855b...cfc49` | `e0a7a5b9...1e014` |

The PASS binding therefore does not authorize deterministic verification from
the current final bytes.

## Passed Checks

- Current lock SHA-256:
  `d7f220920d410e975c44759fa6f5d0cdbf5700bb20adf9b0f21d5f3ae5c4bcd0`.
- The 43-path set matches the boundary exactly with no missing or extra path.
- Recomputed ordered digest matches the lock:
  `c07fdfcc2ee0bfda9c7a73963292b89951751618011c5e38d1ba8a3173e90c4a`.
- All normative inputs match.
- Gate wording authorizes deterministic verification only, not a Journey or
  any WorkItem, Phase, ADR, or Product Owner acceptance.

The required remedy is a non-self-referential final review binding over the 43
source paths, normative inputs, ordered digest, and gate semantics, followed by
exactly one pre-authorized metadata-only update of the lock's review path/hash.
