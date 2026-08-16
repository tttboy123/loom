# P2D-COMP2-C Agent Runtime Construction V1

**Date**: 2026-08-14  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / COMP2-C CONTINUES  
**Contract**: `contracts/P2D-COMP2-product-daemon-strangler.md`  
**Decision**: `../../docs/adr/0021-governed-composition-kernel-and-daemon-strangler.md`

## Accepted boundary

`loom-agent-runtime` now constructs the existing Mission Execution backend,
Side Task Handoff API, and saved-Team materializer during Bundle Start. The
product builder validates and freezes the existing runtime configuration, then
passes only a trusted factory and one-bind typed route slot into composition.
It no longer calls `buildProductMissionExecutionAPI` directly.

The slot exposes only Mission, Handoff, and materialization methods. It does not
contain Provider clients, credentials, leases, Prompt, Context Capsule content,
Journal writer, policy/grant authority, terminal authority, or Evidence content.
Those remain encapsulated by the existing backend created by the factory.

The Bundle graph starts `loom-assets` before `loom-agent-runtime`. Runtime
construction failure disposes the Assets route and prevents Product scope and
IPC admission. Its Start Effect revokes the Runtime routes and closes the
existing execution bundle, including backend, observers, handoff, and Evidence
owners. The product runner no longer independently owns that closer.

## Error fidelity repair

This migration exposed that Composition lifecycle errors kept a safe message
but discarded the original `errors.Is` identity. `lifecycleFailure` now has a
metadata-only Error string and unwraps both the safe composition category and
original cause. Private error text remains absent while startup reconciliation
again preserves `app.ErrMissionExecutionConflict` exactly.

## Verification

Passed on 2026-08-14:

```text
go test -race ./internal/composition ./cmd/loomd -run '^(TestCOMP1|TestCOMP2[ABCD])' -count=20
# after partial-factory cleanup hardening:
go test -race ./internal/composition ./cmd/loomd -run '^(TestCOMP1|TestCOMP2[ABCD]|TestProductMissionExecutionCompositionReconcilesExactJournalLineageBeforeIPC)' -count=10
go test ./cmd/loomd
go test ./...
go vet ./...
git diff --check
```

The focused tests prove Assets-before-Runtime order, Runtime failure rollback,
Product-scope non-admission, route revocation, closer ownership, and AST-level
prohibition on production directly constructing the Agent Runtime. Full daemon
tests prove exact Journal startup reconciliation, cold-model laziness, mission
execution, side-task handoff, Agent credential binding, and shutdown parity.

## Open boundary

This does not complete COMP2-C. Assets authority/Evidence have subsequently
moved into the Assets Bundle, and Agent Runtime now receives only the bounded
`app.TeamAssetMaterializer` port. Observability, Vault, Conversation,
governance/work, local IPC, and protected core construction also remain.
Conversation through Turn scope integration, COMP2-E, installed CV6, mixed-Team
ATL9, and all live UI gates remain open.

No App was built, signed, installed, or launched. No credential, Provider,
network, user workspace, or external tool was accessed.
