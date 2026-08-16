# P2D-W2D One-shot Approval and Content-free Journal v1

Status: `SOURCE VERIFIED / ENCRYPTED PI APPROVAL DETAIL INTEGRATED SEPARATELY / GENERAL TOOL GATEWAY OPEN`

Date: 2026-08-13

## User outcome

An approved local ToolCall can authorize only one exact execution. The approval
is bound to its approval digest, Job, canonical call digest, consumer execution,
operation, and correlation lineage. A second operation cannot reuse it, and two
concurrent consumers cannot both execute.

New execution and permission decision facts no longer persist command, path,
free-form denial text, or raw executor errors. They use schema v2 with the
canonical call digest and controlled reason/error/recovery codes. Existing
schema-v1 history remains strictly replayable.

This does not complete the general Attempt Tool Gateway. The encrypted proposal
detail channel is now source verified for the Pi ask and Attention read path in
a follow-up slice. The TUI still fails closed for Allow whenever that detail is
unavailable; it never asks a user to approve a blank or unauthenticated request.

## Authority changes

- `internal/rules.Authority` records `PermissionApprovalConsumed` as the third
  approval-stream fact. Exact replay by the original consumer is idempotent;
  any different consumer or operation receives a consumed error.
- `internal/execution.Adapter` requires the approval consumer boundary before
  resuming an approved ask decision. Consumption happens before side effects,
  so a race can execute at most one operation.
- `permissions.ProposedCallDigest` is the shared canonical identity used by
  permission and execution facts.
- new `ToolExecutionProposed`, `ToolExecutionDenied`, `ToolExecutionFailed`, and
  `PermissionDecisionRecorded` facts use schema v2 and content-free payloads.
- the Journal accepts schema versions 1 and 2; each domain still rejects v2 for
  event types it does not explicitly own.
- permission attention projects the non-secret call digest and controlled
  recovery code. Raw command/path fields are empty for v2.

## Failure and recovery boundary

Approval consumption and the execution dispatch fact are in separate streams.
A crash after consumption but before dispatch remains fail closed: Loom does
not reuse the approval or infer that the side effect happened. Explicit
interrupted-operation recovery and an authenticated encrypted proposal-detail
store remain required before general local tools can be enabled in the product
Attempt Gateway.

## Privacy checks

Tests prove that new Journal payloads do not contain:

- the proposed command or path;
- free-form permission denial/recovery text supplied by a caller;
- raw errors returned by the local executor.

The Journal contains only IDs, tool kind, call digest, generation, controlled
codes, timestamps, and result/evidence digests. It does not contain Prompt,
credential, Provider response, tool arguments, or tool result content.

## Verification

Passed:

```text
go test -race ./internal/execution ./internal/rules ./internal/permissions -run 'TestP2DApproval|TestP2DExecutionJournalDoesNotPersist|TestP2DPermissionDecisionJournalDoesNotPersist' -count=10
go test ./internal/journal ./internal/rules ./internal/execution ./internal/permissions ./internal/projection ./internal/app ./cmd/loomd -count=1
go test ./internal/app ./internal/tui -count=1
go test ./... -count=1
go vet ./...
```

The full repository run passed, including `cmd/loomd` in 71.317 seconds,
`internal/localipc` in 57.479 seconds, and `internal/runtime/piadapter` in
60.211 seconds.

No App bundle was built, signed, launched, or installed. No real credential,
Provider, network tool, or user workspace was accessed. Source remains
post-build-64; installed Loom remains v0.5.2 build 39.

## Next gate

Compose the local execution Adapter through the production Attempt Tool Gateway
with dispatch-before-side-effect, sandbox enforcement
evidence, operational diagnostics, and interrupted-call recovery.
