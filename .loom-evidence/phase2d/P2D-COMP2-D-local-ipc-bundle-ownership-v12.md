# P2D-COMP2-D Local IPC Bundle Ownership V12

**Date**: 2026-08-14  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / PRODUCTION CONSTRUCTION MIGRATED / COMP2-E OPEN  
**Contract**: `contracts/P2D-COMP2-product-daemon-strangler.md`  
**Decision**: `../../docs/adr/0021-governed-composition-kernel-and-daemon-strangler.md`

## Accepted boundary

Production no longer constructs `newProductRouteHandler` or applies its IPC
decorators before Composition activation. The product builder now supplies a
typed `productLocalIPCHandlerSlot` and no-I/O factory to the built-in
`loom-local-ipc` Bundle. During Bundle Start, Setup is constructed first, then
the typed route registry handler is created and decorated in the unchanged
base -> observability -> controlled-journey order. Bundle Ready requires both
the Setup route and handler slot to be ready before the activation can admit a
request.

The registered `CapabilityLocalIPCHandler` is the stable revocable slot, not an
ambient service locator. It carries only a `localipc.Handler` reference. It
does not carry Prompt, transcript, Provider body, credential, VMK, Journal
writer, policy/grant authority, StateWriter, or Run/Attempt terminal authority.
Bundle cleanup first revokes and quiesces IPC admission, then closes Setup.
Construction failure closes both boundaries and leaves the handler slot
permanently unavailable.

The direct handler argument on `activateProductCompatibilityComposition`
remains only as the COMP2-A compatibility/parity entry. Activation accepts
exactly one handler source: either that direct facade or the Bundle-owned
slot/factory. Ambiguous and absent sources fail closed.

## RED and verification

The mandatory source RED failed because production still called
`newProductRouteHandler` before activation and supplied neither the local IPC
slot nor factory. After migration, the following gates pass:

```text
go test ./cmd/loomd -run TestCOMP2DProductionConstructsLocalIPCHandlerInsideBundle -count=1
go test ./cmd/loomd -run 'TestCOMP2DLocalIPC|TestCOMP2DProductionConstructsLocalIPCHandlerInsideBundle|TestCOMP2ACompatibility' -count=1
go test ./cmd/loomd -run 'TestCOMP2CProductionUsesObservabilitySlotAfterBootstrap|TestCOMP2BProductStartupCannotCallPositionalCompatibilityHandler|TestCOMP2DLocalIPC|TestCOMP2DProductionConstructsLocalIPCHandlerInsideBundle' -count=1
go test ./cmd/loomd -count=1
go test -race ./cmd/loomd -run 'TestCOMP2DLocalIPC|TestCOMP2DProductionConstructsLocalIPCHandlerInsideBundle|TestCOMP2AProductionBuilderActivatesDesktopFacade' -count=1
go test -race ./internal/composition -count=1
go test -p 1 ./... -count=1
go vet ./...
git diff --check
```

Tests prove Bundle-only production construction, one Start-time factory call,
direct-facade parity retention, source ambiguity rejection, construction
failure rollback, old-reference revocation, in-flight request quiescence,
observability decorator ownership, product startup, shutdown, race, and full
repository compatibility.

## Remaining boundary

This closes the final local IPC construction ownership slice of COMP2-D. It
does not authorize COMP2-E removal. `legacy.product.dispatch`, the direct
COMP2-A handler facade, compatibility decoders, and the old behavior oracle
remain until restart, crash-window, privacy, shutdown, and installed App
startup parity all pass. Installed CV6, ATL9 mixed-Team live acceptance,
explicit recovery/UI, accounting, and real Provider replies also remain open.

No App was built, signed, installed, or launched. No network, Provider,
credential, user workspace, or external Runtime was accessed.
