# P2D-COMP2-D Team Agent Attempt Scope V3

**Date**: 2026-08-14  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / COMP2-D PARTIAL  
**Contract**: `contracts/P2D-COMP2-product-daemon-strangler.md`  
**Decision**: `../../docs/adr/0021-governed-composition-kernel-and-daemon-strangler.md`

## Accepted boundary

The real mission execution runner now opens one Product-owned execution scope
before constructing or invoking an executor:

```text
Product -> opaque mission Conversation -> Team
                                     -> Agent A -> Attempt -> Turn
                                     -> Agent B -> Attempt -> Turn
```

The mission Conversation is an internal hierarchy owner, not a user-visible
Conversation. Its identity and the Team identity are domain-separated SHA-256
opaque values derived from the exact Team and execution correlation. Raw Team,
Agent, Run, and correlation identifiers do not enter Composition diagnostics.

Every source or verifier execution passes through `productMissionScopedExecutor`.
Before the delegate can reach a Runtime or Provider adapter, the wrapper freezes
the exact `RuntimeProfile` plus `RuntimeInstance` as a full
`runtime.FrozenExecutionBinding`, opens the stable Agent child, and opens an
Attempt plus Turn carrying the active Composition Snapshot digest and binding
digest. Different Agents in the same Team retain independent Agent scopes and
independent immutable binding digests. A scope or binding failure returns before
the delegate executes.

Attempt and Turn close on every executor return. The Team execution lease owns
the stable Agent children and closes after the underlying executor closes, so
reverse cleanup cannot leave an executor using revoked Team capabilities.
Production injects the same one-time-bound scope slot used by Conversation into
the Agent Runtime mission configuration; tests that construct execution without
Composition retain the previous compatibility behavior.

Capability Context remains separate from Go `context.Context`, Attempt Context,
and Context Capsule. No Prompt, transcript, Provider body, credential, secret,
VMK, Journal writer, StateWriter, grant authority, or terminal authority is
placed in these scopes.

## RED and verification

The RED failed on the missing Team execution admission, per-Agent Attempt
request, mission executor wrapper, and production wiring. The implemented
boundary passed:

```text
go test ./cmd/loomd -run 'TestCOMP2D(Team|ScopedMission)' -count=1
go test ./cmd/loomd -run '^TestCOMP2D' -count=1
go test ./internal/api ./cmd/loomd -run '^(TestCOMP2|TestProduct.*Mission|TestProduct.*Team|TestLocalProduct.*Chat)' -count=1
go test -race ./internal/api ./cmd/loomd -run '^(TestCOMP2D|TestCOMP2C)' -count=10
go test ./cmd/loomd -count=1
go test ./... -count=1
go vet ./...
git diff --check
```

Tests prove two-Agent scope isolation with different binding digests, exact
snapshot freeze, fail-closed admission before delegate execution, source and
verifier wrapping, production slot injection, opaque diagnostics, lifecycle
revocation, race safety, and repository parity.

## Remaining boundary

COMP2-D remains open. Attempt scopes do not yet own exact credential leases,
Provider native-session handles, Tool Loop channels, temporary roots, component
processes, or cancellable workers. Multi-turn Tool Loop Turn generation,
restart/crash recovery of scoped resources, broader Runtime adapters, and the
COMP2-E parity/removal gates also remain open. Installed CV6, mixed-Team ATL9,
UI/accounting, and live Provider acceptance remain mandatory.

No App was built, signed, installed, or launched. No credential, Provider,
network, user workspace, or external tool was accessed.
