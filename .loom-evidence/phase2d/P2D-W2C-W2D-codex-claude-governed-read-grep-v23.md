# P2D-W2C/W2D Codex and Claude Governed Read/Grep V23

**Date**: 2026-08-15  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / READ+GREP ONLY / WEB+MCP+INSTALLED LIVE OPEN  
**Related**: `contracts/P2D-W2C-agent-attempt-binding-dispatch.md`, `contracts/P2D-W2D-observability-governance.md`

## Accepted boundary

Codex and Claude Code now receive governed workspace Read and Grep through the
same Attempt-scoped private MCP boundary as Context retrieval. The common
proposal, frozen binding, result, delivery and acknowledgement contracts are
owned by `internal/runtime`; Pi retains aliases to those types and errors so its
existing protocol and `errors.Is` behavior do not change.

The Harness MCP exposes only these exact tools when their frozen capabilities
permit them:

```text
loom_read_context
loom_read_file
loom_grep_files
```

It does not expose Bash, Edit, Web search/fetch, arbitrary MCP or Provider-native
tools. Codex exec/app-server and Claude one-shot/stream-json derive their tool
allowlists from the same lease. The private bearer token remains environment
only and never enters process arguments.

Read and Grep require the separately audited exact executable conformance for
the selected Harness. Capability publication and adapter startup both fail
closed on executable identity drift. The adapter also verifies the exact
Attempt, claim, generation, Run, Agent, Runtime, Route Segment, Execution
Binding, Capsule and Incident lineage before opening the private MCP service and
before credential access.

The service uses the daemon's authoritative Attempt invocation context for
gateway execution while propagating HTTP cancellation into that context. A
loopback request context cannot supply or replace execution authority. Read and
Grep results are encrypted into the Attempt Payload store and remain
`accepted/pending` while the Harness runs. Only validated final output seals the
service and acknowledges every exact result in sequence with
`harness_final_output` proof.

The production daemon injects the existing `bridgeExecutionHook` into Codex and
Claude after constructing `AttemptLoopAuthority`. No second permission or
execution authority is introduced. Read/Grep calls use parallel admission with
a content-free conflict digest scoped to Attempt plus path: distinct paths may
be pending together, while Read/Grep against the same path conflict. Paths and
patterns do not enter the Event Journal.

## Verification

```text
go test ./internal/runtime ./internal/runtime/piadapter ./internal/runtime/harnessadapter ./cmd/loomd -run 'Test.*(ToolCall|Governed|MCP|ReadAndGrep|Conformance|Historical|Capability)' -count=1
go test ./internal/runtime ./internal/runtime/harnessadapter ./internal/runtime/piadapter ./cmd/loomd -count=1
go test -race ./internal/runtime/harnessadapter ./cmd/loomd -run 'TestHarnessAttemptMCPExecutesOnlyGovernedReadAndGrepUntilFinalOutput|TestHarnessAdaptersRouteGovernedReadAndGrepThroughAttemptMCP|TestCodexHarnessMCPUsesProductAuthorityAndDeliversAfterFinalOutput|TestProductAttemptReadAndGrepUsePathScopedParallelConflicts' -count=10
go vet ./internal/runtime ./internal/runtime/harnessadapter ./internal/runtime/piadapter ./cmd/loomd
gofmt -w internal/runtime/tool_gateway.go internal/runtime/piadapter/toolcall.go cmd/loomd/wbridge_wiring.go
git diff --check
```

The first process RED found hard-coded Context-only tool lists. The product
integration RED then found that the HTTP handler had lost the outer daemon
Attempt context; after restoring that authority, encrypted result reads exposed
the same issue and were corrected through one merged authority-and-cancellation
context. Neither fix relaxed identity validation.

Focused tests cover both Harnesses through real loopback MCP HTTP, all four
Harness process modes, exact executable drift, historical capability migration,
Pi alias compatibility and path-scoped conflicts. The product test uses the
real permission/execution adapter, Attempt Loop authority, encrypted payload
store, Read and Grep calls, pending-before-final state and ordered final-output
delivery. Complete package tests, vet and the ten-run race gate pass.

## Remaining boundary

This is source verification only. This slice does not expose Bash/Edit through
the new Harness MCP, does not expose Web/MCP or Provider-native tools, and does
not prove daemon-restart reattachment, TTL/compaction, accounting/UI, installed
CV6, mixed-Team ATL9 or COMP2-E. No App was built, signed, installed or launched.
No network, Provider, real credential, user workspace or external Runtime was
accessed. Phase 2D remains the sole `ACTIVE / PARTIAL` Goal.
