# S2-W20 Repair 1 Contract

- Lineage: `S2-W20`
- Repair attempt: `1/3`
- Product ownership: `internal/state/runtime_discovery_writer.go`
- Test ownership: `internal/state/runtime_discovery_writer_test.go`
- Evidence ownership: `.loom-evidence/phase1-slice2/S2-W20/`

## Required repair

1. Add a focused mismatch test proving that an appender result with the same
   `EmittedAt` instant but a non-UTC `Location` is rejected with
   `ErrRuntimeDiscoveryCommitResultMismatch` and returns a zero Candidate.
2. Make the smallest product change necessary to require UTC locations on both
   the requested and returned Events during exact-content comparison.
3. Do not change Event construction, payloads, digest semantics, journal
   authority, source validation, or any other frozen contract behavior.
4. Re-run the complete S2-W20 strict matrix and obtain a fresh independent
   Repair 1 implementation review.

No discovery execution, projection, status inference, scheduling, daemon,
Runtime activation, dependency change, or external action is authorized.

VERDICT: REPAIR_FROZEN
