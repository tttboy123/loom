# Phase 2C Repair 19 Contract, Source, and Status Review 1

**Result**: `FAIL`  
**Findings**: `P0=0`, `P1=0`, `P2=1`  
**Scope**: read-only independent review of Repair 19 contract, source, test,
status, deterministic evidence, and Attempt 022 J9 evidence

## Finding

The deterministic evidence says duplicate title/status inputs at positions
three and four were proven, but the focused test uses positions one and two.
The implementation makes any two positive positions distinct, so this is not a
source correctness defect. It is an exact-evidence P2 and must be corrected
before replacement source-lock or carry-forward authorization.

## Verified

- Attempt 022 correctly fails J9: its audit contains two
  `Open recent task Recent work` actions and two
  `Open recent task Attempt 022 Review Team` actions.
- Displayed Recent rows use `dropFirst().prefix(6).enumerated()` and pass the
  one-based visible position into the shared action-label helper.
- Title and subtitle pass through `SafeText` with bounds 48 and 32; no raw task
  ID enters the helper.
- The exact helper result feeds both `.accessibilityLabel` and `.help`.
- Position affects only action naming. The action closure retains the existing
  task selection and activation behavior.
- Repair 19 remains narrow; replacement lock, complete matrix, signed Release,
  J9/J10, Phase, and ADR acceptance are not authorized.
- Staged paths: zero.

## Reviewed Hashes

- `ab152eb0101269ae85e9ca90a17d0b4447883ab755c98e8aa6bd6eb6ffd71e5c`
  `apps/macos/Sources/LoomLocalAppUI/LoomWorkspaceShell.swift`
- `422fe485ac9b1e601d54db08726b8a312ddcfa41aafa67ae369646ce8bd1bc1c`
  `apps/macos/Tests/LoomLocalAppTests/LoomGraphiteViewTests.swift`
- `90d4f946eb05c7e155f33155a3f50ae46136f0b3acc74559df893abe3923b11b`
  `.loom-evidence/phase2c/contracts/PHASE-2C-REPAIR-AMENDMENT.md`
- `6ebb169284618de2f85d4b8e843ee12ec1c10b4d63f0e64fd67101dc765c6961`
  `.loom-evidence/phase2c/repair-candidate-boundary.md`
- `fa2490b69c17b00aa03368101f0c653c00ccde6873df29d0d243cbdaf8b9bd03`
  `docs/CURRENT.md`

## Verdict

`FAIL_P0_0_P1_0_P2_1`. Append an exact correction and obtain an independent
exact-byte re-review. Do not generate a replacement source lock yet.
