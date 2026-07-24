# S2-W3 Contract Repair 1

- Repair type: contract clarification before product RED
- Original contract SHA256:
  `9c30f92f0a4c8540099f6f8be338f935db9bbd38a0ac345425ddb027433ad8d9`
- Product changes: none
- Owned-file changes: none
- Acceptance expansion: none

## Frozen clarification

- Agent maximum counts selected catalog entries after complete S2-W1
  validation and scope/version resolution.
- Runtime maximum counts online catalog entries after status filtering.
- Model maximum counts Runtime-scoped `(runtime_instance_id, model_id)` pairs
  under online Runtime entries; identical model strings under two Runtime IDs
  count as two pairs.
- Skill, member, and permission maxima count their normalized unique sets after
  empty/duplicate validation.
- Every bound accepts exactly `max` entries and rejects `max + 1` without
  truncation.

The repair changes no product boundary, trust boundary, dependency, ownership,
or later Team/Draft/Run authority.

STATUS: CONTRACT_REPAIR_FROZEN
