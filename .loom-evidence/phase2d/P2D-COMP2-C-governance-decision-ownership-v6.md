# P2D-COMP2-C Governance Decision Ownership V6

**Date**: 2026-08-14  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / COMP2-C CONTINUES  
**Contract**: `contracts/P2D-COMP2-product-daemon-strangler.md`  
**Decision**: `../../docs/adr/0021-governed-composition-kernel-and-daemon-strangler.md`

## Accepted boundary

`loom-governance` now constructs and owns the Prepared Mission Decision backend,
fallback decision StateWriter and preparer, local Mission Decision service/API,
and Mission Execution decision router during Bundle Start. The production daemon
builder no longer creates or retains any of these concrete decision objects.

The decision backend and fallback StateWriter remain private to the trusted
built-in Governance factory. The typed Governance slot exposes four bounded
contracts: Mission Decision IPC operations, prepared-command listing for the
Read projection, execution-control routing for Agent Runtime, and explicit
fallback preparation. None exposes the backend map, StateWriter, Journal append
authority, terminal authority, or a general decision mutation surface.

Read service may hold the command-source proxy before activation, but no IPC is
admitted until Governance Ready. Agent Runtime receives only the execution and
fallback interfaces and starts after Governance and Work. Governance failure
prevents downstream Bundles, Product scope, and IPC admission. Closing the slot
revokes all decision routes together with Permission and approval routes.
Decision construction failures preserve `build_decision`.

## Verification

```text
go test ./cmd/loomd -run '^(TestCOMP2CGovernance|TestCOMP2CProductionBuilderDoesNotConstructGovernance|TestProductDaemonRoutesOneStrictMissionDecisionMethod)' -count=1
go test ./cmd/loomd -run '^(TestCOMP2C|TestProductDaemonRoutesOneStrictMissionDecisionMethod|TestProductDaemonRoutesOneStrictMissionExecutionMethod|TestProductDaemonSnapshotRebindsPreparedDecisionsAfterRuntimeDiscovery|TestProductDaemonProductionRunnerWiresFailClosedDecisionRegistry|TestProductDaemonProductionRunnerLoadsControlledMissionFixture|TestProductDaemonProductionRunnerServesAuthoritativeMissionPreflight|TestProductMissionExecutionCompositionPreflightIsZeroWriteAndLazy|TestProductMissionExecutionCompositionReconcilesExactJournalLineageBeforeIPC)' -count=1
go test ./internal/app -run '^(TestProjectionMissionFallback|TestPreparedMissionFallback|TestDynamicFallback|TestFourProviderTeamApprovedAccountFallback|TestPhase2D.*Fallback)' -count=1
go test -race ./internal/composition ./cmd/loomd -run '^(TestCOMP1|TestCOMP2[ABCD]|TestProductDaemonRoutesOneStrictMissionDecisionMethod|TestProductDaemonRoutesOneStrictMissionExecutionMethod|TestProductDaemonSnapshotRebindsPreparedDecisionsAfterRuntimeDiscovery|TestProductMissionExecutionCompositionReconcilesExactJournalLineageBeforeIPC)' -count=10
go test ./cmd/loomd -count=1
go test ./...
go vet ./...
git diff --check
```

Focused tests prove decision delegation and revocation, exact failure identity,
prepared-decision Read projection, strict Mission Decision IPC, Agent Runtime
control routing, explicit fallback preparation, fail-closed registry behavior,
and AST-level absence of migrated constructors from production construction.
The real mission preflight, controlled fixture, four-Provider approved fallback,
and rejection of unapproved fallback remain green. Full daemon and repository
suites preserve startup, shutdown, Journal authority, diagnostics, and privacy.

## Open boundary

This does not complete `loom-governance` or COMP2-C/D. At this V6 boundary,
Provider-account policy, setup/read route construction, Conversation
construction, observability handoff, and deeper Capability Context scopes
remained open. Provider Account Policy and Model Rate Card authority were
subsequently migrated and source verified by
`P2D-COMP2-C-governance-provider-account-policy-v7.md`; the other listed
boundaries remain open. The operational store retains
its documented pre-composition bootstrap role until Conversation migration can
preserve all startup failures.

COMP2-E, installed CV6, mixed-Team ATL9, and all live UI gates remain open. No
App was built, signed, installed, or launched. No credential, Provider, network,
user workspace, or external tool was accessed.
