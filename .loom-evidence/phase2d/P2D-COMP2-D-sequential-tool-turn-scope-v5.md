# P2D-COMP2-D Sequential Tool Turn Scope V5

**Date**: 2026-08-14  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / COMP2-D PARTIAL  
**Contract**: `contracts/P2D-COMP2-product-daemon-strangler.md`  
**Decision**: `../../docs/adr/0021-governed-composition-kernel-and-daemon-strangler.md`

## Accepted boundary

The real mission Agent Attempt lease now owns the current Turn scope and can
advance it deterministically after a resolved governed tool call:

```text
Attempt -> Turn 1 -> resolved tool result -> Turn 2
                                      ... -> Turn N
```

`productMissionScopedExecutor` binds a private Turn controller into the
Attempt-owned Go execution context. `bridgeExecutionHook.ExecuteToolCall` reads
the already validated Pi tool sequence and advances only after the execution
result is resolved and all required encrypted Attempt payload persistence has
succeeded. `ask` polling remains in the current Turn. Allow and deny results
advance from generation N to N+1. Duplicate, stale, missing, cancelled, or
out-of-order sequences fail closed.

Advancing closes the current Turn before opening the next child under the same
immutable Attempt. The new Turn inherits the Attempt's Composition Snapshot and
full Frozen Execution Binding digests; neither can change during the sequence.
Turn IDs are domain-separated opaque hashes of internal execution lineage and
generation, so raw Team, Agent, Run, tool arguments, Prompt, or result content do
not enter Composition diagnostics.

The controller is a private lifecycle reference, not a Capability payload. It
does not merge Capability Context, Go `context.Context`, Attempt Context, or
Context Capsule. Journal, approval, encrypted payload, delivery proof, and tool
execution authorities remain in their existing owners.

## RED and verification

The RED failed on the missing Attempt Turn transition, execution-context
controller binding, and resolved-result integration. The implementation passed:

```text
go test ./cmd/loomd -run 'TestCOMP2D(AttemptAdvances|ToolResultAdvances|AttemptOwns|ScopedMission)' -count=1
go test ./cmd/loomd ./internal/runtime/piadapter -run '^(TestCOMP2D|TestWBridge|TestProduct.*Tool|TestP2D|TestPi.*Tool|TestAttempt.*Tool)' -count=1
go test -race ./cmd/loomd -run '^(TestCOMP2D|TestWBridgeHookAllowExecutes)' -count=10
go test ./cmd/loomd -count=1
go test ./... -count=1
go vet ./...
git diff --check
```

Tests prove Turn 1 to 2 to 3 progression, duplicate-sequence rejection without
corrupting the next valid transition, ask non-progression, a real WBridge allow
execution advancing the scoped Turn, mission context wiring, scope-close
cancellation, tool protocol parity, race safety, and repository parity.

## Remaining boundary

This slice does not claim the complete sequential ToolCall product gate. Pi's
bounded multi-call protocol, all tool-result acknowledgement/crash windows,
other Runtime adapters, and installed live execution remain under ATL3-ATL8.
Credential leases, Provider native-session handles, tool extension/channel and
component-process Effects, temporary roots, COMP2-E, installed CV6, mixed-Team
ATL9, UI/accounting, and live Provider acceptance remain open.

No App was built, signed, installed, or launched. No credential, Provider,
network, user workspace, or external tool was accessed.
