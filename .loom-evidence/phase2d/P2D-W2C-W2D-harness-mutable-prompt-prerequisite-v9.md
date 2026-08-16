# P2D-W2C/W2D Harness Mutable Prompt Prerequisite V9

Status: `SOURCE VERIFIED PREREQUISITE / CODEX AND CLAUDE CONTINUATION OPEN`

Date: 2026-08-14

## Finding

The current production Codex path launches `codex exec --ephemeral ... -`; the
current Claude Code path launches `claude --print --no-session-persistence`.
Both use one stdin prompt and return after one process result. Re-running either
command would create another one-shot process and would not prove same-Harness
continuation. Neither Adapter therefore advertises `AgentInputConsumer`.

Phase 2D must not call repeated `RunHarness` invocations Queue/Steer/Inject.
Codex requires a version-locked persistent app-server/stream contract; Claude
Code requires a version-locked persistent stream-json contract. Each needs
process identity, native session-handle binding, exact output checkpoint,
cancellation, accounting, privacy, and restart tests before capability
publication.

## Implemented prerequisite

`HarnessProcessRequest.Prompt` and `HarnessCommandRequest.Stdin` now use owned
mutable `[]byte` rather than immutable Go strings. The Codex and Claude Adapters
create a bounded mutable dispatch copy and clear it after runner completion.
Both Provider-specific process runners clear the prompt on every return path;
the system command runner clears stdin after validation, execution, failure, or
cancellation.

Prompt bytes remain stdin-only. They are not added to argv, environment,
diagnostics, Journal, Evidence, or ordinary Bridge metadata. Test command
recorders explicitly clone synthetic prompt bytes before the production owner
clears them, and the process/system tests assert the original slices are all
zero after return.

This is a transport-safety prerequisite, not Codex/Claude Agent-input support.
No process/session resume protocol, ExternalSessionHandle, new capability, or
installed Runtime behavior is claimed.

## Verification

```text
go test ./internal/runtime/harnessadapter -count=1
go test -race ./internal/runtime/harnessadapter \
  -run 'Test(SystemHarnessCommandRunnerUsesExactExecutableAndBoundedIO|ClaudeCodeProcessUsesAttemptGatewayWithoutExposingProviderSecret|CodexProcessUsesExactGatewayConfigAndClosedJSONL|ClaudeCodeAdapterConsumesOnlyItsFrozenAnthropicAccount)$' \
  -count=10
go vet ./...
git diff --check
go test -p 1 ./... -count=1
```

All gates passed. No App, network, real Provider, credential, user workspace,
Codex process, or Claude Code process was accessed.
