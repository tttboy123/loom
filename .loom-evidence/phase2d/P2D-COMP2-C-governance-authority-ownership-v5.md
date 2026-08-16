# P2D-COMP2-C Governance Authority Ownership V5

**Date**: 2026-08-14  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / COMP2-C CONTINUES  
**Contract**: `contracts/P2D-COMP2-product-daemon-strangler.md`  
**Decision**: `../../docs/adr/0021-governed-composition-kernel-and-daemon-strangler.md`

## Accepted boundary

`loom-governance` now constructs and owns the Permission approval port and its
Rules authority, Permission service/API, Customer Rule service/API, and Standing
Order authority/service/API during Bundle Start. Product construction no longer
creates any of these authority or route objects.

The Rules authority remains inside the trusted built-in Governance factory. It
is not published through Capability Context, stored in a generic registry, or
handed to IPC. Three typed route ports expose only Permission, Customer Rule,
and Standing Order operations. A fourth private inter-Bundle port exposes only
`RequestPermissionApproval` and `ConsumePermissionApproval`; `loom-work`
acquires that port after Governance Ready for governed Execution. It cannot
activate rule sets, decide approvals, append arbitrary Journal facts, or obtain
the underlying authority.

Governance must become Ready before Work starts because Work requires the
Governance-provided scoped-context capability. Governance failure prevents Work,
Product scope, and IPC admission. Reverse cleanup closes Work before Governance,
waits for in-flight Governance calls, revokes all route and approval ports, and
closes the owner exactly once. Construction failures preserve the exact
`build_permissions`, `build_customer_rule`, or `build_standing_order` stage.

## Verification

```text
go test ./cmd/loomd -run '^(TestCOMP2CGovernance|TestCOMP2CProductionBuilderDoesNotConstructGovernance|TestCOMP2CWork|TestCOMP2CAgentRuntime)' -count=1
go test ./cmd/loomd -run '^(TestCOMP2C|TestBp1PermissionHandlerWiredThroughComposition|TestBp1DaemonApprovalPortWiredEndToEnd|TestWRulesHandlerWiredThroughComposition|TestWAutonomy|TestBw1ExecutionHandlerWiredThroughComposition|TestCw1ProductionHandlerWiredThroughComposition)' -count=1
go test -race ./internal/composition ./cmd/loomd -run '^(TestCOMP1|TestCOMP2[ABCD]|TestBp1PermissionHandlerWiredThroughComposition|TestBp1DaemonApprovalPortWiredEndToEnd|TestWRulesHandlerWiredThroughComposition|TestWAutonomy|TestBw1ExecutionHandlerWiredThroughComposition)' -count=10
go test ./cmd/loomd -count=1
go test ./...
go vet ./...
git diff --check
```

Focused tests prove atomic Ready, typed route delegation, exact failure stages,
Governance-before-Work startup, Work-before-Governance cleanup, failed-Start
non-admission, route and approval revocation, and AST-level absence of all
migrated constructors from product construction. Existing Permission ask and
decision, Customer Rule, Standing Order, Execution approval consumption, and
socket routing remain green. Full daemon and repository suites preserve startup,
shutdown, diagnostics, Journal authority, and privacy behavior.

## Open boundary

This does not complete `loom-governance`, `loom-work`, or COMP2-C/D. At this V5
boundary, Mission decision, Provider-account policy, setup/read routes,
Conversation construction, observability handoff, and deeper Capability Context
scopes remained open. Mission Decision backend/router/API and explicit fallback
preparation were subsequently migrated and source verified by
`P2D-COMP2-C-governance-decision-ownership-v6.md`; the other listed boundaries
remain open. The
operational store retains its documented pre-composition bootstrap role until
Conversation migration can preserve all startup failures.

COMP2-E, installed CV6, mixed-Team ATL9, and all live UI gates remain open. No
App was built, signed, installed, or launched. No credential, Provider, network,
user workspace, or external tool was accessed.
