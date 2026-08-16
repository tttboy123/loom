# P2D-COMP2-D Conversation Provider Session Lifecycle V7

**Date**: 2026-08-14  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / COMP2-D PARTIAL  
**Contract**: `contracts/P2D-COMP2-product-daemon-strangler.md`  
**Decision**: `../../docs/adr/0021-governed-composition-kernel-and-daemon-strangler.md`

## Accepted boundary

Ordinary Conversation dispatch now receives only the execution context owned by
its admitted Attempt scope:

```text
Product -> Conversation -> Team -> Agent -> Attempt -> Turn
                                      |
                                      +-> cancellable execution Effect
                                            -> Provider responder
                                            -> exact encrypted ExternalSessionHandle callback
```

`LocalProductConversationScopeLease` requires an `ExecutionContext`. The Chat
API validates it before persistence or Provider dispatch, fails closed and
rolls back Message/Segment/Attempt/Capsule state when it is absent or already
cancelled, and never falls back silently to the wider request context after a
scope has been admitted.

The Conversation Attempt owns a `context.WithCancelCause` through an idempotent
Composition Effect. Normal Attempt close and wider Product close cancel the
context with `composition.ErrScopeClosed`. The Provider responder therefore
cannot outlive its Attempt merely because the outer UDS request remains alive.

A cross-component test stores a real encrypted ExternalSessionHandle in the
Loom Vault with the exact Conversation, Segment, Provider, Provider Account,
model, auth mode, credential reference, and credential revision binding. The
handle is decrypted only inside the Attempt context callback. Product close
cancels that callback and the runtime zeroizes the plaintext handle in place
after callback exit. No handle, Provider body, Prompt, secret, or authority is
placed in Capability Context or Go context values.

## Verification

```text
go test ./internal/api ./cmd/loomd -run 'TestCOMP2D(ChatOpensFrozenAttemptScopeBeforeDispatchAndClosesIt|ChatScopeFailureRollsBackBeforeProviderDispatch|ChatRejectsMissingScopeExecutionContextBeforeProviderDispatch|ConversationAttemptScopeFreezesSnapshotAndBinding|ConversationAttemptScopeCancelsActiveExecutionOnProductClose|ConversationAttemptOwnsExternalSessionHandleLease)' -count=1
go test ./internal/api ./internal/credentials/vault ./cmd/loomd -run 'Test(COMP2D|ProductCredentialVaultRuntimeOwnsBoundedExternalSessionHandleAccess|ExternalSessionHandle)' -count=1
go test -race ./internal/api ./cmd/loomd -run 'TestCOMP2D(Chat|ConversationAttempt|ConversationScope)' -count=10
go test ./cmd/loomd -count=1
go test ./... -count=1
go vet ./...
git diff --check
```

All commands pass. The RED first proved that the responder received the outer
context and that Conversation leases exposed no execution context. A second RED
proved a nil execution context could panic before dispatch; it now produces a
fail-closed governed dispatch error, closes the scope, rolls back state, and
never calls the responder.

## Remaining boundary

This verifies the lifecycle contract and encrypted session-handle lease path;
it does not claim every Provider adapter currently creates or reuses a native
session handle. Adapters that remain stateless must adopt this exact binding and
Attempt context when they add `previous_response_id`, `conversation_id`, or
prompt-cache support. Cross-route/account/Segment/revision reuse remains
forbidden.

COMP2-D remains open for tool extension/channel ownership, component processes,
temporary roots, broader Runtime adapters, and restart/crash cleanup. COMP2-E,
installed CV6, mixed-Team ATL9, UI/accounting, and live Provider acceptance
remain mandatory.

No App was built, signed, installed, launched, or changed. No real credential,
Provider, network, user workspace, or external tool was accessed.
