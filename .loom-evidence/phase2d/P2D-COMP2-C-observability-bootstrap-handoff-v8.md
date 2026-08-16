# P2D-COMP2-C Observability Bootstrap Handoff V8

**Date**: 2026-08-14  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / COMP2-C CONTINUES  
**Contract**: `contracts/P2D-COMP2-product-daemon-strangler.md`  
**Decision**: `../../docs/adr/0021-governed-composition-kernel-and-daemon-strangler.md`

## Accepted boundary

The owner-only operational diagnostic store remains a minimal pre-composition
bootstrap dependency so credential-runtime selection, encrypted Conversation
migration, and Composition compile/start failures cannot disappear before
Bundles are Ready. `loom-observability` Start now performs an explicit handoff
of bounded operational ports into `productObservabilityRouteSlot`.

After that handoff, product Read diagnostics, Agent Attempt diagnostics,
Context Capsule retrieval audit, governed tool diagnostics, and local IPC
operation recording use only the slot. They no longer retain the bootstrap
store directly. Composition lifecycle and pre-Ready migration diagnostics keep
their deliberate direct bootstrap recorder because they must survive a failed
Observability Start.

The slot exposes no diagnostic path, file handle, Journal writer, Provider
client, credential, Prompt, transcript, tool result, Provider body, or terminal
authority. It is not registered in Capability Context. Bundle failure prevents
Product scope and IPC admission, preserves `build_diagnostics`, and closing the
Composition revokes all downstream diagnostic/query ports. The IPC wrapper
captures only the bounded recorder before dispatch and does not hold the slot
lock across request execution.

## Verification

```text
go test ./cmd/loomd -run '^(TestCOMP2CObservabilityBootstrapHandoff|TestCOMP2CProductionUsesObservabilitySlotAfterBootstrap)' -count=1
go test ./cmd/loomd -run '^(TestCOMP2CObservabilityBootstrapHandoff|TestCOMP2CProductionUsesObservabilitySlotAfterBootstrap|TestCOMP2C|TestProductOperationalDiagnostics|TestProductDaemonServesRealReadOnlySQLiteOverPrivateUDSAndCleansUp)' -count=1
go test -race ./internal/composition ./cmd/loomd -run '^(TestCOMP1|TestCOMP2[ABCD]|TestProductOperationalDiagnostics)' -count=10
go test ./cmd/loomd -count=1
go test ./...
go vet ./...
git diff --check
```

Focused tests prove one-time bind, downstream IPC recording, diagnostic query,
atomic Start failure, exact failure-stage mapping, close revocation, and
AST-level production ownership. The real UDS journey, ten-run race suite, full
daemon suite, full repository suite, vet, and whitespace checks pass.

## Open boundary

This did not complete `loom-observability` or COMP2-C/D at V8. V10 subsequently
moved Vault/Conversation construction and V11 moved persistent store
construction into `loom-observability` with a bounded pre-start handoff.
Diagnostic export/UI, account-level accounting projections, deeper
Conversation/Team/Agent/Attempt scopes, and support-package acceptance remain
open.

Setup/read construction, COMP2-E, installed CV6, mixed-Team ATL9, and all live
UI gates remain open. No App was built, signed, installed, or launched. No
credential, Provider, network, user workspace, or external tool was accessed.
