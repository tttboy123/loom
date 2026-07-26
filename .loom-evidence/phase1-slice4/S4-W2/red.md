# S4-W2 Mandatory RED

Date: 2026-07-26
Baseline: `87ea092`

Command:

```text
go test ./internal/verification ./internal/rules ./internal/evidence ./internal/work ./internal/app ./internal/projection -count=1
```

Observed expected compile failures before product behavior:

- `internal/verification`: missing immutable OutputContract, OutputObservation,
  Classification, and all four classification values;
- `internal/rules`: missing immutable RecoveryPolicy and deterministic bounded
  decisions;
- `internal/evidence`: `AttemptReceipt.OutputSummary` absent;
- `internal/work`: pre-dispatch `SemanticBindings` and
  `TeamNodeSemanticBinding` absent;
- `internal/projection`: legacy-unbound marker plus semantic, classification,
  recovery, budget, and workflow metadata absent; and
- `internal/app`: new semantic and workflow execution request boundary cannot
  compile through its missing lower-layer dependencies.

These failures directly trace to the frozen S4-W2 acceptance boundary and are
not fixture, environment, network, Runtime, daemon, or pre-existing failures.
No production behavior was changed before this RED.

VERDICT: PASS
