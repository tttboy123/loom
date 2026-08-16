# ADR-0021: Governed composition kernel and daemon strangler migration

**Date**: 2026-08-14
**Status**: accepted
**Deciders**: Product Owner, Loom Architecture Controller

## Context

Loom is a Harness Platform. Its Core owns Agent lifecycle, Runtime contracts,
Provider Account binding, Context Capsule disclosure, tools, policy, Vault,
Journal, recovery, and terminal authority. The product daemon nevertheless
still assembles these capabilities through a large manual composition root and
routes IPC methods through a handler with fifteen service dependencies.

That shape makes desktop, headless, and test products difficult to compare,
spreads extension points across constructors, and makes partial startup cleanup
hard to prove. Cordis demonstrates useful Profile, Bundle, scoped service
context, Effect, and lifecycle ideas, but its shared context must not become a
Prompt context, global service locator, secret bag, or second authority.

## Decision

1. Phase 2D adds a Loom-owned, in-process, typed Composition Kernel as
   `P2D-COMP1`. It compiles a Launch Profile and versioned built-in Bundles into
   an immutable `CompositionSnapshot` before activation.
2. A Bundle declares stable ID/version, required and provided capability keys,
   route descriptors, lifecycle hooks, and compatibility constraints. Bundle
   ordering, capability graph, route table, Profile identity, and schema
   version are canonicalized into the snapshot digest.
3. `CapabilityContext` is a process-local capability container with scopes
   `Root -> Product -> Conversation -> Team -> Agent -> Attempt -> Turn`.
   Children may remove or narrow capabilities but cannot override protected
   capabilities or expand authority.
4. `CapabilityContext`, Go `context.Context`, `AttemptContext`, and
   `ContextCapsule` remain distinct. Capability references never become model
   input. Prompt, transcript, tool-result content, Provider response, hidden
   reasoning, plaintext credentials, or Vault master keys cannot be registered.
5. Core-only capabilities include Journal append authority, policy and grant
   authorities, credential Vault root access, authoritative state writers,
   Composition activation, and terminal Run/Attempt decisions. Bundles receive
   bounded ports, not replaceable authority objects.
6. Registration and startup return reversible Effects. Failed activation
   disposes started Effects in exact reverse order. Effects may undo
   registrations and temporary resources; they cannot undo committed Journal
   facts or claim external side effects were reversed.
7. Lifecycle is `Register -> Validate -> Start -> Ready -> Stop -> Dispose`.
   A snapshot is not visible to requests before every mandatory Bundle is
   Ready. Stop is forward quiescence; Dispose is reverse cleanup.
8. Active Conversations and Attempts retain the exact admitted snapshot digest.
   Runtime, Provider Account, credential revision, policy, tool schema, or
   protected capability cannot be hot-replaced underneath them.
9. Phase 2D supports only compiled-in, versioned Loom Bundles. Arbitrary dynamic
   third-party code loading, hot plugin replacement, and remote Bundle download
   remain out of scope.
10. `P2D-COMP2` migrates the product daemon by strangulation. It first wraps
    existing services in compatibility Bundles without changing behavior, then
    replaces the positional handler with a capability/route registry, and only
    then moves service construction out of the monolithic builder.
11. Route availability is declared by closed `RouteDescriptor` values and
    validated at compile time. Duplicate methods, missing handlers, unavailable
    required capabilities, ambiguous ownership, and protected-route overrides
    fail startup.
12. The target `product_daemon.go` responsibility is limited to selecting a
    Profile, compiling and activating a snapshot, starting local IPC, and
    stopping the lifecycle. Journal, Vault, Conversation, Runtime, governance,
    work, assets, diagnostics, and IPC remain Loom-owned built-in Bundles.

## Consequences

- Desktop, headless, and test products can prove composition equivalence from
  explicit Profiles and snapshot digests.
- Runtime and Provider expansion gains one governed registration seam without
  weakening per-Agent binding or Loom Harness Core authority.
- Migration is incremental and testable, but both the legacy composition path
  and Bundle facade temporarily coexist.
- Snapshot schema and protected capability keys become durable compatibility
  contracts and require versioned migration.
- This decision did not itself implement COMP1 or COMP2. Their contracts were
  accepted as `TARGET / NOT YET IMPLEMENTED` Phase 2D prerequisites.

## Implementation status

`P2D-COMP1` became `SOURCE VERIFIED / PRODUCT ACTIVATION OPEN` on 2026-08-14.
The kernel implementation and evidence are recorded in
[`P2D-COMP1-governed-composition-kernel-v1.md`](../../.loom-evidence/phase2d/P2D-COMP1-governed-composition-kernel-v1.md).
`P2D-COMP2-A` and `P2D-COMP2-B` became source verified on 2026-08-14. Product
startup now activates the compatibility composition and consumes the exact
47-method declarative route registry without calling the fifteen-parameter
handler. The first `P2D-COMP2-D` slice also opens and owns the Product scope
before IPC admission. `P2D-COMP2-C` now moves Assets service/API, Asset
authority/Evidence ownership, startup recovery, and route/materializer lifetime
into `loom-assets` Start/Effect. Mission Execution, Handoff, saved-Team
materialization, and their closer now live in `loom-agent-runtime` Start/Effect,
which consumes only the bounded Team asset materializer port. Remaining C
service families, deeper D scopes, and E remain target work. The first
`loom-work` now constructs and owns Queue, Workers, Integration, governed
Execution, and Production as one atomic typed route set. Execution recovery
completes before Ready, and `loom-agent-runtime` consumes only the bounded
private tool-execution port after Work starts; the concrete Adapter and Evidence
lifetime remain owned by Work. Production keeps its Journal-replayed AdminLock
and degraded-write policy behind a narrow route port. `loom-governance` now
owns Permission, Customer Rule, and Standing Order construction; its Rules
authority remains internal, and Work receives only Request/Consume approval
methods after Governance Ready. It also owns Prepared Mission Decision,
execution-control routing, and explicit fallback preparation; the backend and
fallback StateWriter stay internal while Read and Agent Runtime receive bounded
interfaces. Provider Account Policy and Model Rate Card authority now also live
inside Governance; Setup receives only their exact configuration interfaces.
Operational diagnostics now use an explicit bounded `loom-observability` slot
for Read, Agent Attempt, Context retrieval, tool, and IPC operations after
Bundle Ready. The owner-only store remains a pre-composition bootstrap
dependency only for credential-runtime selection, Conversation migration, and
Composition lifecycle failures until its own construction can move without
losing startup evidence. Legacy construction and the monolithic dispatch switch
remain compatibility oracles until parity passes.
The bounded handoff evidence is recorded in
[`P2D-COMP2-C-observability-bootstrap-handoff-v8.md`](../../.loom-evidence/phase2d/P2D-COMP2-C-observability-bootstrap-handoff-v8.md).
`LocalProductReadService` now also consumes a narrow Conversation port, and
`loom-conversation` owns its bind/revoke lifecycle. Evidence:
[`P2D-COMP2-C-conversation-route-handoff-v9.md`](../../.loom-evidence/phase2d/P2D-COMP2-C-conversation-route-handoff-v9.md).
`loom-vault` now constructs and privately owns the real Vault or recovery
runtime, and `loom-conversation` constructs Provider clients, the Profile
router, migration recorder, and encrypted/persistent Chat after Vault Ready.
The production builder retains no constructor for those resources. The lazy Pi
Conversation runtime, Setup/read construction, deeper scopes, and COMP2-E
remain open. Evidence:
[`P2D-COMP2-C-vault-conversation-construction-v10.md`](../../.loom-evidence/phase2d/P2D-COMP2-C-vault-conversation-construction-v10.md).
`loom-observability` now also constructs its persistent store during Start. A
bounded privacy-safe bootstrap sink preserves pre-start Composition records and
flushes them before route publication; it performs no filesystem I/O and is not
Capability Context. Lazy Pi Conversation ownership, Setup/read construction,
deeper scopes, and COMP2-E remain open. Evidence:
[`P2D-COMP2-C-observability-construction-v11.md`](../../.loom-evidence/phase2d/P2D-COMP2-C-observability-construction-v11.md).
The final built-in `loom-local-ipc` Start now constructs the existing Setup
aggregate after all required dependencies are Ready and binds it through a
revocable slot. Vault and Governance roots remain behind bounded ports, exact
build reasons survive rollback, and product startup no longer constructs
Setup. Read construction, lazy Pi Conversation ownership, deeper scopes, and
COMP2-E remain open. Evidence:
[`P2D-COMP2-C-setup-construction-v12.md`](../../.loom-evidence/phase2d/P2D-COMP2-C-setup-construction-v12.md).
The existing Read aggregate now constructs atomically with Governance and
publishes only a revocable snapshot/timeline/chat/observer port. Agent Runtime
no longer receives the concrete Read service; Journal, Projection, cached view,
and observer state remain private. Lazy Pi Conversation ownership, deeper
scopes, and COMP2-E remain open. Evidence:
[`P2D-COMP2-C-read-construction-v13.md`](../../.loom-evidence/phase2d/P2D-COMP2-C-read-construction-v13.md).
`loom-conversation` now owns the optional shared Pi local-model runtime used by
both Conversation and Agent Runtime. Agent Runtime consumes only its bounded
port and closes before the server. The product builder no longer constructs or
retains that resource. The constructor ownership audit is completed by V15
below. Evidence:
[`P2D-COMP2-C-shared-local-model-ownership-v14.md`](../../.loom-evidence/phase2d/P2D-COMP2-C-shared-local-model-ownership-v14.md).
The protected `loom-core` now constructs and owns SQLite, Journal, Projection,
replay, controlled fixture bootstrap, and verified built-in Runtime records.
Dependent built-in factories use only a private revocable construction slot,
never Capability Context. The legacy/test SecretStore adapter likewise moved
into `loom-vault`. A production constructor allowlist closes the COMP2-C source
construction exit; deeper COMP2-D scopes and COMP2-E remain open. Evidence:
[`P2D-COMP2-C-protected-core-ownership-v15.md`](../../.loom-evidence/phase2d/P2D-COMP2-C-protected-core-ownership-v15.md).
Ordinary Conversation dispatch now uses a Product-owned, one-time-bound scope
manager to open Conversation, Team, stable Loom Agent, Attempt, and Turn before
Provider dispatch. Attempt freezes the active snapshot and Conversation binding
digests; dispatch failure rolls back before Provider access, and raw thread IDs
are replaced by domain-separated opaque diagnostic IDs. Team mission and
resource-owning scopes remain open. Evidence:
[`P2D-COMP2-D-conversation-attempt-scope-v2.md`](../../.loom-evidence/phase2d/P2D-COMP2-D-conversation-attempt-scope-v2.md).
The real Team mission runner now opens an opaque internal execution
Conversation and Team scope before executor construction. Every source and
verifier Run freezes its full per-Agent `FrozenExecutionBinding` and opens its
Agent, Attempt, and Turn before delegate execution; admission failure therefore
cannot reach a Runtime or Provider. Reverse cleanup closes Attempt/Turn and the
executor before Team ownership is revoked. Resource-owning Effects and
multi-turn Tool Loop scopes remain open. Evidence:
[`P2D-COMP2-D-team-agent-attempt-scope-v3.md`](../../.loom-evidence/phase2d/P2D-COMP2-D-team-agent-attempt-scope-v3.md).
Each mission Agent Attempt now also owns its cancellable Go execution context
as an idempotent Composition Effect. The delegate receives that context only
after scope admission; ordinary Attempt close and wider Team/Product revocation
both cancel it. This links lifecycle without conflating Capability Context, Go
context, Attempt Context, or Context Capsule. Evidence:
[`P2D-COMP2-D-attempt-cancellation-ownership-v4.md`](../../.loom-evidence/phase2d/P2D-COMP2-D-attempt-cancellation-ownership-v4.md).
Mission Attempt leases now also own deterministic Turn transitions. A private
controller in the Attempt execution context advances N to N+1 only after a
resolved governed tool result and required payload persistence; ask remains in
N and stale/duplicate transitions fail closed. The new Turn inherits the same
snapshot and full execution binding from the immutable Attempt. Evidence:
[`P2D-COMP2-D-sequential-tool-turn-scope-v5.md`](../../.loom-evidence/phase2d/P2D-COMP2-D-sequential-tool-turn-scope-v5.md).
The existing Vault lease path is now verified under those Agent Attempt
contexts. Exact Provider Account/reference/revision leases inherit Attempt
cancellation; account-local revoke terminates and zeroizes only the matching
Agent while a peer remains active until its own scope closes. Evidence:
[`P2D-COMP2-D-credential-lease-isolation-v6.md`](../../.loom-evidence/phase2d/P2D-COMP2-D-credential-lease-isolation-v6.md).
Ordinary Conversation Attempts now own the execution context passed to their
Provider responder. Product close cancels active responder work with the stable
scope-close cause, and a real encrypted ExternalSessionHandle callback inherits
that cancellation and zeroizes its plaintext after return. Missing scope
contexts fail closed before Provider dispatch. This verifies the native-session
lifecycle boundary but does not claim currently stateless Provider adapters
have adopted native handle reuse. Evidence:
[`P2D-COMP2-D-conversation-provider-session-lifecycle-v7.md`](../../.loom-evidence/phase2d/P2D-COMP2-D-conversation-provider-session-lifecycle-v7.md).
The current Pi 0.82.1 governed Tool invocation now has an integrated
cancellation gate across its live child process, private UDS, pending Tool
request, extension file, and temporary socket roots. Attempt-context
cancellation performs a content-free denial, native abort, process-group reap,
connection/listener close, and residue removal. This does not extend the claim
to other Runtime adapters, arbitrary MCP component processes, or installed
crash/restart behavior. Evidence:
[`P2D-COMP2-D-pi-tool-process-resource-cleanup-v8.md`](../../.loom-evidence/phase2d/P2D-COMP2-D-pi-tool-process-resource-cleanup-v8.md).
Claude Code and Codex Harness runners now include owner-only system-prompt
cleanup in their fail-closed result. Attempt-context cancellation reaps each
real local process group and removes the private prompt file/directory; a
blocked cleanup path returns `ErrHarnessProtocol` instead of success. This does
not claim live CLI/Provider acceptance or crash-safe gateway/Context MCP
shutdown. Evidence:
[`P2D-COMP2-D-harness-process-private-prompt-cleanup-v9.md`](../../.loom-evidence/phase2d/P2D-COMP2-D-harness-process-private-prompt-cleanup-v9.md).
The Attempt credential gateway and Harness Context MCP now inherit Attempt
cancellation directly. Their loopback listeners are revoked before a blocked
trusted callback or runner returns, and Context MCP clears its bearer token
under synchronized access. This is normal-process lifecycle ownership only;
callback-bound plaintext cleanup and daemon crash/restart residue remain open.
Evidence:
[`P2D-COMP2-D-attempt-loopback-service-revocation-v10.md`](../../.loom-evidence/phase2d/P2D-COMP2-D-attempt-loopback-service-revocation-v10.md).
The production Pi Context extension now also inherits Agent Attempt
cancellation. A blocked Context delivery is cancelled and its private UDS,
extension file, socket roots, and connection are removed without waiting for
the outer Adapter return. The locked Pi binary gate remains open. Evidence:
[`P2D-COMP2-D-pi-context-extension-revocation-v11.md`](../../.loom-evidence/phase2d/P2D-COMP2-D-pi-context-extension-revocation-v11.md).
Production local IPC construction now also belongs to the built-in
`loom-local-ipc` Bundle. The builder supplies a typed revocable handler slot and
factory; Bundle Start constructs Setup and the typed route handler, while Ready
and reverse cleanup gate admission and quiesce old references. The direct
COMP2-A handler remains only as a parity oracle, so this does not authorize
COMP2-E deletion or claim installed startup parity. Evidence:
[`P2D-COMP2-D-local-ipc-bundle-ownership-v12.md`](../../.loom-evidence/phase2d/P2D-COMP2-D-local-ipc-bundle-ownership-v12.md).

## Alternatives Considered

### Keep adding constructor parameters

Rejected because dependency presence, route ownership, startup order, and
cleanup remain implicit and increasingly difficult to verify.

### Adopt Cordis as Loom's daemon core

Rejected because Loom must retain its Journal, Vault, binding, policy,
disclosure, recovery, and terminal authority. Loom ports composition behavior,
not Cordis authority or plaintext session semantics.

### Global mutable service locator

Rejected because it permits hidden dependencies, runtime replacement, scope
expansion, and accidental secret/authority sharing.

### Rewrite the product daemon in one change

Rejected because it would combine composition redesign with behavioral changes
across credentials, conversation, execution, governance, and IPC. The accepted
path is compatibility-first strangulation with parity gates at every step.

## References

- [P2D-COMP1 contract](../../.loom-evidence/phase2d/contracts/P2D-COMP1-composition-kernel.md)
- [P2D-COMP2 contract](../../.loom-evidence/phase2d/contracts/P2D-COMP2-product-daemon-strangler.md)
- [Attempt Tool Loop architecture](../architecture/attempt-tool-loop.md)
- [ADR-0019](0019-per-agent-execution-profiles-and-provider-accounts.md)
