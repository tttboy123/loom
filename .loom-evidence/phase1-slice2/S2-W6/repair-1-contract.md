# S2-W6 Implementation Repair 1 Contract

- Lineage: `S2-W6`
- Repair attempt: `1/3`
- Product changes authorized: none
- Test ownership: `internal/teams/structured_draft_test.go`
- Evidence ownership:
  `.loom-evidence/phase1-slice2/S2-W6/deliverable.md`

## Required repair

1. Prove `AnswerStructuredTeamDraft` propagates invalid current, stale
   revision, catalog mismatch, wrong state, wrong question, empty answer,
   invalid next question, and invalid next references as typed errors.
2. Prove `EditStructuredTeamDraft` propagates invalid current, stale revision,
   catalog mismatch, wrong state, invalid question, and invalid next references
   as typed errors.
3. Every failed wrapper call must return the zero `StructuredTeamDraft`.
4. Re-run the entire S2-W6 strict matrix and obtain a fresh independent repair
   Reviewer PASS.

No production behavior, contract semantics, dependencies, persistence,
resources, execution surface, or external action may change.
