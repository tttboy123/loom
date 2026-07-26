# Slice 4 Exit Contract Amendment 1 / S4-W1 Contract Review 1

Reviewer: independent read-only contract reviewer

Date: 2026-07-26

## Round 1 findings

1. Non-approved resolution did not fence the bound claimed/running Run, so an
   old successful terminal could overwrite blocked/cancelled WorkItem status.
2. Paused terminal behavior did not distinguish failure/cancellation from a
   success bypass or define races with approval decisions.
3. `human_required` timeout had no S4-W1 WorkItem resolution mapping.
4. RuleSet uniqueness/current revision and CAS head budgets were not bounded
   tightly enough.
5. RuleSet activation authorization did not enumerate its complete binding.

Round 1 verdict: FAIL.

## Contract repair

S4-W1 is now limited to pre-claim `start_run` approval:

- WorkItem is exactly `assigned`;
- Run stream is empty, claim ID empty, and generation zero;
- no Runtime capacity or Grant exists;
- `Claim` requires WorkItem `assigned`; and
- request-vs-claim uses common WorkItem/Run heads so only one CAS wins.

Therefore no running process terminal can race approval. Approval restores
`assigned`, or resolves to `blocked`/`cancelled`, both non-claimable.

The repair also:

- restricts `on_timeout` to `reject` or `cancel`, deferring
  `human_required` to S4-W2;
- permits at most one current RuleSet for each of four exact scopes;
- caps request/decision at seven unique read/CAS streams; and
- binds RuleSet activation authorization to exact scope, revision, digest,
  actor, command/authorization digests, issued/expiry, correlation, and request
  identity.

## Round 2 result

All Round 1 findings are closed. The request-vs-claim CAS, approved restore,
rejected/expired/cancelled Claim fencing, RuleSet revision semantics,
authorization unforgeability, idempotency, replay/projection failure
preservation, and trust boundaries are precise enough for RED.

Round 2 verdict: PASS.

## Round 3 authorizer repair

RED API preparation exposed a contract contradiction: `Authority` injected a
`CustomerAuthorizer`, but the public mutation methods accepted caller-supplied
`Authorized*` values, leaving no coherent point where Authority invoked the
authorizer.

The contract now:

- accepts bounded requests plus opaque authorization presentation at public
  activation/decision methods;
- calls the injected authorizer internally;
- prevents callers from constructing authorized values;
- compares the complete returned binding before any new mutation; and
- discards presentation without Journal/Projection/View/log/error/Evidence or
  public digest exposure.

An exact already-committed retry may read only its target stream and return the
same public record without reauthorization or mutation. A divergent retry
cannot use that path; otherwise authorization precedes all new mutation.

Round 3 verdict: PASS.

## Round 4 ActionContext repair

RED fixture design exposed that the flattened ApprovalRequest input did not
carry the complete project/team/work-package/risk/Agent fields needed to
recompute its caller-supplied Decision. The contract now requires the complete
immutable `ActionContext`; Authority reloads current RuleSets, recomputes the
Decision, and validates every context/binding field before mutation.

Independent Round 4 confirmed that this closes the caller-supplied Decision
trust gap without changing pre-claim approval, seven-head CAS, idempotency, or
secret boundaries.

Round 4 verdict: PASS.

VERDICT: PASS
