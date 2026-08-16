# P2D-W2C Contract: Agent Attempt binding and dispatch

Status: `ACTIVE / PARTIAL`

Version line: `v0.5.x`

Parent Goal: `Phase 2D - Multi-Provider, Multi-Model Agent Team Orchestration`

Authority: ADR-0019, P2D-W2A, P2D-W2B, and Product Owner confirmations on
2026-08-09 through 2026-08-11.

## Objective

Every Agent Attempt must execute one immutable, role-local Execution Binding.
No dispatcher, retry, recovery, fallback, verifier, or adapter may resolve a
Team-global Provider client or silently change Runtime, Provider Account,
credential revision, Model, limits, capabilities, or admitted context.

## CURRENT execution and Role Capsule boundary

- Team roles resolve independent Runtime Profiles and Runtime instances.
- Each primary, retry, recovery, fallback, and Verifier Attempt freezes a
  validated `FrozenExecutionBinding` containing legacy Harness identity,
  Runtime, Provider,
  Provider Account, Model, endpoint fingerprint, credential reference/revision,
  reasoning effort, timeout, budget, capabilities, and binding digest.
- Work Authority persists the exact binding in authoritative Attempt facts.
  Replay and Projection validate it; ordinary retry rejects silent drift.
- Supervisor freezes Profile and Runtime again at the final dispatch boundary,
  requires the digest to match the authoritative Run, and passes the exact
  binding to the selected adapter.
- Credential access resolves the full Provider, Provider Account, opaque
  reference, and revision tuple through a short-lived Vault lease. It never
  resolves by Provider ID alone.
- Provider, credential, Runtime, and capacity failure remains role-local in
  preflight and Attempt state. A required blocked role prevents Team Start but
  does not relabel ready peer Agents as offline or unavailable.
- `internal/contextcapsule` now builds a versioned, content-addressed Role
  Context Capsule from structured items. Every item freezes trust, scope,
  priority, token count, provenance reference, and content digest. Input order
  does not affect the Capsule digest.
- The builder enforces `conversation_shared`, `team_shared`, `agent_private`,
  `role_restricted`, `artifact_scoped`, and `secret_reference_only`. Denied or
  budget-exceeded items enter a content-free omission manifest; a required item
  that cannot be admitted fails closed.
- Model-output provenance is always `untrusted_model_output` and cannot be
  promoted to authority. A secret-reference item may contain only an opaque
  reference and no secret body.
- Capsule and disclosure-receipt digests bind Agent, role, Provider, Provider
  Account, Model, auth mode, Context Adapter, Disclosure Policy, token budget,
  disclosed count, and omission count. Work Authority accepts only a validated
  Capsule value object whose target matches the exact Execution Binding.
- Team Attempt facts, replay, Projection, API Board rows, and the Swift Board
  model preserve only the safe Authority record. Journal and Board payloads do
  not contain Capsule content. Ordinary retry retains the Capsule digest;
  explicit Provider fallback creates a new target-bound Capsule.
- The built-in Mission compiler no longer sends the raw Mission objective as a
  parallel dispatch authority. A source-neutral Context Adapter renders only
  the Capsule's admitted text, explicit trust/provenance labels, and
  content-free omission metadata into a canonical v2 dispatch payload. The
  payload binds both Capsule and disclosure-receipt digests. Team coordinator
  recomputes the exact bytes before Journal mutation, and Pi, Loom Native,
  Codex, and Claude Code adapters share the strict v2 decoder while retaining
  the legacy v1 prompt only for explicit non-Team compatibility paths.
- The built-in Mission source now assembles role-local authoritative items for
  the confirmed Mission objective, exact WorkPackage policy, current task and
  plan identity, role governance, and each exact Artifact revision binding.
  Artifact items are optional under deterministic packing so every omission is
  explicit; required policy, task, and governance items fail closed.
- Ordinary retry reuses the exact admitted Capsule and disclosure receipt. The
  attempt number remains an authoritative Attempt fact outside Capsule content,
  preventing an unapproved retry from changing context. Only an approved
  fallback with a changed Execution Binding receives a new target-bound Capsule
  containing the safe approval ID, version, time, approval digest, and source
  and target binding digests. Approval actor and credential identity are not
  disclosed to the model.

## TARGET remaining Role Context Capsule boundary

The minimum source authority chain is now implemented. Completion still
requires:

1. Extend the current Mission objective, WorkPackage, task, role-governance,
   exact Artifact bindings, and observed managed-source baseline with the
   authoritative Goal, confirmed constraints, accepted decisions, observed
   test state, and policy-filtered prior outputs.
2. Model-specific deterministic tokenization and richer ContextAdapter wire
   output. The current source-neutral v1 renderer produces one canonical
   trust-labelled prompt and each concrete adapter maps that prompt to its
   Provider/Harness request. It does not yet perform model-tokenizer accounting,
   multi-message/tool-result mapping, or Provider-specific retrieval formats.
3. Encrypted, content-addressed Capsule body storage and restart lookup. Journal
   remains metadata-only and cannot reconstruct or authorize Capsule content.
4. Versioned disclosure approval for trust-domain or retention-policy changes,
   plus user-visible receipt inspection rather than only safe Board summary.
5. Parallel sibling Attempts, aggregation Capsule inputs, scoped retrieval, and
   the complete four-role Role Capsule matrix under real dispatch.
6. Transcript, Capsule body, and ExternalSessionHandle use the domain-separated
   encrypted local-state boundary required by P2D-W2D before Phase exit.

## Acceptance

1. The four-role Saved Team product path materializes, projects, compiles, and
   dispatches independent Execution Bindings without reducing to the main
   Agent.
2. Four controlled Attempts produce four distinct binding digests and retain
   exact Provider Account, credential reference/revision, Model, limits, and
   capabilities through replay and Projection.
3. Revoking one account blocks only its role in preflight; ready peers remain
   ready. One Provider failure leaves peer Attempts executable and auditable.
4. Missing, malformed, stale, or mismatched execution binding fails before
   adapter process or Provider request.
5. A concrete Role Capsule implementation passes scope isolation, deterministic
   packing/omission, untrusted-output, restart, fallback, and disclosure-receipt
   tests before this WorkItem may be marked complete.
6. Installed acceptance runs Codex/OpenAI, Claude Code/Anthropic, Loom/Kimi,
   and Loom/MiniMax with exact Vault leases; one revoked/rate-limited/timed-out
   account affects only its Agent.

The current source satisfies acceptance items 1-4 and the source-neutral
scope/trust/packing/replay/canonical-dispatch, WorkPackage, role-governance,
Artifact-binding, retry, and approved-fallback subset of item 5. Encrypted
restart, complete Goal/constraint/decision/test-state assembly, model-specific
ContextAdapter expansion, installed mixed-Team, and live Provider portions of
items 5-6 remain Phase 2D exit gates. P2D-W2C therefore remains
`ACTIVE / PARTIAL`.

## Authenticated Agent Inbox product ingress v6 status

`SOURCE VERIFIED / INSTALLED LIVE OPEN`: the private `agent_input` UDS route
now admits Queue, Steer, and Inject against the exact active public Segment,
Agent, WorkItem, Run, and claim generation. The daemon re-resolves the shared
mission-scoped active-Attempt registry and creates every internal input,
payload, ordering, Turn, Step, Execution Binding, and Capsule field itself.
Request ID is the Incident and idempotency anchor; replay with identical bytes
is idempotent while content, identity, mode, or generation substitution fails
closed. Plaintext request bytes are cleared on every return path.

The same Attempt Loop, Inbox coordinator, and active registry are now shared by
the mission executor and IPC route rather than reconstructed per executor.
Swift sends a strict base64 content wire, validates a content-free receipt, and
exposes Agent-local Queue/Steer/Inject controls only when Board and timeline
authority agree. Exact evidence is
`../P2D-W2C-W2D-authenticated-agent-input-ipc-swift-v6.md`.

## Agent input pre-model crash recovery v7 status

`SOURCE VERIFIED / BOUNDED PRE-MODEL WINDOW`: consumed Inbox authority may now
re-release the exact encrypted payload only when its frozen Step has no
ModelRequest, or its Queue Turn has no Step/only the exact first Step with no
ModelRequest. Product recovery binds the prior output checkpoint, recomputes
the model-input digest, restores the deterministic cursor and admits one model
request. Wrong checkpoints and post-ModelRequest recovery fail closed.

This closes status-write and pre-model delivery loss, not general Provider
exactly-once semantics. Daemon restart checkpoint reconstruction and uncertain
post-ModelRequest recovery remain open. Exact evidence is
`../P2D-W2C-W2D-agent-input-pre-model-crash-recovery-v7.md`.

Crash-mid-transition recovery, Pi/Codex/Claude input consumption, installed
CV6, mixed-Team ATL9, and COMP2-E remain open. P2D-W2C remains
`ACTIVE / PARTIAL`.

## Managed source workspace observation amendment

The built-in Mission compiler now emits one path-free
`managed_source_baseline` observation per compile and binds its tree digest to
each primary and verifier execution. It is an `observed`, `team_shared`
workspace item, not an authoritative Goal or decision. It contains only the
tree digest, entry/file/directory counts, and total bytes; `.git`, paths, names,
and contents are not disclosed.

Ordinary retry keeps the same Capsule digest. Source drift before adapter
dispatch fails with `source_changed`. Exact Artifact revisions remain separate
authority-bound items. Evidence is
`../P2D-W2C-managed-source-workspace-snapshot.md`. Missing authoritative Goal,
confirmed-constraint, accepted-decision, and observed-test-state sources remain
open rather than being synthesized.

This source is frozen in unlaunched, uninstalled v0.5.2 build 47 with passing
complete Go, affected race, vet, Swift, release, signature, permission,
no-symlink, contract-string, and ZIP byte-equivalence gates. Installed
mixed-Team and real Provider acceptance remain open.

## CURRENT governance projection closure

Mission compilation now projects the same immutable primary and fallback
Execution Profile facts that dispatch freezes: Harness, Provider, Provider
Account, Model, auth mode, credential revision, reasoning effort, timeout,
budget, capabilities, readiness, and explicit fallback approval. The daemon
validates those tuples before publishing preflight; Swift validates them again
before presentation. This prevents a brokered credential failure from being
misrepresented as native auth or a partial fallback from appearing runnable.

The UI exposes the complete non-secret route and limit posture on the exact
Agent row. It does not add a Team-global Provider, resolve by Provider ID alone,
or disclose credential reference, endpoint fingerprint, secret, Prompt, or
Provider response. P2D-W2C remains `ACTIVE / PARTIAL` for the installed and
Context Capsule exit gates above.

The complete Swift suite, focused cross-language probe, affected Go race,
repository vet, and clean serialized repository Go suite pass for this source
closure.

## Encrypted Team Context Capsule Store amendment

Target item 3 now has a source implementation for Team Mission dispatch. The
Credential Vault database contains a domain-separated encrypted Capsule Store:
one random DEK per Conversation, VMK-derived
`loom/conversation-wrap/v1` wrapping, one AES-256-GCM data nonce per Capsule,
and canonical AAD bound to the validated Authority record, Conversation,
Capsule digest, disclosure receipt, schema, and cipher version. Unique database
indexes fail closed on wrap-nonce or per-Conversation data-nonce reuse.

Exact writes are idempotent. Restart lookup requires the same Conversation and
Capsule digest, revalidates both Authority and canonical dispatch payload, and
rejects metadata, identity, AAD, tag, or ciphertext substitution. Deleting a
Conversation removes its wrapped DEK and cascades only its Capsule ciphertext;
peer Conversations remain readable. VMK rotation rewraps Conversation and
Credential DEKs inside the same Vault transaction.

The built-in Mission compiler persists each primary/fallback Attempt Capsule
before creating the dispatch frame. Locked, recovering, unavailable, or
tampered storage aborts compilation with no plaintext fallback. The Store is
observational content storage, not Event Journal authority and not execution
authorization.

Focused RED/GREEN, complete affected packages, targeted race, repository vet,
and the serialized repository Go suite pass. The full suite reported every Go
package `ok`; its post-test shell marker alone failed because `status` is a zsh
reserved variable. W2C remains `ACTIVE / PARTIAL` for ordinary-conversation
transcript/route-Capsule encryption, Provider-native handle storage, encrypted
context export/restore, installed restart evidence, complete context assembly,
and the mixed-Team live matrix.

## Shared Conversation nonce and transcript boundary

The Conversation DEK storage boundary now covers both Team Context Capsule
payloads and ordinary thread documents. A single nonce-reservation table spans
both ciphertext classes, preventing each table from independently accepting
the same AES-GCM nonce under one Conversation DEK. Conversation-local crypto-
erasure cascades Capsule, transcript document, and nonce history while leaving
peer Agent/Conversation state intact.

This does not change Attempt authority or execution binding identity. Agent
dispatch still freezes its Capsule digest and exact per-Agent execution binding;
the encrypted Store remains content persistence only. Installed mixed-Team
dispatch and encrypted restart evidence remain open.

## Shared structured Conversation Capsule amendment

`CURRENT / SOURCE VERIFIED / POST-BUILD-64`: ordinary Conversation dispatch now
shares W2C's structured Role Context Capsule model, canonical target binding,
trust classes, scope labels, deterministic omissions, and encrypted
Conversation-DEK Store. Prior model output cannot become authoritative context;
`summary_only` records a policy-filtered omission instead of silently dropping
or promoting it.

The production Vault composition persists the exact Capsule Authority and
canonical payload before Conversation thread metadata. Failure before dispatch
rolls back the new thread state and performs an exact Authority-bound ciphertext
delete when needed. A forged Conversation, digest, receipt, or Authority cannot
delete another Capsule. LocalKeyFile close/reopen tests recover the same Capsule
while database scans find no plaintext context.

Exact evidence is
`../P2D-W2A-W2C-structured-conversation-capsule-v1.md`. This closes the
ordinary-Conversation structured Capsule encryption and source-restart subset
of W2C acceptance item 5. W2C remains `ACTIVE / PARTIAL` for model-specific
tokenizer and Provider wire adapters, scoped retrieval, parallel sibling and
Aggregation Attempts, complete context-source assembly, encrypted export, and
installed mixed-Team/live-Provider evidence. Build 64 does not contain this
source; installed Loom remains build 39.

## Build 40 packaging boundary

The shared Conversation nonce registry and encrypted transcript boundary are
included in uninstalled v0.5.2 build 40. Packaging and static transport checks
pass, but no Candidate process was started and no installed Conversation or
Agent Attempt was migrated. Attempt authority remains unchanged; installed
Capsule/transcript restart and mixed-Team dispatch evidence remain open.

## Scoped Context retrieval broker amendment

`CURRENT / SOURCE VERIFIED / POST-BUILD-64`: token-budget omissions may now be
persisted as an encrypted supplement to the structured Capsule and retrieved
only through an exact Attempt-bound daemon broker. Eligibility is closed to
`budget_exceeded`; policy-filtered, access-denied, credential, hidden-reasoning,
and Provider-body material is never retrievable.

The broker freezes Capsule Authority, WorkItem, Run, claim generation, Runtime,
execution-binding digest, Agent, Role, item/content digest, and artifact scope.
It emits a content-free operational audit before disclosing a mutable result and
fails closed on audit, scope, identity, digest, Vault, or revision failure.
Envelope v2 is encrypted under the existing Conversation DEK, supports exact
v1-to-v2 upgrade, and rejects downgrade.

The Team coordinator and Supervisor now carry and validate immutable Capsule
Authority independently of whether a retrieval capability exists. Production
Vault composition supplies store and auditor as a pair and receives a scoped
broker. Explicit legacy/test runtimes receive no retriever and therefore cannot
read omitted content.

Exact evidence is `../P2D-W2C-scoped-context-retrieval-broker-v1.md`. This
closes the daemon-side encrypted retrieval and Attempt broker subset. It does
not close the Harness/model tool protocol, Provider-specific context adapters,
parallel Aggregation Attempts, encrypted export, installed restart, or CV6 live
matrix. P2D-W2C remains `ACTIVE / PARTIAL`.

The same source now adds one bounded Loom Native ContextAdapter result wire.
DeepSeek, Kimi, and MiniMax publish `loom_read_context` only when the daemon
injects a scoped retriever, accept one strict item/digest/optional-artifact
proposal, and return the validated result only as a Provider-native tool message
for one second round. A second tool request, duplicate/unknown JSON field,
retrieved-body digest drift, trust/scope/source drift, secret marker, denial, or
missing capability fails closed. Usage across both Provider rounds is summed on
the same Attempt.

The permission is now an explicit `context_retrieval` Runtime capability, not a
daemon-global switch. Loom Native discovery publishes it, new Provider-account
Profiles require it, and each Attempt freezes it in the binding digest. The
daemon creates a retriever only from that frozen capability and the Adapter
checks the same fact again before any credential lease. An exact historical
empty-capability Runtime may receive one append-only, idempotent discovery
upgrade; arbitrary capability drift is rejected and old Team bindings are not
rewritten.

This does not activate a general multi-round Tool Loop or persistent result
queue, and it does not cover Pi, Codex, or Claude Code transports. Those ATL
gates and installed live acceptance remain open.

## Pi Context retrieval transport amendment

`CURRENT / SOURCE VERIFIED / POST-BUILD-64`: the exact locked Pi 0.82.1
Harness now consumes the existing scoped broker through one private,
Attempt-owned extension and one-use UDS. Capability, Capsule Authority, frozen
execution identity, exact child PID, item/content digest, artifact scope, and
trust/source classification are all revalidated. One first-turn read and one
second-turn final response are permitted; a second tool or any drift fails
closed. Retrieved content is not emitted as a Bridge frame, Journal fact,
Evidence payload, or diagnostic body.

The Runtime probe publishes `context_retrieval` only after an explicit
conformance succeeds for exact Pi 0.82.1. The production runner offers that
conformance only when its executable bytes match the locked digest and all
filesystem bindings remain valid. Version-only, ordinary, historical,
typed-nil, and failed-conformance probes receive no capability.

Exact evidence is `../P2D-W2C-pi-context-retrieval-transport-v1.md`. This closes
the bounded Pi source transport subset. Codex/Claude Code Attempt-scoped MCP,
persistent result/crash resume, general multi-tool rounds, parallel aggregation,
encrypted export, and installed CV6 remain open. P2D-W2C stays
`ACTIVE / PARTIAL`.

## Encrypted Attempt Payload Store amendment

`CURRENT / SOURCE VERIFIED / POST-BUILD-64`: the Vault now contains an
encrypted Attempt Payload Store that freezes Conversation, WorkItem, Run,
claim generation, Runtime, execution-binding digest, Capsule digest, call,
sequence, content digest, and authenticated `pending`/`delivered` state in
canonical AAD. Payloads use the existing Conversation DEK and shared nonce
registry. Status transition re-encrypts under a fresh nonce, restart restores
only the exact pending scope, stale generation/binding fails closed, rotation
preserves access, and Conversation deletion crypto-erases only its rows.

Exact evidence is `../P2D-W2C-attempt-payload-store-v1.md`. This closes the
encrypted payload-storage prerequisite only. The Store is not yet connected to
Journal tool-result facts or a common transport delivery acknowledgement, so
no Harness crash-resume claim is made. General multi-tool loops, parallel
Aggregation Attempts, encrypted export, installed CV6, and live mixed-Team
acceptance remain open under P2D-W2C `ACTIVE / PARTIAL`.

## Journal-backed Attempt Payload delivery amendment

`CURRENT / SOURCE VERIFIED / POST-BUILD-64`: the encrypted Attempt Payload
Store, scoped Retriever, Work Authority, daemon composition, and four execution
transports now share one delivery coordinator. The coordinator freezes exact
Conversation, WorkItem, Run, claim ID/generation, Runtime, Agent,
execution-binding digest, Capsule digest, semantic call, sequence, content
type/digest, and Incident ID.

The result is encrypted as `pending` before the content-free
`ToolResultAccepted` Journal fact. Restart reuses that exact payload without
calling the Retriever again. `ToolResultDelivered` requires one of two closed
proofs: a valid Provider continuation for Loom Native, or a validated Harness
final output for Pi, Codex, or Claude Code. HTTP/UDS write, MCP response, and Pi
`tool_execution_end` are not consumption proofs. Journal delivery precedes the
authenticated Vault `delivered` transition so restart can repair the latter.

Provider-native call IDs are transport-local and may change across retry; the
Loom persistence identity is deterministically derived from the authorized
proposal and sequence. A changed binding, generation, Capsule, result digest,
sequence, or proof fails closed. Journal facts never contain result content,
Prompt, credential, Authorization Header, or Provider body.

Exact evidence is
`../P2D-W2C-journal-backed-attempt-payload-delivery-v1.md`. This closes the
bounded single-context-read persist/recover/ack source slice. It guarantees no
side-effecting tool re-execution during recovery and permits at-least-once
redelivery of the same encrypted result until strong acknowledgement. It does
not claim general exactly-once execution, a multi-tool state machine,
terminal-run reconciliation, parallel Aggregation Attempts, installed CV6, or
live mixed-Team acceptance. P2D-W2C remains `ACTIVE / PARTIAL`.

## Terminal Attempt Payload reconciliation amendment

`CURRENT / SOURCE VERIFIED / POST-BUILD-64`: execution runtime startup now
strictly replays every dedicated Attempt Payload fact stream. Only a complete,
valid `ToolResultAccepted -> ToolResultDelivered` lineage whose exact Run,
claim generation, Runtime, Agent, execution binding, Capsule, call, sequence,
content digest, Incident ID, causation and strong proof still match authority
may repair a Vault row from `pending` to `delivered` after the Run is terminal.

Missing rows are accepted only as Conversation crypto-erasure. A malformed
Journal lineage fails closed. A single Vault row read/authentication or commit
failure becomes an account-local `blocked` outcome and does not stop repair of
another Agent's payload. Repaired and blocked outcomes emit privacy-safe
`context_delivery_reconcile` Agent Attempt diagnostics with no result body,
Prompt, credential, nonce, ciphertext or Provider response.

Exact evidence is
`../P2D-W2C-W2D-attempt-payload-terminal-reconciliation-v1.md`. This closes the
bounded Context payload terminal-reconciliation gap only. Accepted-but-unproved
terminal payload expiry, general side-effecting tools, multi-call state
machines, TTL/compaction, parallel Aggregation, installed CV6, and live
mixed-Team acceptance remain open. Phase 2D remains `ACTIVE / PARTIAL`.

## Parallel sibling and Aggregation Attempt amendment

`CURRENT / SOURCE VERIFIED / POST-BUILD-64`: ExecutionPlan, Work Authority,
Journal replay, Projection, Team Board, and the Swift read model now preserve an
explicit node kind and versioned route-group identity. A valid group contains
at least two route siblings for the same stable Agent and role, and exactly one
Aggregation Attempt depending on the entire sibling set. Planning reconstruction
retains this identity across waves and retries.

The coordinator freezes and dispatches each sibling independently. Only exact
succeeded sibling receipts may become aggregation inputs. The Evidence Store
strictly extracts authorized output events from the digest-verified Attempt
artifact and binds the source Attempt, Evidence digest, and output-summary
digest. The resulting Capsule records source authority separately and labels
all model-produced content untrusted. Agent, route-group, digest, source-node,
or plan-node substitution fails closed.

Provider Account accounting remains per sibling Run and per Aggregation Run. A
failed sibling does not rewrite or cancel a healthy sibling, and it does not
silently select another Provider; the Aggregation Attempt remains unready until
its exact dependency contract is satisfied. Board rows distinguish parallel
Provider routes from Synthesis without exposing internal output or authority
digests.

Exact evidence is
`../P2D-W2A-W2C-parallel-route-aggregation-v1.md`. Product RouteSet authoring,
real Harness aggregation, installed CV6, and the live mixed-Team matrix remain
open. P2D-W2C stays `ACTIVE / PARTIAL`.

## Accepted generalized Agent Loop amendment

`ACTIVE / PARTIAL`: the authoritative Turn/Step/ToolCall core and the durable
Queue/Steer/Inject Inbox authority now have source-verified slices. The Inbox
freezes exact Agent, Segment, claim generation, execution/capsule digests,
ordered target and context scope; payload bytes use the Conversation-DEK Vault.
Queue consumption is atomic with `TurnStarted`, and Steer/Inject consumption is
atomic with `StepStarted`. Daemon ingress, Runtime model-input assembly, Swift
controls and installed-live acceptance remain open, so this is not a product
availability claim. Exact Inbox evidence is
`../P2D-W2C-W2D-durable-agent-inbox-v1.md`.

The V2 product-composition slice is also `SOURCE VERIFIED`: the real Loom Vault
Bundle now owns the encrypted Inbox store port, recovery and facade close revoke
that bounded capability, and the mission executor composes one Inbox
coordinator over its existing Attempt Loop authority. This does not expose a
client route or make input model-visible. Active-Attempt resolution, multi-Step
Runtime consumption, plaintext zeroization, authenticated IPC and Swift
controls remain open. Exact evidence is
`../P2D-W2C-W2D-agent-inbox-product-composition-v2.md`.

V3 adds a source-verified, revocable active-Attempt runtime projection. A
governed adapter registers only its already validated Attempt Loop and Frozen
Execution Binding, exact Conversation/Agent/Run/generation lookup is available
only during delegate execution, and every return path revokes the entry. This
is not a second authority and no IPC route exists. The current Capsule authority
still lacks a Route Segment ID, so V3 uses a deterministic Attempt-local Inbox
Segment and keeps authoritative Route Segment propagation plus multi-Step
consumption open. Exact evidence is
`../P2D-W2C-W2D-active-attempt-runtime-projection-v3.md`.

V4 closes that temporary Segment limitation with a separate versioned mapping
contract rather than mutating persisted Context Capsule v1. Every Team Attempt
now journals and replays a `RouteSegmentBinding` over Conversation, Team, Agent,
Role, Attempt number, Capsule digest and Frozen Execution Binding digest. The
same binding crosses the application coordinator and Supervisor, scopes the
active-Attempt registry, and projects to the strict Swift board contract.
Aggregation rebuilds the Segment when its Capsule changes; retry/fallback
Attempts receive distinct bindings. Multi-Step model-input consumption, IPC and
installed-live acceptance remain open. Exact evidence is
`../P2D-W2A-W2C-W2D-authoritative-route-segment-binding-v4.md`.

P2D-W2C generalizes the bounded payload-delivery slice into an authoritative
Turn/Step/ToolCall state machine with explicit tool concurrency and
Queue/Steer/Inject admission. The detailed design input is
[`attempt-tool-loop.md`](../../../docs/architecture/attempt-tool-loop.md); this
amendment remains the complete target beyond the verified slices.

### Authoritative state machine

An Agent Attempt contains ordered Turns. A Turn contains ordered Steps. A Step
contains one model request and zero or more ToolCalls, their normalized results,
and the next-step or terminal decision. The minimum durable authority facts are:

- `TurnStarted` and `TurnEnded`
- `StepStarted` and `StepEnded`
- `ModelRequestAdmitted`
- `ToolCallAdmitted`
- `ToolDispatchCommitted`
- `ToolResultAccepted`
- `ToolResultDelivered`

Every fact binds Attempt, claim generation, Turn, Step, semantic ToolCall,
Execution Binding digest, Context Capsule digest, causation, sequence, and
Incident ID. Journal facts contain only non-secret authority metadata and
digests. Model-visible messages, ToolCall arguments, and tool results live only
in the encrypted payload store. The exact model-visible input must be
reconstructable from authority plus authenticated encrypted payloads without
placing Prompt or content in the Journal.

The tool pipeline is fixed as admission, schema validation, frozen-binding and
capability validation, monotonic policy guards, approval, execution wrapper,
tool invocation, post-policy validation, normalized result persistence, and
delivery acknowledgement. Once `ToolCallAdmitted` is committed, its argument
digest and resource scope cannot be rewritten by an adapter or observer.

### Tool concurrency modes

Each tool definition and frozen Harness capability declares a
`ToolExecutionMode` of `parallel` or `exclusive`, plus its resource/conflict
scope. Parallel calls may overlap only when the tool contract, frozen adapter
capability, policy, budget, and resource scopes all permit it. Exclusive calls
form a barrier and cannot overlap conflicting work. Unknown capability,
ambiguous resource identity, or mismatched mode fails closed.

Concurrency does not weaken ordering or accounting. Every call receives its own
Incident lineage, timeout, budget charge, cancellation state, encrypted result,
and delivery proof. A failed ToolCall blocks only the dependent Step/Agent
unless the explicit plan contract requires broader cancellation.

### Runtime input admission

Queue, Steer, and Inject use a durable Agent inbox rather than transient UI
messages. Queue targets the next Turn; Steer targets the nearest still-open
admissible Step; Inject contributes scoped context without creating execution
authority. Each record binds its stable input ID, exact Agent and Segment,
expected generation, mode, ordering, scope, encrypted payload digest, and
consumption fact.

Generation drift, terminal Attempts, closed Steps, duplicate consumption, and
target substitution fail closed. Crash recovery may redeliver the same
authenticated encrypted payload but may not rerun a side-effecting tool without
an explicit idempotency or non-execution proof. Cancellation, retry, resume, and
new Attempt remain distinct authority transitions.

Acceptance requires transition-table tests, illegal-transition and replay
tests, parallel/exclusive conflict tests, cancellation races, ordered
Queue/Steer/Inject consumption, crash reconstruction, and content-negative
Journal/diagnostic/Evidence scans across Loom Native, Pi, Codex, and Claude
Runtime capability declarations.

## Harness Platform Core and Runtime boundary amendment

`TARGET / ACCEPTED / NOT YET IMPLEMENTED`: the generalized Agent Loop is Loom
Harness Core, not a shared `HarnessAdapter` abstraction. Loom owns Attempt,
Turn, Step, ToolCall, Inbox, context admission, authorization, scheduling,
approval, sandbox, recovery, Journal facts, encrypted payloads, and terminal
authority.

`RuntimeContract` is the stable execution seam. `RuntimeAdapter` implements that
contract for Pi, Codex, Claude Code, or a version-pinned DeepSeek Harness
process. Loom Native implements the contract in daemon without an adapter.
Provider adapters remain a lower model-protocol concern used by Loom Native;
they do not become Runtime or Harness authority.

The DeepSeek Harness Core behavior selected for semantic porting is:

- rollback-covered Agent create/resume and exact lifecycle ownership;
- ordered Turn/Step state transitions and one durable completion anchor;
- one Agent Inbox with Queue/followup, Steer, and non-waking Inject;
- bounded rolling parallel ToolCalls with exclusive barriers;
- cooperative cancellation, wake latching, drain, and deterministic recovery;
- Service Definition, Service Provider, and Consumer capability separation.

The reference source is pinned to
`deepseek-ai/deepseek-harness@47f943859bef60e4160492346772ded9b24f765a`.
Loom ports behavior into its Go authority model and conformance tests. It does
not import Cordis, DeepSeek session authority, plaintext Prompt/session logs,
credentials, or runtime self-modification into the trusted Core.

## Product RouteSet compiler status

`SOURCE VERIFIED / INSTALLED LIVE OPEN`: product Team configuration now compiles
the saved role RouteSet into explicit `route_sibling` Attempts and one
`aggregation` Synthesis Attempt using the accepted generic execution authority.
The original logical node is the aggregation node; downstream dependencies do
not bind directly to a Provider sibling. Mission restart reconstruction
reproduces the route group, node kinds, plan digest, executions, and semantics.

A revoked account blocks only the matching route binding at resolution; healthy
sibling routes remain ready and no fallback is inferred. Exact evidence is
`../P2D-W2A-W2B-W2C-product-parallel-routeset-authoring-v1.md`. Real Harness
Synthesis and installed/live dispatch remain open, so P2D-W2C stays
`ACTIVE / PARTIAL`.

## Attempt Loop authority v1 status

`SOURCE VERIFIED / RUNTIME INTEGRATION OPEN`: ATL1/ATL2 now has a strict
append-only Attempt/Turn/Step/ToolCall authority. It freezes the exact Run and
claim generation, Agent, Runtime, Execution Binding, Capsule, permission
profile, capability set, tool schema set, budget policy, and Incident lineage.
The capability digest is re-derived from the Run binding rather than trusted
from the caller.

The stream admits bounded Turn and Step starts, one model request per Step,
ordered ToolCalls, explicit parallel or exclusive execution mode, exact
conflict scopes, dispatch commitment, result-delivery lineage, and terminal
Step/Turn transitions. Result content remains in the encrypted Attempt Payload
Store. The Journal contains only IDs, versions, status, modes, and digests.

The prior bounded payload authority supported only one result per Attempt. Its
sequence-1 stream remains backward compatible; sequence 2 and later now use
call-scoped streams plus an atomic monotonic call index. Competing call IDs for
one sequence, gaps, result/binding substitution, or a result accepted before
the matching dispatch fail closed. Restart, terminal-Run reads, strict replay,
deep-copy, concurrency, race, and content-negative tests pass. Exact evidence
is `../P2D-W2C-attempt-loop-authority-v1.md`.

This is the authority core, not a Runtime activation claim. Daemon Tool Gateway
composition, approval/sandbox checks, Queue/Steer/Inject, diagnostics, Swift UI,
and installed multi-Provider execution remain open. P2D-W2C stays
`ACTIVE / PARTIAL`.

## Product Attempt Loop Runtime Context v1

`SOURCE VERIFIED / CONTEXT TOOL PRODUCTION-INTEGRATED / GENERAL TOOL GATEWAY OPEN`:
the product daemon now wraps every configured Runtime Adapter with the accepted
Attempt Loop authority after `RunStarted`. The wrapper validates the exact
dispatch, claim, Incident, Runtime, Agent, Execution Binding, Provider route,
and Role Context Capsule before invoking the delegate.

For an Attempt with `context_retrieval`, the wrapper rebuilds the encrypted
Context delivery broker using an Attempt-loop-backed result authority. A real
`loom_read_context` call now records `ToolCallAdmitted`,
`ToolDispatchCommitted`, dispatch-causation-bound `ToolResultAccepted`, and
`ToolResultDelivered` before `StepEnded` and `TurnEnded`. Result size is checked
before acceptance. Prompt, disclosed Context content, arguments, credentials,
and Provider bodies remain outside the Journal.

The exact evidence is
`../P2D-W2C-product-attempt-loop-runtime-context-v1.md`. General Tool Broker
composition, one-shot approval, sandbox enforcement reports, interrupted-call
recovery, Queue/Steer/Inject, Swift projection, installed CV6, and live mixed-
Team acceptance remain open. Attempts without a Context Capsule retain their
prior compatibility path and are not claimed by this slice.

## Composition Kernel and Runtime Contract dependency

`SOURCE VERIFIED / COMP1 AND COMP2-A-D / COMP2-E OPEN`: P2D-COMP1 opens one Attempt scope
only after the exact Composition Snapshot and Frozen Execution Binding have
both passed admission. The Attempt scope provides bounded Runtime dispatch,
credential lease, Context retrieval, Tool Gateway, cancellation, inbox, and
diagnostic ports. Turn scopes may narrow these ports and own temporary Effects;
they cannot add tools, accounts, models, policies, or authority.

Attempt/Turn/Step/ToolCall Journal facts continue to belong to Loom Harness
Core. Runtime Bundles implement `RuntimeContract`; they do not own the Loop or
register a second terminal authority. Queue/Steer/Inject, sequential/parallel
ToolCalls, cancellation, replay, and Provider continuation bind the admitted
snapshot digest in addition to existing Agent/Runtime/claim/Execution Binding/
Capsule fences.

P2D-COMP2 wraps the existing Runtime and Attempt Loop path without behavior
change before moving construction into `loom-agent-runtime`. The source-verified
ATL3 remote-result commit remains valid and is not reset; broader ATL work
resumes after the compatibility Bundle and declarative route boundary are
verified.

## Loom Native Agent input consumption v5 status

`SOURCE VERIFIED / LOOM NATIVE ONLY / IPC OPEN`: the durable Inbox now reaches
one real production Runtime. A daemon-owned per-Attempt input source accepts
only a digest of the current model output, resolves the exact frozen Inbox
target internally, and releases ordered mutable plaintext only after the
authoritative Inbox/Step or Inbox/Turn transition succeeds.

Steer and Inject advance the current Turn to the exact next Step. Queue remains
non-waking until the current Turn succeeds, then starts its frozen next Turn.
Loom Native retains one Execution Binding, Route Segment and credential lease
across Provider rounds, combines usage, moves a content-free Context delivery
cursor, and emits one final Bridge terminal. Mutable payload, message-wire and
HTTP request buffers are cleared after use; Journal facts contain only bindings,
digests and consumption status.

Exact evidence is
`../P2D-W2C-W2D-loom-native-agent-input-consumption-v5.md`. Authenticated daemon
ingress, Swift controls, crash-mid-transition recovery, Pi/Codex/Claude
conformance, installed CV6 and mixed-Team ATL9 remain open. P2D-W2C stays
`ACTIVE / PARTIAL`.

## Pi Agent input continuation v8 status

`SOURCE VERIFIED / MANAGED PI-COMPATIBLE CHILD / INSTALLED LIVE OPEN`: Pi now
requests the next authenticated Agent Inbox batch only after a complete strict
assistant checkpoint. Queue/Steer/Inject is rendered into mutable bytes and
sent as another native RPC prompt to the same child process. Process identity,
Execution Binding, Route Segment, Agent Attempt, Bridge sequence and terminal
authority remain unchanged.

Only the first prompt emits the dispatch ACK. Accepted deltas from every prompt
remain ordered, accounting is combined, and one Evidence/Result terminal closes
the Attempt. Input batch, request wire, per-round prompt/state and accounting
snapshots are cleared after use. The strict managed-child fixture proves two
different checkpoints and excludes input content from Bridge and audit.

Later Agent-input prompts are text-only in this slice; a new Context/Tool event
fails closed instead of granting another capability use. Official Pi 0.82.1,
Codex/Claude transports, restart reconstruction, installed CV6 and mixed-Team
ATL9 remain open. Evidence:
`../P2D-W2C-W2D-pi-agent-input-continuation-v8.md`. P2D-W2C remains
`ACTIVE / PARTIAL`.

## Codex/Claude mutable prompt prerequisite v9 status

`SOURCE VERIFIED PREREQUISITE / CONTINUATION OPEN`: Codex and Claude Code now
receive Harness prompt/stdin through owned mutable byte slices that are cleared
on all Adapter, process-runner and system-command return paths. This closes the
immutable-string prerequisite for future Agent Input consumption.

The present Codex `exec --ephemeral` and Claude Code
`--print --no-session-persistence` commands remain one-shot. Neither Adapter
advertises `AgentInputConsumer`; repeated process launches are forbidden as a
substitute for same-Harness continuation. Version-locked persistent protocol
conformance, native session-handle binding, installed acceptance and mixed-Team
ATL9 remain open. Evidence:
`../P2D-W2C-W2D-harness-mutable-prompt-prerequisite-v9.md`. P2D-W2C remains
`ACTIVE / PARTIAL`.

## Daemon restart Attempt reconstruction v10 status

`SOURCE VERIFIED / EXPLICIT RESUME OPEN`: startup reconstructs current Loop and
Inbox authority from strict Journal replay and revalidates the exact current
Run generation, Runtime, Agent, Execution Binding, Capsule, capability and
budget binding. A consumed input before ModelRequest becomes a content-free
`pre_model_resume` checkpoint candidate. An open admitted ModelRequest becomes
`provider_outcome_uncertain` and cannot be automatically dispatched again.

Recovered state is not inserted into the active-Attempt registry and carries no
execution authority. Journal/Run corruption fails closed; one missing Inbox
payload becomes only the affected Agent's `recovery_blocked` result. Explicit
resume commands, persistent Harness reconstruction and installed acceptance
remain open. Evidence:
`../P2D-W2C-W2D-daemon-restart-attempt-reconstruction-v10.md`. P2D-W2C remains
`ACTIVE / PARTIAL`.

The V13 read-only governance projection is also `SOURCE VERIFIED`: the Team
board now accepts the closed `agent_attempt_reconcile` stage and projects a V10
outcome only onto the exact Incident/Provider Account/Model Agent row. Swift
recognizes the same stage and renders controlled recovery labels while keeping
Retry disabled. This adds no active-Attempt registration, resume command,
Runtime dispatch, Provider replay, or native session reattachment. Evidence:
`../P2D-W2C-W2D-agent-attempt-recovery-projection-v13.md`. Explicit recovery
authority remains open and P2D-W2C stays `ACTIVE / PARTIAL`.

The V14 decision-authority prerequisite is `SOURCE VERIFIED`: one explicit
`resume_pre_model` decision binds the exact recovery-candidate digest and a
separate restart-capability digest supplied by a trusted Runtime resolver. The
append CAS freezes recovery, Attempt Loop, and Run heads; an uncertain Provider
outcome, candidate drift, capability substitution, terminalized Run, or second
decision fails closed. The Journal fact is content-free.

V14 issues no dispatch lease, registers no active Attempt, reattaches no native
session, and performs no Runtime/Provider call. Production composition has no
restart-capability resolver yet. Evidence:
`../P2D-W2C-W2D-agent-attempt-recovery-decision-authority-v14.md`. Decision
consumption, Runtime reattachment and UI remain open; P2D-W2C stays
`ACTIVE / PARTIAL`.

The V15 one-use consumption core is also `SOURCE VERIFIED`: the exact V14
decision may atomically append one content-free consumed fact while fencing the
recovery, Attempt Loop and Run heads. The authority generates the lease ID
internally, re-resolves Runtime capability before commit, and returns one
process-local lease whose frozen grant can be taken once. Concurrent or later
consumers receive no lease.

Production has no restart-safe resolver, active-Attempt recovery registration,
native-session reattachment, IPC or Provider dispatch yet. Evidence:
`../P2D-W2C-W2D-agent-attempt-recovery-consumption-v15.md`. P2D-W2C remains
`ACTIVE / PARTIAL`.

## Codex app-server Agent input continuation v11 status

`SOURCE VERIFIED / SAME-PROCESS ONLY`: exact Codex 0.144.1 executable bytes now
gate one persistent app-server process and one ephemeral thread. Each admitted
Loom input starts a new Turn on that thread only after the previous strict final
output checkpoint. Response, thread, Turn and token-usage identities are
validated; accounting spans all rounds. Ordinary requests keep their prior
one-shot `exec --ephemeral` behavior.

The Adapter advertises `AgentInputConsumer` only when both its system session
runner and exact continuation conformance lock pass. Executable drift fails
before credential access. Native-thread persistence, daemon-restart reattach,
installed live and mixed-Team ATL9 remain open. Evidence:
`../P2D-W2C-W2D-codex-app-server-agent-input-v11.md`.

## Claude stream-json Agent input continuation v12 status

`SOURCE VERIFIED / SAME-PROCESS ONLY`: exact Claude Code 2.1.196 executable
bytes now gate one `--no-session-persistence` stream-json process and stable
emitted session ID. Every Loom input is replayed exactly once. Internal
`tool_use`/`tool_result` pairs and audited compaction remain inside the current
round; only the final result becomes a Loom checkpoint. Usage and reported cost
are combined across rounds.

The one-shot JSON path remains unchanged without Agent Inputs. Native-session
persistence, daemon-restart reattach, installed live, CV6 and mixed-Team ATL9
remain open. Evidence:
`../P2D-W2C-W2D-claude-stream-json-agent-input-v12.md`. P2D-W2C remains
`ACTIVE / PARTIAL`.

## Agent Attempt recovery Runtime attachment v16 status

`SOURCE VERIFIED / PRODUCTION REATTACHER OPEN`: a resumable candidate now
freezes the exact Route Segment recovered from its consumed Inbox bindings;
missing or mixed Segment identity blocks only that Attempt. The candidate
digest, authorization metadata and one-use grant bind the Segment, while the
grant also carries the recovery Incident ID.

Trusted composition validates the grant, reattaches a session, verifies exact
Runtime/session identity, and only then registers the recovered active Attempt.
Failure closes the session; normal close revokes the registration before the
session. The reattachment interface has no Provider-dispatch method. Production
still wires no restart-safe resolver/reattacher, authenticated recovery IPC or
Swift action. Evidence:
`../P2D-W2C-W2D-agent-attempt-recovery-runtime-attachment-v16.md`. P2D-W2C
remains `ACTIVE / PARTIAL`.

## Loom Native restart capability v17 status

`SOURCE VERIFIED / PRODUCTION CONTINUATION OPEN`: the built-in Loom Native
adapter now explicitly implements `loom-owned-checkpoint/v1`. A trusted product
resolver mints a restart capability only after revalidating the exact frozen
Execution Binding, Runtime, Provider Account, Model and encrypted Capsule
authority. Its domain-separated session digest also freezes the recovered Route
Segment, previous output checkpoint and input IDs.

V16 can use the resulting one-use grant to attach an identity-only session and
register the active Attempt. Capability resolution and attachment do not read
Capsule content, acquire credentials, call the adapter, or issue Provider HTTP.
The daemon does not yet construct this service or dispatch a resumed Loom Native
continuation. Codex, Claude Code and Pi remain restart-ineligible. Evidence:
`../P2D-W2C-W2D-loom-native-restart-capability-v17.md`. P2D-W2C remains
`ACTIVE / PARTIAL`.

## Encrypted Loom Native continuation v18 status

`SOURCE VERIFIED / PRODUCTION RECOVERY ACTION OPEN`: a dedicated Agent
Checkpoint Store now encrypts the previous Loom Native output under the
Conversation DEK and authenticates the complete frozen route, Attempt position,
Capsule and content digest. It is separate from Tool Result payloads and the
Event Journal. Exact lookup rejects missing or ambiguous candidates, and
Conversation crypto-erasure removes the checkpoint.

Loom Native persists that checkpoint before consuming a new Agent Input. The
trusted continuation path then consumes the one-use recovery grant, registers
the exact active Attempt, restores the exact Capsule/checkpoint/consumed input,
admits a new ModelRequest, acquires the frozen credential lease, dispatches the
Provider, terminalizes Step/Turn and erases the obsolete checkpoint. Any
binding, Segment, Capsule, checkpoint or input substitution fails before
credential access or HTTP.

This path is verified only through local source fixtures. Production daemon
wiring, authenticated recovery IPC, Swift governance and installed live remain
open. Evidence:
`../P2D-W2C-W2D-encrypted-agent-checkpoint-continuation-v18.md`. P2D-W2C remains
`ACTIVE / PARTIAL`.

## Recovery frame and Run terminal closure v19a status

`SOURCE VERIFIED / PRODUCTION LIFECYCLE OPEN`: the resumed Loom Native stream
now passes through a recovery-specific Bridge frame authority. It binds the
one-use recovery grant, exact unrevoked original grant identity and operation
set, recovery Incident ID, deterministic dispatch ACK, Run stream sequence and
terminal Result. The AdapterResult must exactly equal the accepted frame stream
before Step/Turn success or Run terminal authority can advance.

The completion coordinator resolves accounting against the authoritative Run
and frozen Provider/Model Rate Card, commits terminal status, then revokes the
exact original grant without reconstructing its token. Substitution, observer
failure, ambiguous/missing grant and transcript mismatch fail before Run
terminal commit. Evidence:
`../P2D-W2C-W2D-recovery-frame-terminal-closure-v19a.md`.

Production Bundle construction, Team observer restoration, the terminal-to-
revoke crash-window reconciler, authenticated IPC and Swift governance remain
open. P2D-W2C stays `ACTIVE / PARTIAL`.

## Production recovery lifecycle v19b status

`SOURCE VERIFIED / AUTHENTICATED RECOVERY COMMAND OPEN`: the production mission
Bundle now owns the recovery decision authority and completion coordinator when
Agent Inbox recovery is configured. It composes the exact Loom Native restart
resolver, encrypted continuation, active-Attempt attachment, frame authority,
Run accounting/terminal authority and original-grant closure from existing
production stores and adapters.

Recovered frames regain the ordinary Team evidence and NodeOutput observer only
after an exact projection lookup by WorkItem, Run, generation, Runtime and
Agent. Startup closes the Run-terminal-to-grant-revoke crash window by revoking
only one exact stale grant; mismatch or ambiguity fails closed, replay is
idempotent, and the reconciler has no Provider-dispatch capability. Successful
repairs persist a content-free operational audit without becoming board
failures. Evidence:
`../P2D-W2C-W2D-production-recovery-lifecycle-v19b.md`.

No authenticated preview/confirm/resume IPC or Swift action exists yet. V19C,
installed CV6, mixed-Team ATL9 and broader Runtime recovery remain open;
P2D-W2C stays `ACTIVE / PARTIAL`.

## Authenticated recovery IPC and Swift governance v19c status

`SOURCE VERIFIED / INSTALLED LIVE OPEN`: the production recovery lifecycle now
has a typed authenticated private-UDS route with separate read-only Preview,
principal-bound Confirm and one-use Resume operations. Resume enters the V19B
completion coordinator; Preview cannot mint authority and Confirm cannot
dispatch Provider work.

The Swift Store requires a fresh authoritative candidate before confirmation,
requires its exact confirmed decision before Resume, suppresses duplicate
Resume dispatch and invalidates uncertain local authority after failure. Strict
wire decoding rejects unknown or malformed recovery fields. Mission Inspector
shows the affected Agent's Harness, Provider Account, Model and credential
revision and preserves separate Confirm and Resume actions.

App and daemon diagnostics correlate the operation by Incident ID while
excluding candidate/capability digests, principal identity and all content or
secret fields. Evidence:
`../P2D-W2C-W2D-authenticated-recovery-ipc-swift-governance-v19c.md`.

Installed recovery, broader Runtime reattachment, CV6, mixed-Team ATL9,
ATL3-ATL8 and COMP2-E remain open; P2D-W2C stays `ACTIVE / PARTIAL`.

## ToolCall recovery decision governance v20 status

`SOURCE VERIFIED / TRUSTED RESOLVERS AND INSTALLED LIVE OPEN`: a ToolCall left
at `side_effect_unknown` now has a separate content-free recovery authority.
The exact recovery candidate is domain separated and stream-head fenced. One
winner may abort the Attempt, accept a trusted observed effect, or authorize a
trusted replacement Attempt; all three close the original execution and can
never rerun it.

Production currently supplies neither trusted observation nor replacement
resolver, so only Abort is advertised. The typed authenticated UDS route,
strict Swift Store and Mission Inspector consume only the daemon's exact
`available_actions`, require confirmation and invalidate uncertain local state
after failure. Evidence:
`../P2D-W2C-W2D-tool-recovery-decision-governance-v20.md`.

Observed-result delivery, replacement-Attempt startup, installed live, CV6,
mixed-Team ATL9, broader ATL3-ATL8 and COMP2-E remain open. P2D-W2C stays
`ACTIVE / PARTIAL`.

## Loom Native sequential Context tools v21 status

`SOURCE VERIFIED / GENERAL TOOLS AND INSTALLED LIVE OPEN`: Loom Native now
supports up to four strictly sequential `loom_read_context` calls inside one
Provider exchange. Every call retains the exact frozen Attempt, Route Segment,
Execution Binding, Provider Account, credential revision and Model while using
a distinct monotonic ToolCall/payload sequence.

Attempt-loop admission releases the exclusive slot only after the preceding
result is authoritatively `delivered`; an `accepted` result remains blocking.
The Provider adapter acknowledges each result only after a successful Provider
continuation, aggregates usage over all rounds, clears owned mutable tool-result
buffers and stops advertising the tool at the hard bound. Evidence:
`../P2D-W2C-W2D-loom-native-sequential-context-tools-v21.md`.

General Read/Grep/Web/MCP tools, parallel execution, other Runtime transports,
installed live, CV6, ATL9 and COMP2-E remain open. P2D-W2C stays
`ACTIVE / PARTIAL`.

## Codex and Claude bounded multi-Context v22 status

`SOURCE VERIFIED / GENERAL TOOLS AND INSTALLED LIVE OPEN`: the Attempt-scoped
Harness Context MCP now accepts at most four distinct Context proposals. Each
Prepare receives a monotonic sequence and remains pending until validated
Harness final output; a subsequent tool call never substitutes for an explicit
delivery proof.

The final-output boundary seals the service and acknowledges every exact
binding in order with `harness_final_output`. Duplicate, over-budget,
concurrent, post-seal and Prepare/ACK race paths fail closed. Partial batch ACK
retries begin at the first undelivered binding. Product admission uses four
parallel pending slots only for Codex and Claude, while Native/Pi/default retain
exclusive/one. Evidence:
`../P2D-W2C-W2D-codex-claude-bounded-multi-context-v22.md`.

General tools, arbitrary parallel effects, restart reattachment, installed
live, CV6, ATL9 and COMP2-E remain open. P2D-W2C stays `ACTIVE / PARTIAL`.

## Codex and Claude governed Read/Grep v23 status

`SOURCE VERIFIED / READ+GREP ONLY / WEB+MCP+INSTALLED LIVE OPEN`: the shared
Tool Gateway contract is now owned by `internal/runtime`, with Pi compatibility
aliases retained. Codex and Claude route governed Read/Grep through their exact
Attempt-scoped private MCP and the production daemon's existing permission,
execution and Attempt Loop authorities.

Every call is bound to the frozen Attempt, claim generation, Runtime, Agent,
Route Segment, Execution Binding, Capsule and Incident. Results are encrypted
and accepted before Harness completion, then delivered in order only after
validated final output. Exact executable drift, loopback-context authority
substitution, duplicate/over-budget calls and same-path read conflicts fail
closed. Evidence:
`../P2D-W2C-W2D-codex-claude-governed-read-grep-v23.md`.

Bash/Edit through this Harness MCP, Web/MCP, restart reattachment, installed
live, CV6, ATL9 and COMP2-E remain open. P2D-W2C stays `ACTIVE / PARTIAL`.

## Native-tool bypass closure v24 status

`SOURCE VERIFIED / EXACT EXECUTABLE COMPONENT RECHECK OPEN`: Codex and Claude
now pass a per-Attempt frozen Loom tool policy into the credential gateway.
Request catalogs and successful Provider tool-call output must contain only the
exact Context/Read/Grep tools present in the private MCP lease. Native tools,
dynamic tool search, mixed catalogs, unapproved Loom tools and duplicate JSON
keys fail closed before Provider dispatch or Harness delivery.

All four Harness process modes run from the private Attempt temp directory.
Codex is read-only with native shell/exec/apply-patch/tool-search feature paths
disabled; Claude uses `dontAsk`, MCP-only allowlists and an explicit native
denylist. The system prompt exposes only Loom-governed MCP authority. Evidence:
`../P2D-W2C-W2D-codex-claude-native-tool-bypass-closure-v24.md`.

No Bash/Edit/Web/arbitrary MCP exposure, installed App, real Provider or exact
external executable startup is claimed. P2D-W2C remains `ACTIVE / PARTIAL`.

## Role dependency Context Capsule v25 status

`SOURCE VERIFIED / INSTALLED MIXED-TEAM OPEN`: normal DAG dependencies now
carry context as well as readiness. For each dependent Attempt, the Team
coordinator resolves the exact succeeded dependency Attempt, projected
Evidence/summary digests and finalized receipt, then extracts only authorized
output events from the Evidence artifact. The dependent Agent receives a new
Capsule and Route Segment before Work Authority dispatch freezes the Attempt.

Source authority and source content remain distinct. Exact Attempt lineage is
`dependency_source_authority`; model-produced event content is always
role-restricted `untrusted_model_output` with provenance. It cannot become a
Goal, policy, grant, decision or execution authority. Dependency source, Agent,
receipt, Evidence, summary and plan substitutions fail closed, and unrelated
peer Capsule content is never merged.

`ExtendRoleContextCapsule` is now the deterministic extension boundary for
dependency and Aggregation dispatch. It rejects replacement of an existing
item, repacks by the original priority contract, preserves content-free policy
and access omissions, and retains budget-omitted content only through the exact
target-scoped retrieval authority. Required dependency provenance that cannot
fit fails closed.

The controlled four-Provider DAG verifies Claude/Anthropic, Loom/Kimi and
Loom/MiniMax outputs reaching only the dependent Codex/OpenAI role without
disclosing Provider Account or credential identity. Approved fallback retry
also receives a newly frozen Capsule/Segment while the dependency receipts
remain exact. Evidence:
`../P2D-W2C-W2D-role-dependency-context-capsule-v25.md`.

This closes the source-level normal-DAG prior-output subset of Role Capsule
acceptance item 5. Observed test-state authority, model-tokenizer accounting,
Provider-specific ContextAdapter output, user-visible receipt inspection,
installed CV6 and mixed-Team ATL9 remain open. P2D-W2C stays
`ACTIVE / PARTIAL`.

## Observed acceptance state v26 status

`SOURCE VERIFIED / STRUCTURED TEST REPORT OPEN`: dependent and aggregation
dispatch now admits a source only after validating the exact projected
successful Attempt, versioned Output Contract digest, output classification
digest and accepted decision digest/time. The derived Role Context Capsule
contains three separate trust classes per source: authoritative lineage,
observed acceptance state and untrusted Provider output.

The observed item is role-restricted and compact enough for the existing Pi
ContextAdapter limit. Output Contract and Evidence details remain validated and
bound in adjacent authoritative lineage instead of being redundantly copied
into the prompt. Classification and acceptance substitutions fail closed.
Evidence: `../P2D-W2C-W2D-observed-acceptance-state-v26.md`.

This is not yet an authoritative test-result pipeline. Loom does not parse or
freeze a structured `test_report`, command result or verifier artifact in this
slice, and no model statement can become an observed execution fact. Installed
CV6, mixed-Team ATL9, model-specific ContextAdapters and COMP2-E remain open.
P2D-W2C stays `ACTIVE / PARTIAL`.

## Governed test report v27 status

`SOURCE VERIFIED / COMMAND-LEVEL REPORTS ONLY / INSTALLED LIVE OPEN`: a
recognized single test-runner command executed through the common Tool Gateway
now produces a typed immutable report bound to the exact Bash ToolCall,
arguments digest, encrypted result payload, execution result and frozen Attempt
authority. The Attempt Loop commits and replays that report as content-free
metadata, rejects payload or binding substitution, and exposes it to Team
coordination only after the result is delivered.

The Team coordinator resolves reports from projected dependency Attempt
authority rather than request input. Role Context Capsule assembly adds runner,
coarse scope, exit-derived outcome, call sequence, report digest and a stable
set digest to the existing role-restricted observed item. The underlying model
output remains untrusted. Ordinary Bash creates no report. Evidence:
`../P2D-W2C-W2D-governed-test-report-v27.md`.

This slice does not parse individual test cases, ingest arbitrary verifier
documents, expose diagnostics UI, or claim installed/live acceptance.
P2D-W2C stays `ACTIVE / PARTIAL`.

## Governed test report Board v28 status

`SOURCE VERIFIED / INSTALLED MIXED-TEAM OPEN`: the Team Board now resolves the
V27 report set for each exact current Agent Attempt from the existing Attempt
Loop authority. The query freezes Team, conversation, WorkItem, Run, claim,
Runtime, Agent, Incident, Execution Binding and Capsule identity. A stale peer
Attempt, duplicate sequence, invalid report or source error produces no report
projection.

Each row carries only count, passed/failed counts, latest runner/scope/outcome
and stable report/set digests. It carries no raw command, arguments, output,
Prompt, Provider body or credential content. The Board remains observational;
these fields do not authorize execution, fallback, acceptance or terminal
state. Evidence:
`../P2D-W2C-W2D-governed-test-report-board-v28.md`.

Installed CV6, four-Agent mixed-Team ATL9, per-test parsing and live failure
isolation remain open. P2D-W2C stays `ACTIVE / PARTIAL`.

## Production remote Tool Broker composition v29 status

`SOURCE VERIFIED / BACKEND ENROLLMENT AND INSTALLED LIVE OPEN`: `loom-work`
now constructs the remote Tool Broker inside Bundle Start instead of accepting
an already-assembled production executor. Exact Search, opt-in WebFetch and
allowlisted MCP capabilities are derived from present typed ports. Missing or
contradictory configuration fails before Ready; default `nil` configuration
publishes no remote tools.

The Work-owned Effect cancels active operations and revokes retained executor
references during rollback or Dispose. Existing ATL3 dispatch-before-call,
encrypted payload-before-terminal and final-output delivery proofs remain
unchanged. Evidence:
`../P2D-W2C-W2D-production-remote-tool-broker-composition-v29.md`.

No production Search provider or MCP Server is enrolled by the installed App
yet. Cross-Runtime live continuation, CV6, ATL9 and COMP2-E remain open;
P2D-W2C stays `ACTIVE / PARTIAL`.
