# S2-W16 Implementation Repair 1 Contract

- Lineage: `S2-W16`
- Repair attempt: `1/3`
- Product change authorized: presence-aware decoding for required S2-W15
  schema-v1 fields whose legal value may be zero, empty, or an empty slice
- Test ownership: `internal/projection/projection_test.go`
- Evidence ownership:
  `.loom-evidence/phase1-slice2/S2-W16/deliverable.md`

## Required repair

1. Add mandatory RED cases proving omission of `dormant_sub_agents`,
   `active_sub_agent_count`, `work_item_count`, every count field, and both
   Team/Main scope-identity components fails closed.
2. Decode those fields through private pointer-backed fact structs, require
   every pointer nonnil, then copy scalar values into the frozen read model.
3. Preserve all Event, link, scope, digest, clone, concurrency, import, and
   no-side-effect behavior.
4. Re-run the complete S2-W16 strict matrix and obtain fresh independent Repair
   1 Reviewer PASS.

No schema, public read-model shape, dependency, persistence, CLI, resource,
process, execution, or external action may change.
