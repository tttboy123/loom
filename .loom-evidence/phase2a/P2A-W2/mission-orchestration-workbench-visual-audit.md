# Mission Orchestration Workbench Visual Audit

Date: 2026-07-30

Status: READY FOR INDEPENDENT VISUAL REVIEW

## Evidence digests

| Evidence | SHA-256 |
|---|---|
| mission-workbench-wide-light.png | `ae5d157a5838a04fa973e1e7815bb347d831d4fb3b339cc7d1ee104268a9d33a` |
| mission-workbench-compact-light.png | `3a2eb0535725e04a8e95429931fff575571cc9599f3fb079d93e35240682c741` |
| mission-workbench-wide-dark.png | `f38e6ffb7317156cb97bff7fc74b9b1443311f5391b232eb9cc00e3726b83e92` |
| mission-workbench-compact-dark.png | `2beea8d85a260a1115690813f5cc09c322f4b3f9be5a1f21a23eefbed95b1fb7` |
| mission-room-wide-light.png | `cca9fbb5e0447bf5b1f27dcf88359530a225b4fdf8d6a3dafe6082cc2f257bd6` |
| decision-authorization.png | `a97e4815b8211a3acb04b7eca3682a2d788c8bece8616725f80e996f709c0c01` |
| decision-review.png | `eb7622ae6456568efb99d4d29ab21094d4910e493161c0ed788c0eb4d1e35b50` |
| decision-recovery.png | `42058f99b44ff4e2a056c2359a64633c29bb663c92353ad9098728142a45994b` |
| tui-mission-board.png | `3f811afde168064cce7c646a3714e56ca6b18f646d3f50a90ab1f0c3f0d2e716` |

## Controller inspection

- Wide Board shows all five exact lifecycle lanes at once.
- Mission cards expose the full Mission ID, title, source, priority,
  Team/Attempt presence, current node progress and readable last milestone.
- Compact Board preserves the rail and has an explicit horizontal-lane cue.
- Light and Dark retain the same hierarchy.
- Mission Room is a genuine three-column workspace and visibly offers
  `Plan only`, `Guided` and `Delegated`; selection remains a proposal until
  Loom confirms authority.
- Empty output and Evidence states are truthful.
- The TUI Board includes a selected Mission Detail with Team role/state,
  Attempt, current node, decision availability and milestone.
- All three decision templates share one native sheet host while presenting
  kind-specific, non-misleading authorization, review and recovery semantics.
- each mutation button is enabled only when its exact action appears in the
  strict `prepared_actions` wire collection;
- high-risk actions use full names and have no Return-key default;
- minimum action target is 44 points;
- Reduce Motion removes the Board transition animation;
- color is not the only state signal;
- SF Symbols are used instead of emoji;
- no raw Grant, credential, hidden reasoning or unbounded output is shown.

The screenshots are deterministic AppKit fixtures. They are not an installed
application or live-product claim.
