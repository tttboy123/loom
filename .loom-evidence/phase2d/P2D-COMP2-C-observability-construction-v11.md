# P2D-COMP2-C Observability Construction V11

**Date**: 2026-08-14  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / COMP2-C-D CONTINUES  
**Contract**: `contracts/P2D-COMP2-product-daemon-strangler.md`  
**Decision**: `../../docs/adr/0021-governed-composition-kernel-and-daemon-strangler.md`

## Accepted boundary

`loom-observability` Start now creates the owner-only persistent operational
diagnostic directory and store. The monolithic product builder no longer calls
`newProductOperationalDiagnosticStore` and performs no diagnostics filesystem
I/O before Composition activation.

An in-memory `productOperationalDiagnosticBootstrap` exists before activation
only to preserve privacy-safe compile, validate, route, registration, and early
start diagnostics. It has a fixed 256-record bound, accepts only already
validated structured diagnostic records, and contains no API Key, credential,
Prompt, transcript, tool-result content, Provider body, ciphertext, VMK,
Journal writer, or execution authority. Buffer overflow fails activation.

During Observability Start the Bundle constructs the persistent store, binds the
frozen credential-runtime identity, and flushes all pre-start records in order
before publishing downstream routes. Construction or flush failure prevents
Vault, Conversation, Governance, Work, Agent Runtime, Product, and IPC admission
and preserves `build_diagnostics`. Once bound, the bootstrap forwards only
Composition lifecycle records; Read, Agent Attempt, Context retrieval, Tool,
and IPC operations continue through the revocable Observability slot.

Composition stop/dispose records remain durable after downstream operational
ports are revoked. This is intentional lifecycle evidence, not a route bypass.
The bootstrap is not registered in `CapabilityContext`.

## Verification

```text
go test ./cmd/loomd -run '^TestCOMP2CObservability' -count=1
go test ./cmd/loomd -run '^(TestProductOperationalDiagnostics|TestProductComposition|TestCOMP2CObservability|TestCOMP2CConversation|TestCOMP2CVault|TestCOMP2[ABD])' -count=1
go test -race ./internal/composition ./internal/api ./cmd/loomd -run '^(TestCOMP1|TestCOMP2[ABCD]|TestProduct.*Diagnostic|TestProduct.*Conversation|TestProduct.*Vault|TestChat|TestEncrypted|TestPersistent)' -count=10
go test ./cmd/loomd -count=1
go test ./...
go vet ./...
git diff --check
```

Tests prove that pre-Bundle compile diagnostics flush to the real store, normal
IPC diagnostics continue through the bounded slot, close revokes IPC recording
without losing lifecycle stop/dispose records, real filesystem construction
failure occurs inside Bundle Start, and the exact public build stage remains
`build_diagnostics`. AST/static checks reject persistent store construction in
the production builder. Ten-run race, full daemon, full repository, vet, and
whitespace checks pass.

## Open boundary

This is not completion of COMP2-C/D. V12 subsequently moved Setup construction
into `loom-local-ipc`. The lazy Pi local-model Conversation resource, read
service construction, deeper Conversation/Attempt scopes,
diagnostics UI/export, account-level accounting, COMP2-E removal, installed
CV6, mixed-Team ATL9, and live gates remain open.

No App was built, signed, installed, or launched. No credential, Provider,
network, user workspace, or external tool was accessed.
