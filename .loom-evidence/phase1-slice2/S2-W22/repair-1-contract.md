# S2-W22 Repair 1 Contract

- Lineage: `S2-W22`
- Repair attempt: `1/3`
- Product changes authorized: none
- Test ownership: `internal/runtime/status_reconciliation_test.go`
- Evidence ownership: `.loom-evidence/phase1-slice2/S2-W22/`

## Required repair

1. Add or remove a second valid baseline entry and prove the unordered baseline
   set change alters both `BaselineDigest` and `CandidateDigest`.
2. With status unchanged and stable identity preserved, independently change
   display name, executable version, canonical capabilities, capacity,
   canonical model IDs, and source probe ID; prove each produces a valid
   zero-transition Candidate.
3. Re-run the complete S2-W22 strict matrix and obtain a fresh independent
   Repair 1 implementation review.

No production behavior, Event/write/probe/process surface, absence inference,
dependency, scope, or external action may change.

VERDICT: REPAIR_FROZEN
