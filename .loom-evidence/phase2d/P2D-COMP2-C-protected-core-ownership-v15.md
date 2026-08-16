# P2D-COMP2-C Protected Core Ownership V15

**Date**: 2026-08-14  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / COMP2-C COMPLETE / COMP2-D OPEN  
**Contract**: `contracts/P2D-COMP2-product-daemon-strangler.md`  
**Decision**: `../../docs/adr/0021-governed-composition-kernel-and-daemon-strangler.md`

## Accepted boundary

The protected `loom-core` Bundle now constructs and owns the product SQLite
connection, Event Journal store, Projection, replay, controlled fixture
bootstrap, and verified built-in Runtime records. It publishes those references
only through a private, revocable, process-local construction slot used by the
trusted built-in Bundle factories. The slot is not registered in
`CapabilityContext`, is unavailable before Core Start and after Core Effect
cleanup, and closes the database exactly once.

Vault, Conversation, Governance, Read, Setup, Assets, Work, and Agent Runtime
factories now resolve their exact Journal/Projection/database dependencies from
that private Core slot during their own Start hooks. Prepared Mission Decisions
produced by the controlled fixture bootstrap follow the same boundary. A later
Bundle failure rolls back through Core and leaves no live database handle.

The explicit legacy/test SecretStore adapter also moved out of the product
builder and into the `loom-vault` Start hook behind a revocable credential lease
slot. Normal installed runtime remains Loom Vault only; this compatibility path
does not reintroduce Keychain or a per-request helper.

The production builder now performs only bounded launch/test configuration,
executable validation, no-I/O diagnostic bootstrap, typed Bundle factory and
route assembly, Composition activation, local IPC server creation, and lifecycle
handoff. A constructor-call allowlist and explicit forbidden-call AST checks
prevent domain construction from returning to the monolithic function.

No Journal writer, Projection authority, database, Prompt, transcript, tool
result, Provider body, secret, credential, VMK, StateWriter, policy/grant
authority, or terminal authority enters a descriptor, snapshot, diagnostic, or
`CapabilityContext`.

## RED and verification

The mandatory RED failed on the missing Core slot/factory and construction
fields. The implemented boundary then passed:

```text
go test ./cmd/loomd -run 'TestCOMP2C.*Core' -count=1
go test ./cmd/loomd -run '^(TestCOMP2C.*(Core|Vault|Credential)|TestProductDaemonClassifiesConstruction|TestProductDaemonPreservesSetupConstruction)' -count=1
go test ./cmd/loomd -run '^(TestCOMP2[ABCD]|TestProductDaemonClassifiesConstruction|TestProductDaemonPreservesSetupConstruction|TestProductDaemonClose)' -count=1
go test -race ./cmd/loomd -run 'TestCOMP2C.*(Core|Vault|Credential|Conversation|Read|Setup|Governance|Work|Agent|Assets|Observability|Production)' -count=10
go test ./cmd/loomd -count=1
go test ./... -count=1
go vet ./...
git diff --check
```

Tests prove Core-before-dependent ordering, private slot readiness/revocation,
later-Bundle rollback, database close, exact `build_state` preservation, legacy
lease construction/revocation inside `loom-vault`, production AST ownership,
and existing route/startup/shutdown parity. Ten-run race, full daemon, full
repository, vet, and whitespace checks pass.

## Remaining boundary

This closes the source construction-extraction exit of COMP2-C; it does not
complete COMP2-D or Phase 2D. Conversation, Team, Agent, Attempt, and Turn scope
owners; snapshot plus Frozen Execution Binding scope freeze; lease/session/tool
channel expiry; COMP2-E legacy removal; diagnostics UI/export; accounting; CV6;
ATL9; and installed live acceptance remain open.

No App was built, signed, installed, or launched. No credential, Provider,
network, user workspace, or external tool was accessed.
