# P2D-COMP2-C Work Queue Construction V1

**Date**: 2026-08-14  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / COMP2-C CONTINUES  
**Contract**: `contracts/P2D-COMP2-product-daemon-strangler.md`  
**Decision**: `../../docs/adr/0021-governed-composition-kernel-and-daemon-strangler.md`

## Accepted boundary

`loom-work` now constructs the existing local Queue service/API during Bundle
Start and binds it into a one-bind `productWorkRoutes` slot. The slot is the
typed expansion point for later Workers, Integration, Execution, and Production
ports owned by the same Bundle. Product construction no longer creates the
Queue service/API directly and production dispatch receives only the slot.

Ready validates the frozen Queue port. Failed construction records a failed
`loom-work` Start and prevents Product scope and IPC admission. The reversible
Effect waits for in-flight Queue requests before revoking the route; retained
slot references fail closed. Existing compatibility helpers still accept
`*api.LocalQueueAPI`, including exact nil/unavailable behavior.

The Bundle graph starts Assets, then Work, then Agent Runtime. Focused tests
freeze this order and prove reverse cleanup closes Agent Runtime before Work is
revoked. No Queue object, request content, Journal writer, authority, Provider,
credential, Prompt, or Context Capsule is placed in Capability Context.

## Verification

```text
go test ./cmd/loomd -run '^TestCOMP2CWork|^TestCOMP2CProductionBuilderDoesNotConstructQueue' -count=1
go test -race ./internal/composition ./cmd/loomd -run '^(TestCOMP1|TestCOMP2[ABCD]|TestProductMissionExecutionCompositionReconcilesExactJournalLineageBeforeIPC)' -count=10
go test ./cmd/loomd
go test ./...
go vet ./...
git diff --check
```

The focused tests prove Bundle-Start construction, Ready admission, failed-Start
rollback, Product-scope non-admission, route revocation, Assets/Work/Runtime
ordering, and AST-level prohibition on production directly constructing Queue.
The full daemon and repository suites preserve Queue journeys, route admission,
startup, shutdown, Agent execution, diagnostics, and privacy behavior.

## Open boundary

This Queue-only boundary was extended by
`P2D-COMP2-C-work-routes-construction-v2.md`, which adds Workers and Integration
to the same atomic Work route set and preserves their exact build stages.

This does not complete `loom-work` or COMP2-C/D. Execution, Production,
governance/work authority, setup/read routes, and deeper scopes remain.
Operational diagnostics also remain bootstrap-owned: Conversation migration,
Agent diagnostics, handler wrapping, and Composition lifecycle recording need
the store before Bundle activation. The bounded follow-up is to retain a
minimal pre-composition recorder until Conversation construction migrates, then
bind operational ports into `loom-observability` without dropping startup
failures.

COMP2-E, installed CV6, mixed-Team ATL9, and all live UI gates remain open. No
App was built, signed, installed, or launched. No credential, Provider, network,
user workspace, or external tool was accessed.
