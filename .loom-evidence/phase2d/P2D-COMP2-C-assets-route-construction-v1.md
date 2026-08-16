# P2D-COMP2-C Assets Route Construction V1

**Date**: 2026-08-14  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / COMP2-C CONTINUES  
**Contract**: `contracts/P2D-COMP2-product-daemon-strangler.md`  
**Decision**: `../../docs/adr/0021-governed-composition-kernel-and-daemon-strangler.md`

## Accepted boundary

The `loom-assets` built-in Bundle now constructs the local Assets service/API
during its Start hook instead of product-daemon construction. Product startup
creates only a one-bind typed route slot and a trusted factory. The factory
captures the existing bounded dependencies but does not register Journal,
StateWriter, Evidence content, Prompt, Provider response, credential, or
authority in Capability Context.

Local IPC receives the route slot before activation, but cannot serve requests
until every Bundle is Ready. The Assets Start hook constructs and binds exactly
once; Ready verifies the binding. Construction failure records a failed
`loom-assets` Bundle Start, disposes activation, and prevents Product scope and
IPC admission.

The Start Effect owns route revocation. Close waits for in-flight route calls,
then clears the API port; old slot references fail closed. Assets authority,
Evidence ownership, skill materialization, and the bounded Team materializer
port have subsequently moved into this Bundle. Exact follow-up evidence is
`P2D-COMP2-C-assets-authority-ownership-v1.md`.

## Verification

Passed on 2026-08-14:

```text
go test -race ./cmd/loomd -run '^TestCOMP2[ABCD]' -count=20
go test ./cmd/loomd
go test ./...
go vet ./...
git diff --check
```

The COMP2-C tests prove Bundle-Start construction, failed-Start admission
rollback, route revocation, in-flight quiescence, and AST-level prohibition on
the product builder directly constructing the Assets service/API. Existing
socket Assets journeys, Agent execution, startup cleanup, and shutdown tests
remain green.

## Open boundary

This does not complete COMP2-C. Observability, Vault, Conversation,
governance/work, local IPC, and protected core construction still require
bounded migration. Agent Runtime construction has subsequently moved behind
`loom-agent-runtime`; exact evidence is
`P2D-COMP2-C-agent-runtime-construction-v1.md`. Assets authority/Evidence now
live under `loom-assets` and expose only the bounded materializer port to Agent
Runtime, without mutable post-activation injection.

COMP2-D deeper scopes, COMP2-E, installed CV6, mixed-Team ATL9, and all live UI
gates remain open. No App was built, signed, installed, or launched. No
credential, Provider, network, user workspace, or external tool was accessed.
