# S5-W2 Implementation Repair 2

Verdict: `PASS` repair evidence; fresh independent review still required.

The restart/replay exact-once assertion now requires:

- five `AgentGrantIdentityReserved` facts;
- five distinct Grant streams;
- exactly one `AgentGrantIssued` per Grant stream;
- exactly three bounded `AgentGrantAuthorized` Frame facts per Grant stream;
- exactly one `AgentGrantRevoked` per Grant stream.

This is asserted alongside five Run lineages, five Evidence lineages, three
source WorkItem Done facts, one Team terminal, and one test-only effect marker.

Focused repetition:

```text
go test ./internal/app \
  -run Phase1EngineeringDemoApprovalRestartReconnectAndRecovery \
  -count=20
ok  	loom-pi-rebuild/internal/app	9.284s
```

Focused race repetition:

```text
go test -race ./internal/app \
  -run Phase1EngineeringDemoApprovalRestartReconnectAndRecovery \
  -count=10
ok  	loom-pi-rebuild/internal/app	60.353s
```

No product file or ownership boundary changed.
