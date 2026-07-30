# P2A-W2 Interaction Continuity Repair 2 Implementation Re-review

**Reviewer**: fresh independent read-only Reviewer
**Verdict**: `FAIL`

## Findings

### P1 — unselected alternatives invent Provider/auth metadata

Exact role-option dispatch is present, but the setup snapshot does not carry
Provider/auth for every unselected alternative. Native and TUI filled that
absence with generic `Local provider` and `Native`, which could be wrong for a
future brokered or provider-ephemeral option.

### P2 — alternate-profile proof is missing

Tests covered the current selected role and exact edit dispatch but did not
prove that an unselected option with a different profile avoids false
Provider/auth claims.

## Confirmed closure

The Reviewer confirmed the exact existing `main_role`/`subagent_role`
Builder-edit dispatch, no new protocol/model authority, unchanged strict
Go-to-Swift fixture, and clean focused tests.

## Gate

Repair 3 must remove metadata guesses, show only authoritative alternative
fields, and defer exact Provider/auth presentation until the selected option's
returned preview. It remains inside the same contract and creates no Amendment
or W4.
