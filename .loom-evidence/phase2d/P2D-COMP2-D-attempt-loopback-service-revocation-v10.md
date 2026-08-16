# P2D-COMP2-D Attempt Loopback Service Revocation V10

**Date**: 2026-08-14  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / COMP2-D PARTIAL  
**Contract**: `contracts/P2D-COMP2-product-daemon-strangler.md`  
**Decision**: `../../docs/adr/0021-governed-composition-kernel-and-daemon-strangler.md`

## Accepted boundary

The current Claude Code and Codex Harness paths use two Attempt-private
loopback services:

```text
Agent Attempt execution context
  -> credential gateway listener and opaque one-time token
  -> Harness Context MCP listener and opaque bearer token
  -> Harness runner
```

Both services are now parented directly by the Attempt execution context.
Cancelling that context closes each listener immediately, even when the trusted
gateway callback or Harness runner intentionally remains blocked. Normal return
retains bounded graceful shutdown.

The credential gateway previously waited for its trusted callback to return
before closing the HTTP server. A new cancellation watcher now performs
immediate server close while preserving existing Provider failure
classification, callback result handling, and final token/secret clearing.

The Context MCP service previously depended on an outer Adapter defer, so its
listener and token could survive until a non-cooperative runner returned. Its
production constructor now receives the Attempt context. Cancellation performs
immediate server close and clears the token under the same mutex used by lease
and authorization reads. Repeated explicit close remains idempotent.

The production Codex Adapter test proves the Context MCP listener is revoked
before a deliberately blocked runner is released. The lower-level tests prove
the same boundary for the credential gateway and verify that Context MCP token
revocation is observable without exposing token contents.

No Provider credential, gateway token, Context MCP token, Prompt, Context
Capsule content, Provider response, or child output is written to diagnostics,
Journal, Evidence, or this artifact.

## Verification

```text
go test ./internal/runtime/harnessadapter -run 'Test(AttemptGatewayCancellationRevokesListenerBeforeCallbackReturns|HarnessContextMCPCancellationRevokesListenerAndToken|CodexAdapterCancellationRevokesContextMCPBeforeRunnerReturns)' -count=1 -v
go test ./internal/runtime/harnessadapter -run 'Test(AttemptGateway|OpenAIAttemptGateway|AnthropicAttemptGateway|HarnessContextMCP|CodexAdapter.*Context|ClaudeCodeAdapter.*Context)' -count=1 -v
go test -race ./internal/runtime/harnessadapter -run 'Test(AttemptGatewayCancellationRevokesListenerBeforeCallbackReturns|HarnessContextMCPCancellationRevokesListenerAndToken|CodexAdapterCancellationRevokesContextMCPBeforeRunnerReturns)' -count=10
go test ./internal/runtime/harnessadapter -count=1
go test ./... -count=1
go vet ./...
git diff --check
```

The focused tests were captured RED before each lifecycle hook was added and
GREEN after implementation. All final commands pass; the 10-run race suite
reports no races.

## Remaining boundary

This is normal-process Attempt cancellation, not crash recovery. The trusted
credential callback must still return before its bounded plaintext secret copy
can be cleared. Daemon crash, `kill -9`, restart residue, installed CLI/Provider
compatibility, arbitrary MCP component processes, Harness-owned user workspace
rollback, and other Runtime adapters remain open.

COMP2-D, COMP2-E, CV6, ATL9, UI/accounting, and installed acceptance are not
complete. No App was built, signed, installed, launched, or changed. No real
credential, Provider, network, user workspace, Claude CLI, Codex CLI, or MCP
server was accessed.
