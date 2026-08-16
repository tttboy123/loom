# P2D-W2D Attempt Local Tool Gateway V1

Date: 2026-08-14

Status: SOURCE VERIFIED / PROVIDER CONTINUATION OPEN

Goal: Phase 2D remains the sole active Goal. This is an incremental W2C/W2D
vertical slice and does not change Phase 2C or claim installed acceptance.

## Boundary

- The production Attempt Runtime injects exact Attempt/Turn/Step identity into
  daemon-private context after Run start.
- Pi Bash/Edit calls use a per-proposal execution dispatch gate.
- `ToolCallAdmitted` and `ToolDispatchCommitted` precede the executor call.
- A failed dispatch gate records `dispatch_not_committed` and invokes no side
  effect.
- One-shot approved resume carries exact approval ID/digest into dispatch.
- Digest-only execution metadata is persisted in the Conversation-DEK Attempt
  Payload Store and accepted against the exact dispatch event.
- The bridge result frame contains call digest, Tool, and digest-only result;
  it omits command, path, Prompt, output body, credentials, and Provider body.
- RunStream frame acceptance produces `run_stream_tool_result` and marks the
  encrypted payload delivered.

## Security meaning

`run_stream_tool_result` proves only that Loom's RunStream sink accepted the
bounded result frame. It is not Provider continuation, Harness final-output
proof, or evidence that a model consumed the result. Full multi-round Pi still
requires a result transport back into the child runtime followed by a verified
continuation/final output.

An execution crash after dispatch is never automatically replayed. Existing
execution replay sees an allowed non-terminal operation as interrupted and
fails closed. The complete user-facing uncertain-side-effect recovery workflow
and tool Incident diagnostics remain open.

## Verification

- `go test ./internal/execution -count=1`
- `go test ./internal/runtime/piadapter -count=1`
- `go test ./internal/work -run 'TestAttemptLoop|TestAttemptPayload' -count=1`
- `go test ./cmd/loomd -run 'TestProductAttemptLoopRuntimeGoverns|TestP2DWBridge|TestWBridgeHook' -count=1`
- focused 10-run race over execution and daemon Tool Gateway tests
- `go vet ./...`
- `git diff --check`

The source tests prove dispatch ordering, zero side effects on authority
failure, exact approved dispatch binding, encrypted payload acceptance,
RunStream delivery, command/path absence from Journal/result frames, and the
existing Context delivery path remaining intact.

## Open

- ask pause/resume and native approval controls;
- Pi result injection and real model continuation;
- Read/Grep execution and Web/MCP Broker composition;
- Codex, Claude Code, and Loom Native local Tool adapters;
- sandbox enforcement reports and workspace-drift evidence;
- tool-level operational diagnostics and uncertain-side-effect recovery UI;
- Queue/Steer/Inject;
- installed CV6 and mixed-Provider Team live acceptance.
