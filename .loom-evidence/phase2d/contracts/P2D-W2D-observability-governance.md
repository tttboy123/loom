# P2D-W2D Contract: Observability, Failure Isolation, Governance, Fallback, Accounting and UI

Status: `ACTIVE / PARTIAL`

Parent Goal: `Phase 2D - Multi-Provider, Multi-Model Agent Team Orchestration`

This WorkItem is a horizontal Phase 2D acceptance boundary. It does not create
a separate product Goal, reopen accepted Phase 2C work, replace
`P2D-BLOCKER-1`, or reset the existing P2D-W2A/W2B/W2C Candidates.

## Product outcome

An installed Loom App must let a user understand and govern each Provider
connection, Conversation dispatch, and Agent Attempt without opening Terminal.
One Provider Account failure must affect only the exact conversation or Agent
binding that uses it. Operational evidence must remain privacy-safe and must
never become execution authority.

## Required correlation chain

One privacy-safe Incident/Correlation ID follows the operation across every
participating boundary:

`Swift App -> UDS -> loomd -> Credential Vault/lease -> Provider verification -> Conversation Profile -> Agent Attempt -> Team board`

The Keychain helper is present only in an explicit one-time migration chain and
is absent from normal conversation and Agent dispatch.

The LocalKeyFile source Candidate exposes explicit Lock now / Unlock lifecycle
commands. Lock revokes active leases and closes the VMK-owning store; Unlock
reopens only after key identity, rotation recovery, encrypted database, and
pending mutation reconciliation pass. Both operations retain the same safe
Incident chain and never fall back to Keychain.

The same ID may correlate operational diagnostics with authoritative Journal
facts, but it does not authorize execution. A failure before Keychain access or
metadata commit must still close with a safe terminal diagnostic.

## Closed diagnostic schema

Each operational event records only:

- UTC time, operation, Incident ID, safe stage, elapsed time, result, safe
  error code, and retryability;
- non-secret Provider and Provider Account identity;
- non-secret Conversation Profile, Agent, Attempt, Run, Model, and frozen
  binding identity when the stage has admitted them.

It must never record API keys, Authorization headers, secret bytes, environment
credentials, prompts, conversation content, Provider raw responses, or
unbounded stdout/stderr.

The closed stage vocabulary is:

- `input_admission`, `uds_transport`, `daemon_admission`;
- `helper_validation`, `helper_start`, `helper_authorization`,
  `helper_request`, `helper_timeout`, `helper_response`, `helper_exit`;
- `keychain_access`, `metadata_commit`, `projection_refresh`;
- `provider_dns`, `provider_tls`, `provider_connect`, `provider_http`,
  `provider_auth`, `provider_rate_limit`;
- `profile_publish`, `conversation_dispatch`, `agent_attempt_dispatch`.
- `vault_key_load`, `vault_open`, `vault_encrypt`, `vault_commit`,
  `vault_decrypt`, `vault_aad_validation`, `vault_rotation`;
- `credential_lease_issue`, `credential_lease_expire`,
  `credential_lease_revoke`, `migration_read`, `migration_commit`,
  `migration_cleanup`.
- `tool_authorization`, `tool_approval_wait`, `tool_sandbox_prepare`,
  `tool_binding_validation`, `tool_dispatch`, `tool_result_validation`,
  `tool_result_commit`, `tool_payload_commit`, `tool_result_delivery`, and
  `tool_recovery`.

Adding a stage requires a versioned contract change and strict client decoding;
unknown stages fail to a safe generic presentation without exposing raw errors.

## Installed diagnostics

Installed builds retain bounded and automatically rotated owner-only `0600`
operational diagnostics, or an equivalent safe OSLog plus exportable bounded
store. Bundled `loomd` stdout/stderr cannot be discarded as the only evidence
for startup or helper failure. Console capture must itself be bounded and is
not included verbatim in diagnostic export.

The user can open a diagnostic preview and explicitly export an owner-only
privacy-safe bundle containing App/daemon versions and hashes, process/socket
health, non-secret Provider/Profile/Agent binding state, and recent allowlisted
events. The preview names excluded data. Export excludes keys, credential
references, environment credentials, prompts, conversations, raw console, and
Provider bodies by default.

## User-visible recovery

An error surface cannot collapse a known failure to `Unavailable`. It shows:

- the specific safe stage and error category;
- an actionable recovery and whether retry is appropriate;
- the Incident ID;
- `Retry`, `View diagnostics`, and `Copy incident ID` actions where applicable.

The action is scoped to the exact Provider Account, Conversation, or Agent. A
Team never becomes globally `offline` because one bound account is revoked,
rate-limited, rejected, or timed out.

## Agent governance

Every Agent row and editor projects its Harness, Provider Account, Model,
credential revision, reasoning effort, limits, current state, failure reason,
and configured fallback. A Team-level default can seed a new Agent but cannot
overwrite an existing Agent binding.

Fallback is explicit, versioned, approvable, and auditable. It freezes the
source and target Harness/Provider Account/credential revision/Model binding,
approval identity, actor, and decision. Ordinary retry cannot change a binding,
and Loom never silently substitutes a Provider, account, or Model.

Concurrency, rate limits, configured ceilings, budgets, token/cost usage, and
error rates are attributed to the exact Provider Account and correlate to Run,
Attempt, and Agent. No metric falls back to a Provider-only global bucket.

## Authority separation

The Event Journal contains authoritative business facts only. Operational
diagnostics are bounded support evidence: they cannot create a Team, permit a
dispatch, approve fallback, mutate a binding, or replace Journal replay. The
two stores correlate only through safe IDs.

## Disclosure, retention, and encrypted local state

Every Conversation Segment and Agent Attempt freezes the digest of its admitted
Context Capsule and Execution Binding. A versioned DisclosurePolicy governs the
target Provider Account, trust domain, retention, data region, classifications,
and context scopes. The resulting disclosure receipt records only non-secret
categories, omissions, policy and Capsule digests, destination identity, time,
and Incident ID. It never records prompt text, Provider response bodies, hidden
reasoning, or secret bytes.

The current owner-only `chat-threads.json` is plaintext and cannot be described
as conversation encryption at rest. Phase 2D exit requires a per-Conversation
DEK wrapped under the domain-separated Loom Vault key hierarchy defined by
ADR-0020, with
transcript, Context Capsule, and ExternalSessionHandle encrypted on disk.
Missing key material fails closed. Local deletion supports crypto-erasure, while
Provider-side retention/deletion capability is shown and audited separately.
TLS protects transport only and does not replace least disclosure.

Journal stores authoritative non-secret facts and digests. Operational
diagnostics store safe metadata and stages. Neither store contains transcript or
Capsule bodies.

## Ordered execution plan

1. Keep `P2D-BLOCKER-1` open at the conversation live gate. Package and verify
   the stale-anchor/load-race repair, explicit profile-conflict error,
   persistent chat Incident chain, and actionable Retry/diagnostic UI through a
   real installed DeepSeek reply.
2. Deliver the first same-visible-Conversation Codex-to-DeepSeek Route Segment
   with a `summary_only` Context Capsule, disclosure receipt, and frozen digest.
3. Deliver CV1-CV4 from
   `P2D-W2D-credential-vault.md`: LocalKeyFile envelope encryption, VaultStore,
   short-lived leases, transactional mutations, and replacement of the normal
   Keychain/helper hot path. Keychain remains only explicit migration.
4. Complete installed verification for each concrete Adapter with exact
   per-Agent frozen binding consumption and safe Attempt diagnostics. Source
   includes Kimi/Moonshot, MiniMax, Claude Code + Anthropic, and Codex + OpenAI;
   installed real-account dispatch remains the acceptance boundary.
5. Complete Provider Account health preflight, configured ceilings, accounting
   emitters, Agent fallback editing, and interactive approve/reject UI.
6. Complete encrypted transcript/Capsule/native-handle storage, disclosure and
   retention governance, deterministic omissions, and scoped Role Capsules.
7. Package the installed App and run the four-pair mixed-Team matrix. A revoked,
   rate-limited, or timed-out account must affect only its bound Agent while the
   other Agents continue and retain complete audit chains.

## Four-Provider source isolation amendment

The authoritative Team path now admits up to nine Agent nodes consistently
across TeamDefinition, ExecutionPlan, coordinator admission, dispatch,
Journal replay, and projection replay. This supersedes the Phase 1 three-node
operational ceiling for Phase 2D without changing the maximum of three Attempts
per node.

The controlled source canary binds four roles independently:

- Codex Harness, OpenAI account, `gpt-5.5-codex`, credential revision 3;
- Claude Code Harness, Anthropic account, `claude-sonnet-5`, revision 7;
- Loom Native Harness, Kimi account, `kimi-k2.6`, revision 11;
- Loom Native Harness, MiniMax account, `MiniMax-M3`, revision 13.

A synthetic OpenAI `provider_rejected` terminal reason remains scoped to the
Codex Agent. The Claude, Kimi, and MiniMax Agents succeed, preserve distinct
binding digests, and continue concurrently; the main Agent alone enters its
versioned recovery path. Each first Attempt also freezes distinct closed numeric
usage and cost under its exact Provider Account. Rebuilt Projection preserves
the failed OpenAI accounting and all three peer facts independently; the
scheduled recovery Attempt inherits no accounting and later freezes only its
own result. Repeated focused, race, and complete app tests pass. This closes the
four-Provider source accounting proof, not configured account ceilings,
installed Team Board observation, revocation/rate-limit/timeout, or real
Provider gates.

## Product Mission per-Agent preflight amendment

The product Mission compiler and preflight now consume every exact saved-Team
role binding. Each non-secret Agent row exposes Harness, Provider, Provider
Account, Model, credential revision, reasoning effort, timeout, budget,
capabilities, readiness, and one bounded recovery reason. Credential reference,
endpoint fingerprint, secret body, Prompt, and Provider response are excluded
from the preflight wire and UI.

Provider Account health is resolved by the full
`ProviderID + ProviderAccountID + CredentialReference + revision` tuple. A
missing, unverified, revoked, or drifted credential marks only the matching
Agent blocked. Runtime offline, capability mismatch, and unavailable capacity
are likewise role-local. Preflight remains viewable so the user can identify
and repair the exact Agent; Mission Start fails closed while any row is blocked.
The source regression revokes the DeepSeek account in a Codex/OpenAI plus
Loom/DeepSeek Team and proves that the OpenAI main remains ready.

This closes the source-level Provider Account health-preflight and accounting
attribution slices of ordered step 5. Configured account ceilings, installed
account revocation/rate-limit/timeout behavior, packaged Team Board incident
observation, and real-account accounting remain open exit gates.

## Agent Attempt Incident Board amendment

The source Candidate binds each projected Agent Attempt to the current dispatch
generation's privacy-safe correlation ID. Rebind advances only that Attempt's
Incident; terminal replay fills an absent legacy value but does not replace an
existing dispatch Incident. This preserves the ID emitted by concrete Adapter
diagnostics without rejecting historical Journal lineages whose terminal
operation used a separate correlation.

Team Board admits only the closed IPC request-ID grammar and projects the safe
Incident beside Harness, Provider Account, Model, credential revision, terminal
reason, and accounting. Swift rejects malformed IDs, displays a bounded short
label, and provides Copy incident ID from the affected Agent row. No diagnostic
body, credential reference, endpoint fingerprint, Prompt, or Provider response
enters the Board. Go projection/API/app tests, focused race and vet, strict
Go/Swift timeline interop, focused daemon IPC, and the complete Swift suite
pass. Installed build 20 predates this source Candidate, so installed Agent
failure observation remains open.

## Fallback Board presentation amendment

The source Board contract now carries five safe governance facts per Agent:
fallback configured, recovery approval required, approval available, approval
version, and fallback consumed. It does not expose the route key, approval ID or
actor, approval digest, or source/target binding digests. Swift accepts absent
fields from older daemons as unconfigured, but fails closed on contradictory new
facts such as approval without a configured fallback or a nonzero approval
version without an available approval.

Team Pulse maps the facts to one explicit state: configured, approval required,
approved with version, or executed. This closes the read-only source
presentation slice. It does not authorize a transition, create or mutate an
approval, or replace the prepared Mission recovery decision path. Agent editor
configuration, interactive approve/reject, installed presentation, and live
fallback execution remain open. Installed build 21 predates this amendment.

## Fallback RouteSet authoring amendment

Each saved Team role may carry one optional version-1 fallback Route. The route
is part of the accepted Builder binding digest and freezes a complete secondary
Execution Profile rather than a Provider-only selector. It is constrained to
the same AgentDefinition and role kind as the primary binding, must use a
different Runtime Profile, and must declare `approval_required=true`. The
authoritative setup writer and projection reject partial, mismatched, or
silently enabled routes. Historical events without the optional field remain
valid.

The macOS Builder exposes the Route as an Agent-local menu. Candidate options
are restricted to same-Agent compatible Profiles and show Harness, Provider
Account, Model, credential revision, and the approval requirement. Saved-Team
restart reconstructs an exact frozen fallback option if catalog discovery has
drifted. The client rejects contradictory wire states and never interprets a
configured Route as approved.

This amendment authorizes configuration persistence only. It does not create a
`TeamFallbackApproval`, mutate a running execution, or allow Attempt 2 to use a
changed binding. Those operations require a separate versioned prepared
decision, exact source/target binding digests, user approve/reject, and Journal
facts before the coordinator can materialize the fallback Attempt. Until that
authority path exists, runtime binding changes continue to fail closed.

## Fallback preflight and failure-isolation amendment

Mission binding resolution must evaluate an authored fallback Route as a
separate execution binding for its owning Agent. The resolver validates the
exact Runtime, Provider Account, credential reference/revision, Model, limits,
and capabilities. A revoked credential, offline Runtime, capability mismatch,
or exhausted capacity blocks that fallback only; it does not block a healthy
primary binding or another Agent.

Preflight exposes only safe fallback facts: Harness, Provider Account, Model,
credential revision, capabilities, ready/blocked status, recovery text, and
approval required. It omits credential reference and endpoint fingerprint.
Configuration is not execution authority: without an exact versioned
`TeamFallbackApproval`, the compiler keeps every Attempt on the primary binding
and publishes no fallback RecoveryPolicy. Swift rejects inconsistent facts such
as a configured fallback without approval required or a ready fallback with a
block reason.

## Fallback approval compiler amendment

The product Mission compiler may consume a fallback approval only through an
authority source queried with the exact Team instance, Plan digest, logical
Agent node, primary binding digest, and target fallback binding digest. No
source or no matching fact preserves both bounded Attempts on the primary
binding. A valid, non-future `TeamFallbackApproval` with exact source and target
digests selects RecoveryPolicy v2 and materializes only Attempt 2 on the frozen
fallback binding. Its dispatch Runtime instance and semantic approval must
match that selected binding.

Mismatched or forged digests fail closed. An exact approval whose target has
become unavailable blocks only the owning Agent at preflight and prevents Start;
it does not mark peer Agents unavailable. The safe product projection exposes
approval availability and version only. Approval ID, actor, approval digest,
binding digests, credential reference, and endpoint fingerprint remain outside
this surface.

This compiler seam does not itself create execution authority. The current W2D
source Candidate adds a production prepared-decision source that appends
versioned approve/reject facts to the Journal, resolves them by the complete
query identity, exposes explicit Decision Sheet actions, and projects the
resulting decision to the Board. Installed action handling and live Provider
execution remain fail-closed acceptance gates.

An approved fallback may change Harness Runtime as well as Provider Account and
Model. The immutable semantic binding therefore carries the approved target
Runtime instance. Retry and legacy same-binding fallback retain the current
Runtime; a changed Runtime is valid only for a fallback recovery whose exact
target binding digest matches the approval. Recovery facts, scheduled Attempts,
capacity checks, dispatch bindings, Work Authority replay, and Board Projection
must all agree on that target Runtime.

## P2D-BLOCKER-1 installed bootstrap amendment

Incident `loom-swift-663d309b-d1f2-4ee6-9e53-9e861119174f` proves the Swift
request reached the installed bundled daemon but failed before Provider
verification. The local-App bootstrap must re-exec the bundled daemon with the
canonical expanded non-secret daemon arguments in the real process argv. A
validated managed-parent PID preserves App lifecycle and is stripped before
the public `run` parser, while the kernel argv retains the canonical state,
isolation, and Socket paths required by process-helper attestation.

This repair must preserve existing executable identity, Socket ownership, and
peer-PID checks. It must also preserve an existing closed credential failure
stage through Setup and IPC instead of replacing the wrapped error. Unit tests
cover canonical handoff, recursion prevention, managed-parent admission, and
run-argument stripping; a Darwin process contract covers the real bootstrap
and helper put/read/delete path. The installed fresh-key gate remains unchanged.

Build 16 is the installed Candidate for this amendment. Its App-started daemon
exposes canonical expanded kernel argv, the managed-parent lifecycle flag is
absent from the `run` input, staged errors survive Setup/IPC, and the installed
bootstrap process-helper put/read/delete regression passes. Build 15 fails the
same regression at `daemon_admission`. These are synthetic-path acceptance
facts were followed by a real DeepSeek configure and verify. The installed
snapshot now reports revision 3 verified and publishes
`conversation-deepseek-deepseek-chat-r3`. The credential/bootstrap portion of
the incident is therefore accepted; the same blocker remains open for the
conversation dispatch gate described below.

## P2D-BLOCKER-1 conversation dispatch amendment

The installed build has a persisted Codex thread anchor. After restart, Swift
may not yet have loaded that thread when the user selects DeepSeek. Reusing the
old thread ID with the DeepSeek Profile correctly fails before persistence,
because one compatibility thread freezes one Profile. The previous Swift catch
silently restored state and exposed no Incident ID.

The source Candidate rotates the anchor whenever a non-empty Profile changes,
discards stale asynchronous thread loads by generation, preserves the server's
profile-conflict fail-closed rule, and maps chat input/UDS/dispatch failures to
actionable inline recovery with Retry, diagnostics, and Incident ID. Safe app
and daemon diagnostics correlate `chat_message` by thread/Profile metadata and
never record message content. It is not accepted live until a newly packaged
installed App creates the new DeepSeek thread and receives a real reply.

Build 17 is the installed Candidate for this conversation amendment. Its
retained and installed App/helper bytes match, automatic bundled-daemon startup
and owner-only Socket are healthy, DeepSeek verified revision 3 survives replay,
and the complete source/package gates pass. The implementation is accepted as
an installed Candidate, but the WorkItem remains open because no paid Provider
generation was issued during automated verification. A user-confirmed new
DeepSeek thread, correlated dispatch Incident, and real reply are still the live
gate.

## Provider Account ceiling authority amendment

Configured ceilings belong to a versioned Provider Account policy, never to a
Provider-global bucket, Team default, Conversation, or mutable process-global
client. The policy identity binds `ProviderID + ProviderAccountID`; its revision
and digest freeze maximum concurrent Attempts, maximum dispatch starts in one
bounded time window, and maximum assigned budget units. Cost remains an
observed, currency-qualified accounting fact until an explicit currency policy
is configured; Loom does not compare unlike currencies or invent a price.

One append-only policy stream per account uses revision CAS. A distinct account
capacity stream records non-secret reservations and releases by exact Run,
Attempt, Agent, frozen binding digest, policy revision, and correlation ID.
Dispatch reads the current policy head and account-capacity head alongside the
Runtime-capacity head, validates the complete binding, and atomically writes the
Run claim plus both reservations. Exceeding concurrency, dispatch-rate, or
assigned-budget capacity fails closed before Adapter or credential lease
access. Terminal, cancellation, and claim replacement release exactly the
matching reservation without erasing the historical policy or accounting.

`CURRENT / SOURCE CANDIDATE`: policy authority, account-capacity authority, and
their strict Projection are source-complete. The immutable policy has one
canonical constructor and digest over Provider, exact Provider Account,
revision, concurrency, bounded dispatch window/start count, assigned budget,
and UTC configuration time. The capacity stream records exact non-secret Run,
claim generation, Agent, Runtime, policy revision/digest, execution-binding
digest, and assigned budget reservations/releases. Claim atomically appends the
Run fact plus Runtime and account reservations, while replacement and terminal
transitions atomically release only the exact matching generation. Strict
replay verifies closed payloads, stream/account identity, sequence, causation,
deterministic identity, policy history, binding/account equality, limits, and
matching release. A concurrent two-Runtime same-account gate proves one winner
and no partial mutation for the loser. The admitted Run and Board Attempt freeze
the policy revision/digest and assigned budget; the Provider Account Board row
projects current policy and active capacity without credential reference or
internal reservation identity. Claim CAS includes the exact policy head even
when it is zero, policy revision time is strictly increasing, and selective
peer-Run replay still validates deterministic capacity event/idempotency
identity; clock rollback and forged orphan facts fail closed. Focused
authority/Projection/API tests, race repetitions, complete affected Go suites,
Go vet, strict Swift model tests, the isolated Mission UI suite, and a 63-test
ordered Swift subset pass. The all-tests Swift runner has a separate repeatable
XCTest `@MainActor` ordering hang before the first Mission UI assertion, so no
new full-suite pass is claimed. Installed presentation, real Provider
accounting, and mixed-Team live acceptance remain open.

`CURRENT / PRODUCTION CONFIGURATION SOURCE CANDIDATE`: policy configuration now
uses the existing authority rather than a Swift or Journal shortcut. The path
is `work.Authority -> LocalProductSetupService -> LocalProductSetupAPI ->
private loomd UDS -> strict LocalIPCClient -> LocalProductStore -> Runtime &
Providers UI`. The closed command contains exact Provider/Provider Account,
expected revision, bounded concurrency/dispatch/budget limits, and operation
ID only. It excludes secret, credential reference, policy digest, Prompt, and
Provider body. loomd injects the trusted UDS request ID as correlation and
rejects unknown fields, including `secret`.

The Swift client freezes that same ID before input validation and records safe
input/UDS/success terminal metadata in the owner-only app diagnostic store.
The daemon operational wrapper records the matching policy command terminal
with Provider/account identity, stage, elapsed time, result, error code, and
retryability only. It excludes operation ID, configured limit values,
credential fields, and raw request/result payloads.

Service success requires Projection rebuild plus exact revision/digest match.
The setup account directory emits only policy availability, revision, and
limits. Swift validates the closed result and RFC3339Nano time, then Store
refreshes setup and accepts only the same exact account/revision/limits.
Conflict, stale state, or malformed response remains account-local and does not
turn global setup or the Team offline. Runtime & Providers summarizes the
current ceiling and opens a dedicated Limits sheet with loading, validation,
keyboard, accessibility, and inline recovery states. It never displays a key,
credential reference, internal digest, or Provider response.

Focused RED/GREEN tests cover safe request keys, strict result decoding,
fractional timestamps, exact authoritative refresh, end-to-end safe Incident
diagnostics, conflict isolation, and a large-type native UI render. Complete
affected Go tests, focused race, vet, daemon wire, and strict Swift/Go probes
pass. The latest unfiltered Swift suite passes 174 XCTest cases with one
intentional skip and seven Swift Testing contracts; five consecutive no-rebuild
full-suite runs also pass. The controlled four-Provider source matrix passes
with independent
Codex/OpenAI, Claude Code/Anthropic, Loom Native/Kimi, and Loom Native/MiniMax
bindings, account-local accounting, and peer-safe rejection/revocation.

The current source is packaged as an uninstalled v0.5.2 build 25 Candidate. It
passes deep signing, arm64, owner-only permission, no-symlink, and ZIP extraction
with byte-for-byte App/daemon comparison. The package is dirty-worktree evidence,
not release provenance, and it was neither launched nor installed. Installed
build 22, DeepSeek r6, Vault CV6, real accounting, and mixed-Team live gates are
unchanged.

The subsequent source Candidate closes the ordinary-conversation version of the
same account-identity rule. Setup publishes one immutable Profile per exact
verified Provider Account while preserving primary Profile IDs for migration.
Conversation dispatch resolves that Profile against Provider Account projection
and acquires an exact Provider/Account/reference/revision Vault lease. Ambiguous,
revoked, cross-Provider, or revision-drifted records fail closed before Provider
dispatch. Swift validates the same account scope and shows the exact account in
the route picker. This does not close installed DeepSeek r6 or Vault CV6; those
remain live gates.

Every dispatched Attempt freezes the policy revision and digest it was admitted
under. Two Profiles using the same Provider Account cannot present conflicting
policy facts. Retry on the same account revalidates the current policy; approved
fallback to another account validates and freezes that target account's policy
without inheriting source capacity or budget. Policy drift never silently
changes an already dispatched Attempt, but it can block a future retry or
fallback with an account-local actionable reason.

The Board projects safe configured ceilings, active reservations, bounded
dispatch-window usage, assigned budget, observed token/cost totals, and policy
revision under the exact Provider Account. It omits credential reference,
binding digest, internal reservation IDs, secret bytes, Prompt, and Provider
body. Regression gates cover concurrent Teams sharing one account, two accounts
of one Provider, policy CAS/drift, crash replay, terminal release, approved
cross-account fallback, overflow, and a peer account continuing when one account
is exhausted.

## Exit gates

P2D-W2D exits only when all of the following are true:

- any Provider import, verification, conversation, or Agent Attempt failure is
  attributable to an explicit stage in one attempt;
- the user can see the safe reason and recovery without Terminal;
- support can trace the path by Incident ID and exported diagnostics without
  access to keys or prompts;
- a mixed Team isolates revocation, rate limiting, rejection, and timeout to
  the affected Agent;
- fallback changes are explicit and audited, and account-level accounting is
  complete for every shipped Adapter.

`P2D-BLOCKER-1` closes only after the installed chain shows the already accepted
DeepSeek configure/verify/Profile state followed by a new Profile-bound thread,
one correlated `conversation_dispatch` Incident, and a real DeepSeek reply.
Source tests and synthetic Keychain evidence cannot replace that final gate.

## Persistent fallback decision authority amendment

Fallback approval decisions use one immutable Scope containing Team instance,
Plan digest, logical Agent node, source binding digest, and target binding
digest. One Journal stream per Scope uses revision CAS. An approval freezes a
`TeamFallbackApproval` at that revision; a rejection advances the stream without
an approval and invalidates earlier execution authority while preserving audit.
Projection reconstructs all digest-bearing objects and returns only the latest
exact Scope. Plan, Agent, source, or target drift cannot reuse an approval.

The prepared product decision is a distinct `fallback` kind with Approve
fallback, Reject fallback, and Not now. Not now writes nothing. The writer,
projection, production compiler source, backend, strict Swift model, and native
sheet are source Candidates.

Mission compilation also emits one non-secret Candidate per configured Agent
fallback. Authoritative preflight registers those Candidates without a Journal
write. The projection-backed producer resolves the exact current Scope
revision, creates deterministic CAS-bound Approve/Reject commands, and
atomically replaces only stale dynamic decisions for that Team. Plan or binding
drift therefore removes old actions, while peer-Team decisions remain isolated.
The production daemon injects this producer into Mission execution and the
router derives the closed fallback controls for dynamically registered sheets.
The macOS Store refreshes the authoritative Snapshot after preflight so the
dynamic Decision Sheet is immediately reachable; if the view changed, the
existing preflight invalidation still fails closed. Only explicit submit writes
authority; `Not now` and ordinary preflight remain zero-write. A source
integration gate now proves approved decision replay through recompilation and
controlled execution: a failed Codex/OpenAI primary Attempt recovers onto a
distinct Loom Native/DeepSeek Runtime with the exact approved Account and
credential revision, while a peer MiniMax Agent completes without fallback
state. Work Authority and Team Board replay preserve both bindings. Installed
actions and live Provider execution remain required.

## Exact conversation account routing Candidate

The subsequent W2A/W2D source Candidate closes the ordinary-conversation form
of the account-identity rule. Setup publishes one immutable Profile per exact
verified Provider Account while preserving primary Profile IDs for migration.
Conversation dispatch resolves that Profile against Provider Account projection
and acquires an exact Provider/Account/reference/revision Vault lease. Ambiguous,
revoked, cross-Provider, or revision-drifted records fail closed before Provider
dispatch. Swift validates the same account scope and shows the exact account in
the route picker.

This source is packaged as uninstalled v0.5.2 build 26. Deep signature, arm64,
owner-only permissions, no bundle symlinks, and ZIP extraction with byte-for-byte
App/daemon/plist comparison pass. The candidate was not launched or installed;
installed build 22 and its managed daemon remain unchanged for DeepSeek r6
attribution. Installed DeepSeek r6, Vault CV6, real accounting, and mixed-Team
live acceptance remain open.

The next source increment adds Anthropic Messages to the same account-scoped
Conversation route and diagnostic chain. It uses exact Provider Account Vault
leases and the existing safe auth/rate-limit/timeout/HTTP classification; no
secret, request body, Provider response, or conversation content enters
operational diagnostics. Provider, setup, daemon, focused race/vet, strict Swift
wire, and same-thread Segment transition tests pass. Real installed Anthropic
dispatch and account-local failure observation remain live gates.

## Installed response-stage observation and build 28 diagnostic refinement

The latest read-only installed state is an external-provenance v0.5.2 build 26,
not the previously observed build 22 and not this task's build 26 Candidate.
Its unlocked Vault and verified DeepSeek revision 6 Profile reach real Provider
dispatch. Incident `loom-chat-6135d434-ce27-4f69-bec5-1dabba5e0c8d` terminates
at `provider_http` with `invalid_response`; the persisted Attempt is local to
the DeepSeek Segment and does not mark the Team or Vault offline.

This is sufficient stage attribution to exclude input admission, UDS, daemon
admission, Vault key load/decrypt, lease issue, Profile publication, and route
selection, but it does not reveal which safe response invariant failed. Build
28 therefore records only one bounded reason code from the closed set
`response_json`, `response_model`, `response_choices`, `response_role`, and
`response_content`. It does not persist response bytes, content, headers,
Prompt, credential data, ciphertext, or nonce. A bounded Provider-resolved model
name remains non-authoritative metadata; the frozen Loom binding is unchanged.

Build 27 is retained as uninstalled Anthropic binary evidence and build 28 is
the current uninstalled response-diagnostic Candidate. The complete repository
Go suite passes on build 28 source. Neither Candidate closes the live gate. The
next user-triggered installed attempt must produce a real reply or one exact
safe `response_*` reason before any resolution claim.

## Exact installed build 28 follow-up

Another process installed and restarted the exact retained build 28 Candidate;
this task only observed the result. Candidate and installed App/daemon/plist
hashes match, deep signature passes, and the managed daemon retains canonical
argv. The Vault automatically returns `unlocked`, the verified DeepSeek account
remains at revision 6, and the r6 Profile is available without a new Keychain
helper path. No chat operation was recorded after installation. W2D therefore
accepts startup and state recovery for this bundle but keeps the response-stage
live gate open.

## Build 29 per-Agent freeze and isolation audit

The latest source audit repaired the Codex/OpenAI Agent Profile so its explicit
`high` reasoning value requires the observed `reasoning_effort` capability in
addition to `workspace_edit`. Product catalog tests now freeze every published
role option against the exact Runtime and model, covering Codex/OpenAI, Claude
Code/Anthropic, and Loom Native DeepSeek/Kimi/MiniMax routes.

Existing controlled execution evidence was rerun rather than inferred. In one
four-Provider Team, a Codex/OpenAI Attempt terminates with
`provider_rejected` while the Anthropic, Kimi, and MiniMax Agent Attempts
succeed and preserve independent Account, credential revision, binding digest,
token, and cost facts. Projection evidence revokes one DeepSeek credential and
marks only that role blocked; account-capacity evidence keeps same-Provider
accounts independent for concurrency, dispatch rate, and budget. This closes
the audited source defect but does not replace the installed live matrix.

Uninstalled build 29 contains this repair and passes the complete repository Go
suite, focused race, affected vet, the full 175-XCTest/eight-Swift-Testing
suite, and package integrity checks. Installed build 28 remains unchanged.
DeepSeek reply, Anthropic reply, Vault CV6, real Provider accounting, and the
installed four-Provider mixed-Team matrix remain open.

## Team Board strict wire and incomplete-accounting presentation

Build 30 closes a client governance gap discovered after the per-Agent freeze
audit. Swift now fail-closes contradictory Agent binding availability, invalid
Harness/Provider/Account/Model identity, cross-Provider account scope,
credential revision drift, unsafe terminal reasons, and Attempt usage/cost
facts that violate Work Authority invariants. Provider Account rows also bind
nonnegative attempt/rate/budget/accounting counts, exact error-rate basis
points, usage totals, unique currencies, and policy ceilings.

Daemon `aggregation_overflow` remains an explicit non-authoritative partial
aggregate rather than a failure of the underlying Agent Attempts. The Team
inspector displays `Accounting incomplete` for that account and preserves all
other Agent rows and statuses. RED/GREEN wire and presentation regressions pass,
as does the complete 176-XCTest/eight-Swift-Testing suite. Build 30 is an
uninstalled Candidate; this source evidence does not close real Provider
accounting or the installed mixed-Team gate.

## Build 31 frozen-limit observability amendment

Post-preflight governance now preserves the exact timeout, optional binding
budget, and capabilities from each Agent Attempt's frozen execution binding.
These values are projected and validated per Agent rather than inferred from a
Team default or Provider-global client. The UI places them on the corresponding
Agent row alongside Harness, Provider Account, Model, credential revision,
status, terminal reason, accounting, and Incident ID.

The wire remains privacy-safe: no credential reference, endpoint fingerprint,
secret, Prompt, Provider body, or hidden reasoning is exposed. Build 31 is an
uninstalled Candidate with passing complete Go, focused race/vet, full Swift,
and package-integrity gates. It does not complete installed Provider replies,
Vault CV6, real accounting, or the four-Provider mixed-Team live matrix.

## Build 32 Agent Vault and Provider stage-preservation amendment

The normal Agent dispatch path now carries the exact closed Vault/lease stage
from Envelope and VaultStore through `CredentialLeaseManager`, per-Agent
credential access, and the Adapter diagnostic recorder. AAD identity mismatch
uses `vault_aad_validation`; wrapped-DEK, ciphertext, and version failures use
`vault_decrypt`; revoked and expired leases remain distinct from generic lease
issue. Lower-level errors are replaced with non-secret public sentinels before
they leave the credential boundary.

Provider callback errors are not credential failures. Authentication, rate
limit, rejection, availability, and Harness failures now remain typed through
the credential callback and are classified by the exact Agent Adapter. This
repair is account- and Attempt-local and changes no Team-global availability,
Vault crypto format, secret lifetime, Journal payload, or binding identity.

RED/GREEN tests cover both native Provider and Claude Code Harness diagnostics,
including non-disclosure of private failure text. Complete Go, affected race
and vet, full Swift, and build 32 package-integrity gates pass. Installed
observation and the broader Phase 2D live matrix remain open.

## Build 33 Agent-level diagnostic projection amendment

Operational diagnostics remain observational and separate from Event Journal
authority. The Team timeline may request only a bounded batch of current Agent
Attempt summaries. A returned failure must match Incident ID, Provider,
Provider Account, and Model exactly and must use the closed stage and safe-code
grammars before it can be projected.

The Board publishes `failure_diagnostic_available`, `failure_stage`,
`failure_code`, and `failure_retryable` alongside the existing Incident ID.
Diagnostic file absence, rotation, corruption, or read failure omits these
fields without changing Attempt status, Team status, cursor identity, or view
version. A latest successful record removes an older failure for the same exact
identity. Cross-Agent and cross-account records cannot leak into a peer row.

Swift applies the same closed stage and code boundary and requires an exact
execution binding plus Incident ID whenever diagnostic availability is true.
The Team inspector renders the safe stage and retryability on the affected
Agent row. No credential reference, endpoint fingerprint, secret, Prompt,
Provider body, hidden reasoning, ciphertext, nonce, or wrapped DEK enters the
wire.

Complete Go, focused race/vet, full Swift, reproducible native build/smoke, and
build 33 artifact-integrity gates pass. Build 33 remains uninstalled; installed
stage observation and the broader Phase 2D live matrix remain open.

## Independent Agent route governance editor amendment

The Builder UI now presents Agent role identity separately from Harness,
Provider Account, Model, reasoning, timeout, budget, and fallback. This makes
the governance consequence of each edit visible and prevents a Provider Account
selection from silently replacing unrelated Harness or limit fields. Provider
Account menu rows include the non-secret credential revision and retain distinct
entries when one account has multiple selectable revisions.

The daemon composes a new immutable Profile for each compatibility-aware change
and validates its exact Runtime binding before changing the Draft. Provider
Account selection is restricted to the current Harness/runtime and visibly
selects the source account's compatible Model while preserving reasoning,
timeout, budget, and capabilities. Harness selection requires the current
Provider Account, credential revision, and Model to be published on the target
Harness, then changes only Adapter/runtime. Cross-Agent sources or incompatible
combinations fail closed. Existing
fallback is cleared when it is no longer compatible rather than silently
retargeted.

This improves source governance and UI clarity but does not satisfy W2D's live
exit gates. Installed mixed-Team dispatch, account-local revocation/rate-limit/
timeout observation, real accounting, explicit live fallback, Vault CV6, and
real Provider replies remain required.

## Installed build 39 DeepSeek blocker correction

Read-only installed evidence closes P2D-BLOCKER-1 for the DeepSeek credential
import, verification, Profile publication, conversation dispatch, and real
reply path only. The currently running v0.5.2 build 39 App and bundled daemon
produced five successful DeepSeek r6 Attempts after their recorded startup.
Each Attempt has its own binding and Capsule digests, and the same Incident ID
is reported as `conversation_dispatch / succeeded` by both operational layers.
Earlier Provider timeouts remain Attempt-local and are followed by successful
retries, providing a real recovery observation.

This does not close W2D. Installed Anthropic and four-Provider mixed-Team
execution, account-local revocation/rate-limit/timeout isolation, account-level
cost and token accounting, approved fallback observation, encrypted content
storage, rotation continuity, and complete user-facing governance remain open.
No helper process was active at inspection time, but historical calls were not
instrumented specifically for process creation; the no-Keychain hot-path gate
therefore remains open until that evidence is captured.

## Conversation migration failure-isolation source boundary

The encrypted Conversation migration now emits safe `migration_read`,
`migration_commit`, and `migration_cleanup` records under one startup Incident
ID. Failed validation or commit preserves the legacy source, records a bounded
terminal error, and keeps Vault, Provider setup, diagnostics, and Team
governance online. Chat alone fails closed.

The chat read model may project only code `state_unavailable`, one of those
three stages, a valid Incident ID, and a retryability bit. Swift independently
rejects any larger or contradictory shape and renders the existing diagnostics
and incident controls with a stage-specific recovery action. The operational
record excludes thread identity and content as well as every existing secret
class. Because the unavailable in-memory chat API cannot rerun startup
migration, the UI does not map retryability to resending a message.

Focused Go migration/daemon tests and all selected Swift model/store tests pass.
This source was written after uninstalled build 41; installed observation and
all broader W2D live gates remain open.

## Build 42 Candidate boundary

The migration correlation, daemon survival, bounded chat failure projection,
and Swift recovery presentation are packaged in unlaunched, uninstalled build
42. Complete source and static transport gates pass. Installed observation,
including first-start migration failure/success and post-restart recovery,
remains open and W2D is not complete.

## Vault-only credential runtime observability gate

Ordinary product composition must select the Loom Vault even when a caller
omits a credential override. It must not infer Keychain from the absence of an
override. A setup surface without a Vault mutator or explicitly injected
legacy test store fails closed before accepting credential input.

Non-test daemon source is prohibited from calling `NewProductKeychainStore`
unless the file is an explicit `credential_migration` component. This boundary
is enforced both by an AST test and by the release build script so a skipped
test cannot silently package a Keychain hot path.

Every daemon-session operational record may carry exactly one safe credential
runtime value:

- `vault` for the normal runtime;
- `explicit_legacy` only for deliberately injected legacy/test composition.

Unknown values, including `keychain`, and runtime drift fail closed. The marker
is diagnostic metadata only and cannot authorize execution. It contains no
secret, credential reference, endpoint, path, Prompt, transcript, Provider
body, ciphertext, nonce, or wrapped key. Swift preserves the same closed enum
in diagnostic preview/export.

Source tests prove the generic daemon selects an unlocked Vault and records
`credential_runtime=vault` on a real private-UDS chat operation. They do not
prove that an installed historical call spawned no helper. W2D therefore still
requires approved process-level observation across import, repeated
Conversation and Agent calls, restart, and optional one-time migration.

Build 43 packages this gate as an uninstalled Candidate. `Loom.app` was not
launched, but the exact bundled daemon was run directly against a canonical
owner-only temporary state and UDS. Vault lock/unlock and a content-free invalid
chat admission recorded `credential_runtime=vault`; restart reopened
`local_key_file` as unlocked while preserving key/database identity. Two 2 ms
process-sampling windows observed no `--credential-helper` process. This is
packaged-runtime evidence, not an installed real-Provider or Agent Attempt
gate, and polling cannot exclude a process shorter than the sampling interval.
W2D remains `ACTIVE / PARTIAL`.

## Build 44 direct helper-attempt instrumentation

The daemon now records a process-local monotonic count of every Darwin
`--credential-helper` start attempt. The increment occurs immediately before
the sole helper `command.Start()` boundary, including failed starts. Every new
operational record explicitly snapshots that value as
`credential_helper_spawn_attempts`; Swift preserves it in privacy-safe
diagnostic preview/export. The field contains no credential, content, endpoint,
path, ciphertext, nonce, or wrapped key and cannot authorize execution.

The exact build-44 bundled daemon was run twice against one isolated owner-only
Vault. Five lock, unlock, and content-free invalid-chat operational records all
reported `credential_runtime=vault` and
`credential_helper_spawn_attempts=0`; restart preserved Vault key/database
identity. This replaces the build-43 2 ms polling inference with direct process
instrumentation for the isolated ordinary Vault path.

W2D remains `ACTIVE / PARTIAL`. The same counter must still be observed through
the approved installed import, repeated real Conversation calls, Agent
Attempts, restart, mixed-Team execution, and optional one-time migration matrix.

## Agent-row failure governance action amendment

The Team Pulse inspector must keep recovery and diagnostics scoped to the exact
Agent row whose current Attempt carries a privacy-safe failure diagnostic.
That row exposes three bounded actions when applicable: review retry in the
existing Attention governance surface, preview the allowlisted diagnostic
bundle, and copy the Incident ID. Healthy peer Agents and Provider Account
accounting rows expose none of those actions.

Retry review is presentation and navigation only. It must not dispatch an
Attempt, approve fallback, replace a frozen binding, or create execution
authority. Diagnostic preview remains operational support data, separate from
the Event Journal, and cannot authorize execution. Missing or stale action
metadata fails closed.

The source regression combines one retryable Anthropic Agent failure, one
successful OpenAI/Codex peer, and one account summary. It proves that only the
failed row receives actions. The complete Swift and native rendering matrices
pass. Exact evidence is
`.loom-evidence/phase2d/P2D-W2D-agent-failure-governance-actions.md`.

Build 45 predates this source. The amendment is packaged in unlaunched,
uninstalled build 46 with passing release, arm64, deep-signature,
owner-only/no-symlink, action-string, wire-contract, and ZIP byte-equivalence
gates. Installed mixed-Team isolation and the broader W2D live matrix remain
open, so Phase 2D stays `ACTIVE / PARTIAL`.

## Vault account-local lease concurrency amendment

Credential Vault lifecycle state is no longer held through a remote Provider
callback. Independent account-bound calls obtain exact short-lived leases and
may overlap; revoke, lock, close, and rotation still cancel and zeroize through
the lease manager. This preserves the W2D requirement that one slow Provider
Account cannot become a Team-global availability condition.

The source RED/GREEN and race evidence is
`../P2D-W2D-vault-account-local-lease-concurrency.md`. Real installed
account-local dispatch, capacity, error-rate, token, and cost attribution remain
Phase 2D acceptance work.

The increment is frozen in unlaunched, uninstalled build 47 with passing source
and static package gates. Installed build 39 and the broader W2D live matrix are
unchanged.

## Authoritative initial Agent block amendment

### Authority

- A direct Agent preflight failure is an immutable
  `TeamNodeInitiallyBlocked` fact with node, safe code, stage, reason,
  retryability, Incident ID, and provenance.
- Only declared dependency edges may receive a propagated
  `dependency_blocked` fact. Its stage and reason are canonical and its
  retryability equals the source block.
- An initially blocked node receives no Attempt, grant, capacity reservation,
  or Adapter invocation. An all-blocked Team receives a terminal Team fact
  with zero Attempts.
- The initial public Route summary is frozen in `TeamExecutionPlanned` and the
  first Attempt must match it. Explicit fallback creates a separately approved
  Attempt binding.

### Privacy and projection

- Public Route fields are Harness, Provider, non-secret Provider Account,
  Model, reasoning, timeout, budget, capabilities, and credential revision.
- Credential reference, endpoint fingerprint, binding digest, secret bytes,
  Prompt, Capsule/transcript content, and Provider response are forbidden from
  the public Route, Event Journal diagnostic payload, and Team Board.
- Team Pulse keeps the blocked Agent visible before Attempt 1 and projects the
  exact safe stage/code/retryability, Incident ID, and bounded recovery
  actions. A peer Agent failure must never become a generic Team `offline` or
  `Unavailable` state.

### Dispatch and isolation

- Runtime capacity lookup is performed only for nodes whose current authority
  permits dispatch. Missing capacity for a terminal or initially blocked node
  cannot reject a healthy independent sibling.
- A direct Provider Account, credential revision, Runtime compatibility, or
  capacity failure blocks only the affected Agent and declared dependents.
- App preflight readiness permits Start when at least one Agent is `ready`.
  An all-blocked preflight remains fail closed and cannot submit Start.
- Account-level live revocation, rate-limit, timeout, token/cost accounting,
  approved fallback, and CV6 remain installed acceptance gates; this amendment
  closes the source authority and App projection boundary only.

Exact source and verification evidence is
`../P2D-W2D-authoritative-initial-agent-failure-isolation.md`. Normal runtime
uses Loom Credential Vault. Keychain is permitted only as an explicit optional
one-time migration source and is not part of Conversation or Agent dispatch.

## Account-local Provider timeout amendment

Provider network failure classification is part of each frozen Agent Attempt,
not a Team-global availability flag. A request transport timeout before an
upstream response exists projects `timeout / provider_connect / retryable`.
A timeout after response admission while reading the bounded response projects
`timeout / provider_http / retryable`. The outer Attempt context deadline
retains the runtime-level timeout path so Provider transport and governance
deadline are not conflated.

Native adapters and bounded Harness gateways publish this result through the
normal `MessageResult` and privacy-safe operational diagnostic path. Board
projection requires an exact Incident, Provider, Provider Account, and Model
match. A same-Provider different-account record, unknown stage, or malformed
Incident fails closed and cannot annotate a healthy peer Agent.

No diagnostic may contain secret bytes, credential reference, endpoint
fingerprint, Prompt, Context Capsule content, Provider body, Authorization
header, or hidden reasoning. Exact source and verification evidence is
`../P2D-W2D-account-local-provider-timeout-classification.md`.

This closes the source timeout-classification boundary only. Installed auth,
rate-limit, timeout and revoke isolation, account-level cost observation,
approved fallback, and Credential Vault CV6 remain Phase 2D gates.

## Provider Account governance visibility amendment

Team Pulse must distinguish Agent status rows from Provider Account governance
rows. For each exact non-secret account represented by a frozen Attempt, the
account row presents failed/total Attempts, error rate, rate-limited Attempts,
accounting coverage, token usage, account policy revision, active concurrency,
dispatch-window ceiling, active assigned budget versus ceiling, costs grouped
by currency, and aggregation completeness.

Error rate is formatted from basis points and cost from integer microunits
without floating-point or locale-dependent arithmetic. Agent and account rows
use distinct system icons and accessibility labels while remaining in one Team
Pulse list. Statistics are observational and cannot authorize retry, fallback,
credential mutation, or execution.

Account rows consume the authoritative Board aggregation by Provider plus
Provider Account. Provider-global merging is prohibited. Credential reference,
endpoint fingerprint, secret bytes, Prompt, Provider body, and hidden reasoning
remain excluded. Exact source evidence is
`../P2D-W2D-provider-account-governance-visibility.md`.

This closes source presentation of account-level accounting, not installed
real-cost observation, revocation/auth/rate-limit/timeout isolation, approved
fallback, mixed-Team execution, or Credential Vault CV6.

## Provider Account cost provenance amendment

Every newly committed Run cost fact must identify whether the value was
reported by the Provider response or by the bounded Harness protocol. Cost
identity is `currency + source`; Board aggregation must not combine two values
that share a currency but have different sources. Attempt and account rows
present the source in plain language so reported values cannot be confused with
future estimates.

The closed current source set is `provider_reported` and
`harness_reported`. Historical Journal events written before this amendment may
omit the field and replay as `legacy_unspecified`; an explicitly empty,
explicitly legacy, or unknown source is invalid for a new authority write.
Projection accepts the legacy form only when the JSON field is actually absent;
an explicit JSON `null` is invalid.

`rate_card_estimate` is reserved product vocabulary, not a currently accepted
accounting fact. It must remain fail closed until a versioned Provider Account
and Model Rate Card freezes its identity, revision, digest, currency, effective
rates, and calculation basis with the Attempt. Loom must not hardcode a current
website price in an Adapter or label an estimate as observed.

Claude Code and Pi RPC costs are `harness_reported`. Native
OpenAI-compatible DeepSeek, Kimi, and MiniMax calls currently expose token usage
without a cost and remain honestly unobserved. Exact source evidence is
`../P2D-W2D-provider-account-cost-provenance.md`.

This closes source provenance and presentation, not installed real-account
cost observation, Rate Card authority, revocation/auth/rate-limit/timeout
isolation, approved fallback, mixed-Team execution, or Credential Vault CV6.

## Provider Account and Model Rate Card authority amendment

`rate_card_estimate` is now a closed accounting source backed by append-only
authority rather than reserved vocabulary. The identity is the exact Provider,
Provider Account, and Model. Every revision freezes its digest, currency,
token basis, integer microunit input/output/cache rates, and deterministic
rounding rule. Loom ships no official prices and does not infer one from a
Provider ID.

Run Claim contract v2 freezes the complete Rate Card beside the exact execution
binding. A terminal estimate may use only observed token counts and that frozen
card. Provider- or Harness-reported cost remains authoritative when present;
an estimate cannot replace it. Missing, stale, malformed, or cross-account
cards fail closed. Board and Swift projections carry only non-secret Rate Card
identity and source.

## Credential Vault implementation revalidation

ADR-0020 and the CV1-CV6 contract remain the single Credential Vault decision
inside P2D-W2D. Source inspection and complete regression revalidate that the
ordinary daemon enables LocalKeyFile Vault composition, and Conversation plus
Agent hot paths obtain exact Provider Account, credential reference, and
revision leases. Normal non-test daemon code does not construct
`ProductKeychainStore`; the legacy lease adapter is reachable only through an
explicitly injected compatibility/test store or a future user-triggered
migration component. No plaintext fallback exists.

CV1-CV5 remain source Candidates with encrypted credential, transcript,
Context Capsule, and Provider-native handle storage, rotation, recovery,
encrypted export, diagnostics, and UI governance. CV6 remains partial until
approved installed counter-backed no-helper, post-rotation continuity,
single-account revoke/corruption isolation, and four-Provider mixed-Team gates
pass. Installed Loom remains v0.5.2 build 39.

## Four-Provider Vault revoke isolation amendment

W2D source acceptance includes a controlled Team with Codex/OpenAI, Claude
Code/Anthropic, Loom Native/Kimi, and Loom Native/MiniMax. Every Agent Attempt
must acquire a lease using its exact frozen Provider Account, credential
reference, and credential revision. A revoke match is the complete identity;
Provider ID alone is never sufficient.

Revoking one exact identity blocks only that Agent and its declared dependents.
Independent healthy Agents remain schedulable and retain their own terminal
facts. A Team terminal status may summarize an incomplete required Team, but it
must not erase per-Agent success or turn the Team into a generic offline state.

The Board may enrich the affected Attempt with
`credential_lease_revoke / credential_unavailable / retryable` only after an
exact Incident, Provider, Provider Account, and Model match. The event is
privacy-safe operational metadata, separate from Journal authority, and cannot
authorize retry, fallback, replacement, or dispatch. Exact source evidence is
`../P2D-W2D-four-provider-vault-revoke-isolation.md`; installed CV6 acceptance
remains open.

The same matrix must also fail locally when one encrypted credential record is
corrupted. Source acceptance uses a real LocalKeyFile and encrypted VaultStore,
closes and reopens the Store, corrupts only one ciphertext, and requires the
affected lease acquisition to preserve `vault_decrypt`. Independent records
must still decrypt and their Agents must still execute. Plaintext secret bytes
must not occur in the Vault database or Team Journal.

The Board may display that record as
`vault_decrypt / credential_unavailable / not retryable` only for an exact
Incident, Provider, Provider Account, and Model match. Source proof advances
the corruption-isolation contract but does not satisfy installed CV6.

The classification is now enforced at the Adapter recording boundary rather
than only in a Board fixture. One shared credential-stage policy is consumed by
Loom Native and Claude/Codex Harness adapters. `vault_key_load`, `vault_open`,
`vault_decrypt`, `vault_aad_validation`, and `vault_recovery` require recovery
or unlock and are not directly retryable; lease issue, expiry, and revoke remain
retryable after authoritative state changes. A real encrypted-record corruption
test proves the stage and retryability survive VaultStore, lease management,
exact frozen binding validation, Adapter diagnostics, persistent JSONL, and
exact Board query without issuing a Provider request or persisting credential
reference, secret, Prompt, ciphertext, or private error text.

Rotation continuity must preserve every Agent's exact frozen execution binding.
The production lease barrier revokes old leases before DEK rewrap, and fresh
leases are permitted only after the new LocalKeyFile is promoted and adopted.
After a Vault restart, Provider Account, credential reference/revision, Model,
Harness, limits, capabilities, and binding digest must remain unchanged.
Source acceptance proves this four-Agent matrix; installed real-Provider
observation remains a CV6 gate.

Explicit fallback source acceptance now uses the real encrypted Vault boundary.
One primary credential record is corrupted while a separate fallback record
remains valid. The primary Attempt may fail only on its exact frozen identity;
the fallback Attempt may acquire the target identity only when a versioned
approval binds both source and target binding digests. It must carry its own
binding and Role Context Capsule and may not reuse the primary secret, account,
revision, or Provider-native state.

The mixed-Team gate composes Codex/OpenAI with an approved second OpenAI
account, Claude Code/Anthropic, Loom Native/Kimi, and Loom Native/MiniMax. The
four Agents create five exact Attempts/credential identities: the failed main
primary, its approved fallback, and three successful peers. Every identity is
resolved once by full Provider, Provider Account, Credential Reference, and
revision. The Team succeeds with peer facts intact; no silent fallback or
Team-global Provider client is permitted. Installed approved fallback and real
Provider replies remain CV6 gates.

Attempt accounting is frozen and projected with the same identity. A failed
credential acquisition publishes no fabricated usage or cost. A successful
approved fallback publishes only against its target Provider Account, while
peer accounting remains unchanged. Source tests use bounded Provider-reported
fixtures; installed real-account token/cost observation remains a live gate.

Agent-level recovery UI must be stage-specific. Durable, non-retryable Vault
failures expose an action that opens the Credential Vault recovery surface on
the affected Agent row. Retryable lease failures continue to route through
Attention, while diagnostics and Incident ID copy remain independently
available. Provider transport/auth/rate-limit failures must not receive the
Vault action, and no recovery action may be inferred from a Team-wide status.

## Conversation Vault failure-governance amendment

Ordinary Conversation dispatch consumes the same closed credential-stage retry
policy as Agent adapters and Board governance. The Conversation Profile Router
must preserve the exact staged failure, IPC must project its retryability as
`recoverable`, and the failed Conversation Attempt must freeze the same value.

`credential_lease_issue`, expiry, and revoke may remain retryable because their
authoritative state can change independently. Durable `vault_key_load`,
`vault_open`, `vault_decrypt`, `vault_aad_validation`, `vault_rotation`, and
`vault_recovery` failures must not produce a retry loop. The Swift chat banner
offers the existing Credential Vault recovery surface only for that closed,
non-recoverable allowlist while retaining separate diagnostics and Incident ID
copy actions.

Exact source evidence is
`../P2D-W2A-W2D-conversation-vault-failure-governance.md`. This amendment does
not close installed DeepSeek conversation recovery or CV6.

## Provider Account disclosure policy v2 amendment

Provider Account governance extends beyond concurrency and budget. A v2 policy
authoritatively records a closed trust domain, retention mode, and data region
and includes all three in its canonical digest. Legacy v1 limit policies remain
replayable without reinterpretation and are explicitly treated as disclosure
policy unspecified.

Setup mutation must verify the response against the rebuilt account-local
projection. Provider Account directory and Conversation Profile expose the
exact policy version, revision, digest, and disclosure values. The App may
describe them only as user-selected account policy; it must not claim that Loom
certified a Provider's ZDR, retention, residency, or enterprise guarantees.

Exact source evidence is
`../P2D-W2A-W2D-provider-account-disclosure-policy-v2.md`. Freezing this policy
identity on Conversation Segments/Attempts and installed acceptance remain
open.

## Conversation execution-binding v3 amendment

Conversation Segments and Attempts now freeze the exact Provider Account
disclosure policy version, revision, digest, trust domain, retention mode, and
data region resolved by the daemon. Swift renders this frozen non-secret state
and never replaces it with a later account projection.

Explicit route review is protected against policy changes between confirmation
and dispatch. The client sends the reviewed expected binding, while the daemon
re-resolves authority and performs an exact comparison before any message,
Attempt, lease, or Provider side effect. Missing or stale expected bindings on
a real route/policy transition fail closed as a governable Conversation route
conflict.

Conflict recovery preserves the same visible Conversation, selected target,
draft, and Incident ID and returns Retry to the review sheet. No fallback,
Provider change, account change, or policy adoption occurs silently. Exact
source evidence is `../P2D-W2A-W2D-conversation-policy-binding-v3.md`; installed
acceptance remains open.

## Attempt Payload terminal-reconciliation diagnostics amendment

Execution runtime startup now treats the Journal delivery proof as authority
for repairing the narrow crash window between `ToolResultDelivered` and the
Vault `delivered` commit. Every repaired or blocked outcome uses stage
`context_delivery_reconcile` and carries only Incident ID, Provider Account,
Model, WorkItem, Run, generation, Runtime, Agent, binding digest and Capsule
digest. A blocked row uses a retryable safe code such as
`attempt_payload_unavailable` or `attempt_payload_commit_failed`; payload body,
Prompt, credential, ciphertext and Provider response remain excluded.

One unreadable Vault row is isolated to its Attempt and does not stop another
Agent's terminal repair. Missing rows after Conversation crypto-erasure are
expected and do not create a false incident. Exact evidence is
`../P2D-W2C-W2D-attempt-payload-terminal-reconciliation-v1.md`. General Tool
Loop governance and installed acceptance remain open.

## Agent Attempt disclosure-policy governance amendment

The Team Board no longer identifies an Agent Attempt's Provider Account policy
only by revision and digest. Run authority and replay freeze the exact policy
version, trust domain, retention mode, and data region from the historical
account-local policy fact, and Team projection carries those values without
consulting the later current-policy view.

Swift accepts legacy v1 only with empty disclosure values and accepts v2 only
with the closed policy tuple. Any version downgrade, unknown trust domain,
unknown retention value, or unknown region fails closed. Agent rows show the
frozen policy while preserving the existing no-secret Board contract.

Exact evidence is
`../P2D-W2B-W2D-agent-disclosure-policy-freeze-v1.md`. This closes the source
governance ambiguity, not installed CV6, real Provider behavior, or the live
four-Provider failure-isolation matrix.

## Accepted tool execution governance amendment

`PARTIAL`: exact approval/call/consumer/operation consumption is source
verified. Full Attempt/Turn/Step binding, authenticated encrypted proposal
inspection, declared sandbox enforcement, tool-level Incident correlation, and
explicit recovery remain target work inside the sole Phase 2D Goal.

### One-shot approval

An approval decision is valid for exactly one frozen ToolCall. It binds the
Attempt, claim generation, Turn, Step, semantic ToolCall ID, canonical argument
digest, Execution Binding digest, Capsule digest, resource scope, approver,
policy version, decision, and expiry. `allowed_once` grants only that call;
reject, cancel, timeout, unavailable authority, or any substitution fails
closed. Approval cannot be inferred from chat text, prior calls, a similar
command, or an Agent recommendation.

Approval requests and outcomes are auditable without persisting command output,
Prompt, credentials, hidden reasoning, or Provider response. The approval UI
may temporarily take over the composer, but must preserve the user's draft and
queued inputs and return them after the decision.

### Sandbox enforcement completeness

Every ToolCall freezes a sandbox requirement and observed enforcement report.
The requirement separates filesystem mode (`read_only`, `workspace_write`, or
`danger_full_access`), canonical workspace root, network capability, process
capability, and resource allowlists. Enforcement reports are `full`, `partial`,
or `unavailable`, with component-level evidence and adapter identity.

`partial` or `unavailable` cannot satisfy a contract requiring `full`.
`danger_full_access` requires explicit high-risk confirmation and a frozen
policy/grant; it cannot be selected by model output, adapter fallback, or an
ambiguous platform capability. Workspace identity drift, unenforced network or
process access, and scope expansion fail closed before dispatch.

### Tool-level Incident correlation

Each ToolCall shares one end-to-end Incident lineage across App, UDS, loomd,
approval, sandbox wrapper, Harness adapter, tool process, result persistence,
delivery, and recovery. Safe stages include `tool_input_admission`,
`tool_binding_validation`, `tool_authorization`, `tool_approval_wait`,
`tool_sandbox_prepare`, `tool_dispatch`, `tool_result_validation`,
`tool_result_commit`, `tool_result_delivery`, and `tool_recovery`.

Diagnostics record only time, operation, non-secret identities and digests,
stage, elapsed time, result, retryability, and Incident ID. The UI shows the
specific failed stage, an actionable recovery choice, retryability, View
diagnostics, and Copy incident ID. It must not collapse ToolCall failure into a
Team-wide `offline` or generic `Unavailable` state.

### Recovery governance

Restart recovery reconstructs interrupted Turns, Steps, ToolCalls, approvals,
and encrypted payload delivery from authority. It never assumes a dispatched
side effect did not occur. Automatic replay is permitted only with a verified
non-execution proof or a tool-defined idempotency key and matching result
identity; otherwise the call becomes `recovery_required` for explicit user
resolution.

Resume, retry, skip, cancel, and create-new-Attempt are separate audited
decisions. Recovery remains Attempt-local: one blocked ToolCall or Provider
Account does not invalidate healthy sibling Agents, Segments, or Attempts. The
governance UI must preserve the exact frozen Provider Account, Model, credential
revision, sandbox report, approval state, and Incident lineage throughout the
decision.

Acceptance requires approval substitution/expiry tests, sandbox capability and
workspace-drift tests, tool-process crash windows, side-effect uncertainty
tests, Incident continuity tests, per-Agent failure isolation, and
plaintext-negative diagnostics and exports.

### Tool diagnostics and unknown-side-effect recovery status (2026-08-14)

`SOURCE VERIFIED / RECOVERY DECISION COMMAND OPEN`: Pi Bash/Edit now emits the
closed ToolCall stage sequence against the exact Attempt, Provider Account,
Model, Run, Agent, Execution Binding, Capsule, call digest, operation, and
Incident identity. The operational record has no representation for command,
path, Prompt, result body, credential, or Provider response. Recorder failure
is observational and cannot grant or deny execution.

Tool-specific path/content validation now precedes the dispatch authority
commit. An invalid Edit cannot create a dispatch fact or invoke the executor.
Raw executor errors are reduced to controlled codes before they can enter the
execution result or RunStream receipt.

On daemon startup, an execution with `Allowed` but no terminal fact is never
replayed. It receives one schema-v2 `ToolExecutionRecoveryRequired` fact with
the fixed `side_effect_unknown` code and `resolve_tool_recovery` action. A
Proposed-only ask remains pending. Missing authorization, code/action
substitution, duplicate recovery, and unknown schema-v2 authority facts fail
closed. Swift decodes and presents the result-unknown state and Incident ID,
but exposes no fake recovery button.

This closes the source diagnostic chain and the no-auto-replay recovery
projection only. Exact Attempt-bound startup `tool_recovery` operational
diagnostics, authoritative Resume/Retry/Skip/Cancel/New-Attempt commands,
component-level sandbox enforcement reports, native controls, and installed
acceptance remain open. Evidence:
`../P2D-W2D-tool-diagnostics-recovery-required-v1.md`.

## Parallel RouteSet product governance status

`SOURCE VERIFIED / INSTALLED LIVE OPEN`: the Agent editor now presents parallel
routes as explicit Provider Account/model rows with add/remove controls and a
separate Synthesis label. Fallback controls are unavailable while parallel mode
is active, preserving distinct audited meanings. Preflight and Board topology
retain route-sibling and Synthesis kinds without displaying opaque route-group
identifiers as user-facing labels.

Account revocation remains route-local in binding status; healthy siblings are
not rewritten or relabeled offline. Evidence is
`../P2D-W2A-W2B-W2C-product-parallel-routeset-authoring-v1.md`. Installed UI,
real Provider accounting, and live failure-isolation acceptance remain open,
so P2D-W2D remains `ACTIVE / PARTIAL`.

## Harness Platform Runtime governance amendment

`TARGET / ACCEPTED / NOT YET IMPLEMENTED`: Loom governance applies to Runtime
instances and Runtime Adapters, not to a peer Harness authority. Every tool
approval, sandbox report, Incident, recovery decision, Provider Account charge,
and fallback decision remains owned by Loom Harness Core and binds the exact
Runtime Contract version plus concrete Runtime identity.

An external Pi, Codex, Claude Code, or DeepSeek Harness Runtime may report
capabilities and execution evidence but cannot grant itself authority or claim
`full` sandbox enforcement without Loom-verifiable component evidence. Loom
Native reports the same Runtime Contract directly from daemon. Runtime failure
remains Agent/Attempt-local and never converts the Team into a generic offline
state.
## One-shot local ToolCall approval and content-free facts (2026-08-13)

`SOURCE VERIFIED / GENERAL TOOL GATEWAY OPEN`: an approval now has an explicit
consumption fact bound to the exact approval digest, Job, call digest, consumer
execution, operation, and correlation lineage. Same-consumer replay is
idempotent; different-consumer and concurrent reuse fail closed.

New local execution and permission decision facts use content-free schema v2:
the Journal retains canonical call digest, tool kind, controlled status/error/
recovery codes, and result digests, but not command, path, free-form denial,
raw executor error, Prompt, result body, or credentials. Schema-v1 history stays
readable. Permission attention exposes the non-secret call digest. Without an
authenticated encrypted proposal detail, Allow is disabled and Reject remains
available.

This is not a completion claim for W2D. The Pi encrypted proposal inspection
slice is now verified separately; broader multi-tool and multi-Runtime Tool
Gateway composition, sandbox enforcement reports, Incident stages,
uncertain-side-effect recovery, Swift governance, and installed live acceptance
remain open. Evidence:
`../P2D-W2D-one-shot-approval-content-free-journal-v1.md`.

## Encrypted Tool Proposal approval inspection (2026-08-14)

`SOURCE VERIFIED / PI ASK PATH INTEGRATED`: Tool Proposal detail now uses a
Conversation-DEK encrypted Vault record whose AAD freezes Conversation, Run,
generation, Runtime, Agent, Execution Binding, Capsule, call, approval,
operation, Incident, and content digest. Permission attention uses the exact
approval continuation digest to retrieve and revalidate that detail. Missing,
tampered, or substituted detail disables Allow and leaves Reject available.

This removes the encrypted-detail blocker for the Pi ask vertical slice. Native
interactive governance controls, generalized Attempt Tool Gateway admission,
sandbox/Incident/recovery evidence, other Runtime/Web/MCP paths, and installed
acceptance remain open. Evidence:
`../P2D-W2D-encrypted-tool-proposal-approval-inspection-v1.md`.

## Pi Attempt local Tool Gateway dispatch slice (2026-08-14)

`SOURCE VERIFIED / PROVIDER CONTINUATION OPEN`: the production Attempt Runtime
now passes its exact binding, Turn and Step through a daemon-private context.
For allowed Pi Bash/Edit calls, the execution adapter requires a per-proposal
dispatch gate to commit ToolCall admission and dispatch before invoking the
executor. Gate failure records `dispatch_not_committed` with zero side effects;
approved resume includes the exact one-shot approval identity.

Successful digest-only results are stored in the encrypted Attempt Payload
Store and accepted against the exact dispatch causation. The Pi result frame no
longer contains the ToolCall Envelope or command/path. RunStream acceptance
creates a distinct `run_stream_tool_result` receipt and marks the encrypted
payload delivered. This receipt does not claim Provider continuation or model
consumption. Pi child-process result injection, ask suspension/resume,
Read/Grep, Web/MCP, other Runtime adapters, sandbox enforcement reports,
tool-level diagnostics/recovery UI, and installed acceptance remain open.
Evidence: `../P2D-W2D-attempt-local-tool-gateway-v1.md`.

## Pi native Tool continuation status (2026-08-14)

`SOURCE VERIFIED / INSTALLED LIVE OPEN`: Pi Bash/Edit no longer stop at a
RunStream receipt. A private per-Attempt extension returns the digest-only
result as a native Pi `toolResult`; Pi must then emit a strictly validated
second-turn final message before Loom records `harness_final_output` delivery.
Ask holds the same extension request and deterministic operation until the
one-shot decision is available. Cancellation and deny have zero delivery
claim.

The same Pi Attempt may expose Context retrieval and the governed Tool path.
The first exact ToolCall locks the protocol route; cross-route or result
substitution fails closed. The child wire receives no command/path, approval
authority reference, payload binding, Prompt or Provider body. The
`governed_tool_loop` capability is conformance-bound to the locked Pi 0.82.1
runtime and must match the daemon Hook before process start. Evidence:
`../P2D-W2D-pi-native-tool-continuation-v1.md`.

Read/Grep execution, multiple sequential calls, Web/MCP and other Runtime
adapters, sandbox reports, recovery decisions, native governance UI and
installed acceptance remain open; W2D remains `ACTIVE / PARTIAL`.

## Pi governed Read/Grep content status (2026-08-14)

`SOURCE VERIFIED / INSTALLED LIVE OPEN`: Read/Grep now pass
the same exact Attempt dispatch and native continuation boundary as the local
Tool Gateway. The executor uses bounded descriptor-relative reads, rejects
symlinks, non-regular/binary/drifting targets and invalid Grep patterns, and
zeroizes result buffers on failure or close. Invalid paths/patterns fail before
dispatch.

Result bytes are stored only in a Conversation-DEK Attempt Payload with the
closed UTF-8 text content type. Journal, Evidence, operational diagnostics,
RunStream frames and Adapter cache retain only digest/identity metadata. The Pi
extension decrypts the exact pending binding, verifies the content digest and
returns it as a private native tool result. Transcript validation rejects
content substitution and only a valid second-turn final output writes
`harness_final_output`. A managed Pi child Read canary traverses the generated
private extension, consumes the content and completes the second turn while
Bridge frames and audit remain content/path free.

Read-only replay is allowed only by re-reading the same target and matching the
committed digest; drift fails closed. Remote Web/MCP results cannot use that
strategy. ATL3 must therefore persist their encrypted result before terminal
execution state and must never repeat a completed remote call to recover lost
content. Sequential calls, Web/MCP, other Runtime adapters, sandbox/recovery/UI
and installed acceptance remain open.
Evidence: `../P2D-W2D-pi-governed-read-grep-content-v1.md`.

## Crash-safe Web/MCP result commit status (2026-08-14)

`SOURCE VERIFIED / PRODUCTION BROKER CONFIG AND INSTALLED LIVE OPEN`: ATL3 now
has a content-bearing commit boundary that remote tools cannot bypass. The
Loom-owned Broker validates exact WebSearch/WebFetch/MCPTool calls before
dispatch. After `ToolDispatchCommitted`, returned bounded UTF-8 bytes are
encrypted under the exact Conversation/Attempt binding and accepted by the
Attempt Loop before the execution terminal fact. Commit failure produces a
controlled terminal, zeroizes plaintext, and never repeats the remote call on
replay.

Remote capability publication is exact rather than boolean. The Adapter, Hook,
Pi JSON schema, Pi prompt and strict transcript parser share the same immutable
set. An empty MCP allowlist omits MCPTool, an undeclared tool fails before
dispatch, and content/digest substitution fails closed. Query, MCP arguments,
result content, Prompt, credential and Provider response remain absent from
Journal, Evidence, diagnostics and ordinary Bridge/RunStream payloads.

Production Broker composition, sequential calls, other Runtime adapters,
sandbox/recovery/UI and installed acceptance remain open. Evidence:
`../P2D-W2C-W2D-crash-safe-web-mcp-result-commit-v1.md`.

## Composition governance amendment

`SOURCE VERIFIED / COMP1 AND COMP2-A-D / COMP2-E OPEN`: P2D-W2D governs P2D-COMP1/COMP2
compilation and activation. Composition diagnostics use one Incident chain and
record only Profile, snapshot digest, Bundle ID/version, lifecycle stage,
elapsed time, controlled result/error, and retryability. Bundle configuration,
Prompt, content, credentials, VMK, Provider body, tool arguments/results, and
internal authority state are excluded.

Journal, policy/grant/approval authorities, Vault root/key provider,
authoritative StateWriters, terminal Run/Attempt decisions, and local IPC peer
attestation are protected capabilities. Duplicate or replacement registration
fails closed. Effects may release temporary resources but cannot undo Journal
facts or silently retry an uncertain external side effect.

The governance UI may later show product Profile, snapshot version/digest,
Bundle readiness, failed stage, Incident ID, and safe recovery action. It must
not expose an arbitrary plugin manager in Phase 2D. Installed CV6, mixed-Team,
failure-isolation, Tool Loop, and governance UI gates remain unchanged by the
composition migration.

## Agent Inbox Incident and native governance status (2026-08-14)

`SOURCE VERIFIED / INSTALLED LIVE OPEN`: Queue, Steer, and Inject now retain one
Incident ID across Swift admission, private authenticated UDS, daemon active-
Attempt resolution, encrypted Inbox commit, and the Agent-local receipt or
failure. The closed diagnostic stage is `agent_input_admission`. App records
only Segment/Agent/WorkItem/Run/generation, mode, timing, result, controlled
error and retryability; daemon reuses the content-free Agent Attempt diagnostic
record. Input bytes, Prompt, credentials, Provider body, and response content
are not diagnostic fields, and a JSONL privacy regression verifies absence of
the submitted text.

Mission center exposes Queue/Steer/Inject per active Agent together with
Harness, Provider Account, Model and status. Drafts, progress, receipts and
failures are keyed per Agent. A failed admission retains the draft and shows an
actionable safe reason plus Copy Incident ID instead of changing Team status to
offline. Exact evidence is
`../P2D-W2C-W2D-authenticated-agent-input-ipc-swift-v6.md`.

The bounded V7 recovery rule distinguishes a recoverable pre-model Inbox
transition from an uncertain Provider operation. Exact consumed input may be
recovered only before `ModelRequestAdmitted` and only with the prior output
checkpoint. After that fact exists, automatic replay is prohibited and the
Attempt must use explicit recovery-required governance. Evidence:
`../P2D-W2C-W2D-agent-input-pre-model-crash-recovery-v7.md`.

Installed diagnostics, crash recovery, cross-Runtime consumption, CV6, ATL9,
accounting completion and COMP2-E remain open. P2D-W2D remains
`ACTIVE / PARTIAL`.

## Pi Agent input continuation governance status (2026-08-14)

`SOURCE VERIFIED / INSTALLED LIVE OPEN`: one Pi Agent Attempt may now consume a
later Queue/Steer/Inject batch without replacing its process or frozen Runtime
binding. Each accepted assistant output supplies only its digest to the Inbox
source. Step ID becomes the next private RPC correlation ID; content remains
outside Journal, diagnostics, transcript audit and ordinary Bridge frames.

The Adapter retains one ACK, monotonic events and one terminal while combining
per-prompt Harness accounting. Mutable input, rendered prompt, wire and
accounting buffers are cleared. Later Context/Tool use is not silently
re-authorized: that composition fails closed until its own conformance slice.

Official Pi, Codex/Claude, installed diagnostics, restart recovery, CV6, ATL9,
account-level accounting completion and COMP2-E remain open. Evidence:
`../P2D-W2C-W2D-pi-agent-input-continuation-v8.md`. P2D-W2D remains
`ACTIVE / PARTIAL`.

## Harness mutable prompt governance prerequisite (2026-08-14)

`SOURCE VERIFIED PREREQUISITE / CONTINUATION OPEN`: Codex and Claude Code
prompt/stdin content now has a bounded mutable owner and is cleared after every
production return path. It remains excluded from argv, environment,
diagnostics, Journal, Evidence and ordinary Bridge metadata.

Governance deliberately publishes no Agent Input capability for the current
one-shot Codex/Claude commands. A persistent protocol must bind process/session
identity, output checkpoint, frozen Attempt/Execution Binding, cancellation and
accounting before capability advertisement. Evidence:
`../P2D-W2C-W2D-harness-mutable-prompt-prerequisite-v9.md`. P2D-W2D remains
`ACTIVE / PARTIAL`.

## Daemon restart recovery observability v10 status (2026-08-14)

`SOURCE VERIFIED / USER RECOVERY ACTION OPEN`: daemon startup now emits the
closed `agent_attempt_reconcile` stage for reconstructed Attempt state. Safe
codes distinguish `agent_input_resume_required`,
`provider_outcome_uncertain`, `agent_input_recovery_unavailable` and
`agent_input_recovery_conflict`. The record carries Incident, Provider Account,
Model, Agent, Run/generation, Runtime, Execution Binding and Capsule digests,
but no Inbox body, Prompt, Provider response, ciphertext or native handle.

The pre-model and uncertain outcomes are deliberately non-retryable until an
explicit resume/recovery authority and UI action exist. Protected Journal/Run
corruption still fails daemon startup closed; an individual encrypted payload
failure is isolated to the corresponding Agent. Installed diagnostics, recovery
UI, CV6, ATL9, accounting completion and COMP2-E remain open. Evidence:
`../P2D-W2C-W2D-daemon-restart-attempt-reconstruction-v10.md`. P2D-W2D remains
`ACTIVE / PARTIAL`.

## Agent Attempt recovery projection v13 status (2026-08-14)

`SOURCE VERIFIED / READ-ONLY GOVERNANCE`: Team board diagnostics now retain the
closed `agent_attempt_reconcile` stage and attach a reconstructed outcome only
to the exact Incident/Provider Account/Model Agent. Swift accepts the same stage
and distinguishes resume approval, uncertain Provider outcome, unavailable
encrypted input, and recovery-state conflict. View diagnostics and Copy
incident ID remain available.

V10 records are non-retryable, so the inspector deliberately exposes no Retry
or Resume command. The projection does not register recovered state as active,
reattach a native session, or dispatch a Runtime/Provider request. Explicit
recovery authority and installed acceptance remain open. Evidence:
`../P2D-W2C-W2D-agent-attempt-recovery-projection-v13.md`. P2D-W2D remains
`ACTIVE / PARTIAL`.

## Agent Attempt recovery decision authority v14 status (2026-08-14)

`SOURCE VERIFIED / DECISION AUTHORITY ONLY`: an explicit user decision may now
authorize only the exact V10 `pre_model_resume` candidate. Candidate and Runtime
restart-capability digests use separate domains. Capability comes from an
injected trusted resolver and binds Attempt, Runtime, Execution Binding,
Capsule, checkpoint resume mode, and session identity. The decision atomically
checks recovery, Attempt Loop, and Run heads and writes one content-free Journal
fact. A concurrent second decision has one winner.

`provider_outcome_uncertain` cannot be approved by this path. No dispatch lease,
active-Attempt registration, native-session reattachment, Provider replay,
authenticated IPC or Swift action exists yet. Evidence:
`../P2D-W2C-W2D-agent-attempt-recovery-decision-authority-v14.md`. P2D-W2D
remains `ACTIVE / PARTIAL`.

## Agent Attempt recovery consumption v15 status (2026-08-14)

`SOURCE VERIFIED / RUNTIME COMPOSITION OPEN`: a V14 approval now has one durable
consumer and one process-local lease Take. Consumption revalidates candidate and
trusted Runtime capability, fences recovery/Loop/Run heads, and uses an
authority-generated lease ID so concurrent calls have one winner. Capability
drift leaves the decision unconsumed. Later calls receive no lease.

The content-free consumed fact authorizes no Provider replay. Production still
has no restart-safe resolver, native-session reattachment, active-Attempt
registration, authenticated IPC or Swift action. Evidence:
`../P2D-W2C-W2D-agent-attempt-recovery-consumption-v15.md`. P2D-W2D remains
`ACTIVE / PARTIAL`.

## Codex and Claude same-process continuation governance v11-v12

`SOURCE VERIFIED / INSTALLED LIVE AND RESTART REATTACH OPEN`: Codex 0.144.1 and
Claude Code 2.1.196 now publish Agent Input capability only behind exact
executable-digest locks and a bounded persistent system-session runner. Process,
native thread/session, frozen Execution Binding, Route Segment, credential
lease, model and Context delivery remain stable for all same-process rounds.

Codex binds response/thread/Turn IDs and accumulates exact per-Turn token usage.
Claude binds one session ID, exact replayed user input, typed internal ToolCall
pairing, compaction boundaries, and accumulates usage plus harness-reported
cost. Substitution, unknown lifecycle events, incomplete ToolCalls, output
overflow, timeout and cancellation fail closed. Agent Input content, Prompt,
Provider body and raw stderr remain outside operational diagnostics and
authority facts.

Encrypted ExternalSessionHandle persistence, daemon-restart reattachment,
installed diagnostics/live Provider execution, CV6, ATL9, accounting UI and
COMP2-E remain open. Evidence:
`../P2D-W2C-W2D-codex-app-server-agent-input-v11.md` and
`../P2D-W2C-W2D-claude-stream-json-agent-input-v12.md`. P2D-W2D remains
`ACTIVE / PARTIAL`.

## Agent Attempt recovery Runtime attachment v16 status (2026-08-14)

`SOURCE VERIFIED / PRODUCTION REATTACHER AND UI OPEN`: recovery now preserves
the exact Route Segment from consumed Inbox authority and binds it into the
candidate digest. Missing or cross-Segment metadata yields the affected Agent's
controlled `agent_input_recovery_conflict`; it cannot become an active Attempt.
The one-use grant also freezes the recovery Incident ID for the new operational
chain.

Trusted composition reattaches and validates exact Runtime/session identity
before registering the active Attempt. Every failure after attachment closes
the session, and normal close revokes registration first. The port exposes no
Provider request operation. No production resolver/reattacher, authenticated
recovery IPC, Swift action or Provider replay is wired. Evidence:
`../P2D-W2C-W2D-agent-attempt-recovery-runtime-attachment-v16.md`. P2D-W2D
remains `ACTIVE / PARTIAL`.

## Loom Native restart capability v17 status (2026-08-14)

`SOURCE VERIFIED / PRODUCTION CONTINUATION AND UI OPEN`: only a built-in Loom
Native adapter with the explicit Loom-owned checkpoint conformance can mint a
restart capability. The resolver revalidates exact Runtime, Execution Binding,
Provider Account, credential revision, Model and Capsule authority, then binds
Segment/checkpoint/input identity into a content-free session digest.

Unsupported Runtime, duplicate identity, exact Capsule duplication, route
substitution and post-consumption Capsule drift fail closed. An unrelated
corrupt Capsule does not block another Agent. No Capsule body, credential,
Prompt or Provider response enters the capability or diagnostics, and no
Provider call occurs. Production recovery composition, operational stage,
authenticated IPC and Swift Resume remain open. Evidence:
`../P2D-W2C-W2D-loom-native-restart-capability-v17.md`. P2D-W2D remains
`ACTIVE / PARTIAL`.

## Encrypted Loom Native continuation v18 status (2026-08-14)

`SOURCE VERIFIED / AUTHENTICATED RECOVERY UI OPEN`: the safe pre-model recovery
chain now has exact encrypted material continuity. The checkpoint is written
before Agent Input consumption, retrieved only by the complete content-free
frozen route, and deleted after authoritative success or failure
terminalization. Multiple matching candidates, binding drift, Capsule drift,
tamper and unavailable encrypted input remain isolated failures for the exact
Agent.

The controlled product path carries the recovery Incident through one-use grant
consumption, active-Attempt registration, ModelRequest admission, credential
lease, Provider continuation and Step/Turn terminal state. Journal assertions
exclude checkpoint, input, reply and credential fixture bytes. No plaintext
checkpoint is added to operational diagnostics or authority facts.

Production does not yet expose this operation through authenticated IPC or
Swift, and no user recovery action or installed Provider replay is claimed.
Evidence:
`../P2D-W2C-W2D-encrypted-agent-checkpoint-continuation-v18.md`. P2D-W2D remains
`ACTIVE / PARTIAL`.

## Recovery frame, accounting and authorization closure v19a status (2026-08-14)

`SOURCE VERIFIED / BUNDLE OBSERVER AND RECONCILIATION OPEN`: V18 continuation
can no longer be called as an ungoverned terminal path. A recovery-specific
frame authority checks exact recovery Incident correlation, original grant
operation limits, Bridge sequence, dispatch ACK, terminal Result and optional
authorized-frame observation. The returned AdapterResult transcript is matched
against accepted frames before Attempt Loop success is written.

Only a complete valid terminal stream reaches authoritative Run accounting and
terminal commit. Accounting is re-resolved against the exact current Run and
its frozen Rate Card. The original grant token is never reconstructed; exactly
one unrevoked grant is found by the complete frozen execution tuple and revoked
after terminal commit. Evidence:
`../P2D-W2C-W2D-recovery-frame-terminal-closure-v19a.md`.

Production still must restore the Team frame/evidence observer, persist the
recovery operation diagnostic stages, reconcile a crash between terminal commit
and grant revocation without Provider redispatch, and expose preview/confirm/
resume through authenticated IPC and Swift. P2D-W2D remains `ACTIVE / PARTIAL`.

## Production recovery lifecycle v19b status (2026-08-14)

`SOURCE VERIFIED / AUTHENTICATED RECOVERY UI OPEN`: production now restores the
exact Team frame/evidence observer before resumed output is admitted, owns the
V19A completion coordinator inside the mission Bundle, and closes the
Run-terminal-to-grant-revoke crash window during startup without any Runtime or
Provider redispatch port.

One exact stale grant is revoked by full frozen execution tuple. Already closed
state is ignored, while stale identity or duplicate pending authority stops
startup. Every successful repair emits a persistent, content-free
`authorization_reconcile` record at `agent_attempt_reconcile`, carrying the
startup Incident and non-secret execution identity but no Capsule, Prompt,
transcript, Provider body, credential, token or ciphertext. These success
records are deliberately excluded from Team failure projection.

Evidence:
`../P2D-W2C-W2D-production-recovery-lifecycle-v19b.md`. Authenticated
preview/confirm/resume IPC, Swift stale-command governance, installed
diagnostics/live acceptance, CV6, ATL9 and COMP2-E remain open. P2D-W2D stays
`ACTIVE / PARTIAL`.

## Authenticated recovery IPC and Swift governance v19c status (2026-08-14)

`SOURCE VERIFIED / INSTALLED LIVE OPEN`: one authenticated private-UDS route
now exposes read-only Preview, explicit principal-bound Confirm and a separate
one-use Resume operation for the V19B recovery lifecycle. The server rebuilds
and validates exact candidate, capability and decision state; no Swift value is
trusted as execution authority.

Mission Inspector limits recovery controls to the exact affected Agent and
shows its Harness, Provider Account, Model and credential revision. Confirm and
Resume remain separate user actions. Stale candidates, missing confirmation,
duplicate in-flight Resume and uncertain failures cannot silently dispatch or
reuse authority; the user receives a concrete message, Refresh when permitted
and Copy incident ID.

App and daemon use one Incident chain and persist only operation, safe stage,
elapsed time, result, stable error and retryability. Recovery diagnostics omit
candidate/capability digests, principal ID, Prompt, transcript, Provider body,
credential, Authorization header and API key. Evidence:
`../P2D-W2C-W2D-authenticated-recovery-ipc-swift-governance-v19c.md`.

Installed App recovery, broader Runtime restart support, CV6, ATL9, remaining
ATL3-ATL8 work and COMP2-E remain open. P2D-W2D stays `ACTIVE / PARTIAL`.

## ToolCall recovery decision governance v20 status (2026-08-14)

`SOURCE VERIFIED / TRUSTED RESOLVERS AND INSTALLED LIVE OPEN`: the
`side_effect_unknown` projection now leads to an authenticated, content-free
Preview/Resolve authority instead of an invented retry button. Candidate
identity is domain separated and CAS-fenced. A successful decision terminalizes
the original ToolCall as aborted, observed-effect accepted, or replacement
Attempt authorized; original executor replay is impossible.

Production advertises only Abort until trusted observation and replacement
resolvers exist. Swift displays exactly the server allowlist, requires explicit
confirmation and discards stale authority after uncertain failure. The
ToolCall-specific Inspector shows only Tool, Job and Incident metadata and
provides Refresh, privacy-safe diagnostics and Incident copy. App and daemon
diagnostics exclude candidate/decision/principal identity and every content or
secret field. Evidence:
`../P2D-W2C-W2D-tool-recovery-decision-governance-v20.md`.

Observed result continuation, replacement-Attempt dispatch, installed live,
CV6, ATL9, remaining ATL3-ATL8 and COMP2-E remain open. P2D-W2D stays
`ACTIVE / PARTIAL`.

## Loom Native sequential Context tools v21 status (2026-08-14)

`SOURCE VERIFIED / GENERAL TOOLS AND INSTALLED LIVE OPEN`: the Loom Native
Provider path now executes at most four sequential governed Context retrievals
within one frozen Attempt. Each result must reach `delivered` with
`provider_continuation` proof before the next exclusive call is admitted;
accepted-only, stale or over-budget state fails closed.

Usage is accumulated across every Provider round and remains attributable to
the same frozen Provider Account and Run. Product tests prove two independent
payload lineages, terminal Step/Turn success, exact continuation proof, and no
Context/credential content in Journal or Bridge frames. The fifth request no
longer advertises a tool and a nonconforming Provider response is rejected with
the existing content-free Context stage. Evidence:
`../P2D-W2C-W2D-loom-native-sequential-context-tools-v21.md`.

This slice adds no UI, network, installed App, general Tool Gateway, parallel
tool execution or new Runtime adapter. P2D-W2D remains `ACTIVE / PARTIAL`.

## Codex and Claude bounded multi-Context v22 status (2026-08-14)

`SOURCE VERIFIED / GENERAL TOOLS AND INSTALLED LIVE OPEN`: Harness Context
governance now records content-free counts for calls, prepared bindings,
acknowledged bindings, sealing and failure. It does not log proposal content,
payload content or Harness output. Codex/Claude Context results remain pending
until the validated final-output proof, then transition to delivered in exact
sequence.

The state machine fail-closes duplicates, the fifth call, concurrent calls,
calls after final sealing and final-output acknowledgement during an in-flight
Prepare. A failed acknowledgement batch can resume without duplicating an
already authoritative delivery. Product policy preserves Loom Native/Pi
exclusive continuation semantics instead of silently broadening every Runtime.
Evidence:
`../P2D-W2C-W2D-codex-claude-bounded-multi-context-v22.md`.

This slice adds no UI, real network, installed App, general Tool Gateway,
parallel effects or Runtime restart. P2D-W2D remains `ACTIVE / PARTIAL`.

## Codex and Claude governed Read/Grep v23 status (2026-08-15)

`SOURCE VERIFIED / READ+GREP ONLY / WEB+MCP+INSTALLED LIVE OPEN`: Codex and
Claude now use the Runtime-neutral Tool Gateway and existing product authority
for governed workspace Read/Grep. Process tool allowlists are derived from the
same private lease, exact executable conformance is checked separately, and
capability drift fails before credential access.

Read/Grep content is encrypted in the Attempt Payload store and is not added to
Journal, Evidence metadata or operational diagnostics. Calls remain pending
until validated Harness final output supplies the ordered delivery proof. The
daemon uses content-free path conflict digests, allowing distinct-path reads
without recording path or pattern. HTTP cancellation is merged with the outer
Attempt authority instead of treating request context as authority. Evidence:
`../P2D-W2C-W2D-codex-claude-governed-read-grep-v23.md`.

No Harness Bash/Edit/Web/MCP exposure, UI, installed App, real Provider,
restart reattachment, TTL/compaction or complete accounting is claimed.
P2D-W2D remains `ACTIVE / PARTIAL`.

## Codex and Claude native-tool bypass closure v24 status (2026-08-15)

`SOURCE VERIFIED / INSTALLED LIVE OPEN`: Harness-native tools can no longer
bypass Loom permission, Attempt Loop and Journal authority while the private
MCP is active. The credential gateway freezes the exact non-secret tool names
for one Attempt and validates both Provider-bound catalogs and successful
Provider-returned tool calls. Rejections retain the existing content-free
Harness protocol failure; raw Prompt, tool arguments, Provider body, credential
and loopback bearer tokens are not diagnosed or journaled.

Codex/Claude process confinement is defense in depth around that gateway:
private temp CWD, read-only Codex, disabled Codex native feature paths, and
Claude MCP-only allow/deny controls. A CLI version that ignores or rejects the
configuration fails before a governed Provider exchange can succeed. Evidence:
`../P2D-W2C-W2D-codex-claude-native-tool-bypass-closure-v24.md`.

Exact executable component recheck, installed diagnostics/live acceptance,
Web/MCP, CV6, ATL9, accounting UI and COMP2-E remain open. P2D-W2D stays
`ACTIVE / PARTIAL`.

## Role dependency disclosure isolation v25 status (2026-08-15)

`SOURCE VERIFIED / DISCLOSURE UI AND INSTALLED LIVE OPEN`: ordinary Team DAG
dependency output now crosses into a dependent Agent only through the frozen
Role Context Capsule boundary. Evidence artifacts are revalidated against the
projected source Attempt; only authorized output events are extracted. Exact
lineage is authoritative metadata, while model output remains untrusted and
role-restricted.

Provider Account, credential reference/revision, peer Capsule content and
private Evidence frames are excluded from the dependent prompt. No new output
content is written to Journal or operational diagnostics; those surfaces retain
only existing content-free Capsule/Segment/Attempt authority. Substitution or a
required-provenance budget failure blocks the affected dependent Agent without
rewriting source Agents or silently selecting fallback.

Evidence: `../P2D-W2C-W2D-role-dependency-context-capsule-v25.md`. User-visible
disclosure receipt inspection, complete accounting UI, installed CV6, mixed-
Team ATL9 and live failure-isolation acceptance remain open. P2D-W2D stays
`ACTIVE / PARTIAL`.

## Observed acceptance state v26 status (2026-08-15)

`SOURCE VERIFIED / OPERATIONAL TEST-RESULT INGESTION OPEN`: Role Capsule
assembly now projects only already-authoritative, content-free acceptance facts
as observed state: succeeded terminal status, output classification and digest,
accepted decision and digest, and UTC decision time. Model output remains a
separate untrusted item and cannot manufacture a successful test or acceptance
fact.

Substituted classification or decision digests fail closed before dependent
dispatch. The observed item is role-restricted and does not add Prompt,
Provider body, credential or output content to Journal or operational
diagnostics. Evidence:
`../P2D-W2C-W2D-observed-acceptance-state-v26.md`.

This slice does not add a structured `test_report`, tool-result authority,
diagnostic UI or installed acceptance. A full serial repository run exposed a
`cmd/loomd` package timing/stability residual even though both implicated tests
passed in isolated reruns; the full-repository gate is not recorded as passing.
P2D-W2D remains `ACTIVE / PARTIAL`.

## Governed test report v27 status (2026-08-15)

`SOURCE VERIFIED / CONTENT-FREE REPORT AUTHORITY / UI OPEN`: controlled test
commands now yield an Attempt-authoritative report containing only non-secret
classification, outcome and lineage metadata. Raw command, stdout/stderr,
Prompt, Provider body and credential content are not added to Journal or
operational diagnostics by this report path. Report replay and query are bound
to the exact Attempt, ToolCall, payload and delivery state.

Dependency disclosure uses the existing role-restricted observed item and
includes only runner, scope, outcome, sequence and digests. A Provider/model
statement cannot create or modify a report, and a non-test Bash call is not
projected as test evidence. Evidence:
`../P2D-W2C-W2D-governed-test-report-v27.md`.

User-visible report/diagnostic inspection, per-test parsing, installed CV6,
mixed-Team ATL9, accounting UI and live failure isolation remain open.
P2D-W2D stays `ACTIVE / PARTIAL`.

## Governed test report Board/UI v28 status (2026-08-15)

`SOURCE VERIFIED / COMPACT AGENT SUMMARY / INSTALLED LIVE OPEN`: Mission
Inspector now shows the affected Agent's V27 governed test pass/fail counts and
latest closed runner, scope and outcome. The strict Swift wire model accepts
legacy rows with no report fields and rejects contradictory availability,
count, enum or digest combinations.

The product read path injects the existing Attempt Loop report authority into
both normal and live-observer timeline construction through the compatibility
Bundle facade. Source failures and invalid report sets are omitted, not
translated into invented success or Team-wide unavailability. The UI receives
only content-free summary metadata and cannot use it as execution authority.
Evidence: `../P2D-W2C-W2D-governed-test-report-board-v28.md`.

Raw report/output inspection, per-test parsing, installed App verification,
CV6, mixed-Team ATL9, complete accounting UI and live failure isolation remain
open. P2D-W2D stays `ACTIVE / PARTIAL`.

## Production remote Tool Broker lifecycle v29 status (2026-08-15)

`SOURCE VERIFIED / ENROLLMENT UI AND INSTALLED LIVE OPEN`: the remote Broker is
now a revocable Work Bundle resource. Its exact capability set comes from
present Search, WebFetch and MCP ports; an MCP allowlist without the matching
client, an unused client, invalid ceilings or an empty Broker fails startup.
Default production startup remains remote-tool unavailable rather than
inventing an offline Team or silently exposing network access.

Composition rollback and Dispose cancel active calls, close the owned HTTP
transport and revoke stale references. This path adds no query, URL, MCP
arguments/result, endpoint credential, Prompt, Provider body or secret fields
to operational diagnostics or Journal. Evidence:
`../P2D-W2C-W2D-production-remote-tool-broker-composition-v29.md`.

User-facing backend enrollment, account/policy projection, installed Web/MCP
diagnostics and live failure isolation remain open. P2D-W2D stays
`ACTIVE / PARTIAL`.

## Persisted remote backend Enrollment v30 status (2026-08-15)

`SOURCE VERIFIED / CLIENT EDITING, RUNTIME MATERIALIZATION AND INSTALLED LIVE
OPEN`: Web Search and MCP Enrollment now has Journal authority, deterministic
projection and authenticated product routes. One record binds the exact
Provider Account Policy version/revision/digest, adapter, endpoint fingerprint,
MCP allowlist and bounded account-local limits. Configure CAS-fences both
Enrollment and policy heads; revoke does not depend on the current policy, so
policy drift cannot prevent capability withdrawal.

Setup exposes active and revoked records only beneath the owning Provider
Account and marks exact policy currency. Strict Swift decoding rejects unknown
or secret-shaped fields, raw endpoints, malformed identifiers/digests,
unsorted allowlists and contradictory current-policy state. Operational
diagnostics retain Incident, operation, Provider/Account, stage, elapsed,
result and retryability only; Enrollment input, endpoint fingerprints and tool
names do not enter the diagnostic record. Evidence:
`../P2D-W2D-remote-tool-backend-enrollment-v30.md`.

This slice does not construct Search/MCP clients from persisted Enrollment,
grant capabilities to an Agent, add the editing/preflight UI or run installed
Web/MCP acceptance. Default production remains remote-tool unavailable.
P2D-W2D stays `ACTIVE / PARTIAL`.

## Remote backend Enrollment client governance v31 status (2026-08-15)

`SOURCE VERIFIED / TRUSTED NEW ENROLLMENT, AGENT PREFLIGHT, MATERIALIZATION AND
INSTALLED LIVE OPEN`: Swift now has a strict account-scoped configure/revoke
contract for the V30 authority. The concrete UDS client validates exact response
lineage, and the product store refreshes the authoritative setup projection
before reporting success. Conflict, stale policy, missing record and unavailable
service remain Enrollment-local failures with safe stage, retryability and
Incident correlation.

The native Provider Account policy sheet projects each active/revoked Search or
MCP record, policy currency and bounded limits. Its editor can update limits or
the MCP allowlist, explicitly rebind policy drift, restore a revoked record and
confirm revocation. Failure presentation provides Retry, redacted diagnostic
preview and Incident ID copy. Adapter, endpoint fingerprint and MCP server
identity remain read-only and are never reconstructed from user-entered raw
endpoint text.

V31 deliberately does not add an arbitrary backend creation form. New records
must come from a future trusted built-in candidate registry that can materialize
the corresponding Work Bundle client. Until that source exists, default
production exposes no Web/MCP capability and no Agent preflight can freeze one.
Evidence: `../P2D-W2D-remote-tool-enrollment-client-governance-v31.md`.

P2D-W2D remains `ACTIVE / PARTIAL`; installed Web/MCP diagnostics, per-Agent
failure isolation, accounting and ATL9 acceptance are still mandatory.

## Agent remote tool Enrollment binding V32 status (2026-08-16)

`SOURCE VERIFIED / EXECUTION PROFILE ENROLLMENT SELECTION, PREFLIGHT, FROZEN
ATTEMPT BINDING AND WORK BUNDLE MATERIALIZATION BOUNDARY PRESENT / INSTALLED
LIVE OPEN`: the per-Agent Enrollment selection now flows through the
ExecutionProfile, per-Agent preflight, Frozen Attempt Binding digest and the
trusted Work Bundle materialization boundary. The setup builder accepts
`none` or `<enrollment_id>:<digest>` per Agent and validates against the
authoritative projection; preflight blocks revoked / drifted / mismatched /
conflicted / unsupported / unavailable Enrollments with closed codes and never
blocks peer Agents. A trusted built-in backend candidate registry
(`builtin.search.deepseek.v1`, `builtin.mcp.stdio.v1`) plus the
`internal/toolbroker/enrollment` materialization boundary is the only path
from a persisted, active, policy-current Enrollment to a typed
`execution.RemoteToolExecutor` with the exact allowlist and bounded limits.
The native Agent editor adds a per-Agent "Remote tools" picker with strict
Swift models; one-sided Enrollment pairs and unknown fields are rejected. No
real Search/MCP transport, App bundle, network request, Provider, real
credential or user workspace was used; default production publishes no remote
capability. Evidence:
`../P2D-W2D-agent-remote-tool-enrollment-bindings-v32.md`.

P2D-W2D remains `ACTIVE / PARTIAL`; installed Web/MCP diagnostics, per-Agent
failure isolation, accounting and ATL9 acceptance are still mandatory.
