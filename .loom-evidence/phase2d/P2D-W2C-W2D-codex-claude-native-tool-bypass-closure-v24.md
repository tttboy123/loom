# P2D-W2C/W2D Codex and Claude Native-Tool Bypass Closure V24

**Date**: 2026-08-15  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / EXACT EXECUTABLE COMPONENT RECHECK OPEN  
**Related**: `contracts/P2D-W2C-agent-attempt-binding-dispatch.md`, `contracts/P2D-W2D-observability-governance.md`

## Accepted boundary

V23's private MCP allowlist is now also enforced at the Provider trust
boundary. Each credential-gateway call receives an exact per-Attempt policy
derived from the same `HarnessContextMCPLease` used to configure the Harness.
The gateway copies and validates only these current Loom tools:

```text
loom_read_context
loom_read_file
loom_grep_files
```

OpenAI and Anthropic request tool catalogs must be a subset of that frozen
policy. Native, mixed, unknown and unapproved Loom tools fail before the
credential is attached to an upstream request. Duplicate JSON keys fail closed
recursively, preventing Loom and a Provider from interpreting `model`, `tools`
or tool identity differently. Dynamic Codex `tool_search` is not permitted.

Successful Provider responses are also untrusted. OpenAI `function_call` and
Anthropic `tool_use` output must target the same exact Loom MCP namespace and
tool set. Codex shell/web/custom calls, Claude native/server tool use and future
unknown call-shaped output are rejected before the response reaches the
Harness.

Codex exec and app-server now run from the private Attempt temp directory with
read-only sandboxing. Exact strict config disables shell, unified exec,
freeform apply-patch and tool search. Claude one-shot and stream-json run from
the same private directory with `dontAsk`, an MCP-only tool list and an
explicit native-tool denylist. The generated system prompt advertises only
Loom-governed MCP authority, not direct Bash/Edit/Read/Web authority.

The gateway remains the final authority boundary: if a locked executable
ignores a CLI restriction or advertises a native tool, the request fails before
Provider dispatch. If strict config itself is rejected, the Harness process
fails closed. No fallback broadens tools or changes Provider/Account/Model.

## RED evidence

The request-side RED reached the fake Provider once for both Codex `shell` and
Claude `Read`. Duplicate `model/tools/type` envelopes and Codex `tool_search`
also reached upstream before the implementation. The response-side RED passed
Codex `local_shell_call` and Claude native `tool_use` bytes back to the fake
Harness. The process-contract RED showed all four modes still used workspace
CWD and native/write-capable settings.

After implementation, the same cases return `ErrHarnessProtocol`; rejected
requests produce zero Provider calls and rejected responses do not expose raw
Provider bytes to the Harness. Exact Loom request and response tool calls remain
accepted.

## Verification

```text
go test ./internal/runtime/harnessadapter -count=1
go test ./internal/runtime -count=1
go test ./internal/runtime/piadapter ./internal/runtime/harnessadapter ./cmd/loomd -count=1
go test -race ./internal/runtime/harnessadapter -run 'Test(AttemptGateway|ClaudeCodeProcessUsesAttemptGatewayWithoutExposingProviderSecret|ClaudeCodeProcessInjectsGovernedAttemptMCPTools|ClaudeStreamJSONContinuesAgentInputInOneSession|CodexProcessUsesExactGatewayConfigAndClosedJSONL|CodexProcessInjectsGovernedAttemptMCPTools|CodexAppServerContinuesAgentInputOnOneThread|CodexAdapterConsumesExactFrozenOpenAIAccount|ClaudeCodeAdapterConsumesExactFrozenAnthropicAccount)' -count=10
go vet ./internal/runtime ./internal/runtime/harnessadapter ./internal/runtime/piadapter ./cmd/loomd
gofmt -l <V24 source and test files>
git diff --check
```

The complete Harness package, affected runtime/Pi/daemon package matrix,
ten-run race gate, vet, formatting, privacy scan and diff check pass. Tests bind
the exact per-Attempt policy, all four process modes, request and response
catalogs, mixed/unapproved tools, dynamic search and ambiguous JSON.

## Remaining boundary

This is source verification only. The exact external Codex 0.144.1 and Claude
Code 2.1.196 processes were not launched during this slice. Their new strict
argument combinations and eager MCP catalog behavior require the gated
component test before installed/live acceptance. No App was built, signed,
installed or launched. No network, Provider, real credential, user workspace
or external Runtime was accessed.

This slice does not expose Bash/Edit/Web/arbitrary MCP, implement broader
Runtime restart, close CV6 or mixed-Team ATL9, finish accounting/UI, or remove
the COMP2 legacy path. Phase 2D remains the sole `ACTIVE / PARTIAL` Goal.
