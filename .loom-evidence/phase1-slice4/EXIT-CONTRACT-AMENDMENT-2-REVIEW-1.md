# Slice 4 Exit Contract Amendment 2 / S4-W2 Contract Review 1

Reviewer: independent read-only contract Reviewer

Date: 2026-07-26

Baseline: `87ea092`

The Reviewer edited no files and returned `FAIL`.

## Blocking findings

1. `CommitTeamAttemptEvidence` still decomposed the Evidence Store receipt into
   caller-supplied Evidence ID/digest/summary fields. Those strings were not a
   sufficient authority boundary.
2. Per-node OutputContract, RecoveryPolicy, attempt-credit budget, workflow
   fallback, and approval requirement were supplied only on a Coordinator
   invocation and were not frozen before the first attempt.
3. S4-W2 allowed an alleged already-approved recovery input even though S4-W1
   approval is intentionally limited to pre-claim `start_run`; recovery
   approval could not be verified under the preserved authority.
4. The Windows compile check omitted the new `internal/rules` package.

## Required bounded repair

- Work authority must accept the non-constructible Store-returned
  `evidence.AttemptReceipt` and derive all Evidence/summary identity from it.
- `TeamExecutionPlanned` must freeze one exact semantic binding for every node
  before any attempt dispatch.
- A recovery policy requiring approval must deterministically return
  `human_required`; S4-W2 accepts no approval ID/digest/boolean bypass.
- Windows compilation must cover evidence, verification, and rules.

The Reviewer found the amendment necessity, reopened scope, no-Provider
fallback boundary, finite scheduling, projection-not-authority boundary, and
S4-W3/S4-W4 exclusions otherwise sound.

VERDICT: FAIL
