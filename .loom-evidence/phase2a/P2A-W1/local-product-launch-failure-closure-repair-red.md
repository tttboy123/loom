# P2A-W1 Local Product Launch Failure Closure RED

**Date**: 2026-07-29  
**Status**: `RED - CAUSAL`  
**Production files changed before RED**: none  
**Live allowance**: `0`

## Test inputs

```text
cmd/loomd/run_test.go
c84d5852356ef5e608f8590d7f95a4a52722fc476b6969e2c748d9cd187a8729

cmd/loomd/product_daemon_test.go
da65d75c4655b88bf233a001e42ef075d728ba76fec3389232e8768810e5ddea

local-product-live-closure-transaction-test.sh
8c3bf0e692e7ee9e8afc92031ed7c7c5529b0a7b3cb9093b70320b231c861e74
```

## Focused Go RED

Command:

```text
go test ./cmd/loomd -run 'TestRunWritesClosedDaemonFailureReasonCodes|TestRunClassifiesResultEncodingWithoutDisclosingWriterError|TestProductDaemonClassifiesLifecycleFailureBoundaries' -count=1
```

Exit status: `1`

Observed causal failures:

- `server_before_ready` returned an unclassified local IPC error instead of
  code `local_ipc`;
- `observer_after_ready` returned an unclassified joined observer error
  instead of code `observer`;
- observer and local IPC failures both emitted only `daemon failed`;
- unknown run and close failures emitted only `daemon failed` instead of
  closed fallback `daemon failed: shutdown`;
- result encoding emitted only `daemon failed` instead of
  `daemon failed: result`.

No private error detail appeared in daemon stderr. The test failure output's
synthetic strings contain only the literal `private` test marker and no real
path, credential, Provider output, model output, or hidden reasoning.

## Transaction-fixture RED

Command:

```text
sh .loom-evidence/phase2a/P2A-W1/local-product-live-closure-transaction-test.sh
```

Exit status: `1`

Exact bounded failures:

```text
RED: transaction does not own the closed failure-reason path
RED: transaction has no exact-path preflight boundary
RED: transaction cannot atomically record a closed pre-ready reason
RED: Candidate installation occurs before original-service absence
```

## Preservation

The RED phase changed tests and evidence only. It did not edit production Go
code or the production transaction, launch/stop/restart a service, touch the
installed product or resident Journal, create a live allowance, stage files,
or unlock P2A-W2.
