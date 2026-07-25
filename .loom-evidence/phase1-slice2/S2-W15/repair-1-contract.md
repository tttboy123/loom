# S2-W15 Implementation Repair 1 Contract

- Lineage: `S2-W15`
- Repair attempt: `1/3`
- Product change authorized: only encode Event `PayloadJSON` as exact bytes in
  the commit-digest canonical form; no other production behavior may change
- Test ownership: `internal/state/saved_team_writer_test.go`
- Evidence ownership:
  `.loom-evidence/phase1-slice2/S2-W15/deliverable.md`

## Required repair

1. Replace the mislabeled reusable case with a real simultaneous same-ID
   project/reusable Team shadow and prove the committed facts preserve the
   reusable resolution selected through the reusable default.
2. Add table-driven commit-digest sensitivity for every immutable Event field,
   source record-set digest, count, dormant membership, Main definition
   version/scope, Runtime binding, identity, and timestamp. Prove valid semantic
   source changes alter the applicable canonical payload and commit digest.
3. Prove trailing canonical-JSON whitespace changes the digest so the digest
   binds exact payload bytes, not only decoded JSON semantics.
4. Keep every non-digest production path unchanged.
5. Re-run the entire S2-W15 strict matrix and obtain a fresh independent Repair
   1 Reviewer PASS.

No production behavior, contract semantics, dependencies, persistence schema,
projection, resources, execution surface, or external action may change.
