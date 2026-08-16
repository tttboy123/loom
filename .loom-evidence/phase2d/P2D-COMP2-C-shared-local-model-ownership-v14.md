# P2D-COMP2-C Shared Local Model Ownership V14

**Date**: 2026-08-14  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / CONSTRUCTOR AUDIT RESOLVED BY V15  
**Contract**: `contracts/P2D-COMP2-product-daemon-strangler.md`  
**Decision**: `../../docs/adr/0021-governed-composition-kernel-and-daemon-strangler.md`

## Accepted boundary

`loom-conversation` now constructs and owns the optional shared Pi local-model
runtime. The same runtime is used by the Pi Conversation responder and is
exposed to `loom-agent-runtime` only through a bounded `BaseURL`/lifecycle
interface on the existing Conversation slot. The product builder no longer
calls `newProductSharedLocalModel`, constructs the Pi Conversation responder,
retains the shared server, or closes it separately.

Agent Runtime already requires Conversation. It acquires the local-model port
only when its frozen configuration includes a local model catalog. Reverse
cleanup therefore closes Agent Runtime and its adapters before Conversation
closes the shared server exactly once. Conversation construction rollback also
closes a newly created or injected shared runtime.

The server process, executable/model paths, local endpoint, mutex, and process
owner remain private to the trusted built-in Bundle. No server object, path,
Prompt, transcript, Provider body, credential, VMK, Journal writer, StateWriter,
or terminal authority enters `CapabilityContext`. No global environment or argv
secret injection is introduced.

## Verification

```text
go test ./cmd/loomd -run '^(TestCOMP2CConversationOwnsSharedLocalModelForAgentRuntime|TestCOMP2CProductionReadUsesConversationBundleSlot|TestProductSharedLocalModel|TestProductPiConversation)' -count=1
go test ./cmd/loomd -run '^(TestCOMP2CConversation|TestCOMP2CAgentRuntime|TestProductDaemonConfiguredPiConversationJourney|TestProduct.*LocalModel|TestProduct.*PiConversation|TestProduct.*Mission)' -count=1
go test -race ./internal/composition ./internal/api ./cmd/loomd -run '^(TestCOMP1|TestCOMP2[ABCD]|TestProduct.*LocalModel|TestProduct.*PiConversation|TestProduct.*Mission|TestProductDaemonConfiguredPiConversationJourney)' -count=10
go test ./cmd/loomd -count=1
go test ./...
go vet ./...
git diff --check
```

Tests prove slot ownership, shared identity, one-time close, Conversation and
Mission lazy load, Agent-before-Conversation shutdown, rollback, and production
AST ownership. Ten-run race, full daemon, full repository, vet, and whitespace
checks pass.

## Open boundary

This evidence did not itself claim all COMP2-C/D work complete. The required
constructor ownership audit and protected Core migration subsequently passed in
`P2D-COMP2-C-protected-core-ownership-v15.md`, closing the COMP2-C source exit.
Deeper Conversation/Attempt scopes, diagnostics UI/export, account-level
accounting, COMP2-E removal, installed CV6, mixed-Team ATL9, and all live gates
remain open.

No App was built, signed, installed, or launched. No credential, Provider,
network, user workspace, or external tool was accessed.
