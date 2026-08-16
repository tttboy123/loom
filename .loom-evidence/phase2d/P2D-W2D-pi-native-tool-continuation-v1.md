# P2D-W2D Pi native Tool continuation v1

**Status**: SOURCE VERIFIED / INSTALLED LIVE OPEN  
**Date**: 2026-08-14  
**Goal**: sole Phase 2D Goal

## Acceptance boundary

This increment replaces the Pi bridge's prior RunStream-only Tool result path
with a private native Pi extension path. One frozen Agent Attempt can expose
`loom_read_context` and `loom_tool` together. The first ToolCall locks the
protocol route; a result from the other route, a substituted digest, or a
second unbound tool fails closed.

For a governed local ToolCall, the child connects over a per-Attempt owner-only
UDS. The daemon attests the child PID, validates the exact Attempt/Run/Agent,
Execution Binding, Capsule, claim and Incident lineage, and submits the exact
proposal to the existing execution authority. `ask` keeps the same extension
request pending and rechecks the same deterministic operation until an
authoritative approval or rejection is visible. Cancellation closes the wait
without execution.

An allowed result is returned to Pi as a native `toolResult` containing only
the call digest, Tool kind, controlled status and digest-only execution
metadata. The command/path, encrypted payload, delivery binding, approval ID,
approval digest, Prompt and Provider body are not returned to the child or a
Bridge frame. Only after Pi emits a validated second-turn final assistant
message does Loom acknowledge the encrypted Attempt payload with
`harness_final_output`. A deny has no delivery acknowledgement.

The locked Pi 0.82.1 runtime probe now advertises
`governed_tool_loop` only through an explicit conformance provider. Product Pi
profiles require both `context_retrieval` and `governed_tool_loop`; the adapter
requires the frozen capability and daemon Hook to match before process start.
The combined prompt and explicit extensions disable built-in tools and
automatic extension discovery.

## Verification

```text
go test ./... -count=1
PASS

go test -race ./internal/runtime/piadapter -run 'TestPiRPCToolExecuteReturnsNativeResultBeforeFinalOutput|TestPiRPCHybridProtocolLocksSelectedRoute|TestPiToolExtension' -count=10
PASS

go test -race ./cmd/loomd -run 'TestProductAttemptLoopRuntimeGovernsLocalToolDispatchAndDelivery|TestProductAttemptToolDiagnostic|TestProductPiProfileCapabilities' -count=10
PASS

go vet ./...
PASS

git diff --check
PASS
```

The Execute-level test launches an actual managed child process, uses the
generated private extension and UDS, traverses Ask then Allow, consumes the
native result, emits a second model turn, and proves cleanup and
`harness_final_output`. It also proves that command bytes and approval
authority references are absent from the child result, Bridge frames and
content-free audit.

## Open boundary

This closes Pi Bash/Edit native continuation and Ask suspension/resume in
source. Read/Grep execution, multiple sequential ToolCalls, Web/MCP, Codex,
Claude Code and Loom Native Tool adapters, component sandbox reports,
authoritative recovery commands, Queue/Steer/Inject, Swift governance controls,
installed CV6 and live mixed-Team acceptance remain open. No App bundle was
built, signed, launched or installed, and no real credential, Provider or user
workspace was accessed.
