# P2D-COMP2-C Setup Construction V12

**Date**: 2026-08-14  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / COMP2-C-D CONTINUES  
**Contract**: `contracts/P2D-COMP2-product-daemon-strangler.md`  
**Decision**: `../../docs/adr/0021-governed-composition-kernel-and-daemon-strangler.md`

## Accepted boundary

The final built-in `loom-local-ipc` Bundle now constructs and binds the existing
Setup aggregate only after Observability, Assets, Vault, Conversation,
Governance, Work, and Agent Runtime are Ready. The monolithic product builder
no longer calls `buildProductSetupService`.

The product handler is compiled against a revocable `productSetupRouteSlot`.
The slot exposes only the existing Setup commands and snapshots. It does not
expose a Vault root, credential plaintext, Provider client, Provider Policy
authority, Journal writer, VMK, Prompt, transcript, or terminal Run/Attempt
authority, and it is not registered in `CapabilityContext`.

The Setup aggregate continues to consume the exact bounded Vault credential
mutator/availability/status ports and Governance Provider Account Policy/Model
Rate Card ports. Account identity, credential reference/revision, runtime
catalog admission, native-auth behavior, transaction boundaries, and
projection refresh semantics are unchanged.

Construction failure prevents local IPC admission and preserves the original
`build_state`, `build_setup_runtime`, `build_setup_credential`,
`build_setup_provider`, or `build_setup_native_auth` reason through Composition
rollback. Reverse cleanup revokes Setup and closes its native-auth owner before
lower dependency Bundles. The runner does not close the same Setup owner a
second time. An unavailable or nil Setup slot clears credential input bytes
before returning.

## Verification

```text
go test ./cmd/loomd -run '^(TestCOMP2CSetup|TestCOMP2CProductionDoesNotConstructSetup)' -count=1
go test ./cmd/loomd -run '^(TestProductDaemonOwnsCredentialVaultWhenEnabled|TestProductDaemon.*Credential|TestProductDaemon.*Setup|TestProductSetup)' -count=1
go test ./cmd/loomd -run '^(TestCOMP2|TestProductDaemon.*Close|TestProductDaemonShutdown|TestProductDaemonServes|TestProductDaemonOwns|TestProductSetup|TestProductCredentialVault|TestProviderAccountPolicy|TestProviderModelRateCard)' -count=1
go test -race ./internal/composition ./internal/api ./internal/app ./cmd/loomd -run '^(TestCOMP1|TestCOMP2[ABCD]|TestProduct.*Setup|TestProductDaemon.*Credential|TestProductDaemonOwnsCredentialVaultWhenEnabled|TestProviderAccountPolicy|TestProviderModelRateCard)' -count=10
go test ./cmd/loomd -count=1
go test ./...
go vet ./...
git diff --check
```

Tests prove one-time late construction, snapshot delegation, credential input
clearing, exact failure preservation, close revocation, no double close,
default Vault setup, Provider policy/rate-card parity, strict unavailable
handling, production AST ownership, and real product route behavior. Ten-run
race, full daemon, full repository, vet, and whitespace checks pass.

## Open boundary

This does not complete COMP2-C/D. V13 subsequently moved Read construction
under Governance. The lazy Pi local-model Conversation resource remains
compatibility-constructed. Deeper
Conversation/Attempt scopes, diagnostics UI/export, account-level accounting,
COMP2-E removal, installed CV6, mixed-Team ATL9, and all live gates remain open.

No App was built, signed, installed, or launched. No credential, Provider,
network, user workspace, or external tool was accessed.
