# S2-W10 Implementation Repair 1 Contract

- Attempt: 1 of 3
- Product changes authorized: none
- Test ownership: `internal/teams/instantiation_plan_test.go`
- Evidence ownership: `.loom-evidence/phase1-slice2/S2-W10/`

## Required repair

1. Directly compare every accepted Main/SubAgent AgentDefinition,
   RuntimeProfile, RuntimeInstance, Skill, member, and permission selection
   against the emitted role seeds and prove no widening.
2. Add direct digest-sensitivity mutations for catalog/content/binding digests,
   terminal revision, role AgentDefinition/RuntimeInstance/Skill/member
   selections, and WorkItem ID/owner.
3. Add direct `ErrStructuredTeamDraftReferenceMismatch` propagation with zero
   output.

Production behavior and the repaired S2-W10 contract must not change. Rerun the
complete strict matrix and obtain a fresh independent implementation review.

VERDICT: REPAIR_FROZEN
