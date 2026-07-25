# S2-W20 Repair 2 Contract

- Lineage: `S2-W20`
- Repair attempt: `2/3`
- Product changes authorized: none
- Test ownership: `internal/state/runtime_discovery_writer_test.go`
- Evidence ownership: `.loom-evidence/phase1-slice2/S2-W20/`

## Required repair

1. Add direct deterministic proof that an already-expired deadline context
   returns `context.DeadlineExceeded`, a zero Candidate, and zero
   `AppendBatch` calls.
2. Do not change production behavior, Event construction, payloads, digest
   semantics, journal authority, source validation, or any other frozen
   contract behavior.
3. Re-run the complete S2-W20 strict matrix and obtain a fresh independent
   Repair 2 implementation review.

No discovery execution, projection, status inference, scheduling, daemon,
Runtime activation, dependency change, or external action is authorized.

VERDICT: REPAIR_FROZEN
