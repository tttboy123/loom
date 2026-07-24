# S2-W8 Implementation Repair 1 Contract

- Lineage: `S2-W8`
- Repair attempt: `1/3`
- Product ownership: `internal/teams/team_definition.go`
- Test ownership: `internal/teams/team_definition_test.go`
- Evidence ownership:
  `.loom-evidence/phase1-slice2/S2-W8/deliverable.md`

## Required repair

1. Detect duplicate Candidates only after target, active status, scope,
   identity, and greatest-version selection; duplicate archived, unrelated, or
   lower-precedence non-winners must not block the selected winner.
2. Make every zero `TeamDefinition` accessor safe, including
   `SubAgentDefinitionIDs()`.
3. Add regression tests proving duplicate archived/unrelated Candidates are
   ignored, duplicate winners still fail, and all zero copied accessors are
   non-panicking and empty.
4. Re-run the full S2-W8 strict matrix and obtain a fresh independent Repair 1
   implementation Reviewer PASS.

No contract semantic, dependency, persistence, resource, Mode/Draft/instance,
execution, or external-action expansion is authorized.
