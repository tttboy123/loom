# P2A-W2 Interaction Continuity Repair 3 RED

**Status**: `RED — UNSELECTED ROLE METADATA DRIFT REPRODUCED`

Fresh independent Repair 2 Re-review returned `FAIL` with one P1 and one P2.
Exact role-option dispatch exists, but unselected alternatives were presented
with invented generic `Local provider` and `Native` metadata that the bounded
setup snapshot does not carry.

Frozen owned Swift and TUI tests prove the unsafe fallback:

- Swift expected empty Provider/auth for the unselected alternate but received
  the invented values;
- TUI expected `Select to review provider and sign-in` but rendered the same
  invented values.

Repair 3 removes those guesses. An unselected option shows only authoritative
responsibility, Runtime and model data plus an explicit select-to-review cue.
After selection, the returned Builder preview supplies the exact Provider/auth
presentation. No protocol, application service, catalog or authority changes.
