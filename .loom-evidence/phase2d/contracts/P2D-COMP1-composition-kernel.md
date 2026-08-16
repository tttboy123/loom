# P2D-COMP1 - Governed Composition Kernel

**Status**: SOURCE VERIFIED / PRODUCT ACTIVATION OPEN  
**Goal**: Phase 2D only; this is not a new product Goal  
**Owner**: Loom Harness Platform Core  
**Decision**: ADR-0021

## Outcome

Loom compiles one explicit Launch Profile and a closed set of versioned built-in
Bundles into an immutable, content-addressed Composition Snapshot. The snapshot
provides typed capabilities and routes through scoped contexts, activates them
deterministically, and rolls back temporary registration/startup Effects in
reverse order when activation fails.

This is composition authority, not execution authority. It cannot create a
Team, dispatch an Agent, approve a ToolCall, read a secret, append arbitrary
Journal facts, or disclose model context merely because a capability exists.

## Vocabulary

- `LaunchProfile`: immutable product shape. Phase 2D defines `desktop`,
  `headless`, and `test`; `safe-mode` is reserved for a later recovery slice.
- `BundleDescriptor`: stable ID/version plus requires, provides, routes,
  lifecycle identity, compatibility constraints, and protected-core marker.
- `CapabilityKey`: closed, versioned, typed identity for a bounded service port.
- `CapabilityContext`: scoped process-local lookup of admitted capability ports.
- `Effect`: one registered or started temporary resource with idempotent close.
- `CompositionSnapshot`: canonical Profile, ordered Bundles, capability graph,
  route table, lifecycle plan, schema version, and SHA-256 digest.
- `AttemptContext`: frozen execution identity and governance metadata. It is not
  a service container and is not replaced by `CapabilityContext`.
- `ContextCapsule`: disclosure-filtered content sent to a model. It never
  contains a `CapabilityContext` or service object.

## Profile contract

Each Profile freezes:

1. profile ID and schema version;
2. mandatory and optional Bundle IDs with exact compatible versions;
3. required product routes and unavailable-route policy;
4. startup policy, shutdown timeout, and diagnostic mode;
5. whether native UI projections are required;
6. the protected capability set and product feature flags.

The Profile may select product shape. It cannot select a global Provider,
credential, model, Agent binding, permission decision, or fallback. Those remain
per Conversation/Agent/Attempt governed state.

## Bundle contract

Every built-in Bundle declares before Register:

```text
id
version
schema_version
requires[]
optional_requires[]
provides[]
routes[]
lifecycle_id
core_protected
compatibility[]
```

Descriptors contain no secret, Prompt, filesystem content, Provider response,
mutable client, or process handle. Factories receive only the bounded context
and typed configuration admitted by the Profile.

Initial built-in Bundle ownership is:

- `loom-core`: Journal/projection roots and protected authorities;
- `loom-vault`: Credential Vault, leases, encrypted payload/document stores;
- `loom-conversation`: Conversation/Segment/Profile/Capsule coordination;
- `loom-agent-runtime`: Runtime registry, adapters, Supervisor, Attempt Loop;
- `loom-governance`: policy, grants, approvals, sandbox, fallback, accounting;
- `loom-work`: Team, Queue, WorkItem, Run and scheduling services;
- `loom-assets`: Evidence and governed asset services;
- `loom-observability`: operational diagnostics and diagnostic export ports;
- `loom-local-ipc`: local route activation and UDS server ownership.

Bundle boundaries may be refined only by a versioned contract amendment. They
must not create a Team-level Provider client or bypass exact per-Agent
Execution Binding resolution.

## Protected capability contract

The following are core-only and non-overridable:

- Event Journal append authority and migration ownership;
- authoritative StateWriter and Projection activation;
- policy, grant, approval-consumption, and terminal decision authorities;
- Vault VMK/key provider/root store access;
- Composition compiler and snapshot activation;
- Run/Attempt/ToolCall authority writers;
- credential migration authorization;
- local IPC peer-attestation policy.

Consumers receive narrowed ports such as `CredentialLeaseIssuer`,
`RuntimeDispatcher`, `ScopedContextRetriever`, `ApprovedToolInvoker`, or
read-only projections. Duplicate protected registration always fails compile or
Register; last-writer-wins is forbidden.

## Scoped Capability Context

The only scope chain is:

```text
Root -> Product -> Conversation -> Team -> Agent -> Attempt -> Turn
```

Rules:

1. child lookup may inherit, narrow, mask, or bind a scoped facade;
2. child registration cannot widen parent policy or add protected authority;
3. each scope has a stable non-secret scope identity and parent digest;
4. Attempt scope binds Composition Snapshot digest and Frozen Execution Binding;
5. Turn scope binds the exact Attempt/Turn generation and closes with the Turn;
6. closing a scope expires its leases, tool channels, Provider session leases,
   temporary directories, and cancellable workers through owned Effects;
7. capability objects are never serialized into Prompt, Journal, Evidence,
   diagnostics, argv, ordinary environment, or Provider payload;
8. secret bytes never become a capability value; only a short-lived lease port
   may yield bounded plaintext to an admitted daemon consumer.

## Lifecycle and Effect contract

Lifecycle is strictly:

```text
Register -> Validate -> Start -> Ready -> Stop -> Dispose
```

- Register installs descriptors/factories only and performs no external work.
- Validate resolves the complete graph and route table without starting a
  Provider, helper, Runtime, listener, or user-visible side effect.
- Start follows the canonical dependency order and records each returned Effect.
- Ready is published atomically only after all mandatory Bundles are ready.
- Stop quiesces request admission and active workers in forward dependency order.
- Dispose closes Effects in exact reverse successful-start order.
- Close is idempotent and every failure remains inspectable with Bundle/lifecycle
  stage and Incident ID.
- Effects may remove registrations and temporary resources. They cannot delete
  or invert committed Journal facts, pretend a Provider call was undone, or
  silently re-execute an uncertain side effect.

## Deterministic compiler

The compiler rejects:

- duplicate Bundle ID/version identities;
- missing or cyclic required capabilities;
- duplicate capability providers without an explicit closed aggregation type;
- duplicate/ambiguous route methods;
- protected capability or route override;
- unknown schema/profile/capability/route version;
- nondeterministic descriptor input;
- a Bundle whose declared dependencies differ from factory consumption.

For equal canonical input, Bundle order, capability graph, route order,
lifecycle plan, canonical bytes, and snapshot digest must be byte-identical.
Map iteration, registration timing, pointer identity, and process state cannot
affect the digest.

## Snapshot admission and upgrade

- Product startup admits one snapshot before IPC becomes available.
- Conversation and Attempt records retain the admitted snapshot digest.
- A new snapshot applies only to new scopes unless an explicit migration proves
  compatibility and no active binding is rewritten.
- Runtime, Provider Account, model, credential revision, tool schema, policy,
  budget, and disclosure bindings remain frozen independently of composition.
- Snapshot metadata is non-secret. It may enter diagnostics and support bundles,
  but cannot contain endpoint credentials, Prompt, content, or Provider body.

## Diagnostics

Minimum safe stages are:

```text
composition_compile
composition_validate
bundle_register
bundle_start
bundle_ready
bundle_stop
bundle_dispose
route_compile
scope_open
scope_close
```

Records include Incident ID, Profile ID, snapshot digest, Bundle ID/version,
stage, elapsed time, result, controlled error code, and retryability. They omit
configuration bodies, secrets, Prompt, tool arguments/results, Provider bodies,
and internal service state.

## RED and acceptance matrix

1. identical descriptors in different input order produce identical snapshots;
2. desktop/headless/test compile to their exact expected Bundle/route sets;
3. missing dependency, cycle, duplicate capability, and duplicate route fail;
4. protected capability/route replacement fails before Start;
5. Bundle N startup failure disposes N-1..1 in exact reverse order once;
6. Validate failure starts zero resources;
7. Ready is not visible before all mandatory Bundles are ready;
8. Stop rejects new requests and Dispose is idempotent under concurrent close;
9. child scopes cannot widen capabilities or retain leases after close;
10. active Attempt retains its original snapshot while a later snapshot exists;
11. canonical snapshot/diagnostic bytes contain no secret or content marker;
12. race tests prove deterministic compile and scope close under concurrency;
13. Journal facts survive Effect rollback and are never treated as reversible;
14. Profile/Bundle schema substitution and unknown versions fail closed.

## Exit criteria

COMP1 exits only when the kernel, typed descriptors, compiler, snapshot digest,
scope rules, lifecycle/Effect rollback, diagnostics, replay/version tests, race
tests, and privacy-negative tests are source verified. Product behavior does not
switch to it until COMP2 parity gates pass.

## Source verification

`CURRENT / SOURCE VERIFIED (2026-08-14)`: `internal/composition` now implements
the closed Profile/Bundle model, typed capability keys, deterministic compiler,
canonical snapshot digest, read-only narrowed Bundle lifecycle context, exact
operational scope chain, protected-Core masking, Effect rollback, atomic Ready,
safe diagnostics, and Attempt snapshot/execution-binding freeze.

Focused tests pass at 83.0% statement coverage, including 20 race-enabled runs;
the full repository Go suite and `go vet ./...` also pass. Product Bundle
facades and complete product route manifests remain P2D-COMP2-A/B, so no daemon
or IPC production behavior has switched. Evidence:
`../P2D-COMP1-governed-composition-kernel-v1.md`.

## Non-goals

- arbitrary third-party dynamic plugins or downloaded code;
- hot replacement beneath active Attempts;
- replacing Go `context.Context` cancellation/deadline propagation;
- merging Capability Context with Context Capsule or Agent private memory;
- exposing Vault, Journal writer, or policy authority to extensions;
- changing Provider, credential, model, fallback, budget, or disclosure policy;
- claiming installed readiness before the existing Phase 2D live matrix.
