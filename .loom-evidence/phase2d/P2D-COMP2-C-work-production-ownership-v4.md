# P2D-COMP2-C Work Production Ownership V4

**Date**: 2026-08-14  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / COMP2-C CONTINUES  
**Contract**: `contracts/P2D-COMP2-product-daemon-strangler.md`  
**Decision**: `../../docs/adr/0021-governed-composition-kernel-and-daemon-strangler.md`

## Accepted boundary

`loom-work` now constructs and owns the existing Production core, local service,
and API during Bundle Start. Production joins Queue, Workers, Integration, and
governed Execution in the atomic typed Work route set. Product construction no
longer resolves Production paths or creates any Production service object.

The migrated factory preserves the exact existing path behavior: an absolute
`LOOM_PRODUCTION_SANDBOX_ROOT` selects its private Application Support and
LaunchAgents paths; otherwise the current user's Library paths are used; the
daemon path remains the current executable. A relative sandbox root, path
resolution failure, core/service/API construction failure, or partial Work
construction fails activation as `build_production`. The existing AdminLock
projection is replayed from the authoritative Journal for every Production
decision.

The typed Production port exposes only Snapshot, Command, and the read-only
Degraded admission check. Work Ready requires this port. Work shutdown waits for
in-flight calls, revokes the port with the rest of the atomic route set, and
reports degraded after revocation so writes fail closed. Production receives no
Capability Context content or protected authority beyond its existing explicit
Journal dependency inside the trusted built-in factory.

## Verification

```text
go test ./cmd/loomd -run '^(TestCOMP2CWork|TestCOMP2CProductionBuilder)' -count=1
go test ./cmd/loomd -run '^(TestCOMP2C|TestCw1ProductionHandlerWiredThroughComposition|TestP3AProductionRunnerWiresControlledJourneyAuditOverRealSocket|TestProductDaemonProductionRunnerWiresFailClosedDecisionRegistry|TestCOMP2AProductionBuilderActivatesDesktopFacade)' -count=1
go test -race ./internal/composition ./cmd/loomd -run '^(TestCOMP1|TestCOMP2[ABCD]|TestCw1ProductionHandlerWiredThroughComposition|TestP3AProductionRunnerWiresControlledJourneyAuditOverRealSocket|TestProductDaemonProductionRunnerWiresFailClosedDecisionRegistry)' -count=10
go test ./cmd/loomd -count=1
go test ./...
go vet ./...
git diff --check
```

Focused tests prove atomic Ready, route delegation, fail-closed revocation,
exact `build_production` classification, and AST-level absence of all three
Production constructors from product construction. Existing Composition facade,
real socket Production journey, authoritative projection, AdminLock decision,
and degraded-write behavior remain green. Full daemon and repository suites
preserve startup, shutdown, routing, diagnostics, and privacy behavior.

## Open boundary

This does not complete `loom-work` or COMP2-C/D. At this V4 boundary,
Governance/work authority, saved-Team setup/read routes, Conversation
construction, observability handoff, and deeper Capability Context scopes
remained open. Permission, Customer Rule, Standing Order, and the bounded Work
approval port were subsequently migrated and source verified by
`P2D-COMP2-C-governance-authority-ownership-v5.md`; the other listed boundaries
remain open. The operational store retains
its documented pre-composition bootstrap role until Conversation migration can
preserve all startup failures.

COMP2-E, installed CV6, mixed-Team ATL9, and all live UI gates remain open. No
App was built, signed, installed, or launched. No credential, Provider, network,
user workspace, or external tool was accessed.
