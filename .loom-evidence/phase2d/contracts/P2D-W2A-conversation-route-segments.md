# P2D-W2A Contract: Conversation Route Segments and Context Capsules

Status: `ACCEPTED / ACTIVE PHASE SCOPE COMPLETE`

Version line: `v0.5.x`

Parent Goal: `Phase 2D - Multi-Provider, Multi-Model Agent Team Orchestration`

This contract extends the existing P2D-W2A WorkItem. It does not create a new
product Goal, replace P2D-BLOCKER-1, or reset the accepted W2B/W2C Candidates.

## Product outcome

The user sees one Loom Conversation even when its execution route changes.
Each change of Runtime, Provider, Provider Account, Model, or credential
revision creates an immutable Conversation Segment and one or more Agent
Attempts. Loom never mutates an earlier Segment binding and never passes a
Provider-native session identifier to a different Provider, account, Model, or
Segment.

The early Build 17 thread-rotation boundary remains historical compatibility
behavior. Current source and installed evidence preserve one visible Loom
Conversation with immutable Segments, frozen Route bindings and governed
Context disclosure.

## Route and Attempt contract

- AgentDefinition and AgentInstance identity remain stable.
- A versioned RouteSet contains compatible Execution Profiles.
- Every turn or Attempt selects and freezes exactly one Execution Profile.
- One-turn parallel evaluation creates sibling Attempts, followed by a separate
  Aggregation Attempt. One Attempt cannot contain multiple Providers.
- Fallback creates a new Attempt and cites an explicit, versioned, approved
  Route Transition. Silent substitution fails closed.
- Provider compatibility is declared by the Runtime Adapter. Codex and Claude
  Code cannot be assumed compatible with arbitrary Providers; Loom Native may
  expose only the Provider combinations its concrete Provider Adapters validate.

Loom is the Harness Platform and owns Conversation/Segment/Attempt authority.
Runtime Adapter here means the bridge to a concrete external Runtime; Loom
Native implements the Runtime Contract in daemon and does not require a
Harness Adapter.

An ExternalSessionHandle is encrypted and bound to Provider, Provider Account,
Model, Segment, and credential revision. Provider response/conversation IDs and
prompt-cache handles cannot cross any of those boundaries. Loom-owned canonical
state is preferred, with stateless or `store=false` Provider calls where the
Provider capability permits it.

## Context Capsule

A route change builds a target-specific, versioned, content-addressed Context
Capsule. It is not an unfiltered copy of a messages array. The Capsule contains
only policy-admitted fields such as the conversation goal, confirmed user
constraints, accepted decisions, current task state, unresolved questions,
recent user turns, artifact references and digests, workspace snapshot, Team
governance state, prior visible model outputs, omissions, disclosure classes,
provenance, schema version, and digest.

Capsule inputs have three trust classes:

- `authoritative`: user confirmations, Goal/WorkItem/policy, code, Journal, and
  accepted Evidence facts;
- `observed`: tool results, tests, and Provider/runtime observations;
- `untrusted`: earlier model output, summaries, suggestions, and hypotheses.

Earlier model output cannot become system or authoritative content. Hidden
reasoning is never transferred. A generated summary is a Candidate and cannot
be the sole source of continuity.

The switch pipeline freezes the source Segment, builds a source-neutral
Candidate, applies classification/redaction/ACL/target-account disclosure
policy, deterministically packs the target context window, records omissions,
computes the Capsule digest, uses a target ContextAdapter, freezes the new
Segment/Execution Binding/Capsule digest, and dispatches with one Incident ID
and a non-secret disclosure receipt.

Supported switch modes are `continue_with_context`, `summary_only`, and
`start_clean`. Packing priority is policy and identity, confirmed constraints
and current state, workspace/artifacts/recent user turns, relevant history and
untrusted prior output, then scoped retrievable references. No omission is
silent.

At minimum, context scopes are `conversation_shared`, `team_shared`,
`agent_private`, `role_restricted`, `artifact_scoped`, and
`secret_reference_only`. W2B/W2C generate one Role Context Capsule per Agent and
freeze its digest on the Attempt. Credentials enter only as opaque references
and revisions in the Execution Binding.

## UI contract

Each Segment or turn shows Harness, Provider Account, and Model within the same
visible Conversation. Route switching offers Continue with context, Summary
only, and Start clean. A trust-domain, retention, region, or sensitive-data
policy change requires explicit confirmation. The user can inspect disclosed
context categories, omissions, and the safe disclosure receipt, but never sees
keys, hidden reasoning, or internal complete prompts.

Provider Directory, Runtime inventory, and Conversation Routes are distinct
authoritative projections. A Harness such as OpenCode cannot appear as a Model
Provider row merely because it discovers or dispatches Provider models. A
Conversation Route may bind OpenCode + DeepSeek, but its Provider display name
is derived only from `provider_id`; the Harness/Provider combination is rendered
only as an explicitly labelled Route. Synthetic Route/Profile IDs must never be
accepted as Provider IDs or credential destinations.

The Route picker groups compatible execution options by Harness. Each option
shows the Provider and a human-readable Provider Account name only when the
account is not the default Primary account. Internal account identifiers such as
`deepseek.primary` are not user-facing labels. A Harness-owned route without an
external Provider is named `Built-in`; it is not projected as another Provider.
The collapsed selection remains an explicit Harness + Route summary, while Model
selection remains a separate control.

## Incremental delivery

1. Preserve the current profile-conflict fail-closed server rule and repair the
   installed stale-anchor/load race with actionable conversation diagnostics.
2. Deliver one visible Codex/OpenAI to Loom Native/DeepSeek Segment transition
   using `summary_only`, a frozen Capsule digest, and a real installed reply.
3. Add full-context packing, disclosure receipts, scoped retrieval, restart
   recovery, and Provider-native handle isolation.
4. Reuse RouteSet and Role Capsules for parallel Attempts and the four-Provider
   mixed Team matrix.

## Current source Candidate

- one visible thread survives Profile transitions;
- schema 1 threads migrate to one immutable `start_clean` Segment;
- every routed call creates one Attempt with Profile, Segment, Context mode,
  Capsule digest, Binding digest, and status;
- `summary_only` includes bounded recent user input and omits model output;
- `continue_with_context` wraps prior model output as explicit untrusted
  policy-filtered context instead of target assistant history;
- `start_clean` sends only the new turn;
- Swift fences stale loads, sends the explicit transition, and shows Harness,
  Provider Account, and Model for routed turns.

This vertical slice does not yet implement the full structured Capsule schema,
deterministic model token accounting, omission/disclosure receipts, external
session handles, trust-domain confirmation, or encryption at rest. Those remain
acceptance gates in this WorkItem and W2D.

## Exact Provider Account conversation profiles

Conversation routing freezes the exact Provider Account and credential revision,
not only the Provider. Every verified brokered account published by setup receives
an immutable account-scoped Profile ID. The existing primary-account Profile IDs
remain byte-compatible with persisted threads; non-primary accounts receive a
distinct suffix derived from their validated account identifier. Revoked,
cross-Provider, malformed, ambiguous, or revision-zero records are not published
and cannot route.

The daemon resolves a selected Profile against all current accounts for that
Provider, requires one unambiguous exact match, and acquires the Loom Vault lease
with Provider ID, Provider Account ID, opaque credential reference, and credential
revision. It never falls back to a Provider-primary credential. Swift rejects
cross-Provider account identity or auth/revision drift at the wire boundary and
the conversation picker displays Provider, exact account, and Model. Focused Go,
race, vet, strict Swift model, Store, and UI tests cover this chain. The current
full Swift suite passes 175 XCTest cases with one intentional visual skip and
eight Swift Testing contracts.

## Acceptance

The Phase 2D matrix includes Codex to DeepSeek and DeepSeek to Anthropic route
changes, same-Provider account/model changes, parallel sibling Attempts plus
aggregation, explicit fallback, four Agents with independent Role Capsules,
credential revocation isolation, deterministic context-window omissions,
prompt-injection trust separation, restart isolation, and secret-free
Journal/diagnostics/Evidence. W2D additionally gates encrypted-at-rest storage
for transcript, Capsule, and ExternalSessionHandle before Phase 2D exit.

The exact-account source Candidate is packaged as uninstalled v0.5.2 build 26.
Its arm64 identity, deep ad-hoc signature, owner-only permissions, no-symlink
boundary, and transport extraction byte equality pass. It was not launched or
installed and does not replace the installed DeepSeek r6 acceptance gate.

## Anthropic Messages route source Candidate

The ordinary-conversation route set now also publishes every exact verified
Anthropic Provider Account as an immutable `anthropic_messages` Profile. The
Profile freezes account identity, credential revision, and
`claude-sonnet-5`. Dispatch resolves that Profile through the same exact Loom
Vault lease used by Agent Attempts and never falls back to an Anthropic-primary
credential.

The Provider client uses the fixed official HTTPS Messages endpoint, a private
proxy-free and redirect-free transport, bounded request/response bodies,
`x-api-key` only on the single request header, and the fixed
`anthropic-version` contract. It accepts only a matching assistant message with
bounded text content blocks; model drift, tool blocks, unsafe content, redirect,
oversize, auth, rate-limit, timeout, and malformed responses fail closed through
the existing privacy-safe conversation failure taxonomy.

Go Provider/setup/daemon tests, focused race, vet, strict Swift snapshot
decoding, and the Store route transition prove one visible Conversation can move
from DeepSeek to Anthropic by creating a new Segment while preserving the Loom
thread identity. The complete Swift suite passes 175 XCTest cases with one
intentional visual skip and eight Swift Testing contracts. Installed Anthropic
reply, disclosure receipt, full Capsule packing, and Provider-native handle
isolation remain open acceptance gates.

## Installed Segment evidence and response acceptance follow-up

The latest read-only installed observation supersedes the earlier build-22
statement. An external-provenance v0.5.2 build 26 is running without any install
action from this task. Its setup projection reports an unlocked Vault,
`deepseek.primary` verified at revision 6, and the immutable r6 Profile.
Persisted safe metadata proves one visible Loom Conversation contains Codex,
DeepSeek r3, and DeepSeek r6 Segments, so the Segment transition survives
restart and revision change.

The live reply gate is not closed. The latest correlated r6 Attempt reached
`provider_http` and failed `invalid_response`; no successful Provider Attempt is
present. Build 28 adds a bounded response-model compatibility rule and separates
future privacy-safe failures into `response_json`, `response_model`,
`response_choices`, `response_role`, or `response_content`. The observed model
is metadata only and cannot replace the Profile's frozen requested model. The
Provider body and conversation content remain excluded from diagnostics.

Build 27 packages Anthropic Messages but predates this repair. Uninstalled build
28 supersedes it for binary attribution; the complete repository Go suite also
passes on the build 28 source. W2A still requires a user-triggered installed
DeepSeek reply, followed by the installed DeepSeek-to-Anthropic Segment and
reply matrix.

## Exact installed build 28 follow-up

After the Candidate was packaged, an external process installed and restarted
the exact build 28 bytes. This task did not perform the install. Deep signature,
managed daemon argv, automatic Vault unlock, verified DeepSeek revision 6, and
r6 Profile republication all pass after restart. No post-install chat Attempt
has occurred, so the Segment reply gate is still open and no earlier failure is
attributed to build 28.

## Installed build 39 DeepSeek Segment acceptance

The build-28 no-Attempt statement is superseded by read-only evidence from the
currently running v0.5.2 build 39 installed bundle. Exact installed executable
hashes and process start times show that five DeepSeek r6 Attempts occurred
after the current App and bundled daemon started. All five completed
successfully. Every Attempt carries a distinct immutable execution binding and
Context Capsule digest, and its Incident ID correlates with successful App and
daemon `conversation_dispatch` diagnostics.

The same persisted schema-v2 Loom Conversation contains Codex and DeepSeek
Segments and ends in a `summary_only` DeepSeek r6 Segment. This accepts the
installed Codex-to-DeepSeek same-visible-Conversation transition, immutable
Segment boundary, revision-6 route, and repeated real DeepSeek reply portion of
W2A without inspecting message content or Provider responses. Exact build-39
bundle provenance is recorded, but no retained build-39 delivery manifest was
available for Candidate byte comparison.

W2A remains `ACTIVE / PARTIAL`. Installed DeepSeek-to-Anthropic transition and
reply, full Capsule packing and disclosure controls, parallel sibling Attempts
and aggregation, Provider-native handle isolation, encrypted restart storage,
and the complete mixed-Team route matrix remain open.

## Encrypted ordinary Conversation source amendment

The normal production source path now persists each Loom thread as an encrypted
Conversation document rather than rewriting plaintext `chat-threads.json`.
Each document uses the thread's per-Conversation DEK and freezes a monotonic
revision in canonical AAD. Exact retries are idempotent; stale revision,
same-revision drift, cross-Conversation substitution, nonce reuse, tag failure,
and ciphertext corruption fail closed.

Legacy JSON is accepted only as a resumable migration source. All encrypted
thread documents commit before the legacy file is atomically renamed to a
migration-pending identity. Chat does not start until pending plaintext is
identity-checked, overwritten, fsynced, removed, and its parent directory
fsynced. Partial writes resume idempotently; conflicting Vault and legacy state
preserves the source and stops startup. Unsafe directory, owner, permission,
symlink, or hard-link boundaries are rejected.

The currently installed build 39 has not received this source change and still
retains its existing plaintext schema-v2 store. W2A remains `ACTIVE / PARTIAL`
for installed migration/restart/reply, user-visible migration diagnostics,
Provider-native handle encryption, DeepSeek-to-Anthropic live transition, and
the broader Route Segment matrix.

## Build 40 source Candidate boundary

The encrypted ordinary Conversation Store is packaged as uninstalled v0.5.2
build 40. Static arm64, deep-signature, owner-only bundle, no-symlink, and ZIP
byte-equivalence checks pass; exact hashes are frozen in the Candidate
manifest. The Candidate was not launched because its first production startup
could migrate the current build-39 transcript. W2A therefore does not yet claim
installed migration, encrypted restart lookup, or a post-migration Provider
reply.

## Encrypted ExternalSessionHandle source boundary

The Vault now has a source-complete encrypted Store for Provider-native
`response_id`, `conversation_id`, and `prompt_cache_id` values. Canonical AAD
freezes Conversation, Segment, Provider, exact Provider Account, Model, auth
mode, credential reference/revision, handle kind, and monotonic handle
revision. Same-revision exact retries are idempotent; route drift, stale or
same-revision content changes, metadata/ciphertext substitution, tag failure,
or cross transcript/Capsule/handle nonce reuse fail closed.

Brokered and provider-ephemeral handles require the exact account and positive
credential revision. Native-auth handles explicitly require no account,
credential reference, or revision, so Codex state does not acquire a fictitious
Provider Account. Restart, VMK rewrap, Conversation-local crypto-erasure, and
daemon callback-lease zeroization are covered. Current DeepSeek, Anthropic,
Kimi, and MiniMax clients remain stateless and do not manufacture a reusable
handle from ordinary response metadata.

This is post-build-40 source only and is not packaged or installed. W2A still
requires a Provider adapter that explicitly advertises native state, installed
restart/isolation proof, DeepSeek-to-Anthropic live transition, and the broader
Route Segment matrix.

## Migration availability and governance isolation amendment

Build 41 packages the encrypted handle source but predates this amendment. The
encrypted Conversation startup path now correlates migration read, encrypted
commit, and plaintext cleanup with one Incident ID. Any failure preserves the
legacy source and leaves the daemon's governance surfaces online; the chat API
alone returns a non-replyable thread with a bounded migration availability
failure. It never selects plaintext or encrypted state silently.

Swift accepts only `state_unavailable` at `migration_read`,
`migration_commit`, or `migration_cleanup`, with a safe Incident ID and
`can_reply == false`. The Store presents the exact stage and recovery action
without discarding a user draft or offering a misleading send retry. This is
post-build-41 source only. Installed migration, encrypted restart lookup,
post-migration reply, and recovery observation remain W2A live gates.

## Build 42 Candidate boundary

The migration availability source is packaged in unlaunched, uninstalled build
42 with passing source and static transport gates. Installed build 39 remains
unchanged. W2A still requires explicitly approved migration, encrypted restart
lookup, recovery observation, and a real post-migration reply.

## Ordinary Conversation disclosure receipt projection

`CURRENT / SOURCE VERIFIED`: ordinary Conversation Segments now freeze and
project a non-secret disclosure receipt digest plus disclosed and omitted item
counts. The same tuple is frozen on each dispatch Attempt and passed to the
Provider responder request. Execution binding schema version 2 binds the tuple
beside Segment, Profile, context mode, and Context Capsule digest.

Persisted validation recomputes the v2 binding for records that contain a
receipt, so receipt or count drift with an unchanged binding fails closed.
Legacy records remain compatible only with an absent receipt and zero counts;
their pre-v2 binding is preserved. Schema-1 migration creates a current receipt
and v2 binding when it synthesizes the immutable Segment.

Swift strictly decodes the optional tuple, rejects contradictory combinations,
and renders a native expandable summary on the first visible turn of each
Segment: shared count, omitted count, and the selectable receipt digest. It
does not render Prompt text, transcript content, Provider body, credential,
ciphertext, nonce, or hidden reasoning.

Focused persistence/drift tests, 20-run stability, complete serialized Go,
API/daemon race, vet, diff checks, and the complete Swift suite pass. Exact
evidence is in `../P2D-W2A-conversation-disclosure-receipt-projection.md`.
Build 44 does not contain this source. It is packaged as unlaunched,
uninstalled build 45 with passing release, arm64, deep-signature,
owner-only/no-symlink, wire-contract, and ZIP byte-equivalence gates. Installed
build 39 remains unchanged.

W2A remains `ACTIVE / PARTIAL` for full deterministic packing, scoped
retrieval, trust-domain approval, installed encrypted migration/restart,
DeepSeek-to-Anthropic reply, sibling Attempts and aggregation, and the broader
mixed-Team route matrix.

## Provider Account policy binding v3 amendment

Every daemon-created Conversation Segment and Attempt now freezes the exact
daemon-resolved Provider Account disclosure policy identity and values in an
execution-binding schema v3. The binding includes Provider, Provider Account,
policy version/revision/digest, trust domain, retention mode, and data region.
Segment and Attempt must carry the same binding, and the binding participates in
the immutable Segment digest.

Route and same-route policy transitions require the App to submit the exact
non-secret binding reviewed by the user as an expected binding. The daemon
re-resolves authority immediately before mutation and rejects any mismatch
before message persistence, Attempt creation, credential lease acquisition, or
Provider dispatch. A stale confirmation cannot silently adopt a newer account
policy.

Legacy schema-2 records remain byte-valid and are never rewritten. Their next
turn creates a new schema-3 Segment with `continue_with_context`. Swift preserves
one visible Conversation, the draft, selected target Profile, and Incident ID
after a route conflict; Retry returns to explicit Segment/context review.

Exact source evidence is
`../P2D-W2A-W2D-conversation-policy-binding-v3.md`. Installed route transitions
and the broader live matrix remain open.

## Structured ordinary Conversation Capsule v1 amendment

`CURRENT / SOURCE VERIFIED / POST-BUILD-64`: ordinary non-Agent Conversation
dispatch now builds the same structured, content-addressed Role Context Capsule
used by Team execution. User turns remain authoritative; prior visible model
output is always untrusted. `summary_only` represents that output as an explicit
content-free `policy_filtered` omission, `continue_with_context` preserves its
untrusted classification, and `start_clean` carries only the current user turn.

The daemon resolves the exact Harness ContextAdapter, Provider Account, Model,
auth mode, and disclosure-policy identity from the immutable Conversation
Profile. It encrypts the Capsule through the Conversation DEK Store before
thread metadata is committed. Capsule failure leaves no message, Segment,
Attempt, or Provider call; later thread-persistence failure performs an exact
Authority-bound compensation delete. Restart recovery and raw database scans
prove the structured Capsule survives without plaintext persistence.

Exact evidence is
`../P2D-W2A-W2C-structured-conversation-capsule-v1.md`. This closes the
source-level structured ordinary Capsule, encrypted persistence, compensation,
and restart subset. W2A remains `ACTIVE / PARTIAL` for model-specific tokenizer
accounting and ContextAdapter expansion, scoped retrieval, complete receipt and
trust-domain approval UI, sibling/Aggregation Attempts, encrypted export, and
installed real-Provider route/restart acceptance. Build 64 predates this source;
installed Loom remains build 39.

## Parallel RouteSet execution authority amendment

`CURRENT / SOURCE VERIFIED / POST-BUILD-64`: the shared Team execution core now
represents one stable Agent's parallel Provider evaluation as two or more
explicit `route_sibling` nodes plus exactly one `aggregation` node in a
versioned route group. Every sibling remains one independent Attempt with one
frozen Execution Binding. The aggregation node depends on the complete sibling
set and cannot become ready after only a partial or failed set.

Execution-plan canonicalization preserves historical bytes for ordinary nodes
while binding kind and route group for explicit parallel nodes. Duplicate Agent
identity is legal only inside a valid group; direct consumers of sibling output,
missing/extra sources, role/Agent drift, and group substitution fail closed. The
nine-Agent product limit still counts unique stable Agents; bounded physical
route and aggregation nodes use a separate limit.

The Aggregation Attempt reads only exact succeeded Attempt Evidence. It admits
authorized output events and exact Evidence/summary digests, never private
runtime frames or scratchpad. Source authority is authoritative metadata;
model output is always `untrusted_model_output` with provenance in a new Role
Context Capsule. Each sibling and aggregation Run retains separate Provider
Account accounting.

Exact evidence is
`../P2D-W2A-W2C-parallel-route-aggregation-v1.md`. This closes the generic
source execution-authority slice, not RouteSet authoring in the product builder,
ordinary Conversation parallel UX, installed execution, or live Provider
acceptance. P2D-W2A remains `ACTIVE / PARTIAL`.

## Accepted conversation runtime UX amendment

`TARGET / ACCEPTED / NOT YET IMPLEMENTED`: P2D-W2A adds a governed runtime UX
for Context Meter, Fork/Segment, compaction disclosure, and input while an Agent
is running. This is part of the existing Phase 2D Goal and does not create a new
Goal or reinterpret the current RouteSet aggregation evidence.

### Context Meter

The conversation shell must project a non-secret Context Meter from
authoritative runtime facts. It shows the selected Provider Account and Model,
the known context-window capacity, estimated admitted tokens, reserved output
capacity, and the relative contribution of system policy, tool schemas,
Conversation/Capsule context, and the current turn. If the route cannot provide
reliable capacity or token accounting, the meter must say `Unavailable` or
`Estimated`; it must not invent precision or expose Prompt content.

The meter is advisory and cannot authorize dispatch. Admission continues to use
the daemon's exact frozen Execution Binding, Context Capsule digest, disclosure
policy, and deterministic budget packing. A client-side estimate that differs
from authority must fail closed at dispatch with a governable Incident.

### Fork and Segment continuity

Fork keeps one visible Loom Conversation lineage while creating a new immutable
branch Segment. The source Segment, Attempt, Execution Binding, Context Capsule,
disclosure receipt, and stable boundary are frozen; the source is never mutated
and Provider-native session handles are never shared across the fork.

A fork is admitted only at a balanced boundary with no open Turn, Step, or
ToolCall. Forking from an interrupted boundary requires an explicit recovery
decision. The new Segment selects `continue_with_context`, `summary_only`, or
`start_clean`, creates a target-specific Capsule, and records omissions and
provenance before dispatch.

### Compaction disclosure

Compaction is an explicit, recoverable transaction rather than a silent message
rewrite. Its authority records the source sequence range and digests, target
budget, compaction start/end state, Capsule digest, omitted-context manifest,
and disclosure receipt. The compacted summary is an untrusted candidate and
cannot replace authoritative user constraints, Goal, WorkItem, policy, code,
Journal facts, or Evidence.

The UI displays a collapsed compaction boundary with source turn count, token
estimate, Provider/Model identity, completion state, omissions, and a safe
disclosure view. It does not display hidden reasoning, private scratchpad,
credentials, internal Prompt, or raw Provider response. An interrupted
compaction remains visibly incomplete and cannot be treated as committed.

### Queue, Steer, and Inject input

Input submitted while an Agent is running has an explicit durable mode:

- `Queue` appends a new user Turn after the active Turn reaches a stable end.
- `Steer` targets the nearest admissible next Step in the current Turn.
- `Inject` adds scoped context for a future Step without waking or retargeting
  an Agent by itself.

Every input has a stable ID, exact Conversation/Segment/Agent target, ordering
key, scope, admission result, and consumption acknowledgement. Input must never
be silently dropped, broadcast to the Team, or moved to another Agent. A stale
or closed Steer may become Queue only when that fallback was explicitly
accepted in the input contract; otherwise it fails closed and preserves the
draft for retry. Approval takeover may temporarily replace composer controls,
but the draft and queued inputs remain intact under W2D governance.

Acceptance requires restart and race tests for meter staleness, fork boundary
validation, incomplete compaction, ordered Queue/Steer/Inject delivery,
stale-target rejection, draft preservation, and plaintext-negative Journal,
diagnostic, and projection scans.

### Context Meter minimum vertical slice status

`CURRENT / INSTALLED LIVE / EXTENDED METER OPEN`: ordinary Conversation Capsule
authority now projects its non-secret token budget and admitted token count into
the immutable Segment and each Attempt. Both values are digest-bound in the new
schema-5 execution binding; independent Segment and later-Turn Attempt values
remain separately frozen, and substitution fails closed. Legacy records keep
their prior digest and decode as 0/0.

Swift renders this as `Estimated shared context` beside the Segment disclosure
receipt. It does not label the Capsule budget as the model's full context
window. Installed build 75 received a real DeepSeek reply with a valid bounded
token projection while preserving 25 Provider, 4 Runtime, and 4 Profile setup
projection. Full Provider/model capacity discovery, reserved-output projection,
source-category contribution, stale-meter dispatch admission, compaction, Fork,
and Queue/Steer/Inject remain `TARGET / NOT YET IMPLEMENTED`.

Evidence: `../P2D-W2A-W2D-installed-model-segment-matrix-v54.md`.

## Product Team Builder RouteSet authoring status

`SOURCE VERIFIED / INSTALLED LIVE OPEN`: the Team Builder now authors a
role-owned versioned RouteSet with primary, additional, and explicit Synthesis
routes. It persists through Team confirmation, Projection rebuild, saved-Team
reopen, Mission preflight, and Mission reconstruction. Each route remains one
exact Execution Profile and one Attempt binding; the visible Agent identity is
stable while physical nodes are explicit siblings plus aggregation.

Parallel and fallback meanings remain mutually exclusive in this slice. Exact
evidence is
`../P2D-W2A-W2B-W2C-product-parallel-routeset-authoring-v1.md`. Ordinary
Conversation parallel UX and installed/live acceptance remain open, so P2D-W2A
stays `ACTIVE / PARTIAL`.

## Composition Kernel dependency amendment

`TARGET / ACCEPTED / NOT YET IMPLEMENTED`: P2D-W2A consumes the accepted
P2D-COMP1/COMP2 composition boundary. One Product scope opens Conversation
scopes; each visible Conversation then owns immutable Segment and Capsule state
without placing transcript or Capsule content in the Capability Context.

Conversation and Segment admission record the active Composition Snapshot
digest. A later snapshot cannot replace a Segment's Provider Account, model,
credential revision, disclosure policy, native handle binding, or Context
Capsule. Scope close releases temporary leases and channels but cannot erase or
reverse authoritative Conversation/Journal facts except through the existing
governed deletion and crypto-erasure contracts.

The `loom-conversation` Bundle provides bounded Conversation/Profile/Segment/
Capsule ports and declares its IPC routes through RouteDescriptors. It cannot
own Vault root access, a global Provider client, or Team/Agent execution
authority. Normative composition contracts are P2D-COMP1 and P2D-COMP2.

## Build 124 installed Conversation checkpoint (2026-08-24)

Status: `INSTALLED CORE PATH VERIFIED / W2A PARTIAL`.

Build 124 projects six executable Conversation Profiles and freezes the exact
OpenCode + `deepseek.primary` revision 2 + `deepseek/deepseek-chat` binding.
The installed App received the exact real reply before and after its owned
daemon restarted, without a Conversation tool event or Keychain helper hot
path. Strict Runtime decoding and account-scoped responder routing fail closed.

Trust-domain confirmation, full-capacity Context Capsule disclosure and the
broader route-transition matrix remain open. Evidence:
`../P2D-W2A-W2D-full-review-remediation-build124.md`.

## Live matrix scope deferral (2026-08-24)

Status: `ACCEPTED / DEFERRED / NO FUTURE PHASE ACTIVATED`.

The installed Codex/OpenAI + Claude/Anthropic + Loom/Kimi + Loom/MiniMax Team
matrix is no longer a Phase 2D completion gate. Existing RouteSet, per-Agent
binding, Role Capsule and narrower mixed-Team implementations remain part of
Phase 2D and must not be removed. A later Phase may reactivate the real
four-Provider matrix without changing this contract's immutable binding rules.

## Build 127 trust, capacity and Route Transition acceptance (2026-08-24)

Status: `ACCEPTED / SOURCE + INSTALLED GOVERNANCE VERIFIED`.

The remaining active W2A cells are closed:

- transition review derives source policy from the immutable source Segment
  binding and requires explicit acknowledgement for changed trust domain,
  retention mode or data region;
- Context Capsule capacity authority distinguishes exact, estimated and
  unavailable data, freezes counter identity and deterministic admitted
  budget, and exposes only aggregate contribution and omission metadata;
- authenticated private-UDS acceptance covers missing/stale expected bindings,
  all three disclosure modes, exact target binding/digest/Incident freeze,
  persistence and privacy-negative checks;
- Codex explicit OpenAI models remain exact through binding and Context target
  resolution while invalid substitutions fail closed.

The installed Build 127 sheet verifies the acknowledgement gate and all three
context modes. App/daemon restart restores the authoritative Conversation
projection. Existing legacy Segments use the compatibility projection and say
capacity is unavailable rather than inventing a model limit.

Evidence: `../P2D-W2A-W2D-trust-capacity-route-build127.md`.

## Post-Build-127 governance hardening (2026-08-24)

Status: `SOURCE VERIFIED / INSTALL PENDING`.

Parallel review tightened the accepted boundary: stale dispatch conflicts may
not auto-retry around explicit review; missing immutable source authority is an
acknowledged unavailable boundary rather than mutable Profile fallback; model
and reasoning changes use the same reviewed target-binding transition. Capacity
tokenization now follows policy/scope admission, and Pi freezes unavailable
capacity authority instead of emitting a legacy capacity-free Capsule.

Evidence:
`../P2D-W2A-W2D-trust-capacity-route-hardening-post-build127.md`.

## Complete capacity and all-Segment review closure (2026-08-25)

Status: `SOURCE VERIFIED / INSTALL PENDING`.

Parallel review found that dispatch-safe Capsule rebuilding and two production
callers could still reconstruct a legacy capacity-free Capsule. The source now
preserves the exact Capacity Projection during safe rebuild, requires the
matching TokenCounter identity, and removes capacity-free fallback from
controlled Mission, Conversation and Team role/dependency/aggregation paths.
Conversation runtime composition rejects a resolver that cannot supply frozen
capacity authority. Unknown model capacity remains explicitly `unavailable`;
it is never treated as unlimited.

Route Transition review now applies to every existing-thread new Segment with
binding authority, even when trust domain, retention and data region do not
change. Model, reasoning, Provider, Provider Account and credential revision
changes all require a canonical v3 acknowledgement. Swift freezes that exact
target review and the daemon recomputes it while holding Conversation authority
before any mutation. Missing, stale or tampered review fails with zero message,
Segment, Attempt, Capsule and responder mutation.

The ADR-0022 acceptance now combines same-visible-Conversation Codex-to-Loom-
Native Segment transition, Gateway Session lifecycle, immutable binding and
capacity authority. Source verification is recorded in
`../P2D-W2A-W2D-trust-capacity-route-hardening-post-build127.md`; Build 127
remains the installed predecessor.

## Second parallel governance revalidation (2026-08-24)

Status: `SOURCE VERIFIED / INSTALL PENDING`.

The accepted review authority is now version 2 and binds target reasoning
effort as well as the exact Execution Binding, disclosure mode and changed
trust dimensions. Confirmation consumes its App-side generation immediately;
the daemon recomputes the same canonical digest while holding Conversation
authority before any Segment, Attempt, Capsule or responder mutation. Persisted
v1 review digests remain readable only through stored-thread compatibility;
they are not accepted as authority for a new request.

Context capacity now revalidates TokenCounter identity after tokenization,
applies integer-safe cumulative bounds across authority, contributions and
fixed extension omissions, and compares the complete canonical Capacity
Projection between Segment and Attempt. The private-UDS matrix additionally
rejects reasoning added after trust review and independently covers credential
revision, trust domain, retention and region transitions with zero mutation on
failure.

Evidence:
`../P2D-W2A-W2D-trust-capacity-route-hardening-post-build127.md`.
