# S3-W2 Contract Amendment 3 Review 1

- Baseline: `c21a8f1`
- Date: `2026-07-26`

## Blocking finding

Separate status and capacity streams preserve accepted status sequencing, but
live CAS ordering is not reconstructable from lexicographically replayed
Events. Without persisting the exact observed Runtime status head, projection
cannot distinguish an offline status committed before a reservation from one
committed after the reservation linearized.

The same-stream incompatibility itself was confirmed. `git diff --check`
passed and the Reviewer edited no files.

VERDICT: FAIL
