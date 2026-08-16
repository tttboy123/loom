# P2D-COMP2-C Conversation Route Handoff V9

**Date**: 2026-08-14  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / CONSTRUCTION MIGRATION OPEN  
**Contract**: `contracts/P2D-COMP2-product-daemon-strangler.md`  
**Decision**: `../../docs/adr/0021-governed-composition-kernel-and-daemon-strangler.md`

## Accepted boundary

`LocalProductReadService` now depends on the narrow `LocalProductChatSource`
port instead of concrete `*LocalProductChatAPI`. `loom-conversation` Start binds
the existing chat implementation into `productConversationRouteSlot`, and its
Effect revokes both `ChatThread` and `SendMessage` when Composition closes.

The current persistent/encrypted chat API, Conversation Profile router,
Provider clients, Segment and Attempt state, Context Capsule behavior, migration
diagnostics, and Provider-account execution binding are unchanged. The slot
contains no Prompt, transcript, Provider response, credential, native session
handle, Journal writer, or terminal authority and is not placed in Capability
Context. It serializes close against in-flight chat calls without changing the
chat API's existing send serialization.

Conversation Start failure prevents Agent Runtime activation and Product/IPC
admission. Its compatibility failure preserves `build_setup_provider`. The
compiled dependency remains Vault before Conversation and Conversation before
Agent Runtime.

## Verification

```text
go test ./cmd/loomd -run '^TestCOMP2CConversation' -count=1
go test ./internal/api -run '^(TestLocalProductRead|TestChat|TestEncrypted|TestPersistent|TestConversation)' -count=1
go test ./cmd/loomd -run '^(TestCOMP2CConversation|TestProductDaemonConfiguredPiConversationJourney|TestProductDaemonInjectsTentativeConversationWithoutJournalFacts|TestProductDaemonMigratesLegacyChatIntoCredentialVault|TestProductDaemonKeepsGovernanceOnlineWhenChatMigrationFails|TestProduct.*Conversation|TestProductDaemonServesRealReadOnlySQLiteOverPrivateUDSAndCleansUp)' -count=1
go test -race ./internal/api ./internal/composition ./cmd/loomd -run '^(TestCOMP1|TestCOMP2[ABCD]|TestLocalProductRead|TestChat|TestEncrypted|TestPersistent|TestProduct.*Conversation)' -count=10
go test ./cmd/loomd -count=1
go test ./...
go vet ./...
git diff --check
```

Focused tests prove one-time bind, exact chat delegation, close revocation,
failure ordering, failure-stage identity, and AST-level Read ownership. Profile,
Segment, encrypted migration, real UDS, ten-run race, full daemon, full
repository, vet, and whitespace checks pass.

## Open boundary

At this V9 boundary this was a route and lifecycle handoff, not Conversation
construction extraction. V10 subsequently moved Provider client/router,
encrypted document load/migration, and Chat factory construction into
`loom-conversation` after `loom-vault` Ready. See
`P2D-COMP2-C-vault-conversation-construction-v10.md`.

The lazy local-model Conversation runtime, Setup/read construction, full
Observability construction, Conversation and Attempt scopes,
disclosure/accounting UI, COMP2-E, installed CV6, mixed-Team ATL9, and all live
gates remain open. No App was built, signed, installed, or launched. No
credential, Provider, network, user workspace, or external tool was accessed.
