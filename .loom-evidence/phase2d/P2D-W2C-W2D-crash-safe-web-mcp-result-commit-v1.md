# P2D-W2C/W2D crash-safe Web/MCP result commit v1

Date: 2026-08-14

Status: `SOURCE VERIFIED / PRODUCTION BROKER CONFIG AND INSTALLED LIVE OPEN`

This evidence belongs to the sole Phase 2D Goal. It does not create a new Goal
or close P2D-W2C/P2D-W2D.

## User-visible outcome

Loom can now carry an injected WebSearch, WebFetch, or MCPTool result through
the same governed Pi native continuation path as local Read/Grep without
placing the query, arguments, or returned content in the Event Journal,
Evidence metadata, operational diagnostics, ordinary RunStream frames, or the
execution cache.

Production remains fail closed and does not advertise remote tools until a
Loom-owned Broker is explicitly composed. No production Search backend or MCP
registry was configured, and no real network or MCP call was made in this
increment.

## Authority and crash boundary

1. The Broker strictly validates the complete Web/MCP proposal before dispatch.
2. `ToolDispatchCommitted` is authoritative before the remote executor runs.
3. Returned UTF-8 content is bounded and digest-checked in owned mutable bytes.
4. The exact Conversation/Attempt/Tool binding and encrypted result payload are
   committed before `ToolExecutionCompleted`.
5. `ToolResultAccepted` precedes the execution terminal fact. A persistence
   failure produces `result_persistence_failed`, zeroizes content, and replay
   does not call the remote tool again.
6. Pi decrypts only the accepted binding, rechecks the digest, returns a native
   `toolResult`, and records delivery only after validated final model output.

The remote operation identity uses the canonical call digest. Web query and MCP
arguments therefore affect the authority identity without being copied into
content-free facts.

## Capability publication

The remote executor publishes an exact immutable tool set. The execution
Adapter, daemon Hook, Pi extension JSON schema, Pi system prompt, pre-dispatch
validation, and strict transcript parser all derive from that same set.

- A Broker without an MCP allowlist publishes WebSearch and WebFetch only.
- An undeclared remote tool fails before validation, dispatch, or execution.
- A Hook that publishes an unknown tool makes the Pi extension unavailable.
- Legacy/local-only Hooks retain the frozen Bash/Edit/Read/Grep set.
- Remote result-content or digest substitution fails closed.

## Verification

Passed:

```text
go test ./...
go test -race ./internal/runtime/piadapter ./internal/execution ./internal/toolbroker ./cmd/loomd
go vet ./...
git diff --check
```

Focused tests cover pre-dispatch Broker validation, exact capability
publication, result commit before terminal authority, persistence failure and
no remote replay, product Attempt event ordering, encrypted payload delivery,
Pi dynamic schema/prompt generation, local-only rejection, Bridge privacy, and
remote content substitution.

The privacy scan found test markers only in test fixtures and existing safe
authorization field names; it found no remote query/result content field in
Journal or Evidence schemas.

## Open gates

- production Search backend and MCP registry composition;
- configured-provider capability projection and governance UI;
- multiple sequential ToolCalls in one Attempt;
- other Runtime Adapter continuation paths;
- component sandbox reports and authoritative recovery controls;
- installed App live `WebSearch -> WebFetch -> final` and MCP acceptance;
- Credential Vault CV6 and mixed-Team live acceptance.

Source remains post-build-64. Installed Loom remains v0.5.2 build 39.
