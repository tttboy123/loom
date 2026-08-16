# P2D-COMP2-D Pi Context Extension Revocation V11

**Date**: 2026-08-14  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / COMP2-D PARTIAL  
**Contract**: `contracts/P2D-COMP2-product-daemon-strangler.md`  
**Decision**: `../../docs/adr/0021-governed-composition-kernel-and-daemon-strangler.md`

## Accepted boundary

The Pi Context extension now inherits the Agent Attempt execution context at
its production construction boundary:

```text
Agent Attempt execution context
  -> Pi Context delivery
  -> private Context UDS
  -> generated owner-only extension file/root
```

Previously the production Pi RPC Adapter created the Context extension without
the Attempt context. The extension was closed only by an Adapter defer after
the child process returned. An active Context delivery could therefore outlive
Attempt cancellation while a runner or process return was delayed.

The existing standalone constructors remain available for bounded unit tests
and use a background lifetime. The production RPC Adapter now uses the new
context-bound constructor. Parent cancellation calls the same idempotent
`Close()` path used by ordinary return. That path closes the listener and
accepted connection, cancels the in-flight delivery through the existing
shutdown context, waits for the serving goroutine, and removes the socket,
optional short socket root, extension file, and private extension root.

The RED introduced a delivery that blocks until its context is cancelled. It
also keeps the Pi connection active and records all generated private paths.
Before implementation no context-bound constructor existed. After the change,
Attempt cancellation delivers `context.Canceled` and removes all resources
without waiting for the outer Adapter defer.

Capability Context, Go context, Attempt Context, and Context Capsule remain
separate. No Context content, Prompt, Provider response, credential, capability
value, socket payload, or child output enters diagnostics, Journal, Evidence,
or this artifact.

## Verification

```text
go test ./internal/runtime/piadapter -run '^TestPiContextExtensionAttemptCancellationRevokesActiveDeliveryAndResources$' -count=1 -v
go test ./internal/runtime/piadapter -run 'Test(PiContextExtension|PiRPC.*Context|LockedPi0821Context)' -count=1 -v
go test -race ./internal/runtime/piadapter -run '^TestPiContextExtensionAttemptCancellationRevokesActiveDeliveryAndResources$' -count=10
go test ./internal/runtime/piadapter -count=1
go test ./... -count=1
go vet ./...
git diff --check
```

The focused test was captured RED at compile time before the context-bound
constructor and GREEN after implementation. The protocol suite, 10-run race,
full Pi Adapter, full repository, vet, and diff checks pass.

`TestLockedPi0821ContextExtensionTwoTurnContract` was selected but skipped by
its existing locked-Pi environment gate. It is not counted as live binary
acceptance.

## Remaining boundary

This proves normal-process Attempt cancellation for the Pi Context extension.
It does not prove daemon crash, `kill -9`, restart residue recovery, arbitrary
MCP component processes, locked Pi 0.82.1 binary execution, other Runtime
adapters, or installed App behavior.

COMP2-D, COMP2-E, CV6, ATL9, UI/accounting, and installed acceptance remain
open. No App was built, signed, installed, launched, or changed. No real
credential, Provider, network, user workspace, Pi binary, or MCP server was
accessed.
