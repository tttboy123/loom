# P2D-COMP2 - Product Daemon Strangler Migration

**Status**: ACCEPTED FOR PHASE 2D / COMP2-A-C SOURCE VERIFIED / COMP2-D RESIDUAL MIGRATION TRACKED / COMP2-E SOURCE + INSTALLED VERIFIED
**Goal**: Phase 2D only; depends on P2D-COMP1  
**Owner**: Loom Harness Platform product composition  
**Decision**: ADR-0021

## Outcome

The product daemon selects a Launch Profile, compiles and activates built-in
Bundles, starts local IPC from the immutable Composition Snapshot, and closes
the lifecycle. Existing Conversation, Provider, Vault, Agent Runtime,
governance, Work, assets, diagnostics, and IPC behavior remains equivalent
during migration.

This is a strangler migration. It does not replace the product daemon in one
rewrite, change wire protocols, or relax Loom authority boundaries.

## Current seams to replace

1. `newProductDaemonRunnerWithPreparedDecisions` constructs most product
   services and owns manual dependency order.
2. `localProductHandlerWithComposition` receives fifteen service categories and
   performs method availability checks in a large conditional router.
3. `productDaemonRunner.Close` manually owns shutdown order.

These functions remain the compatibility oracle until parity gates permit their
individual responsibilities to move. Parameter bundling alone is not COMP2.

## Migration slices

### COMP2-A - Compatibility Bundle facade

`SOURCE VERIFIED (2026-08-14)`: production service construction remains the
compatibility oracle, while its final wrapped handler now runs behind an
activated desktop snapshot before local IPC admission. Headless/test manifests,
exact delegation, persistent diagnostics, fail-closed startup, quiescent close,
race, and full-repository parity pass. Evidence:
`../P2D-COMP2-A-compatibility-bundle-facade-v1.md`.

- Build the accepted initial built-in Bundle descriptors.
- Factories call existing constructors and handlers without behavioral change.
- Compile desktop/headless/test snapshots around the legacy implementation.
- Record snapshot and lifecycle diagnostics before IPC admission.
- Keep existing handler tests as black-box parity tests.

Exit: the legacy daemon can run wholly behind one activated snapshot, with the
same routes, responses, state paths, startup failure behavior, and shutdown
effects.

### COMP2-B - Declarative route registry

`SOURCE VERIFIED (2026-08-14)`: the compatibility snapshot now freezes all 47
local product methods as exact, sorted RouteDescriptors owned by the applicable
built-in Bundle. Product startup constructs a typed `productRouteServices` and
`productRouteRegistry` directly; the fifteen-parameter function remains only as
a compatibility wrapper for existing tests. Availability errors, Journey and
Conversation staging, degraded writes, desktop/headless/test digests, and
legacy dispatch responses remain compatible. Evidence:
`../P2D-COMP2-B-declarative-route-registry-v1.md`.

Introduce closed `RouteDescriptor` values with:

```text
method
schema_version
owner_bundle
required_capabilities[]
handler_capability
availability_failure
incident_policy
privacy_class
```

- Compile route ownership and requirements before Start.
- Replace positional service parameters with typed handler capabilities.
- Preserve exact current local IPC response/error compatibility.
- Reject duplicate methods, missing handlers, ambiguous ownership, unknown
  schema, protected route override, and undeclared availability behavior.
- Route descriptors contain no request/response content or credential data.

Exit: `localProductHandlerWithComposition` no longer owns method availability
logic or fifteen positional dependencies. A compatibility wrapper may remain
temporarily for old tests but cannot be used by product startup.

### COMP2-C - Service construction extraction

`PARTIAL / ASSETS OWNERSHIP AND AGENT RUNTIME CONSTRUCTION SOURCE VERIFIED
(2026-08-14)`: the
`loom-assets` Start hook now constructs and binds the typed Assets service/API
route, Assets authority, Evidence store, skill materializer, and bounded Team
asset materializer; its Effect quiesces, revokes both ports, and closes Assets
ownership. Product startup no longer directly owns these resources.
`loom-agent-runtime` Start constructs Mission Execution, Handoff, and Team
materialization routes, owns the old execution closer, and consumes only the
bounded Team asset materializer port. Other service families remain
compatibility-owned. Evidence:
`../P2D-COMP2-C-assets-route-construction-v1.md` and
`../P2D-COMP2-C-agent-runtime-construction-v1.md` and
`../P2D-COMP2-C-assets-authority-ownership-v1.md`.

`PARTIAL / WORK ROUTES CONSTRUCTION V2 SOURCE VERIFIED (2026-08-14)`:
`loom-work` Start now atomically constructs Queue, Workers, and Integration
through one typed Work route slot; its Effect quiesces and revokes all six
methods. Product startup no longer constructs these services/APIs directly.
Assets -> Work -> Agent Runtime order, reverse cleanup, exact build-stage
identity, and real socket journeys are source verified. Execution, Production,
work/governance authority, setup/read routes, and deeper scopes remain
compatibility-owned. Evidence:
`../P2D-COMP2-C-work-queue-construction-v1.md` and
`../P2D-COMP2-C-work-routes-construction-v2.md`.

`PARTIAL / WORK EXECUTION OWNERSHIP V3 SOURCE VERIFIED (2026-08-14)`:
`loom-work` Start now also constructs the existing governed Execution Evidence
store, decision recorder, sandbox gate, Adapter, pending recovery, service, and
API. It publishes the Execution route plus a private tool-execution port only
after recovery succeeds. `loom-agent-runtime` consumes that bounded port after
Work Ready; no concrete Adapter or execution content enters Capability Context.
Reverse cleanup closes Agent Runtime before Work, revokes both ports, and closes
the Evidence owner exactly once. Exact `build_execution` and
`build_execution_recovery` failures, rollback, shutdown, race, AST ownership,
and repository parity are source verified. Production, governance/work
authority, setup/read routes, Conversation/observability construction, and
deeper scopes remain compatibility-owned. Evidence:
`../P2D-COMP2-C-work-execution-ownership-v3.md`.

`PARTIAL / WORK PRODUCTION OWNERSHIP V4 SOURCE VERIFIED (2026-08-14)`:
`loom-work` Start now also resolves the existing Production paths and constructs
the Production core, local service, and API. Its typed route set requires
Snapshot, Command, and the read-only Degraded gate before Ready. The existing
sandbox/user path semantics, daemon executable, Journal-replayed AdminLock,
authoritative projection, and degraded-write policy remain unchanged. Failed
construction preserves `build_production`; cleanup revokes the route and fails
closed as degraded. Product construction no longer calls any Production
constructor. Composition, real socket journey, race, AST ownership, shutdown,
and repository parity are source verified. Governance/work authority, setup/read
routes, Conversation/observability construction, and deeper scopes remain
compatibility-owned. Evidence:
`../P2D-COMP2-C-work-production-ownership-v4.md`.

`PARTIAL / GOVERNANCE AUTHORITY OWNERSHIP V5 SOURCE VERIFIED (2026-08-14)`:
`loom-governance` Start now constructs the Permission approval port and Rules
authority, Permission API, Customer Rule API, and Standing Order authority/API.
The authority remains private to the trusted built-in factory. IPC receives only
typed route proxies; Work receives only Request/Consume approval methods after
Governance Ready. Governance failure prevents Work and Product admission;
reverse cleanup closes Work first and revokes the approval port. Exact
Permission/Customer Rule/Standing Order failure stages, route behavior,
Execution approval consumption, race, AST ownership, shutdown, and repository
parity are source verified. Mission decision, Provider policy, setup/read,
Conversation/observability construction, and deeper scopes remain
compatibility-owned. Evidence:
`../P2D-COMP2-C-governance-authority-ownership-v5.md`.

`PARTIAL / GOVERNANCE DECISION OWNERSHIP V6 SOURCE VERIFIED (2026-08-14)`:
`loom-governance` Start now also constructs the Prepared Mission Decision
backend, fallback StateWriter/preparer, Decision API, and Mission Execution
decision router. Backend and writer remain private. Read receives only command
listing, Agent Runtime receives execution-control and explicit fallback ports,
and IPC receives the typed Decision route. Closing Governance revokes all four;
construction preserves `build_decision`. Strict decision routing, prepared-view
projection, mission control, explicit mixed-Team fallback, race, AST ownership,
shutdown, and repository parity are source verified. Provider policy,
setup/read, Conversation/observability construction, and deeper scopes remain
compatibility-owned. Evidence:
`../P2D-COMP2-C-governance-decision-ownership-v6.md`.

`PARTIAL / GOVERNANCE PROVIDER ACCOUNT POLICY V7 SOURCE VERIFIED
(2026-08-14)`: `loom-governance` Start now also constructs Provider Account
Policy and Model Rate Card authority. Setup receives only their two exact
configuration interfaces and no longer constructs the authority. Account-local
identity, revision, concurrency, rate, budget, disclosure and cost freezing,
trusted correlation, projection refresh, revocation, exact
`build_setup_policy`, race, AST ownership, shutdown, and repository parity are
source verified. Setup/read and Conversation/observability construction,
accounting UI, and deeper scopes remain compatibility-owned. Evidence:
`../P2D-COMP2-C-governance-provider-account-policy-v7.md`.

`PARTIAL / OBSERVABILITY BOOTSTRAP HANDOFF V8 SOURCE VERIFIED
(2026-08-14)`: the owner-only operational store remains available before
Composition solely so credential-runtime selection, encrypted Conversation
migration, and lifecycle failures are recorded even when activation fails.
`loom-observability` Start now binds a revocable bounded slot used by Read,
Agent Attempt, Context retrieval, tool, and IPC diagnostics after Ready.
Downstream production no longer retains the store directly. Atomic failure,
`build_diagnostics`, close revocation, privacy, UDS parity, race, AST ownership,
shutdown, and repository checks are source verified. V10/V11 below subsequently
move Vault/Conversation and persistent store construction. Setup/read
construction, UI/export/accounting, and deeper scopes remain open. Evidence:
`../P2D-COMP2-C-observability-bootstrap-handoff-v8.md`.

`PARTIAL / CONVERSATION ROUTE HANDOFF V9 SOURCE VERIFIED (2026-08-14)`:
Read now consumes `LocalProductChatSource`, while `loom-conversation` Start owns
binding and revocation of the existing chat route. Failure blocks Agent Runtime
and Product/IPC admission and preserves `build_setup_provider`. Profile,
Segment, encrypted migration, Context Capsule, Provider Account binding, UDS,
race, AST ownership, shutdown, and repository parity are source verified.
Provider/router/chat construction remains compatibility-owned until Vault
bounded ports are available during activation. Evidence:
`../P2D-COMP2-C-conversation-route-handoff-v9.md`.

`PARTIAL / VAULT AND CONVERSATION CONSTRUCTION V10 SOURCE VERIFIED
(2026-08-14)`: `loom-vault` Start now constructs and owns the real LocalKeyFile
Vault or recovery runtime while exposing only bounded ports to trusted built-in
consumers. `loom-conversation` Start constructs Provider clients, the
account-aware Profile router, migration recorder, and persistent encrypted Chat
API after Vault Ready. Reverse cleanup revokes Conversation before Vault, exact
credential/native-auth/Provider failure stages remain stable, recovery reset
keeps the same bounded slot, and unavailable writes clear sensitive content.
The production builder no longer calls those constructors. The lazy Pi
local-model Conversation resource, Setup/read construction, minimal diagnostics
bootstrap construction, deeper scopes, and COMP2-E remain open. Evidence:
`../P2D-COMP2-C-vault-conversation-construction-v10.md`.

`PARTIAL / OBSERVABILITY CONSTRUCTION V11 SOURCE VERIFIED (2026-08-14)`:
`loom-observability` Start now constructs the persistent owner-only store. A
no-I/O, fixed-bound, privacy-safe bootstrap sink buffers validated Composition
records generated before Observability Start, then binds the credential-runtime
identity and flushes in order before routes publish. Failure remains
`build_diagnostics`; lifecycle stop/dispose records persist after route
revocation. The production builder no longer constructs the store. Evidence:
`../P2D-COMP2-C-observability-construction-v11.md`.

`PARTIAL / SETUP CONSTRUCTION V12 SOURCE VERIFIED (2026-08-14)`: the final
`loom-local-ipc` Start constructs the existing Setup aggregate after every
required built-in dependency is Ready and binds a revocable typed slot into the
precompiled product handler. Vault and Governance roots remain behind bounded
ports; credential input is cleared on unavailable routes; exact Setup build
reasons, transaction behavior, reverse cleanup, UDS parity, race, and repository
checks are source verified. The production builder no longer constructs Setup.
Read construction, lazy Pi Conversation ownership, and deeper scopes remain
open. Evidence: `../P2D-COMP2-C-setup-construction-v12.md`.

`PARTIAL / READ CONSTRUCTION V13 SOURCE VERIFIED (2026-08-14)`: the existing
Read aggregate now constructs atomically with Governance and publishes only a
revocable typed snapshot/timeline/chat/observer port. Agent Runtime no longer
receives the concrete Read service. Journal, Projection, cached views, streams,
and observer state stay private; failure rolls back Governance as `build_state`;
reverse cleanup closes observers before Governance. The production builder no
longer constructs Read. Evidence: `../P2D-COMP2-C-read-construction-v13.md`.

`PARTIAL / SHARED LOCAL MODEL OWNERSHIP V14 SOURCE VERIFIED (2026-08-14)`:
`loom-conversation` now constructs and owns the optional shared Pi local-model
runtime. Pi Conversation and Agent Runtime consume the same bounded runtime;
reverse cleanup closes Agent Runtime first and the server exactly once. The
product builder no longer constructs or retains that resource. The required
constructor ownership audit is completed by V15 below. Evidence:
`../P2D-COMP2-C-shared-local-model-ownership-v14.md`.

`SOURCE VERIFIED / PROTECTED CORE OWNERSHIP V15 (2026-08-14)`: `loom-core`
now constructs and owns SQLite, Journal, Projection, replay, controlled fixture
bootstrap, and verified built-in Runtime records. Trusted built-in factories
resolve those resources only through a private revocable construction slot that
is never registered in Capability Context. The explicit legacy/test credential
adapter also constructs in `loom-vault` behind a revocable lease slot. A
constructor allowlist proves the product builder retains only bounded launch
configuration, no-I/O diagnostics bootstrap, factory/route assembly,
Composition activation, IPC server creation, and lifecycle handoff. The COMP2-C
construction-extraction exit is source verified; COMP2-D/E remain open.
Evidence: `../P2D-COMP2-C-protected-core-ownership-v15.md`.

Move existing construction into Bundles in bounded order:

1. `loom-observability` and `loom-assets`;
2. `loom-vault`;
3. `loom-conversation`;
4. `loom-governance` and `loom-work`;
5. `loom-agent-runtime`;
6. `loom-local-ipc`;
7. protected `loom-core` remains the root owner throughout.

Every extraction must preserve existing injected test ports, transaction
boundaries, exact credential/Vault behavior, Conversation encryption,
per-Agent execution binding, Attempt Loop, diagnostics, and shutdown ownership.
No extraction may introduce a global Provider client or Team-level credential.

Exit: `newProductDaemonRunnerWithPreparedDecisions` no longer constructs domain
services. A temporary compatibility entry may only translate old test config to
the same Profile/Bundle compiler.

### COMP2-D - Scope integration

`PARTIAL / PRODUCT SCOPE SOURCE VERIFIED (2026-08-14)`: the production
composition now opens and owns the exact Product scope before IPC admission,
persists its open/close diagnostics, and closes it before activation/root
cleanup. Conversation through Turn scopes remain open and depend on bounded
COMP2-C construction inversion. Evidence:
`../P2D-COMP2-D-product-scope-v1.md`.

`PARTIAL / CONVERSATION ATTEMPT SCOPE V2 SOURCE VERIFIED (2026-08-14)`:
ordinary Chat dispatch now opens Product-owned Conversation, Team, stable Loom
Agent, Attempt, and Turn scopes before Provider dispatch. Attempt freezes the
active Composition Snapshot and Conversation execution binding digests; every
return path closes Attempt/Turn, while higher scopes are reused until Product
close. Scope-open failure rolls back before Provider dispatch, and diagnostics
use domain-separated opaque IDs rather than source thread IDs. Team mission
scope identities, full Frozen Execution Binding, multi-turn resource ownership,
and lease/session/tool cleanup remain open. Evidence:
`../P2D-COMP2-D-conversation-attempt-scope-v2.md`.

`PARTIAL / TEAM AGENT ATTEMPT SCOPE V3 SOURCE VERIFIED (2026-08-14)`:
the real mission runner now opens an opaque execution Conversation and Team
scope before executor construction, then wraps every source and verifier
execution. Each Agent Run freezes its full `FrozenExecutionBinding` before the
delegate can reach a Runtime or Provider and opens an independent stable Agent,
Attempt, and Turn hierarchy carrying the active Composition Snapshot and
binding digests. Scope admission failure prevents delegate execution; reverse
cleanup closes Attempt/Turn, executor, and Team ownership in order. Raw Team,
Agent, Run, and correlation identities stay out of diagnostics. Credential,
session, tool, process, temporary-root, and worker Effects remain open.
Evidence: `../P2D-COMP2-D-team-agent-attempt-scope-v3.md`.

`PARTIAL / ATTEMPT CANCELLATION OWNERSHIP V4 SOURCE VERIFIED (2026-08-14)`:
each mission Agent Attempt now owns a cancellable Go execution context as an
idempotent Composition Effect. The delegate receives only that context after
binding and scope admission. Ordinary return cancels it during Attempt close;
Team/Product revocation cancels an in-flight delegate before wider cleanup can
complete. Capability Context, Go context, Attempt Context, and Context Capsule
remain separate. Credential, session, tool, process, and filesystem Effects
remain open. Evidence:
`../P2D-COMP2-D-attempt-cancellation-ownership-v4.md`.

`PARTIAL / SEQUENTIAL TOOL TURN SCOPE V5 SOURCE VERIFIED (2026-08-14)`:
mission Attempt leases now own and deterministically advance Turn scopes. The
private controller is bound only to the Attempt execution context; a resolved
governed tool result advances N to N+1 after required payload persistence,
while ask polling stays in N. Duplicate, stale, cancelled, or out-of-order
sequences fail closed. Snapshot and full binding digests remain inherited from
the immutable Attempt, and diagnostics retain opaque IDs. Complete ATL
sequential execution and resource Effects remain open. Evidence:
`../P2D-COMP2-D-sequential-tool-turn-scope-v5.md`.

`PARTIAL / CREDENTIAL LEASE ISOLATION V6 SOURCE VERIFIED (2026-08-14)`:
the existing Vault hot path is now cross-component verified against real Team
Agent Attempt scopes. Each exact Provider Account/reference/revision lease is
parented by its Agent Attempt context. Revoking one identity terminates and
zeroizes only that Agent's lease; a peer account remains active until its own
Attempt/Team closes, which then cancels and zeroizes it. No Provider-only
credential resolution or Team-wide offline collapse is introduced. Provider
session, tool/process, filesystem, and remaining Effects stay open. Evidence:
`../P2D-COMP2-D-credential-lease-isolation-v6.md`.

`PARTIAL / CONVERSATION PROVIDER SESSION LIFECYCLE V7 SOURCE VERIFIED
(2026-08-14)`: each ordinary Conversation Attempt now owns a cancellable
execution context as a Composition Effect, and Chat passes only that context to
the Provider responder after scope admission. Missing or cancelled scope
contexts fail closed and roll back before Provider dispatch. The real encrypted
ExternalSessionHandle callback is verified under this context: Product close
cancels it and plaintext is zeroized after callback exit while the exact
Provider Account/model/Segment/credential revision binding remains frozen.
Provider adapters that are currently stateless have not yet adopted native
handle reuse. Tool/process/filesystem and remaining Effects stay open.
Evidence:
`../P2D-COMP2-D-conversation-provider-session-lifecycle-v7.md`.

`PARTIAL / PI TOOL PROCESS RESOURCE CLEANUP V8 SOURCE VERIFIED
(2026-08-14)`: the current Pi 0.82.1 governed Tool invocation is verified under
its Attempt execution context through a real child process and private UDS.
Cancellation while an `ask` request is active returns a content-free denial,
completes Pi abort, reaps the process group, closes listener/connection, and
removes the extension root, file, socket, and optional short-path socket
directory. No generic Effect registrar or ambient authority was introduced.
Other Runtime adapters, MCP component processes, and crash/restart cleanup stay
open. Evidence:
`../P2D-COMP2-D-pi-tool-process-resource-cleanup-v8.md`.

`PARTIAL / HARNESS PROCESS AND PRIVATE PROMPT CLEANUP V9 SOURCE VERIFIED
(2026-08-14)`: Claude Code and Codex now join owner-only system-prompt cleanup
into every process result, so cleanup failure cannot be reported as Harness
success. Real local child tests prove Attempt-context cancellation reaps each
process group and removes the private prompt file/directory. Existing closed
Provider failure classifications remain intact. Live CLI/Provider dispatch,
Attempt gateway/Context MCP crash cleanup, and other Runtime adapters stay
open. Evidence:
`../P2D-COMP2-D-harness-process-private-prompt-cleanup-v9.md`.

`PARTIAL / ATTEMPT LOOPBACK SERVICE REVOCATION V10 SOURCE VERIFIED
(2026-08-14)`: the credential gateway and Harness Context MCP now inherit the
Attempt execution context directly. Cancellation closes each loopback listener
before a deliberately blocked trusted callback or Harness runner returns; the
Context MCP bearer token is cleared under synchronized access, while ordinary
completion retains graceful shutdown. This proves normal-process revocation,
not daemon crash/restart cleanup. The credential callback must still return
before its bounded plaintext copy can be cleared. Installed CLI/Provider,
arbitrary MCP component process, `kill -9` residue, and other Runtime adapters
remain open. Evidence:
`../P2D-COMP2-D-attempt-loopback-service-revocation-v10.md`.

`PARTIAL / PI CONTEXT EXTENSION REVOCATION V11 SOURCE VERIFIED
(2026-08-14)`: production Pi RPC Context delivery now receives the Agent
Attempt execution context at extension construction. Cancellation terminates a
blocked delivery, closes its private UDS/connection, and removes the generated
extension file, socket, optional short socket root, and private root without
waiting for the outer Adapter defer. Existing protocol, delivery
acknowledgement, and fail-closed authority checks remain intact. The locked Pi
0.82.1 test was environment-gated and skipped, so binary/live, crash/restart,
arbitrary MCP component process, and other Runtime acceptance remain open.
Evidence: `../P2D-COMP2-D-pi-context-extension-revocation-v11.md`.

`PARTIAL / LOCAL IPC BUNDLE OWNERSHIP V12 SOURCE VERIFIED (2026-08-14)`:
production no longer creates `newProductRouteHandler` or applies its decorators
before Composition activation. The built-in `loom-local-ipc` Bundle now owns a
typed revocable handler slot and factory, constructs Setup before the handler
during Start, requires both during Ready, quiesces in-flight requests, and
revokes the handler before Setup cleanup. The direct COMP2-A facade remains as
the parity oracle, but production cannot use it; absent or ambiguous handler
sources fail closed. Full daemon, focused race, Composition race, repository
Go, vet, AST ownership, and diff gates pass. Installed App startup and COMP2-E
legacy removal remain open. Evidence:
`../P2D-COMP2-D-local-ipc-bundle-ownership-v12.md`.

- Open Product scope after snapshot activation.
- Conversation, Team, Agent, Attempt, and Turn owners open/close exact child
  capability scopes at their existing lifecycle boundaries.
- Attempt scope binds snapshot digest plus Frozen Execution Binding.
- Scope close expires owned credential leases, Provider session leases, tool
  channels, temporary roots, and cancellable workers.
- Scope ownership cannot replace Journal, policy, Vault root, or terminal
  authorities.

Exit: all active temporary resources have one scope owner and reverse Effect
cleanup while existing Journal/recovery semantics remain authoritative.

### COMP2-E - Legacy path removal

`PARTIAL SOURCE VERIFIED (2026-08-23)`: the fifteen-parameter
`localProductHandlerWithComposition` entry and the obsolete
`legacy.product.dispatch` symbol are removed. All former test callers now use
the closed `productRouteServices` aggregate. A manifest-wide parity test proves
typed handler availability matches the declarative registry, and an AST gate
prevents the positional handler from returning. A typed-nil regression exposed
by the full daemon suite is fixed through explicit nil normalization. The typed
method dispatch still resides in `product_daemon.go`, so COMP2-E remains open
until route-module ownership and its remaining privacy gates close. The same
slice passes installed Build 114 cold-start/restart parity with the full
Provider/Runtime catalog and real OpenCode replies before and after restart.
Evidence: `../P2D-COMP1-COMP2-E-positional-removal-v1.md`.

`ROUTE MODULE SOURCE + INSTALLED VERIFIED (2026-08-23)`: the typed service
aggregate and full method dispatch now live in `product_route_handler.go`, with
an AST ownership gate preventing both declarations from returning to
`product_daemon.go`. Full Go/race/vet gates pass. Installed Build 115 preserves
the full Provider/Runtime directory and returns real OpenCode replies before
and after cold restart. Production still calls the COMP2-A-named activation
facade, so direct legacy composition-root reachability remains the next E
boundary rather than being declared complete.

`SOURCE + INSTALLED VERIFIED (2026-08-23)`: production composition activation
now accepts exactly one non-variadic `productCompatibilityConstruction` and no
direct `localipc.Handler`. AST gates require product startup to call only this
closed entry, while the direct-handler compatibility facade remains reachable
only from parity tests. Typed/declarative route availability, lifecycle,
shutdown, privacy, full Go, focused race, vet, reproducible build, transaction
install and cold-restart gates pass. Installed Build 117 preserves the complete
Provider/Runtime catalog and returns a real OpenCode reply before and after
restart. COMP2-E is closed; COMP2 overall remains partial while its earlier C/D
scope migration status remains partial.

- Product startup uses only Profile -> compiler -> snapshot -> activation.
- Remove obsolete positional composition and conditional availability router.
- Retain only compatibility decoders required for persisted/wire history.
- `product_daemon.go` owns Profile selection, snapshot activation, local IPC
  startup, and lifecycle close; domain construction lives in Bundles.

Exit: AST/import tests prove production cannot call the legacy composition root.

## Required Profiles

- `desktop`: bundled App lifecycle, native projections, Vault, Conversation,
  Agent Runtime, governance, work, assets, diagnostics, and local IPC.
- `headless`: same authority/runtime semantics without native UI contributors.
- `test`: deterministic injected clocks/IDs/backends and no ambient Provider,
  credential, network, user workspace, or global environment dependency.

Profiles select product capabilities, not Agent Providers or models.

## Compatibility and parity gates

For each migration slice:

1. existing Conversation, setup/provider, Vault, execution, Queue, permission,
   governance, asset, diagnostics, and IPC tests remain green;
2. legacy and Bundle paths return canonical-equivalent responses for every
   registered method and unavailable-capability case;
3. desktop/headless/test route manifests are explicit and snapshot-digested;
4. startup failure before Ready exposes no listener or half-active route;
5. startup failure after N Effects closes N..1 exactly once;
6. shutdown preserves current process cleanup and owner-only file/socket rules;
7. Vault, Journal, policy, grants, and state writers cannot be overridden;
8. no Prompt, transcript, tool result, Provider response, secret, VMK,
   credential reference body, or Authorization header enters descriptors,
   snapshots, Journal, diagnostics, or lifecycle errors;
9. active Attempt snapshot/binding cannot change after daemon recomposition;
10. race, restart, crash-window, and installed App startup tests pass before
    deleting a legacy path.

## Phase 2D ordering

The ATL3 crash-safe remote-result persistence slice is source verified and is
preserved. Before Phase 2D broadens into sequential multi-tool execution, more
Runtime adapters, Queue/Steer/Inject, or additional product composition,
execution proceeds in this order:

1. COMP1 kernel RED -> implementation -> source verification;
2. COMP2-A compatibility facade;
3. COMP2-B declarative routes;
4. COMP2-C/D bounded extraction and scoped ownership;
5. resume remaining ATL3-ATL8 breadth on the new composition boundary;
6. COMP2-E removal only after parity and restart acceptance;
7. existing installed CV6 and mixed-Team ATL9 gates remain final.

## Exit criteria

COMP2 exits when product startup no longer relies on the fifteen-parameter
composition handler or monolithic service construction, all product Profiles
compile deterministic snapshots, route and lifecycle parity is proven, scoped
resource ownership is active, legacy production reachability is prohibited,
and complete source/race/restart/privacy tests pass.

This does not complete Phase 2D. Real installed credential, conversation,
mixed-Provider Team, single-Agent failure isolation, Tool Loop, and governance
UI acceptance remain mandatory.

## Remote Tool Broker Work ownership v29 status

`SOURCE VERIFIED / COMP2-E AND LIVE ENROLLMENT OPEN`: the `loom-work` Start
hook now constructs the Loom-owned Web/MCP Broker from explicit typed ports and
owns its revocable Effect. The product builder no longer accepts an assembled
remote executor. Startup failure closes Broker resources before Evidence; Work
Dispose cancels active calls and revokes retained references before releasing
its remaining owned state.

Default composition provides no remote-tool configuration and therefore
publishes no capability. Search/MCP enrollment, UI, installed ATL9 and COMP2-E
remain open. Evidence:
`../P2D-W2C-W2D-production-remote-tool-broker-composition-v29.md`.

## Remote backend Enrollment governance ownership v30 status

`SOURCE VERIFIED / MATERIALIZATION AND COMP2-E OPEN`: the existing Governance
Bundle now owns the same Journal-backed authority used for Provider Account
Policy and remote backend Enrollment. Setup consumes only its typed revocable
port, while Local IPC exposes strict configure/revoke routes and trusted
correlation through the existing Bundle-owned handler. Startup/shutdown and
protected Journal/policy ownership remain unchanged.

Enrollment is a versioned business fact, not a Bundle descriptor or
CapabilityContext payload. It contains no secret, raw endpoint, Prompt,
Provider body or dynamic plugin code. Persisted records are not yet compiled
into Work Bundle Search/MCP client ports, so the V29 Broker remains absent by
default. Client editing, runtime materialization, per-Agent capability binding,
installed ATL9 and COMP2-E remain open. Evidence:
`../P2D-W2D-remote-tool-backend-enrollment-v30.md`.

## Remote backend Enrollment client boundary v31 status

`SOURCE VERIFIED / MATERIALIZATION AND COMP2-E OPEN`: the Swift client now
governs existing V30 records through the typed Local IPC facade. It can update
bounded policy-owned fields or revoke one exact Enrollment, but it cannot inject
an Adapter, endpoint, Bundle, capability or lifecycle resource into
Composition. Identity fields are preserved from the current projection and
validated again after authoritative refresh.

The UI therefore does not expose arbitrary backend creation. A later COMP2-D
slice must compile trusted built-in Enrollment candidates into typed Search/MCP
ports, bind their lifecycle to the Work scope and prove parity before any record
can produce a runtime capability. V31 changes no protected-core ownership and
does not advance COMP2-E. Evidence:
`../P2D-W2D-remote-tool-enrollment-client-governance-v31.md`.

## Non-goals

- changing local IPC wire schemas merely for composition convenience;
- replacing Loom's modular monolith with microservices;
- runtime-loaded arbitrary plugins, hot code replacement, or remote bundles;
- moving authority or secrets into Bundle descriptors or contexts;
- refactoring domain code unrelated to a bounded extraction;
- declaring the installed App fixed from source-only parity tests.
