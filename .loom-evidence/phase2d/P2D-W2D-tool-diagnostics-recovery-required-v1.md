# P2D-W2D Tool Diagnostics and Recovery-required v1

Status: `SOURCE VERIFIED / RECOVERY DECISION COMMAND OPEN`

Date: 2026-08-14

## Scope

This increment belongs to the existing Phase 2D Goal and P2D-W2D. It adds a
privacy-safe ToolCall diagnostic chain to the Pi Bash/Edit product path and an
authoritative restart state for a dispatched side effect whose result is
unknown. It does not create a Goal, complete the general Tool Gateway, or claim
installed acceptance.

## Tool diagnostic chain

The execution Adapter emits the closed stages:

```text
tool_authorization
tool_approval_wait
tool_sandbox_prepare
tool_binding_validation
tool_dispatch
tool_result_validation
tool_result_commit
tool_payload_commit
tool_result_delivery
tool_recovery
```

The product daemon enriches each emitted record from its frozen Attempt
invocation. The record binds the Incident, Provider, Provider Account, Model,
WorkItem, Run, claim generation, Runtime, Agent, Execution Binding digest,
Capsule digest, execution ID, call digest, Tool, and operation ID.

The schema has no command, path, Prompt, result content, secret, Authorization
header, or Provider-response field. Diagnostic recorder failure is ignored by
the authority path and can neither authorize nor block execution.

## Dispatch and error privacy

Tool-specific preflight now occurs before dispatch authority:

- Edit path ownership and content digest are validated before any
  `ToolDispatchCommitted` fact;
- unsupported Read/Grep calls fail before dispatch;
- a dispatch-gate failure invokes no executor;
- raw executor errors are mapped to controlled error codes before entering the
  execution result or RunStream receipt.

Successful Pi Bash/Edit therefore produces the ordered diagnostic progression
from authorization through delivery, while every failure closes at the exact
safe stage reached.

## Unknown-side-effect authority

On daemon construction, the execution Adapter reconciles its dedicated Journal
streams before the product service starts:

- Proposed-only records remain pending, preserving `ask` approval state;
- Allowed records with no completed, failed, denied, or recovery terminal are
  never executed again;
- each such record receives one schema-v2
  `ToolExecutionRecoveryRequired` fact;
- the only accepted recovery tuple is
  `side_effect_unknown / resolve_tool_recovery`;
- the projected result is non-retryable until a separate governed decision is
  implemented.

Replay rejects recovery without Allow, substituted code/action, duplicate
recovery, and a recovery after any other terminal. The recovery payload is
content-free.

## Restart projection repair

The global read projection previously rejected every schema-v2 event before
delegating to execution and permission authorities. That made App restart fail
after valid content-free execution facts.

Projection now admits schema v2 only for exact dedicated streams and event
types:

- `permission-decision/*` with `PermissionDecisionRecorded`;
- `execution/*` with ToolExecutionProposed, ToolExecutionDenied,
  ToolExecutionFailed, or ToolExecutionRecoveryRequired.

Unknown v2 types and a known type on the wrong stream still fail closed. The
dedicated execution/permission replayers remain responsible for strict payload
decoding.

## Swift presentation

The macOS execution model decodes recovery timestamp, code, and action. The
read-only execution view shows that the result is unknown and displays the
Incident ID. It intentionally does not render Retry, Skip, Cancel, or Resume:
no authoritative command currently exists for those actions.

## Verification

Passed:

```text
go test ./internal/execution ./internal/projection ./internal/runtime/piadapter ./internal/work -count=1
go test ./cmd/loomd -run 'TestProductDaemonStartupProjectsUnknownSideEffectRecoveryWithoutReexecution|TestProductOperationalDiagnostics|TestProductAttemptLoop|TestP2DWBridge|TestWBridgeHook|TestSandboxWire' -count=1
go test -race ./internal/execution -run 'TestP2DExecution|TestReplayRejectsInvalidRecoveryRequiredTransitions|TestRedB1_AllowedWithoutTerminalNeverReexecutes|TestP2DPendingApprovalSurvivesRecoveryReconciliation' -count=10
go test -race ./cmd/loomd -run 'TestProductAttemptLoopRuntimeGovernsLocalToolDispatchAndDelivery|TestProductOperationalDiagnosticsRecordsContentFreeToolCallIdentity|TestProductDaemonStartupProjectsUnknownSideEffectRecoveryWithoutReexecution' -count=10
go test ./... -count=1
go vet ./...
swift test
git diff --check
```

The full Go repository passed. The macOS suite passed 203 XCTest cases with one
intentional visual-export skip, plus 10 Swift Testing contracts.

## Open boundary

- exact Attempt-bound operational `tool_recovery` emission during startup;
- authoritative Resume/Retry/Skip/Cancel/New-Attempt commands and native UI;
- component-level sandbox enforcement reports;
- Pi child result injection and Provider/model continuation;
- ask suspension/resume;
- Read/Grep, Web/MCP, and other Runtime tool paths;
- Queue/Steer/Inject;
- installed CV6 and live mixed-Provider Team acceptance.

No App bundle was built, signed, launched, or installed. No real credential,
Provider, user workspace, or external tool was accessed. Source remains
post-build-64; installed Loom remains v0.5.2 build 39.
