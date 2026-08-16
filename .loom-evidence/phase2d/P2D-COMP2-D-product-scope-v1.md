# P2D-COMP2-D Product Scope V1

**Date**: 2026-08-14  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / DEEPER SCOPES OPEN  
**Contract**: `contracts/P2D-COMP2-product-daemon-strangler.md`  
**Decision**: `../../docs/adr/0021-governed-composition-kernel-and-daemon-strangler.md`

## Accepted boundary

The activated compatibility composition opens the exact `loom-product`
Capability Context after every Bundle is Ready and before the local IPC server
can be constructed. The Product scope inherits only the immutable Composition
Snapshot digest, carries no Execution Binding digest, masks all protected Core
capabilities, and cannot contain content or secrets.

The facade owns this scope. Shutdown first stops request admission and waits for
in-flight requests, then closes the Product scope, then closes the activation
and Root scope. Close is concurrent and idempotent. Failure to open the scope or
persist its metadata-only diagnostic fails startup closed and disposes the
activation.

## Verification

Passed on 2026-08-14:

```text
go test -race ./cmd/loomd -run '^TestCOMP2[ABD]' -count=20
go test ./cmd/loomd
go test ./...
go vet ./...
git diff --check
```

The tests prove Product scope identity and snapshot binding, open/close
diagnostics, concurrent idempotent close, and real production-builder admission
with the Product scope already open and durably diagnosed before Run.

## Open boundary

This is the first COMP2-D vertical slice, not completion of COMP2-C/D. The
Conversation, Team, Agent, Attempt, and Turn scopes cannot be safely injected
until COMP2-C inverts construction so Runtime and Conversation services are
created from the activated composition. The current adapters are constructed
before activation; a mutable post-construction scope injection would create an
unversioned hidden dependency and is prohibited.

The next RED must therefore move one bounded service-construction family behind
a built-in Bundle/factory while preserving the current constructor and shutdown
oracle. It must not pass Journal writer, Vault root, policy/grant authority,
terminal authority, Prompt, transcript, Provider body, or secret through a
Capability Context.

No App was built, signed, installed, or launched. No credential, Provider,
network, user workspace, or external tool was accessed.
