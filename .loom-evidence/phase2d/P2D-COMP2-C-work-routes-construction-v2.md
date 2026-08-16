# P2D-COMP2-C Work Routes Construction V2

**Date**: 2026-08-14  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / COMP2-C CONTINUES  
**Contract**: `contracts/P2D-COMP2-product-daemon-strangler.md`  
**Decision**: `../../docs/adr/0021-governed-composition-kernel-and-daemon-strangler.md`

## Accepted boundary

`loom-work` now constructs Queue, Workers, and Integration as one atomic typed
route set during Bundle Start. The factory creates the existing Queue service,
Worker execution/service API, Integration service, domain observability service,
and local Integration API. Product construction no longer creates any of these
objects directly.

All three ports must be valid before Ready. Failed Queue, Workers, or Integration
construction aborts activation before Product scope and IPC admission while
preserving the exact `build_queue`, `build_workers`, or `build_integration`
failure identity. The reversible Effect holds one route slot lock, waits for
in-flight calls on all six methods, then revokes the complete route set.

Production dispatch receives only `productQueueRoute`, `productWorkersRoute`,
and `productIntegrationRoute`. Legacy helpers still accept the concrete APIs and
preserve typed-nil unavailable behavior. No service object, request content,
Journal writer, authority, Provider, credential, Prompt, or Context Capsule is
stored in Capability Context.

## Verification

```text
go test ./cmd/loomd -run '^(TestCOMP2CWork|TestCOMP2CAgentRuntime|TestCOMP2CProductionBuilder|TestSF[123])' -count=1
go test -race ./internal/composition ./cmd/loomd -run '^(TestCOMP1|TestCOMP2[ABCD]|TestProductMissionExecutionCompositionReconcilesExactJournalLineageBeforeIPC|TestProductDaemonClassifiesLifecycleFailureBoundaries)' -count=10
go test ./cmd/loomd
go test ./...
go vet ./...
git diff --check
```

Focused tests prove atomic Ready, failed-Start rollback, exact build-stage
classification, Product-scope non-admission, route revocation,
Assets -> Work -> Agent Runtime order, reverse cleanup, and AST-level absence of
all migrated constructors from production construction. Real socket Queue,
Workers, and Integration journeys pass. Full daemon and repository suites
preserve startup, shutdown, routing, diagnostics, Agent execution, and privacy
behavior.

## Open boundary

This does not complete `loom-work` or COMP2-C/D. At this V2 boundary, Execution,
Production, governance/work authority, saved-Team setup/read routes, and deeper
scopes remained. Execution ownership was subsequently migrated and source
verified by `P2D-COMP2-C-work-execution-ownership-v3.md`; the other listed
boundaries remain open.
Operational diagnostics retain the documented pre-composition bootstrap
dependency until Conversation construction can migrate with lossless lifecycle
recording.

COMP2-E, installed CV6, mixed-Team ATL9, and all live UI gates remain open. No
App was built, signed, installed, or launched. No credential, Provider, network,
user workspace, or external tool was accessed.
