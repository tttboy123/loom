# S2-W11 Repair 1 Contract

- Attempt: 1 of 3
- Product ownership: `internal/teams/saved_team_binding.go`
- Test ownership: `internal/teams/saved_team_binding_test.go`
- Evidence ownership: `.loom-evidence/phase1-slice2/S2-W11/`

## Required repair

1. Revalidate every selected and unselected discovery RuntimeInstance through
   accepted `runtime.NewRuntimeInstance`, and validate every observation's
   model inventory before selection.
2. Add zero-output validation proof for changed selections, changed discovery,
   and insufficient current capacity.
3. Add direct invalid/duplicate AgentDefinition/Profile catalog, malformed
   selection, duplicate RuntimeInstance discovery, and archived exclusion
   proof.

No reservation, persistence, resource creation, execution, or scope expansion
is authorized. Rerun the full strict matrix and obtain fresh review.

VERDICT: REPAIR_FROZEN
