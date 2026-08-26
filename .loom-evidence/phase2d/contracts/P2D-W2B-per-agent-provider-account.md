# P2D-W2B Contract: Per-Agent Provider Account identity

Status: `ACCEPTED / ACTIVE PHASE SCOPE COMPLETE`

Version line: `v0.5.x`

Authority: ADR-0019 and Product Owner confirmation on 2026-08-09.

## Objective

Close the P0 gap where a Team role identified a Provider and Model but could
not distinguish or freeze one Provider Account and credential revision.

## In scope

- Preserve `TeamDefinitionRole.RuntimeProfileID` as the role-local reference.
- Treat `RuntimeProfile` as the v0.5.x Execution Profile contract.
- Separate Runtime Adapter/instance, Provider, Provider Account, Model,
  endpoint fingerprint, credential reference/revision, timeout, and required
  capabilities.
- Freeze optional reasoning effort as part of the Execution Profile and Agent
  Attempt binding. Explicit values require a Runtime that advertises the
  `reasoning_effort` capability; an empty value means Provider default.
- Let the user author role-local Model, reasoning effort, timeout, and budget
  changes as a new immutable Execution Profile without mutating a peer Agent.
- Persist the complete non-secret versioned Execution Profile snapshot with
  each saved role and restore it after App/daemon restart even when the dynamic
  catalog no longer publishes that custom Profile ID.
- Freeze one immutable, digest-bound execution binding per role in
  `BuildSavedTeamRuntimeBinding`.
- Let each Agent own a versioned RouteSet of Runtime-compatible Execution
  Profiles while freezing exactly one selected Profile per Attempt.
- Target P2D-W2C freezing of the role-scoped Context Capsule digest beside the
  exact Execution Binding; private or role-restricted context cannot leak into
  peer Agents. This remains missing until a real Capsule record, scope/ACL,
  packing, omission manifest, and disclosure receipt exist.
- Require a complete Provider Account tuple for non-native role execution.
- Preserve native-auth Profiles without inventing credential bytes or a remote
  endpoint.
- Keep secret body bytes outside Profile, Team, Prompt, Journal, Run, Attempt,
  and Evidence structures.
- Allow up to nine independently bound Agent roles in one governed Team. The
  same limit must apply at definition, plan, dispatch, Journal replay, and
  projection replay boundaries; one main Agent remains mandatory.

## Out of scope

- Provider Account persistence and management UI.
- Agent Attempt Journal payloads and adapter credential resolution.
- Explicit fallback, cooldown, and account usability projections.
- Provider Account concurrency, rate-limit, budget, token/cost, and error-rate
  accounting.
- The four-Provider live Team acceptance matrix.

These are owned by P2D-W2C and P2D-W2D. Phase 2D does not require a separate
product Goal or external service for this closure.

## Acceptance

1. Two roles using the same Provider but different Provider Account IDs and
   credential revisions produce different frozen binding digests.
2. Every saved-Team role exposes its own exact frozen binding.
3. Missing account ID, endpoint fingerprint, credential reference, or positive
   revision fails closed for non-native execution.
4. Returned binding fields cannot mutate the accepted Candidate.
5. Focused Runtime and Team tests, full repository tests, vet, and formatting
   pass before this WorkItem is marked `CURRENT`.
6. Legacy Profiles with empty reasoning effort retain the published v1 binding
   digest; explicit values use a domain-separated v2 digest and reject
   post-freeze mutation.
7. A custom role Profile survives confirmation, Journal replay, projection,
   daemon materialization, and reopening the Saved Team after a fresh Setup
   Service start; same-ID content drift fails closed.
8. Invalid Model, reasoning effort, timeout, or budget edits leave the Draft
   revision, selected role, and session catalog unchanged.
9. Two sibling Attempts for one Agent may select different approved RouteSet
   entries, but each Attempt freezes one Profile and one Role Capsule digest.
10. An Agent cannot select a Harness-incompatible Profile or inherit another
    Agent's private Capsule, Provider Account, or credential revision.
11. One source canary creates Codex/OpenAI, Claude Code/Anthropic, Loom/Kimi,
    and Loom/MiniMax roles in the same Team and freezes four distinct binding
    digests with exact account and credential revisions.
12. Team capacity accepts nine Agent roles and fails closed at ten without
    creating a definition, plan, dispatch, or replay mismatch.
13. Product Mission compilation consumes every saved role instead of reducing
    the Team to its main Agent. Each DAG node and Attempt retains the role's
    exact Profile, and multi-node reconstruction preserves the same plan digest
    after daemon restart.

## Catalog freeze repair evidence

The build 29 source audit found and repaired one P0 catalog mismatch: the
Codex/OpenAI Profile froze `high` reasoning without declaring the Runtime's
`reasoning_effort` capability. The Profile now requires both
`reasoning_effort` and `workspace_edit`. A shared catalog assertion freezes
every published role option against its exact Runtime instance and model, so a
visible but non-executable Agent Profile fails before packaging. The complete
Go and Swift suites pass. Build 29 is an uninstalled Candidate; installed mixed
Team acceptance remains owned by W2D.

## Build 31 governance projection amendment

Every Agent Board row now carries the exact non-secret limits and capability
facts frozen by its Attempt: timeout nanoseconds, optional binding budget, and
the canonical sorted capability set. The projection copies them from the
immutable execution binding and does not expose credential reference, endpoint
fingerprint, secret material, Prompt, or Provider response.

The Swift boundary rejects contradictory availability, nonpositive timeout,
negative budget, and invalid, duplicate, or unsorted capabilities. The Team
inspector displays these values on the affected Agent row and keeps binding
budget distinct from Provider Account policy budget. Focused RED/GREEN Go and
Swift tests, the complete Go suite, focused race/vet, and the full
176-XCTest/eight-Swift-Testing suite pass. Installed mixed-Team execution and
failure-isolation acceptance remain open.

## Build 36 cardinality repair evidence

The four-Provider acceptance Team previously failed before preflight because
Saved Team binding, structured draft content, accepted-draft role seeding, and
accepted-plan validation retained an obsolete two-sub-Agent ceiling. Those
boundaries now share the accepted `MaxTeamAgentCount - 1` contract already used
by Team definitions, execution plans, saved instantiation, and saved records.

Regression coverage carries four roles through independent frozen bindings,
dormant instantiation, record projection, structured draft acceptance, and
accepted-draft instantiation. It also rejects `MaxTeamAgentCount + 1`. Complete
Go, focused Teams/Provider race, affected vet, and full Swift suites pass. This
is source closure only; installed mixed-Team execution and account-local live
failure isolation remain W2D gates.

## Four-role product-path revalidation

The daemon materialization test now rebuilds four persisted custom brokered
Execution Profiles for Codex/OpenAI, Claude Code/Anthropic, Loom Native/Kimi,
and Loom Native/MiniMax without relying on matching dynamic catalog entries.
Every role retains its exact Runtime instance; a same-ID credential revision
drift remains a fail-closed configuration conflict.

The Projection Mission binding test carries the same four-role shape into
preflight and compilation. It verifies four role bindings, four plan nodes,
four semantics, eight Attempt candidates, and each role's exact Harness,
Provider Account, credential reference/revision, Model, timeout, and
capability set. Revoking the Kimi account blocks only the Kimi role in
preflight while the other three remain ready; start remains fail-closed because
the required DAG is incomplete. Existing controlled dispatch evidence then
freezes four distinct binding digests and preserves peer success under one
Provider failure.

Focused normal/race tests, affected vet, complete App/daemon packages, and the
clean repository Go suite pass. Installed mixed-Team and Provider live gates
remain owned by W2D.

## Execution Profile governance projection amendment

The Builder and Mission preflight wire contracts now carry each Agent's exact
non-secret auth mode beside Provider Account and credential revision. A
configured fallback additionally carries its own reasoning effort, timeout,
optional budget, and canonical capability set instead of inheriting or hiding
the primary route's values.

Both Go and Swift fail closed on malformed account-bound or native-auth tuples,
revision drift, invalid limits, noncanonical capabilities, contradictory route
status, or latent fields on an unconfigured fallback. A native fallback with no
Provider Account and revision zero remains valid. The governance UI presents
the primary and fallback Harness, Provider Account, Model, auth posture,
credential revision, reasoning, timeout, budget, and capabilities without
exposing credential reference, endpoint fingerprint, or secret material.

This closes the source projection ambiguity only. Installed four-Provider Team
execution and account-local failure isolation remain W2D acceptance gates.
Complete Swift, `internal/app`, cross-language contract probe, affected race,
repository vet, and clean serialized repository Go gates pass.

## Independent Agent route editor amendment

Agent role identity and execution route selection are now separate Builder
operations. The Role selector may change AgentDefinition and responsibility;
the Runtime and Provider Account selectors may not. Model, reasoning effort,
timeout, budget, and fallback remain explicit sibling controls.

`*_provider_account_route` creates a new immutable Profile from a source on the
same Runtime, replacing the exact Provider, Provider Account, auth mode,
endpoint fingerprint, credential reference/revision, and the source route's
compatible Model. The legacy `*_harness_route` operation is interpreted as a
Runtime route change and accepts only a source already publishing the current
Provider Account, credential revision, and Model, then replaces only Runtime
Adapter and Runtime instance. Reasoning, timeout, budget, and capability
fields remain unchanged. Both operations preserve AgentDefinition, skills,
permissions, resources, responsibility, and peer roles. The candidate must
freeze against the exact target Runtime before the Draft changes; cross-Agent
sources, unsupported Models/capabilities, malformed credentials, and invalid
catalog combinations fail closed without revision or digest mutation.

The Swift editor shows Agent role, Runtime, Provider Account, Model, reasoning,
timeout, budget, and fallback as distinct controls. Account options include the
credential revision and compatible Model but never the credential reference or secret. Route-current
state is resolved by the selected dimension, so a generated custom Profile does
not lose its checkmark merely because its Profile ID is absent from the base
catalog.

Source acceptance covers account-only and Harness-only changes, exact
credential freezing, confirmation and projection replay, peer isolation,
Store/API forwarding, strict Swift route modeling, and the governance UI source
contract. Complete Go, affected race/vet, and 177-XCTest/eight-Swift-Testing
gates pass. Installed four-Provider Team execution remains a W2D live gate.

## Build 51 multi-Agent Builder evidence

The product Builder previously represented every sub-Agent edit through one
unscoped `subagent` selector. Domain and dispatch supported multiple roles, but
the App could not safely author them: editing any sub-Agent replaced the whole
sub-Agent list, and the concrete Swift IPC client rejected most execution
profile fields before sending a request.

`BuilderEditCommand` now carries an optional structured
`role_agent_definition_id`. Sub-Agent role, Harness, Provider Account, Model,
reasoning, timeout, budget, fallback, and removal operations require that exact
stable identity. Add Agent requires no target and rejects duplicate identities
or cardinality overflow. RouteSet fallback state is stored per Agent identity.
Repeated customization continues from the session-local immutable Profile and
leaves peer Agents unchanged.

The catalog publishes Coordinator, Bounded Worker, Reviewer, Researcher, and
Verifier identities across every compatible verified execution route. The
macOS Builder provides explicit Add Agent and icon-only Remove Agent controls;
each Agent row keeps its own execution controls. The concrete IPC client accepts
the full closed Builder field set and sends the safe target identity without
exposing credential references or secrets.

Focused Go RED/GREEN, complete Swift verification, strict Go/Swift contract
probes, affected race, full vet, and diff checks pass. A full concurrent Go run
had one transient strict-loopback `unavailable`; the exact malformed-response
case passed five consecutive isolated reruns, and the final serialized
repository suite passed every package. Installed mixed-Team authoring,
dispatch, and failure isolation remain W2D live gates.

## Frozen Agent disclosure-policy amendment

Each governed Run and Team Attempt now freezes the exact Provider Account
policy version, revision, digest, trust domain, retention mode, and data region
selected at claim or dispatch. The values are reconstructed from the immutable
historical policy event identified by revision and digest; a later account
policy update cannot rewrite an admitted or terminal Attempt.

Legacy v1 policies remain valid with disclosure explicitly unspecified. A v2
Attempt requires the closed trust-domain, retention, and region tuple. Work
Authority replay, Run projection, Team Attempt overlay, Board wire, and Swift
strict decoding all fail closed on missing or substituted v2 values.

The Agent row displays the frozen policy beside Harness, Provider Account,
Model, credential revision, limits, Context policy, accounting, and failure
state. It does not substitute the Provider Account directory's current policy
or claim that Loom certified Provider retention or residency behavior.

Exact source evidence is
`../P2D-W2B-W2D-agent-disclosure-policy-freeze-v1.md`. Installed mixed-Team and
real Provider acceptance remain W2D gates.

## Product parallel RouteSet authoring status

`SOURCE VERIFIED / INSTALLED LIVE OPEN`: each saved role can now freeze one
versioned parallel RouteSet containing its primary Execution Profile, one or two
additional Provider Account/model bindings, and an explicit Synthesis binding.
No Provider choice is lifted to Team scope. Route/profile/account/credential
revision substitution fails in State and Projection, and Builder reopen
preserves the exact role ownership.

The mixed fixture proves one stable Kimi Agent with independent Kimi and
DeepSeek sibling routes while the other three Agents keep their own bindings.
Evidence is
`../P2D-W2A-W2B-W2C-product-parallel-routeset-authoring-v1.md`. Installed
mixed-Team acceptance remains open; P2D-W2B remains `ACTIVE / PARTIAL`.

## Harness Platform ownership amendment

`TARGET / ACCEPTED / NOT YET IMPLEMENTED`: Loom is the Harness Platform and
owns the Harness Core. Per-Agent authoring selects a Runtime, Provider Account,
and Model; it does not select a competing Harness authority. Runtime choices
include Loom Native, Pi, Codex, Claude Code, and a future version-pinned
DeepSeek Harness Runtime.

The target chain is `TeamRole -> AgentDefinition -> ExecutionProfile ->
RuntimeContract + RuntimeAdapter/Instance + ProviderAccount/CredentialRevision
+ Model + Limits/Capabilities`. Existing `Harness` and `AdapterType` bytes are
legacy v0.5.x schema carriers and remain replayable. Runtime migration must be
deterministic and cannot rewrite accepted Attempt or Evidence history.

## Composition Kernel dependency amendment

`TARGET / ACCEPTED / NOT YET IMPLEMENTED`: Team and Agent Capability Contexts
are child scopes of the admitted Product/Conversation composition snapshot.
They expose only bounded Runtime, Provider Adapter, credential-lease, policy,
accounting, and projection ports. They never contain plaintext credentials,
global Provider clients, or mutable Execution Profiles.

Each Agent/Attempt independently freezes the Composition Snapshot digest plus
Runtime Contract/instance, Provider Account, credential revision, model,
limits, capabilities, policy, fallback, and Context Capsule. A Bundle or
snapshot change applies only to newly admitted scopes and cannot turn a Team
into a single-Provider composition or overwrite one Agent's binding.

The `loom-agent-runtime`, `loom-governance`, and `loom-work` Bundles retain
Agent-local failure and accounting. Missing Bundle capability blocks only the
dependent route/Agent when the Product remains governable; protected-core or
Profile compile failure blocks startup with a specific composition Incident.
