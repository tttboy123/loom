# P2D-COMP2-C Vault and Conversation Construction V10

**Date**: 2026-08-14  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / COMP2-C-D CONTINUES  
**Contract**: `contracts/P2D-COMP2-product-daemon-strangler.md`  
**Decision**: `../../docs/adr/0021-governed-composition-kernel-and-daemon-strangler.md`

## Accepted boundary

`loom-vault` Start now constructs the real LocalKeyFile Credential Vault or its
existing fail-closed recovery runtime. The concrete Vault owner, root store,
VMK lifetime, and recovery object remain private to the trusted built-in
Bundle. Setup, Conversation, Governance, and Agent Runtime receive only the
bounded interfaces they already require: credential mutation, exact-revision
leases and availability, encrypted Conversation documents, Context Capsules,
retrieval, Attempt payloads, Tool Proposals, and external session handles.

`loom-conversation` Start now constructs the Codex native client when selected,
the DeepSeek/Kimi/MiniMax/Anthropic system clients, the account-aware Profile
router, migration recorder, and persistent encrypted Chat API. It requires the
Vault slot to be Ready before using Vault-backed credentials, documents, or
Context Capsules. Conversation construction failure prevents Agent Runtime and
Product/IPC admission while preserving the existing
`build_setup_native_auth` or `build_setup_provider` stage.

The compiled lifecycle is Vault before Conversation and Governance. Reverse
cleanup revokes the Conversation route before closing the Vault owner. A Vault
recovery reset activates the real runtime behind the same bounded slot, so
downstream consumers do not acquire or retain the replacement root object.

The production daemon builder no longer calls the Vault/recovery constructors,
system Provider client constructors, Profile router constructor, or persistent
Chat constructors. It prepares only non-secret configuration and the existing
lazy shared local-model compatibility resource before Composition activation.
That Pi resource remains outside the Conversation Bundle and is an explicit
open boundary.

Unavailable Vault writes now clear caller-owned Context Capsule, transcript,
and Attempt payload buffers before returning. Credential bytes, Prompt,
transcript, tool results, Provider bodies, ciphertext, VMK, Journal writers,
and terminal authority do not enter `CapabilityContext`.

## Verification

```text
go test ./internal/credentials/vault -count=1
go test ./internal/api -count=1
go test ./internal/app -count=1
go test ./cmd/loomd -run '^(TestCOMP2CVault|TestCOMP2CConversation|TestCOMP2CProductionDoesNotConstructVaultOutsideBundle|TestCOMP2CProductionReadUsesConversationBundleSlot|TestCOMP2CConversationConstructionPreservesFailureStage)' -count=1
go test -race ./internal/composition ./internal/credentials/vault ./internal/api ./cmd/loomd -run '^(TestCOMP1|TestCOMP2[ABCD]|TestProduct.*Vault|TestProduct.*Credential|TestProduct.*Conversation|TestChat|TestEncrypted|TestPersistent)' -count=10
go test ./cmd/loomd -count=1
go test ./...
go vet ./...
git diff --check
```

Focused tests prove real Vault file creation and owner-only permissions,
recovery routing, Vault-before-Conversation startup, reverse revocation,
constructor ownership, exact failure-stage mapping, encrypted migration parity,
and sensitive-buffer clearing on unavailable/nil slots. The ten-run race,
complete daemon suite, complete Go repository, vet, and whitespace checks pass.

Static production inspection returns no Vault/recovery, system Provider,
Profile router, encrypted Chat, or persistent Chat constructor call from
`newProductDaemonRunnerWithPreparedDecisions`.

## Open boundary

This is not completion of COMP2-C/D. V11 subsequently moved persistent
diagnostic store construction into `loom-observability` while preserving a
bounded pre-start sink. The lazy Pi local-model Conversation resource and
Setup/read construction remain compatibility-owned. Deeper
Conversation/Attempt scopes, diagnostics UI/export, account-level accounting,
COMP2-E removal, installed CV6, mixed-Team ATL9, and all live gates remain open.

No App was built, signed, installed, or launched. No credential, Provider,
network, user workspace, or external tool was accessed.
