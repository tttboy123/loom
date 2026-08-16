# P2D-COMP2-D Pi Tool Process Resource Cleanup V8

**Date**: 2026-08-14  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / COMP2-D PARTIAL  
**Contract**: `contracts/P2D-COMP2-product-daemon-strangler.md`  
**Decision**: `../../docs/adr/0021-governed-composition-kernel-and-daemon-strangler.md`

## Accepted boundary

The Pi 0.82.1 governed Tool path already constructs all per-call resources
under the execution context passed into the Runtime Adapter:

```text
Agent Attempt execution context
  -> Pi child process group
  -> owner-only extension file
  -> owner-only Unix socket and accepted connection
  -> optional short-path /tmp socket directory
  -> pending governed Tool request
```

V4 proves that real Team mission delegates receive the Attempt-owned context
and that Team/Product close cancels it. This V8 acceptance drives the real Pi
Adapter with a child process that enters its private Tool UDS and remains in an
`ask` Tool request. Cancellation causes the Tool request to return a
content-free denial, performs the native Pi abort handshake, reaps the process
group within the configured bound, closes the listener/connection, and removes
the extension file, private root, socket, and fallback socket directory.

The child intentionally remains alive after acknowledging abort, so Adapter
return proves process termination rather than natural child exit. The test also
checks that the private command does not appear in the cancellation response.
No generic Effect registrar, ambient environment channel, or Capability Context
authority was added. Cleanup remains owned by the Runtime invocation and is
triggered by the exact Attempt execution context.

## Verification

```text
go test ./internal/runtime/piadapter -run '^TestPiRPCToolAttemptCancellationReapsProcessAndExtensionResources$' -count=1 -v
go test ./internal/runtime/piadapter -run 'TestPiRPC(ToolAttemptCancellationReapsProcessAndExtensionResources|ToolExecuteReturnsNativeResultBeforeFinalOutput|BridgeCancellationAcknowledgementAndCleanup)' -count=1
go test -race ./internal/runtime/piadapter -run '^TestPiRPCToolAttemptCancellationReapsProcessAndExtensionResources$' -count=10
go test ./internal/runtime/piadapter -count=1
go test ./... -count=1
go vet ./...
git diff --check
```

All commands pass. The focused process test completes in under the three-second
bound; the 10-run race suite reports no races. Full Pi Adapter and repository
parity remain green.

## Remaining boundary

This closes source verification only for the current Pi 0.82.1 governed Tool
child, UDS, and temporary extension resources. It does not prove cleanup for
Claude Code, Codex, Loom Native, future Pi versions, arbitrary MCP component
processes, App/daemon crash restart, or installed execution. Those adapters and
crash/restart residue gates remain open, along with COMP2-E, CV6, ATL9,
UI/accounting, and installed live acceptance.

No App was built, signed, installed, launched, or changed. No real credential,
Provider, network, user workspace, MCP server, or external tool was accessed.
