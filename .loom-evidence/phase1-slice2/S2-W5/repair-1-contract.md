# S2-W5 Implementation Repair 1 Contract

- Lineage: `S2-W5`
- Repair attempt: `1/3`
- Product changes authorized: none
- Test ownership: `internal/teams/draft_content_test.go`
- Evidence ownership:
  `.loom-evidence/phase1-slice2/S2-W5/deliverable.md`

## Required repair

1. Add a positive one-Main/one-SubAgent/non-empty-task/gap-free case and prove
   validation returns `Valid=true`, `AcceptanceReady=true`, role count `2`, and
   task count `1`.
2. Add a valid task dependency-edge mutation and prove its digest differs from
   the baseline.
3. Create the deliverable with the frozen contract/RED/check/scope/trust
   evidence. Before fresh repair review its last line must remain
   `VERDICT: PENDING_REVIEW`.
4. Re-run the entire S2-W5 strict matrix and obtain a fresh independent repair
   Reviewer PASS.

No production behavior, contract semantics, dependencies, persistence,
resources, execution surface, or external action may change.
