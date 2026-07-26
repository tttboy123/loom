# S4-W1 Mandatory RED

Date: 2026-07-26

Baseline: `7bb9881`

Behavioral tests were written first for:

- immutable canonical RuleSet evaluation and effect merge;
- duplicate/stale scope and four-scope bounds;
- authorizer-required monotonic RuleSet activation;
- atomic pre-claim ApprovalRequest plus WorkItem pause;
- restart-safe terminal-once approval decision;
- Claim fencing while paused; and
- already-claimed Run rejection.

Command:

```text
go test ./internal/rules -count=1
```

Observed:

```text
undefined: RuleSetActivationRequest
undefined: AuthorizedRuleSetActivation
undefined: ApprovalDecisionRequest
undefined: AuthorizedApprovalDecision
undefined: Scope
undefined: Rule
undefined: RuleSet
undefined: ActionContext
FAIL loom-pi-rebuild/internal/rules [build failed]
```

The failure is the expected missing frozen S4-W1 Rule/Approval behavior. No
product code existed when RED was captured.

After the new Rule/Approval authority compiled, the accepted Work authority
boundary received its own focused behavioral test before production replay was
changed.

Command:

```text
go test ./internal/work -run TestApprovalPauseAndResolutionFenceClaimAndReplayStrictly -count=1
```

Observed:

```text
pending_approval_fences_claim_and_approval_restores_assignment:
paused Claim() error = run authority conflict
FAIL loom-pi-rebuild/internal/work
```

This is the expected RED: the accepted Work replay rejects the new
`WorkItemApprovalPaused` fact rather than replaying it as a non-claimable state.
The companion malformed-event case already fails closed.

The Projection/View boundary then received its focused tests before its
production files were changed.

Command:

```text
go test ./internal/projection -run 'TestApprovalProjection' -count=1
```

Observed:

```text
snapshot.RuleSets undefined
snapshot.ApprovalRequests undefined
view.RuleSet undefined
view.ApprovalRequest undefined
FAIL loom-pi-rebuild/internal/projection [build failed]
```

This is the expected RED for the missing atomic RuleSet/ApprovalRequest
projection and immutable typed accessors.

VERDICT: PASS
