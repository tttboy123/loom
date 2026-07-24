# S2-W9 Repair 1 Contract

- Attempt: 1 of 3
- Product changes authorized: none
- Test ownership: `internal/teams/resolver_test.go`
- Evidence ownership:
  `.loom-evidence/phase1-slice2/S2-W9/`

## Required repair

Add direct failure-path and zero-Candidate proofs for:

1. selected Team not found;
2. selected AgentDefinition not found;
3. invalid AgentDefinition catalog;
4. duplicate AgentDefinition catalog;
5. duplicate RuntimeProfile catalog;
6. project default referring to a reusable-scoped TeamDefinition;
7. reusable default referring to a project-scoped TeamDefinition.

The existing invalid RuntimeProfile proof remains applicable. Production
behavior and the frozen S2-W9 contract must not change.

## Verification gate

Run the complete S2-W9 focused, package, focused race, repository,
repository-race, vet, format, diff, import-boundary, and scope matrix. Then
obtain a fresh independent read-only implementation review against the repaired
test digest and this repair contract.

VERDICT: REPAIR_FROZEN
