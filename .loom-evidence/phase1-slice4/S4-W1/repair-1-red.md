# S4-W1 Repair 1 RED

Date: 2026-07-26

The four Reviewer findings were encoded as regression tests before Repair 1
product changes.

Command:

```text
go test ./internal/rules -count=1
```

Observed compile RED:

```text
activation.AuthorizationPresentation undefined
activation.RuleSet undefined
activation.CorrelationID undefined
undefined: NewAuthorizedRuleSetActivation
decisionRequest.AuthorizationPresentation undefined
decisionRequest.ApprovalRequestID undefined
decisionRequest.ApprovalRequestDigest undefined
decisionRequest.Decision undefined
decisionRequest.CorrelationID undefined
FAIL loom-pi-rebuild/internal/rules [build failed]
```

The same test change also adds behavioral assertions for omitted current
RuleSets, divergent exact retries, and restart-stable resume heads. The missing
port surface caused compilation to stop before those assertions could run,
which is the expected mandatory RED for this combined bounded repair.

VERDICT: PASS
