# P2A-W3 Authoritative Acceptance Time Causal RED

**Date**: 2026-08-03  
**Contract Review 3**: `PASS`  
**Scope**: test-only advancing UTC clock before production repair

Command:

```text
go test -count=1 ./cmd/loomd -run '^TestProductMissionExecutionVerticalLoopbackClosesAuthorizedLineage$'
```

Result: `FAIL` after the five-second terminal deadline.

The real product execution composition committed both source and independent
Verifier WorkItem/Run/Grant/Evidence lineages and
`TeamNodeAttemptTerminal`, but committed no `WorkItemVerificationCommitted`,
`TeamNodeAcceptanceCommitted` or `TeamExecutionTerminal`. The test failed at
`terminal Journal fact not committed`, reproducing the retained Pi-005 stop
under a deterministic successive-call advancing UTC clock.

This RED was obtained by changing only the existing vertical test clock from a
constant instant to a mutex-protected clock advancing one millisecond per call.
No production source, process, live lineage, external action, staging or commit
was used to obtain RED.
