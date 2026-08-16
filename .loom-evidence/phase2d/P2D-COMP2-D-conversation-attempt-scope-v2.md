# P2D-COMP2-D Conversation Attempt Scope V2

**Date**: 2026-08-14  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / COMP2-D PARTIAL  
**Contract**: `contracts/P2D-COMP2-product-daemon-strangler.md`  
**Decision**: `../../docs/adr/0021-governed-composition-kernel-and-daemon-strangler.md`

## Accepted boundary

Ordinary Conversation Provider dispatch now runs inside the real Composition
scope hierarchy:

```text
Product -> Conversation -> Team -> Agent -> Attempt -> Turn
```

The Product scope owns a one-time-bound, revocable scope manager. The Chat API
receives only its stable manager port during `loom-conversation` construction;
the port binds after Product scope open and before IPC admission. It is not
placed in `CapabilityContext` and cannot admit work before binding or after
Product close.

`SendMessage` opens the Attempt and Turn after the immutable Conversation
Segment binding digest and Attempt ID exist, but before thread dispatch state is
persisted or the Provider responder runs. The Attempt freezes both the active
Composition Snapshot digest and the exact Conversation execution binding
digest. Scope-open failure rolls back the user message, Segment, Attempt, and
stored Context Capsule before any Provider call. Every success and failure path
closes Attempt/Turn; Conversation/Team/the stable Loom Conversation Agent remain
owned by Product and are reused for later Attempts.

User-visible thread IDs never enter Composition diagnostics. The manager derives
domain-separated SHA-256 opaque IDs independently for Conversation, Team, Agent,
Attempt, and Turn scopes, preserving existing wider Chat IDs without exposing
source identifiers. Diagnostics contain only scope kind, opaque scope ID/digest,
snapshot digest, stage, timing, result, and safe error code.

This slice does not equate Capability Context, Go `context.Context`, Attempt
Context, or Context Capsule. It adds no Prompt, transcript, Provider body,
secret, credential, VMK, Journal writer, StateWriter, or terminal authority to
any scope.

## RED and verification

The RED failed on the missing Chat scope contract, Product scope slot, and
Composition wiring. The implemented boundary passed:

```text
go test ./internal/api ./cmd/loomd -run 'TestCOMP2D.*Scope' -count=1
go test ./internal/api ./cmd/loomd -run '^(TestCOMP2|TestChat|TestLocalProductChat|TestProduct.*Chat|TestProduct.*Conversation|TestProductDaemon.*Conversation)' -count=1
go test -race ./internal/api ./cmd/loomd -run '^(TestCOMP2D.*Scope|TestCOMP2CConversation|TestProduct.*Chat|TestProduct.*Conversation)' -count=10
go test ./cmd/loomd -count=1
go test ./... -count=1
go vet ./...
git diff --check
```

Tests prove dispatch-before-open is impossible, snapshot/binding freeze,
Attempt/Turn close on return, Conversation reuse across Attempts, rollback before
Provider dispatch, Product-close revocation, production wiring, opaque scope
diagnostics, existing Conversation behavior, race safety, and repository parity.

## Remaining boundary

COMP2-D remains open. Team mission scopes must use real Team/Agent/Attempt
identities and full `FrozenExecutionBinding`; multi-turn Attempt Loop boundaries
must own exact Turn resources; credential leases, Provider sessions, tool
channels, temporary roots, and cancellable workers must attach to their actual
scope Effects. COMP2-E, CV6, ATL9, UI/accounting, and installed live gates also
remain open.

No App was built, signed, installed, or launched. No credential, Provider,
network, user workspace, or external tool was accessed.
