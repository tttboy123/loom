# S2-W32 Implementation Review 1

- Reviewer: fresh independent read-only Implementation Reviewer
- Reviewed head: `b00f8d8`
- Contract SHA-256:
  `b6c594213350e7a0c1d5bf1dbf5bc7de429d02c8cc04833efbba6f49625ea77b`
- Product SHA-256:
  `f50a6d5adb7940dfa44578c40d6956ce15bfe4f952dec5fc9158f05cfd9e9c97`
- Test SHA-256:
  `c60cbd01dc3a5c7414a7fdab68285429770c80a415fe22a3e7bb5859a203018c`

## Finding

`FAIL`: `Planned()` is an exposed Candidate fact, but the canonical versioned
digest payload and per-fact digest-sensitivity table omit it. Successful
Candidates currently always set `planned=true`; that does not satisfy the
non-hollow contract requiring every exposed semantic fact to enter the digest.

All other behavior and proof passed review: discovery-priority classification,
complete inventory comparison, current-only/absence handling, status and mixed
precedence, exact validation/error propagation, context checks, mutation and
map-order isolation, and the pure no-writer/no-Journal/no-scheduler/no-daemon/
no-activation/no-Slice-3 boundary.

The Reviewer independently passed the focused, app, impact,
focused-race-50, repository, repository-race, vet, formatting, diff, and ADR
index checks.

VERDICT: FAIL
