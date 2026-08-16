# P2D-COMP2-C Read Construction V13

**Date**: 2026-08-14  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / COMP2-C-D CONTINUES  
**Contract**: `contracts/P2D-COMP2-product-daemon-strangler.md`  
**Decision**: `../../docs/adr/0021-governed-composition-kernel-and-daemon-strangler.md`

## Accepted boundary

The existing Read aggregate is now constructed atomically with
`loom-governance` after Governance routes bind and after Observability,
Conversation, and Vault are Ready. If Read construction fails, Governance is
closed in the same Start rollback and neither route becomes Ready. The
monolithic product builder no longer calls `api.NewLocalProductReadService`.

A revocable `productReadRouteSlot` exposes only snapshot, timeline, chat,
mission observer, SideTask source, diagnostic source, and observer-close ports.
The product handler consumes its snapshot/timeline/chat surface. Agent Runtime
consumes only `MissionExecutionObserver`, `SetSideTaskSnapshotSource`, and
`CloseMissionExecutionObservers`; it no longer receives a concrete
`LocalProductReadService`.

The concrete Read service, Journal, Projection, cached views, stream
subscriptions, tentative records, and observer map remain private to the
trusted built-in construction. None enters `CapabilityContext`. Governance
authority, Journal append authority, StateWriter, Vault root, credential,
Prompt, transcript, Provider body, or terminal Run/Attempt authority is not
exposed by the slot.

Reverse cleanup closes Agent Runtime first. Governance cleanup then revokes
Read, closes all residual mission observers, and finally closes Governance
routes. Read construction or rollback failure maps to `build_state`; existing
chat/diagnostics/SideTask behavior and UDS response contracts remain unchanged.

## Verification

```text
go test ./cmd/loomd -run '^(TestCOMP2CRead|TestCOMP2CProductionDoesNotConstructRead)' -count=1
go test ./cmd/loomd -run '^(TestCOMP2CRead|TestCOMP2CProductionDoesNotConstructRead|TestCOMP2CGovernance|TestCOMP2CAgentRuntime|TestProductDaemonServes|TestProduct.*Mission|TestProduct.*SideTask|TestLocalProductHandler)' -count=1
go test -race ./internal/composition ./internal/api ./cmd/loomd -run '^(TestCOMP1|TestCOMP2[ABCD]|TestProduct.*Mission|TestProduct.*SideTask|TestProductDaemonServes|TestLocalProductRead|TestChat|TestPersistent)' -count=10
go test ./cmd/loomd -count=1
go test ./...
go vet ./...
git diff --check
```

Tests prove Governance-before-Read construction, atomic rollback, typed
delegation, close revocation, observer cleanup, SideTask integration,
Conversation and diagnostics slot use, real product routes, and AST ownership.
Ten-run race, full daemon, full repository, vet, and whitespace checks pass.

## Open boundary

This does not complete COMP2-C/D. V14 subsequently moved the shared Pi
local-model resource under Conversation ownership. Deeper Conversation/Attempt
scopes, diagnostics
UI/export, account-level accounting, COMP2-E removal, installed CV6,
mixed-Team ATL9, and all live gates remain open.

No App was built, signed, installed, or launched. No credential, Provider,
network, user workspace, or external tool was accessed.
