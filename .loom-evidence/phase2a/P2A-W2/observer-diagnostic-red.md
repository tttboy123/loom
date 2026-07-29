# P2A-W2 Observer Diagnostic Mandatory RED

**Date**: 2026-07-30
**Status**: `RED — expected missing frozen behavior`
**Baseline**: `7b9bfcf4f7b5babd3081d95b4660134923282511`

Tests were added before product changes for:

- typed, non-disclosing Pi metadata command-stage/cause retention;
- projection-refresh stage wrapping;
- closed observer reason classification;
- exact CLI allowlisted reason projection;
- ambiguous joined-error fail-closed behavior.

Command:

```text
go test ./internal/runtime ./internal/app ./cmd/loomd \
  -run 'TestPiRuntimeProbeRetainsSafeCommandStageAndTypedCause|TestRunProjectionSynchronizedRuntimeObservationOnceTriggerAndRefreshFailures|TestRunProjectionSynchronizedRuntimeObservationOnceRetainsSuccessfulTupleOnPostRefreshFailure|TestObserverFailureReasonIsClosedTypedAndNonDisclosing|TestProductDaemonClassifiesLifecycleFailureBoundaries|TestRunWritesClosedDaemonFailureReasonCodes' \
  -count=1
```

The command failed to compile only on the missing frozen production symbols:

```text
undefined: PiMetadataFailureCommand
undefined: ErrRuntimeObservationProjectionRefresh
undefined: observerFailureReason
```

No installed Pi, daemon, native app, Provider, Keychain, network, Journal or
live component was used.

The binding-efficiency RED then added an exact per-command validation-count
proof:

```text
go test ./internal/runtime/piadapter \
  -run '^TestPiMetadataProcessRunnerRevalidatesCatalogExactlyOncePerCommand$' \
  -count=1
```

It failed to compile only because the frozen catalog validation seam did not
exist:

```text
catalog.validateBinding undefined
```
