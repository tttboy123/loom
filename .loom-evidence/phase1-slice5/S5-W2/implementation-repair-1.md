# S5-W2 Implementation Repair 1

Verdict: `PASS` repair evidence; fresh independent review still required.

## Repairs

1. Two independent Coordinator callers now use separate Projection,
   Work Authority, Grant Authority, random sources, and Supervisor bindings
   while sharing the same Journal/Evidence authorities. They start from one
   barrier. Exactly one caller may execute `main/2`; the loser must be a known
   CAS/stale-view/grant-identity conflict or an idempotent terminal observer.
2. A valid canonical timeline cursor is captured, the private SQLite fixture
   deliberately disables its no-update trigger and changes the cursor's final
   Team stream head, and a frozen last-good view proves
   `ErrTimelineCursorConflict` plus `ErrStreamGap`.
3. The successful final source adapter increments a dedicated test-only
   external-effect counter. Restart/replay exact-once now requires the counter
   to remain exactly one alongside Run, Grant, Evidence, Done, and Team
   terminal counts.
4. Both Journal ordering assertions now use the accepted `work-item/` stream
   prefix.

## Focused evidence

```sh
go test ./internal/app \
  -run 'Phase1EngineeringDemoApprovalRestartReconnectAndRecovery|Phase1EngineeringDemoFailClosedAndPrivateModes' \
  -count=20
```

```text
ok  	loom-pi-rebuild/internal/app	11.403s
```

```sh
go test -race ./internal/app \
  -run 'Phase1EngineeringDemoApprovalRestartReconnectAndRecovery|Phase1EngineeringDemoFailClosedAndPrivateModes' \
  -count=10
```

```text
ok  	loom-pi-rebuild/internal/app	50.087s
```

The first race attempt exposed a shared `bytes.Reader` in the test fixture.
Repair changed each concurrent caller to independent authorities/random
sources and rebound its private Supervisors accordingly; no production file
was reopened.

Final focused product commands:

```text
ok  	loom-pi-rebuild/internal/work	0.851s
ok  	loom-pi-rebuild/internal/app	2.416s
ok  	loom-pi-rebuild/internal/work	1.623s
ok  	loom-pi-rebuild/internal/app	8.695s
```
