# P2A-W2 Mission Workbench Repair 2 RED

Date: 2026-07-30

Status: CAUSAL RED PRESERVED

Fresh Implementation Re-review found two related prepared-command binding
gaps:

1. Review and Recovery sheets accepted an arbitrary valid-shaped
   `decision_digest` instead of the exact Work decision digest.
2. Authorization could present a different Mission/Team/Node/Attempt while
   delegating the pending Rules request because the prepared boundary did not
   bind the sheet to the original ActionContext.

The focused RED command was:

```text
go test ./internal/app \
  -run 'TestPrepared(AuthorizationRejectsMislabeledMissionContext|ReviewRejectsUnboundDecisionDigest|RecoveryRejectsUnboundDecisionAndClaim)$' \
  -count=1
```

It failed only on the three missing rejection behaviors:

```text
mislabeled authorization error = <nil>
unbound review digest error = <nil>
unbound recovery error = <nil>
```

The repair stays inside the frozen `internal/app/local_product_decision.go`
boundary. It adds no Event, Journal, Rules, Work, Grant or Evidence authority
and does not touch excluded authority files.
