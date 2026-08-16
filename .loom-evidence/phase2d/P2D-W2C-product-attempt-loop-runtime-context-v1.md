# P2D-W2C Product Attempt Loop Runtime Context v1

Status: `SOURCE VERIFIED / CONTEXT TOOL PRODUCTION-INTEGRATED / GENERAL TOOL GATEWAY OPEN`

Date: 2026-08-13

## User outcome

A context-bearing Team Agent Attempt no longer bypasses the generalized Loom
Harness Core authority on its way from the product daemon to a Runtime Adapter.
After the Supervisor commits `RunStarted`, the product Runtime decorator freezes
the exact Attempt identity and records the Turn, Step, and model request before
calling Pi, Codex, Claude Code, or Loom Native.

The existing `loom_read_context` path is the first real governed ToolCall. Its
proposal and schema are admitted, scoped capability authorization and dispatch
are committed, encrypted result acceptance cites the exact dispatch event, and
Provider continuation or Harness output acknowledges delivery before the Step
and Turn can close. Prompt and disclosed Context content remain outside the
Journal.

This does not enable general Web, MCP, Bash, Edit, Read, or Grep execution. It
does not close approval, sandbox, interrupted-call recovery, Queue/Steer/Inject,
Swift UI, installed CV6, or mixed-Team live acceptance.

## Production changes

- `internal/supervisor/managed_execution.go` passes the non-secret exact Claim
  ID and Incident ID to the Runtime Adapter only after the Run starts, while
  preserving the existing workspace, grant, and bridge boundaries.
- `cmd/loomd/product_attempt_loop_runtime.go` decorates product Runtime Adapters,
  validates exact Attempt and route identity, freezes the scoped context policy
  and budget, and rebuilds Context delivery after `RunStarted`.
- `cmd/loomd/product_daemon.go` creates one Attempt Loop authority beside the
  payload authority, wraps every configured product Runtime Adapter, and builds
  only the scoped retriever before Supervisor dispatch.
- `internal/work/attempt_loop_authority.go` exposes canonical capability-set
  digest composition and exact governed result lookup. It rejects
  `TurnCancelled` until Step/Tool cancellation and interrupted side-effect
  recovery transitions exist.

## Failure and privacy behavior

- Execution Binding, Provider Account, Capsule route, Runtime, Agent,
  generation, or Incident substitution fails before the delegate Runtime runs.
- An oversized or missing encrypted result cannot become an accepted fact.
- A dispatched but unacknowledged result leaves the Step open for explicit
  recovery; it is not rewritten as a clean terminal failure.
- A failed call cannot satisfy Step completion without exact accepted-to-
  dispatch causation and delivery proof.
- Journal facts contain IDs, versions, modes, outcomes, and SHA-256 digests.
  Prompt, Context content, credentials, Provider bodies, tool arguments, and
  result bodies are not written there.
- Attempts without a Context Capsule retain the prior compatibility path and
  are not claimed as governed by this slice.

## Verification

Passed:

```text
go test ./cmd/loomd -run TestProductAttemptLoopRuntimeGovernsContextDelivery -count=1
go test ./internal/supervisor -count=1
go test ./internal/work -count=1
go test -race ./cmd/loomd -run TestProductAttemptLoopRuntimeGovernsContextDelivery -count=10
go test -race ./internal/work -run 'TestAttemptLoopAuthority|TestAttemptPayloadAuthority' -count=10
go test ./cmd/loomd ./internal/supervisor ./internal/work ./internal/runtime/nativeadapter ./internal/runtime/harnessadapter ./internal/runtime/piadapter -count=1
go test ./... -count=1
go vet ./...
```

The full repository test run passed, including `cmd/loomd` in 90.341 seconds,
`internal/localipc` in 72.537 seconds, and `internal/runtime/piadapter` in
67.630 seconds.

No App bundle was built, signed, launched, or installed. No real credential,
Provider, network tool, or user workspace was accessed. Source remains
post-build-64; installed Loom remains v0.5.2 build 39.

## Next gate

Compose the existing local execution Hook and Tool Broker through the same
Attempt context. Exact one-shot approval consumption and content-free Journal
v2 are now source verified separately; authenticated encrypted approval detail,
component sandbox evidence, tool-level operational diagnostics, interrupted-
call recovery, Queue/Steer/Inject, and Swift governance remain open.
