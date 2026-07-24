# S2-W5 Contract Repair 1

- Failed contract SHA-256:
  `aa1d001090dbe904b0c22c5f6caf79592f7687a9a8a44be97edcff58b02ea4c8`
- Repaired contract SHA-256:
  `ba32fae6a1155916d77a4a1bcd9838693572968b046438318c41532351e4631b`
- Trigger:
  `.loom-evidence/phase1-slice2/S2-W5/contract-review-1.md`

## Bounded semantic repair

1. A valid content snapshot now requires one Main plus at least one and at most
   two SubAgents.
2. Main-only content returns typed `ErrMissingTeamDraftRole`; it cannot become a
   valid or acceptance-ready Candidate.
3. At least one task is required.
4. Every task remains SubAgent-owned, and every selected SubAgent must own at
   least one task.
5. `AcceptanceReady=true` now explicitly requires the valid role/task shape as
   well as zero capability gaps.
6. Mandatory RED coverage now includes main-only and zero-task inputs.
7. `docs/CURRENT.md` now names S2-W5 Contract Repair 1 as the current gate.

No owned product files, external actions, dependencies, persistence, Draft
transitions, resource creation, or Slice 3 behavior were authorized by this
repair.
