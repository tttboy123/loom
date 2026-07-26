# S4-W1 Fresh Repair 1 Implementation Review 2

Date: 2026-07-26

Reviewer: fresh independent read-only Reviewer

Baseline: `7bb9881`

The Reviewer edited no files and returned `PASS` with no findings.

## Repair verification

1. Both approval request and resolution derive Approval, WorkItem, Run, and all
   four project/team/work-package/work-item RuleSet streams from the immutable
   ActionContext. They transaction-consistently re-evaluate current RuleSets
   and CAS all seven heads, so callers cannot omit a higher-scope RuleSet.
2. Activation, request, decision, and expiration retries bind the complete
   committed command. Divergent correlation or authorization presentation
   conflicts, while an exact committed activation/decision retry remains
   idempotent after the original authorization window.
3. Reopened approved retries reconstruct exactly seven sorted heads from
   immutable Journal facts: approval terminal, WorkItem resolution, empty Run,
   and exact-or-zero heads for all four derived RuleSet streams.
4. CustomerAuthorizer requests expose immutable accessors and exported
   validated response constructors. Authority consumes authorized values only
   through its injected authorizer return path.

## Cross-boundary audit

The Reviewer confirmed:

- Claim still requires WorkItem status exactly `assigned`;
- approval pause/resolution replay and projection causation are fail-closed;
- Projection and GlobalReadView expose copied records and digests, not raw
  authorization presentation;
- malformed replay, concurrency, and mutation-isolation coverage remains
  present; and
- no dependency, daemon/API/CLI, Runtime/Provider, credential, S4-W2, second
  authority, or unrelated dirty-file expansion was added.

Reviewer commands:

```text
go test ./internal/rules ./internal/work ./internal/projection -count=1
go test -race ./internal/rules -run 'TestApprovalLifecycle|TestApprovalRequestCannotOmit|TestApprovalDecisionRejectsRuleSetChanged|TestCustomerAuthorizerPort|TestApprovalExpiry' -count=1
gofmt -d <owned Go files>
git diff --check -- <owned/evidence files>
git diff -- go.mod go.sum
```

All passed or produced no output where expected.

VERDICT: PASS
