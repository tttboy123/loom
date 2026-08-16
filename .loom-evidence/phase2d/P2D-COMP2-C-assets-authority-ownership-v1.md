# P2D-COMP2-C Assets Authority Ownership V1

**Date**: 2026-08-14  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / COMP2-C CONTINUES  
**Contract**: `contracts/P2D-COMP2-product-daemon-strangler.md`  
**Decision**: `../../docs/adr/0021-governed-composition-kernel-and-daemon-strangler.md`

## Accepted boundary

`loom-assets` now constructs and owns the existing owner-only Evidence store,
Asset authority, local Assets service/API, Pi skill materializer, and Team asset
materializer during Bundle Start. Startup recovery runs inside the same Bundle
before Ready. The product construction root now supplies only the trusted
factory, read dependencies, state path, and route slot; it no longer retains
`assetAuthority` or `assetEvidenceStore` owners or closes the Assets store
independently.

The one-bind Assets slot exposes two bounded ports: the existing local Assets
route and `app.TeamAssetMaterializer`. `loom-agent-runtime` resolves only the
materializer port after `loom-assets` is Ready and supplies it to the existing
Team Coordinator. Concrete `assets.Authority` and `evidence.Store` values do not
cross into Agent Runtime or Capability Context.

Composition cleanup remains reverse ordered. Agent Runtime first revokes its
Mission/Handoff/materialization routes and closes execution ownership. Assets
then waits for in-flight route calls, revokes both ports, and closes its
Evidence store exactly once. Partial Assets construction and failed Agent
Runtime construction close created resources and prevent Product scope and IPC
admission.

## Verification

```text
go test ./cmd/loomd -run '^(TestCOMP2CAssets|TestCOMP2CAgentRuntime|TestCOMP2CProductionBuilder)' -count=1
go test -race ./cmd/loomd -run '^(TestCOMP2[ABCD]|TestProductMissionExecutionCompositionReconcilesExactJournalLineageBeforeIPC)' -count=10
go test ./cmd/loomd
go test ./...
go vet ./...
git diff --check
```

The focused tests prove bounded materializer publication, owner revocation,
single close, Assets-before-Runtime order, failed-Start rollback, Product-scope
non-admission, route quiescence, and AST-level absence of compatibility-owned
Assets authority/Evidence identifiers in production construction. Full daemon
and repository suites preserve startup reconciliation, socket Assets journeys,
Mission/Handoff execution, per-Agent binding, shutdown, and privacy behavior.

## Open boundary

This does not complete COMP2-C or COMP2-D. Observability, Vault, Conversation,
governance/work, local IPC, and protected core construction remain. Conversation
through Turn scope integration, the legacy dispatch oracle, COMP2-E, installed
CV6, mixed-Team ATL9, and all live UI gates remain open.

No App was built, signed, installed, or launched. No credential, Provider,
network, user workspace, or external tool was accessed.
