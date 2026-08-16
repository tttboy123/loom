# P2D-COMP2-C Governance Provider Account Policy V7

**Date**: 2026-08-14  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / COMP2-C CONTINUES  
**Contract**: `contracts/P2D-COMP2-product-daemon-strangler.md`  
**Decision**: `../../docs/adr/0021-governed-composition-kernel-and-daemon-strangler.md`

## Accepted boundary

`loom-governance` now constructs and owns the Provider Account Policy and
Provider Model Rate Card authority during Bundle Start. The Setup compatibility
service no longer constructs `work.Authority`; it receives only the two narrow
configuration interfaces through `productSetupRuntimeConfig` and calls them
after Governance Ready.

The authority remains private to the trusted built-in Governance factory and is
not placed in Capability Context. The typed slot exposes only exact
`ConfigureProviderAccountPolicy` and `ConfigureProviderModelRateCard` commands.
It does not expose Journal append, Run/Attempt terminal authority, arbitrary
policy reads, credential access, or Provider clients. Closing Governance revokes
both configuration interfaces and returns their existing unavailable errors.

Provider Account identity remains exact across Provider ID, account ID,
revision, policy digest, and Model Rate Card revision. Existing source behavior
continues to freeze account concurrency, dispatch window/rate, assigned budget,
disclosure policy, token basis, and cost at claim time. Policy or price drift
affects future claims only, and one account's capacity failure does not consume
another account's reservation. Construction failure preserves
`build_setup_policy`.

## Verification

```text
go test ./cmd/loomd -run '^(TestCOMP2CGovernance|TestCOMP2CProductionBuilderDoesNotConstructGovernanceOutsideBundle|TestProviderAccountPolicy|TestProviderModelRateCard)' -count=1
go test ./cmd/loomd -run '^(TestCOMP2C|TestProviderAccountPolicyConfigureWireInjectsTrustedCorrelation|TestProviderModelRateCardConfigureWireInjectsTrustedCorrelation|TestProductOperationalDiagnosticsRecordsSafeProviderAccountPolicyEvent)' -count=1
go test ./internal/app -run '^(TestLocalProductSetupConfiguresProviderAccountPolicy|TestLocalProductSetupProviderAccountPolicy|TestLocalProductSetupConfiguresProviderModelRateCard|TestLocalProductSetupProviderModelRateCard)' -count=1
go test ./internal/work -run '^(TestProviderAccountPolicy|TestProviderModelRateCard|TestProviderAccountCapacity)' -count=1
go test -race ./internal/composition ./cmd/loomd ./internal/work -run '^(TestCOMP1|TestCOMP2[ABCD]|TestProviderAccountPolicy|TestProviderModelRateCard|TestProviderAccountCapacity|TestProductOperationalDiagnosticsRecordsSafeProviderAccountPolicyEvent)' -count=10
go test ./cmd/loomd -count=1
go test ./...
go vet ./...
git diff --check
```

Focused tests prove authority delegation and revocation, exact failure identity,
trusted correlation injection, projection refresh, account-local CAS, immutable
claim-time policy and rate-card freezing, concurrency/budget/rate isolation, and
privacy-safe diagnostics. AST checks prove Setup no longer constructs Provider
policy authority. Full daemon and repository suites preserve Credential, Setup,
Agent Runtime, shutdown, Journal, and privacy behavior.

## Open boundary

This does not complete `loom-governance` or COMP2-C/D. At this V7 boundary,
Setup/read route construction, Conversation construction, observability
handoff, accounting UI, and deeper Capability Context scopes remained open.
The bounded Observability bootstrap handoff was subsequently source verified by
`P2D-COMP2-C-observability-bootstrap-handoff-v8.md`; the store itself retains
its documented pre-composition bootstrap role until Conversation migration can
preserve all startup failures.

COMP2-E, installed CV6, mixed-Team ATL9, and all live UI gates remain open. No
App was built, signed, installed, or launched. No credential, Provider, network,
user workspace, or external tool was accessed.
