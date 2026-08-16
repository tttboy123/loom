# P2D-COMP2-D Attempt Cancellation Ownership V4

**Date**: 2026-08-14  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / COMP2-D PARTIAL  
**Contract**: `contracts/P2D-COMP2-product-daemon-strangler.md`  
**Decision**: `../../docs/adr/0021-governed-composition-kernel-and-daemon-strangler.md`

## Accepted boundary

Each real Team Agent Attempt now owns a cancellable Go execution context as a
Composition Effect. The context is created only after the immutable
`FrozenExecutionBinding` and Attempt scope have been admitted. A dedicated
mission Attempt lease exposes that context to `productMissionScopedExecutor`;
the delegate never receives the wider Team runner context.

Attempt close cancels the execution context with `composition.ErrScopeClosed`.
This occurs after an ordinary delegate return and also when the Team/Product
scope is revoked while the delegate is still running. The latter actively
unblocks a waiting executor before Team cleanup completes. The Effect is
idempotent and remains owned by the Attempt rather than by the executor or a
global registry.

`CapabilityContext`, Go `context.Context`, Attempt Context, and Context Capsule
remain distinct contracts. The Capability scope owns cancellation lifetime; the
Go context carries only cancellation/deadline propagation. It does not carry a
Prompt, transcript, Provider body, secret, credential, VMK, Journal writer,
StateWriter, or execution authority.

## RED and verification

The RED proved that closing a Team scope did not cancel an in-flight delegate.
The implementation passed:

```text
go test ./cmd/loomd -run 'TestCOMP2D(AttemptOwns|ScopedMission)' -count=1
go test ./cmd/loomd -run '^TestCOMP2D' -count=1
go test -race ./cmd/loomd -run '^TestCOMP2D' -count=10
go test ./cmd/loomd -count=1
go test ./... -count=1
go vet ./...
git diff --check
```

Tests prove normal-return cancellation, Team-close interruption of an active
delegate, stable `ErrScopeClosed` cause, idempotent close, existing scope and
mission behavior, race safety, and repository parity.

## Remaining boundary

COMP2-D remains open. Credential leases, Provider native-session handles, Tool
Loop channels, component processes, temporary roots, and other exact Attempt or
Turn resources are not yet registered as Composition Effects. Multi-turn Turn
generation, restart/crash cleanup, COMP2-E, installed CV6, mixed-Team ATL9,
UI/accounting, and live Provider acceptance remain mandatory.

No App was built, signed, installed, or launched. No credential, Provider,
network, user workspace, or external tool was accessed.
