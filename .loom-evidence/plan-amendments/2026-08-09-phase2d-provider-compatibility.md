# Phase 2D Provider Compatibility — Accepted Insertion

Status: `ACCEPTED / ACTIVE`

Date: 2026-08-09

Authority input: the Product Owner explicitly required Loom to replace the
two-entry Provider screen with broad CC Switch-informed compatibility.

## Placement and version

Insert `Phase 2D — Provider Compatibility and Run-bound Profiles` after the
accepted Phase 2C client boundary. The release line is `v0.5.x`. This insertion
does not reopen P2C-W1/W2/W3 or their accepted source lock.

## Unified Goal

Phase 2D is the only active product Goal: complete Loom's multi-Provider,
multi-model Agent Team orchestration so the installed App opens into a usable
real conversation and every Agent in one Team can independently bind and
freeze its Provider Account, credential revision, Model, Harness, limits,
capabilities, and governed context.

This single Goal includes the installed Provider credential path and Loom-owned
Credential Vault; Conversation Profiles, immutable Route Segments, and Context
Capsules; per-Agent Execution Binding, dispatch, and role-scoped context;
Incident diagnostics, failure isolation, explicit fallback, Provider Account
accounting, and governance UI. Completion requires the installed credential
import, real conversation, mixed-Provider Team, and single-Agent failure
isolation matrix. Provider incidents and all named capabilities are WorkItems
or acceptance slices under this Goal; none replaces it or creates a parallel
product Goal. Phase 2C remains accepted and closed.

Observability is a horizontal Phase 2D acceptance gate, not a temporary
DeepSeek debugging aid. Provider connection, conversation dispatch, Agent
Attempt dispatch, failure isolation, fallback, and user-visible recovery must
share one privacy-safe operational correlation chain while the Event Journal
remains the authority for business facts only.

## Build 85 installed mixed-Team checkpoint (2026-08-21)

`CURRENT / VERIFIED SLICE`: installed `v0.5.3 (85)` completes a fresh four-Agent
DeepSeek/MiniMax Mission 4/4. Each Agent freezes its own Provider Account,
credential revision, model, Capsule, Segment and policy binding; each Provider
Account projects two completed Attempts with usage and cost and zero failures.
The same installed runtime publishes 25 Providers, 4 Runtimes and 4
Conversation Profiles, retains the OpenCode Runtime/Profile, and returns a real
OpenCode `E2E-OK` conversation reply.

The repaired boundary maps an encrypted Vault Capsule-not-found cause to the
bounded `ErrContextItemNotRetrievable` domain result without weakening ACLs or
discarding the original cause. A denied Context read now commits encrypted
payload, `ToolResultAccepted`, Provider-continuation delivery, and
`ToolResultDelivered` before the Agent Attempt can finish. This prevents a
successful Agent answer from being downgraded to `runtime_process_failed` while
keeping denied content out of Journal and Evidence. Phase 2D remains partial at
the four-Provider/four-Harness, extended failure-isolation, explicit fallback,
complete governance UI and Capsule privacy gates.

## Build 92 installed Provider/Runtime recovery and CC Switch checkpoint (2026-08-22)

`CURRENT / VERIFIED SLICE`: installed `v0.5.3 (92)` keeps the full Provider
directory and executable-attested Runtime inventory available independently of
Provider Account readiness. A cold launch produced 25 Providers, 6 online
Runtimes, 4 Conversation Profiles, and 5 privacy-safe CC Switch credential
candidates; the App started its bundled daemon without a manual service step.

The new import source discovers only non-secret metadata. Exact official
endpoint candidates require explicit confirmation and cross the private UDS as
candidate/account identity only. The daemon obtains one bounded secret lease,
configures and verifies the Loom-owned Vault entry, then zeroizes it. Custom
compatible endpoints remain review-required; no endpoint is silently promoted
to an official Provider identity. Ordinary conversation and Agent dispatch do
not read CC Switch or Keychain.

Full Go tests and vet, the 293-test macOS suite, 16 strict Swift contracts,
reproducible packaging, strict signing, installed setup inspection, and a real
DeepSeek reply pass. The locked graphical session prevented honest installed
visual acceptance. Phase 2D therefore remains partial at custom endpoint
approval, unlocked UI inspection, the four-Harness/four-Provider Team, complete
failure isolation, explicit fallback, accounting, and Capsule/privacy gates.

## Build 99 installed OpenCode and operational UX checkpoint (2026-08-22)

`CURRENT / VERIFIED SLICE`: installed `v0.5.3 (99)` preserves 25 Providers, 6
Runtimes and 4 Conversation Profiles while the App owns its bundled daemon
lifecycle. Build 97 passed both native OpenCode and OpenCode with the
Vault-backed DeepSeek account as real two-node Teams. The verifier execution
context is no longer cancelled by the short durable-commit deadline, and
acceptance is derived only from an exact terminal output reason.

Mission is a compact title list linked to a Conversation where available;
selection opens one workflow and its Team, Plan, Changes and Evidence. Installed
review confirms readable status and step language and a stable Inspector.
RoundTable exposes draggable Agent candidates, an explicit drop zone and an Add
accessibility equivalent. A successful retire fact now removes the seat from
the active projection; installed restart/reopen changed the same historical
Session from a stale `1/2` view to `0/2` without deleting its append-only fact.

The full Go suite, Go vet, 297 macOS tests with one intentional skip, 16 strict
Swift contracts, strict bundle signing and the installed 25/6/4 setup probe
pass. Phase 2D remains partial at the four-Harness/four-Provider Team, expanded
failure isolation, explicit fallback, full accounting/governance UI and
Capsule disclosure/encryption gates.

## Build 101 installed Runtime and workflow UX checkpoint (2026-08-22)

`CURRENT / VERIFIED SLICE`: installed `v0.5.3 (101)` cold-starts the bundled
daemon and returns 25 Providers, 7 Runtimes and 4 Conversation Profiles. The
inventory contains Claude Code, Codex, OpenCode, Pi and three Loom Native
routes; adding OpenCode no longer hides or disables unrelated detected
Runtimes. OpenAI is a configurable brokered Provider Account while the Codex
native-auth Conversation Profile remains independently available.

Conversation route selection now enumerates every Profile and labels the exact
Harness and Provider Account instead of deduplicating by Provider. Mission
history keeps active and blocked workflows visible when a Team loses executable
bindings, and disabled preflight/setup states offer Runtime & Providers recovery
rather than an inert control or false empty directory. RoundTable writer/target
assignment follows confirmed drag/add order and ignores retired seats.

The installed App remains conversation-first and shows `Local service ready`.
The 305-test macOS suite, 16 strict Swift contracts, affected Go tests, full Go
vet, reproducible App build, strict signing and installed 25/7/4 probe pass.
Phase 2D remains partial at the four-Harness/four-Provider Team, complete
account-local failure matrix, approved fallback, complete accounting UI and
remaining Capsule disclosure/encryption gates.

## WorkItems

### P2D-BLOCKER-1 Installed Provider credential path

`CLOSED / SUPERSEDED BY INSTALLED LIVE EVIDENCE (2026-08-21)`: the following
entries preserve the chronology of the v0.5.1-v0.5.2 incident. Later installed
Vault, restart, immutable Segment, and real DeepSeek reply evidence closes this
credential/bootstrap/stale-anchor blocker. Remaining Conversation disclosure,
Anthropic transition, four-Provider Team, failure-isolation, fallback, and
accounting work belongs to G2-G6 and W2A-W2D; it does not reopen this blocker.

`HISTORICAL / REOPENED`: the installed v0.5.1 App still fails a real DeepSeek key
before authoritative credential metadata is visible. v0.5.1 proved the legacy
Provider-ID restriction was removed, but did not prove the installed Swift ->
UDS -> process Keychain helper -> metadata writer -> projection path. The repair
must expose safe stage/error categories, normalize only bounded ASCII edge
whitespace, exercise DeepSeek generic IPC, preserve rollback and secret
clearing, and validate the ad-hoc installed helper path. Before this blocker can
be described as a live fix, the installed path must provide a minimum
Incident/Correlation ID, explicit safe stage errors, and persistent bounded
diagnostics for failures that occur before Keychain or metadata commit.
Completion then requires a user-supplied fresh key to reach verified revision,
publish a conversation Profile, and return a real DeepSeek conversation
response.

`CURRENT / INSTALLED CANDIDATE (2026-08-10)`: v0.5.2 build 10 exposed a real
observability mismatch: the daemon's owner-only operational record classified
an empty-secret configure failure as `daemon_admission`, while the UDS response
omitted `stage` and therefore could not drive actionable Swift UI. Build 11
repairs the credential-method boundary so existing helper, Keychain, and
metadata stages are preserved and any otherwise unstaged admission failure is
closed to `daemon_admission`. A concrete Client -> UDS -> daemon regression
requires the remote error, persisted diagnostic, and unchanged setup projection
to agree. Build 11 first passed that gate; the current installed build 12
preserves the same safe canary and the installed process Keychain helper
put/read/delete path. No real key or Provider network
completion was used, so this evidence narrows but does not close
P2D-BLOCKER-1.

`CURRENT / ROOT CAUSE CONFIRMED (2026-08-10)`: incident
`loom-swift-663d309b-d1f2-4ee6-9e53-9e861119174f` and two matching retries
reached the installed bundled daemon in 40-47 ms, then closed at
`daemon_admission / credential_unavailable` before Provider verification. The
installed daemon's kernel argv still contained only `--local-app-service` and
`--parent-pid`: the bootstrap expanded canonical `--state`,
`--isolation-root`, and `--socket` arguments only in a Go-local slice. The
process Keychain helper correctly rejected that parent because its unchanged
attestation reads the real parent argv. A second defect in
`setupCredentialResult` replaced the already staged
`helper_authorization` error with a bare store error, erasing the actionable
stage before IPC. The API key was neither written to Keychain nor sent to
DeepSeek; key content is not the root cause established by this incident.

The bounded repair order is now: (1) re-exec the same bundled daemon binary
with canonical expanded non-secret argv while retaining App-parent lifecycle,
(2) preserve classified credential errors through Setup and IPC, (3) prove the
real Darwin bootstrap -> parent argv -> process helper put/read/delete contract
without weakening executable, Socket owner, or peer-PID attestation, and (4)
package a new installed Candidate for the existing fresh-key live gate. An
internal managed-parent argument may carry only a validated PID; it remains in
kernel argv for lifecycle observation but is removed before `run` parses the
public daemon flags. Keys, credential bodies, prompts, and Provider responses
are forbidden from argv and diagnostics.

`CURRENT / INSTALLED REPAIR ACCEPTED`: v0.5.2 build 16 implements that
bounded repair. The App-started daemon's kernel argv now contains the exact
canonical state, isolation, Runtime, and Socket arguments followed by one
validated `--managed-parent-pid`; the internal parent flag is removed before
the public `run` parser. The installed Socket remains owner-only `0600`, daemon
health is current and non-partial, and the real installed
`--local-app-service --parent-pid` contract completes synthetic process-helper
put/read/delete. The same regression fails against build 15 at
`daemon_admission`, providing a direct old/new path comparison. Classified
credential failures now retain their original safe stage through Setup and
IPC. A subsequent real installed configure and verify now reports DeepSeek and
`deepseek.primary` as verified at revision 3 and publishes
`conversation-deepseek-deepseek-chat-r3`. This accepts the credential,
Keychain, metadata, projection, and Profile-publication portion of the blocker.

`HISTORICAL / CONVERSATION LIVE BLOCKER`: switching the installed App from the
persisted Codex thread to the verified DeepSeek Profile can reuse the stale
Codex thread anchor when its asynchronous load has not completed. The daemon
correctly rejects that Profile conflict before persisting the user message or
calling DeepSeek, while the old Swift catch hid the error. The source Candidate
now rotates the anchor from stored Profile identity, rejects stale load results,
keeps one compatibility thread frozen to one Profile, preserves the draft, and
shows a safe `conversation_dispatch` error with Retry, diagnostics, and Incident
ID. P2D-BLOCKER-1 stays open until this Candidate is packaged and a new
DeepSeek thread receives a real installed reply.

`HISTORICAL / INSTALLED CHAT REPAIR CANDIDATE`: v0.5.2 build 17 packages and
installs the stale-anchor/load-generation repair, explicit profile-conflict
transport, safe `chat_message` correlation, and inline recovery controls. The
App automatically starts its canonical bundled daemon, retained and installed
hashes match, the Socket is owner-only `0600`, deep signature verification
passes, and DeepSeek revision 3 survives restart. Full Swift tests, focused and
package-wide Go tests, Go vet, two deterministic Release builds, and launch
smoke pass. This is still `PARTIAL`: no Provider generation was issued by the
automated gate, so the user-confirmed new DeepSeek thread and real reply remain
required.

### P2D-W1 Provider Registry and Connection Directory

`CURRENT`: replace fixed Provider fields as the primary client surface with an
ordered registry; separate Agent Runtimes from model Providers; add search,
category filtering, status, and API-key connect/test/replace/revoke journeys.
Credentials remain in the Loom-owned Credential Vault; Keychain is restricted
to explicit one-time migration. Fixed verification endpoints are
non-generative, origin-bound, redirect-free, proxy-free, timed, and bounded.

### P2D-W2 Run-bound Provider Profiles

W2 is split by authority boundary. A conversation Profile is never inherited
as a Team or Agent execution choice.

#### P2D-W2A Conversation Profile binding

`CURRENT / INSTALLED COMPATIBILITY SLICE`: v0.5.2 build 17 materializes exact conversation Profiles for Codex native
auth and verified DeepSeek, Kimi, and MiniMax credentials. The macOS client
binds one Profile to each conversation; changing Provider starts a new
conversation instead of mutating an existing binding. The daemon requires the
exact verified credential revision, reads the Keychain secret only for that
request, and uses fixed, bounded, non-tool routes. Kimi follows the official
`https://api.moonshot.cn/v1/chat/completions` `kimi-k2.6` contract; MiniMax
follows the official `https://api.minimaxi.com/v1/chat/completions`
`MiniMax-M3` contract and its `max_completion_tokens` request field.

`TARGET / ACCEPTED ARCHITECTURE`: P2D-W2A now owns Conversation Route Segments
and Context Capsules as specified by
`.loom-evidence/phase2d/contracts/P2D-W2A-conversation-route-segments.md`. The
user keeps one visible Loom Conversation. Every Harness, Provider, Provider
Account, Model, or credential-revision change creates an immutable Segment and
one or more single-binding Attempts; Provider-native handles never cross those
boundaries. A stable Agent owns a versioned RouteSet of Harness-compatible
Execution Profiles. Parallel routes create sibling Attempts plus a separate
Aggregation Attempt, and fallback requires an approved Route Transition.

On each route change, Loom builds a target-specific, content-addressed Context
Capsule from authoritative, observed, and explicitly untrusted inputs. It
applies redaction, ACL, target-account disclosure policy, deterministic context
packing, and an omission manifest before a ContextAdapter prepares the target
wire format. Hidden reasoning is never transferred, old model output cannot be
promoted to authority, and the new Segment freezes both Capsule and Execution
Binding digests. Switch modes are Continue with context, Summary only, and
Start clean; each produces a non-secret disclosure receipt and uses scoped
retrieval rather than unbounded history access.

`CURRENT / SOURCE SEGMENT VERTICAL CANDIDATE`: persistence schema 2 now keeps
one visible conversation thread and appends immutable Segments on explicit
Profile transitions. Each dispatch records one Attempt and freezes Profile,
Segment, Context mode, Capsule digest, and Binding digest. Swift preserves the
thread anchor, fences stale loads, sends the selected transition mode, decodes
Segment/Attempt state, and labels routed turns with Harness, Provider Account,
and Model. The compact route menu exposes all three modes. Summary mode carries
recent user input without prior model output; Continue mode wraps prior model
output as explicitly untrusted policy-filtered context rather than target
assistant history; Start clean sends only the new turn. Schema 1 migration and
Profile-conflict fail-closed behavior are covered. This is not yet an installed
or live Provider acceptance, and it does not claim complete structured Capsule
packing, disclosure receipts, native-handle isolation, or encryption at rest.

`DELIVERY ORDER`: package and live-test one visible Codex/OpenAI to Loom
Native/DeepSeek Segment using `summary_only`, correlated diagnostics, and a
real installed reply. Extend that
slice to full context, parallel routes, Role Capsules for mixed Teams, restart
isolation, disclosure/retention governance, and encrypted local state.

#### P2D-W2B Per-Agent Provider Account identity

`CURRENT / PARTIAL`: the existing `RuntimeProfile` contract, presented to the
user as an Execution Profile, now lets each Team role independently identify its
Harness Adapter, Provider, Provider Account, Model, endpoint fingerprint,
credential reference and revision, timeout, and required capabilities.
Provider Account identity is separate from Provider identity and supports
multiple accounts for one Provider. A Team-level default may seed a new role
but cannot overwrite a role binding. Optional reasoning effort is now part of
the Profile contract; explicit values require a matching Runtime capability.

`CURRENT / SOURCE CANDIDATE`: each Agent row can now create a new immutable
Execution Profile by changing its observed Model, reasoning effort, timeout,
or budget. The safe complete Profile snapshot is versioned inside
`TeamDefinitionSaved`, independently projected, and restored into the Builder
and daemon materializer after restart when the dynamic catalog no longer
publishes that custom ID. Existing same-ID catalog content must match exactly;
account, credential revision, Model, limits, or capabilities drift fails
closed. The StateWriter requires the snapshot to equal the Runtime Profile used
for confirmation, and only freeze-ready execution Profiles may enter this path.
Credential references remain opaque; secret body bytes never enter the event.

#### P2D-W2C Agent Attempt binding and dispatch

`CURRENT / PARTIAL`: `BuildSavedTeamRuntimeBinding` resolves every role
independently. Dispatcher inputs and authoritative Agent Attempt events now
freeze the exact Harness, Runtime instance, Provider, Provider Account, Model,
endpoint fingerprint, credential reference/revision, timeout, budget,
capability set, and reasoning effort behind a validated digest. Replay and the
Team execution projection preserve that binding, and ordinary retry rejects any
silent binding change. Independent Verifier Agents use the same optional
binding field on their authoritative Run and must match it during recovery.
Credential body bytes remain Vault-owned and may be resolved only through an
exact short-lived lease at the bounded adapter call.

`CURRENT / PARTIAL`: Team Agent Attempts now freeze a real role-scoped Context
Capsule and disclosure-receipt record beside the exact Execution Binding. The
source-neutral builder enforces trust classes, all six accepted scopes,
target-specific ACLs, deterministic priority/token packing, explicit omissions,
model-output non-authority, and opaque-only secret references. Work Authority,
replay, Projection, API Board, and Swift decoding preserve only safe metadata;
Journal and Board payloads exclude Capsule content. Ordinary retry retains the
Capsule digest, while an approved route fallback creates a new target-bound
Capsule. The exact current and remaining target boundaries are frozen in
`.loom-evidence/phase2d/contracts/P2D-W2C-agent-attempt-binding-dispatch.md`.
The minimum source-neutral Context Adapter now renders admitted Capsule text,
trust/provenance labels, and content-free omissions into a canonical v2
dispatch payload bound to the Capsule and receipt digests. Team admission
recomputes the exact bytes before Journal mutation; Pi, Loom Native, Codex, and
Claude Code consume the same strict decoder. Raw Mission objective text is no
longer a parallel Team dispatch input. Complete authoritative source assembly,
model-tokenizer and multi-message/tool ContextAdapter expansion, encrypted
Capsule storage/restart, receipt inspection, product-level installed four-pair
activation, and the Role Capsule live matrix remain outstanding under W2C/W2D/W3.

`CURRENT / DISPATCH BINDING GATE`: Supervisor freezes the execution Profile and
Runtime again at the final dispatch boundary, compares its digest with the
authoritative Run binding, and passes that exact binding to the concrete
adapter. Pi validates Harness, Runtime, and native auth; Pi RPC also validates
its exact configured Provider and Model. Drift fails before process start.
Consequently the daemon must not publish a brokered Provider Profile merely
because its credential verifies: publication requires a concrete adapter that
can consume the exact Provider Account and credential revision. Build 15 may
publish concrete DeepSeek, Kimi, and MiniMax Profiles for exact verified
accounts; unimplemented brokered Provider Profiles remain absent.

#### P2D-W2D Observability, Failure Isolation, Governance, Fallback, Accounting and UI

`PARTIAL / ACTIVE`: credential, endpoint, rate-limit, timeout, or Provider
availability failure blocks only the affected Agent and names the reason on the
Team board.
The frozen horizontal acceptance boundary and ordered execution plan are in
`.loom-evidence/phase2d/contracts/P2D-W2D-observability-governance.md`.
P2D-W2D prioritizes a low-friction connection and recovery experience and owns
the following cross-layer closure:

1. One Incident/Correlation ID follows a request from Swift App through UDS,
   `loomd`, Credential Vault/lease, Provider verification, Conversation Profile,
   Agent Attempt, and Team board.
2. Safe structured diagnostics record time, operation, non-secret Provider and
   Provider Account identifiers, stage, elapsed time, result, safe error code,
   and retryability. They never record API keys, Authorization headers, secret
   bytes, prompts, conversation content, or raw Provider responses.
3. Installed builds retain bounded, automatically rotated diagnostics with
   owner-only `0600` permissions, or an equivalent safe OSLog plus exportable
   store. Bundled `loomd` stdout/stderr must not be silently discarded as the
   only available failure evidence.
4. The stage vocabulary covers `input_admission`, `uds_transport`,
   `daemon_admission`, `helper_validation`, `helper_start`,
   `helper_authorization`, `helper_request`, `helper_timeout`,
   `helper_response`, `helper_exit`, `keychain_access`, `metadata_commit`,
   `projection_refresh`, `provider_dns`, `provider_tls`, `provider_connect`,
   `provider_http`, `provider_auth`, `provider_rate_limit`, `profile_publish`,
   `conversation_dispatch`, `agent_attempt_dispatch`, `vault_key_load`,
   `vault_open`, `vault_encrypt`, `vault_commit`, `vault_decrypt`,
   `vault_aad_validation`, `vault_rotation`, `vault_recovery`, `vault_export`,
   `credential_lease_issue`,
   `credential_lease_expire`, `credential_lease_revoke`, `migration_read`,
   `migration_commit`, and `migration_cleanup`. Helper/Keychain stages remain
   only for explicit one-time migration.
5. User-visible errors show the safe stage, an actionable recovery, whether
   retry is appropriate, and the Incident ID, with Retry, View diagnostics,
   and Copy incident ID controls.
6. A user-initiated diagnostic bundle supports preview and contains App/daemon
   versions and hashes, process/socket health, non-secret Provider/Profile/Agent
   binding state, and recent redacted events. Keys, environment credentials,
   prompts, conversations, and Provider bodies are excluded by default.
7. Provider Account, Model, credential revision, timeout, fallback decision,
   and failure reason project to each Agent. One Agent failure never becomes an
   undifferentiated Team `offline` or `unavailable` state.
8. Fallback is explicit, versioned, approvable, and auditable. Loom never
   silently changes Provider, Provider Account, or Model.
9. Concurrency, rate limits, budgets, token/cost usage, and error rates are
   attributed to the exact Provider Account and correlated to Run, Attempt, and
   Agent. Loom supports multiple named accounts per Provider; accounting never
   falls back to a Provider-only global bucket.
10. Operational diagnostics are non-authoritative and separate from the Event
    Journal, but correlate to authoritative facts by Incident ID. Failures
    before Vault or metadata commit still produce a safe terminal diagnostic.

`TARGET / ACCEPTED CREDENTIAL VAULT ARCHITECTURE`: ADR-0020 and
`.loom-evidence/phase2d/contracts/P2D-W2D-credential-vault.md` replace the
normal ProductKeychainStore path beneath the existing `SecretStore` contract.
The Phase 2D default LocalKeyFile mode loads a separate owner-only 256-bit VMK
once per daemon session. Per-revision random DEKs encrypt secrets with
AES-256-GCM; an HKDF-SHA256 domain-separated KEK wraps each DEK. Conversation
and Agent dispatch acquire exact account/reference/revision leases, decrypt only
inside `loomd`, and zeroize on close. No normal turn starts a helper or accesses
Keychain. Existing Keychain items require explicit re-entry or a one-time
migration helper and are never auto-deleted. No plaintext fallback exists.

`CURRENT / CV1 FROZEN; CV2-CV4 PRODUCTION SOURCE CANDIDATES`: the Vault now has
the AES-GCM/HKDF envelope, LocalKeyFile, encrypted SQLite store, exact file
identity checks, multi-account/revision isolation, bounded leases, pending
mutation transactions, restart reconciliation, and a coordinator for
configure/verify/replace/revoke. Production daemon construction enables the
Vault, owns its lifecycle, injects exact leases into Conversation and Agent hot
paths, and bypasses ProductKeychainStore setup. Provider Account operation
status is resolved from the exact account stream. Explicit re-entry imports a
new Vault revision when legacy metadata exists without ciphertext; there is no
plaintext or silent Keychain fallback.

`PARTIAL / CV5`: missing exact Vault rows project `migration_required` and do
not publish a Conversation Profile. The Swift sheet names Loom Credential Vault
and runs import plus verify in one recovery action. Setup and Swift now share a
strict top-level Vault projection containing the active storage mode, aggregate
status, and migration/recovery account counts; the UI renders it without inert
management controls. Classified LocalKeyFile load/open failure now leaves the
daemon in a restricted recovery mode so Setup remains visible while mutation
and lease use fail closed at the original Vault stage, clear caller secret
bytes, and never construct a Keychain fallback. Closed Vault/lease/migration
stages cross daemon diagnostics and Swift decoding. Loaded key material retains
the key-file device/inode identity, and each VMK lease plus health/mutation
boundary revalidates it without rereading key bytes; live path replacement
therefore projects recovery and blocks all credential access.
The source Candidate now stages an explicit next-version LocalKeyFile,
transactionally rewraps every active DEK while preserving credential
ciphertext, advances Vault metadata, rejects pending credential mutations, and
rolls back before
commit without consuming the candidate key. A fixed pending-key startup
protocol promotes committed rotation, removes uncommitted rotation, and fails
closed on mismatch. A lease generation barrier revokes active plaintext and
rejects in-flight old-generation acquisitions. `credential_vault_rotate` crosses
the private UDS with an extended deadline and safe `vault_rotation` diagnostic;
Swift exposes a real Rotate key action and refreshes authoritative Setup state,
showing stage and Incident ID on failure. Interactive lock/unlock now revokes
leases, closes the in-process Vault, releases the VMK, and reconstructs the
runtime only after strict key and rotation revalidation. Explicit recovery reset
is restricted to `recovery_required`, requires exact destructive confirmation,
crypto-erases validated Vault files, fsyncs both directories, and leaves legacy
accounts in `migration_required`. Passphrase-protected export uses Argon2id,
the isolated `loom/export-wrap/v1` domain, per-entry DEK rewrap, an encrypted
manifest, bounded `0600` no-overwrite publication, and privacy-safe
`vault_export` diagnostics. Restore UI and optional helper migration are not
claimed.

Full relevant Go packages and focused Vault/daemon race checks including key
identity drift, live path replacement, rotation/restart, malformed-key
recovery, explicit reset/re-entry, and encrypted export/rewrap, vet, diff
checks, 167 XCTest with one intentional visual-export skip, and six Swift
Testing contracts pass on the final source. Installed v0.5.2 build 20 matches
the packaged hashes, passes strict deep signature verification, launches the
canonical daemon and owner-only LocalKeyFile Vault, and correctly projected the
legacy DeepSeek account as `migration_required / vault_entry_missing`. The user
then completed real `Move key to Vault`; installed replace/verify diagnostics
succeeded, DeepSeek is verified revision 6, and Profile r6 is published. A real
App/daemon restart preserved Vault identity, auto-unlocked the Vault, restored
revision 6/Profile r6, and left no active helper. Build 19 remains as the
transactional rollback bundle. CV6 and the installed Keychain-removal claim
remain open until an r6 multi-turn reply proves the Conversation hot path with
no helper and the mixed-Team isolation matrix passes.

The first r6 message after that restart created a new immutable Segment but did
not return a DeepSeek reply. Incident
`loom-chat-d6873ebe-e5f7-41b5-9b24-a204688ca2bb` is authoritative as
`failed / conversation_unavailable`; Build 20 nevertheless wrote
`conversation_dispatch / succeeded` because it observed only the successful IPC
thread response. The fixed fallback message must not be treated as Provider
success. Source now classifies safe Provider transport/HTTP/auth/rate-limit
outcomes, freezes stage/code/retryability plus Incident on the Conversation
Attempt, projects the failed terminal state into daemon/App diagnostics, and
keeps the failed thread plus retry draft visible. Installed root-cause
classification remains open until this source is packaged and the user retries
r6; no key, Prompt, or Provider body is added to diagnostics.

That source is now installed as v0.5.2 build 21. Packaged and installed hashes
match, deep signature verification passes, build 20 is the rollback bundle,
and startup preserves the same owner-only Vault identities while restoring
DeepSeek revision 6/Profile r6 with no helper. The UI is staged on DeepSeek but
has not sent a Build 21 request. One user retry is now the next gate: its frozen
Attempt and same-Incident daemon/App records must expose the exact safe failure
class or a real Provider reply.

Agent rows and the Agent editor expose Harness, Provider Account, Model,
specific status, credential revision, reasoning effort, budget, timeout, and
optional fallback. The primary experience answers: which account this Agent
uses, whether it is usable, what happened, what it cost, and what the user can
do next. The current Board projection already carries the safe Harness,
Provider, Provider Account, Model, credential revision, and terminal reason.
The source Candidate now also carries the current dispatch generation's safe
Incident ID through Attempt projection and Board wire. Rebind advances only the
affected Attempt's Incident, terminal replay preserves an existing dispatch
Incident, and the Board filters values through the strict IPC ID grammar.
Swift shows a bounded Incident label beside the Agent failure and provides an
icon-only Copy incident ID action; malformed IDs fail closed. Installed build
20 predates this addition, so this closes source wiring rather than the
installed mixed-Team observation gate.
The Run authority now also freezes bounded token/cost accounting. Pi RPC,
Codex, Claude Code, and Loom Native/OpenAI-compatible adapters extract only
closed numeric terminal usage, and the Board aggregates active concurrency,
failures, observed rate limits, assigned budgets, tokens, costs, and error rate
by exact Provider Account with Run/Attempt/Agent linkage. A controlled
four-Provider Team now proves distinct accounting survives under the exact
OpenAI, Anthropic, Kimi, and MiniMax accounts; the failed OpenAI Attempt does not
alter peer usage, and its scheduled recovery Attempt cannot inherit the prior
accounting before freezing its own terminal fact. Configured account ceilings
and installed real-Provider usage remain open. The
fallback authority now supports a pre-approved, versioned one-shot binding
transition: its authoritative plan fact freezes the approval ID, actor, time,
source binding digest, target binding digest, and approval digest; the recovery
fact links the exact approval; and dispatch permits the binding change only
for the selected fallback action and exact target. Ordinary retry, a fallback
workflow name without approval, a future-dated approval, and a forged target
remain fail-closed. The current Team Builder now provides one revision-checked
execution-profile selector per Agent and displays the safe Harness, Provider,
exact Provider Account, Model, reasoning effort, credential revision, timeout,
budget, and capability set derived from validated Runtime Profiles. Selecting a different
RoleOption updates only that Agent; peer bindings are preserved and the Team
binding digest changes. Credential references, endpoint fingerprints, binding
digests, secret bytes, and prompts do not enter this UI projection. The
Runtime Profiles and new Attempt bindings now freeze optional reasoning effort;
explicit values require a matching Runtime capability, use a v2 digest, survive
Team/Run replay, and cross Builder, Board, and safe diagnostic projections.
Legacy empty values preserve the published v1 digest and display as Provider
default. Direct versioned Profile authoring now covers Model, reasoning effort,
timeout, and numeric budget independently per Agent, persists through Saved
Team restart, and rejects invalid edits without mutating the Draft. Provider
Account health preflight, Agent-level fallback configuration, and interactive
approval are source-complete but still require installed mixed-Team acceptance.
Configured account ceilings and installed real-account governance remain active
work.

Configured ceilings use a separate revision-CAS Provider Account policy and
account-capacity stream. Dispatch atomically reserves both Runtime and exact
Provider Account capacity, freezes the admitted policy revision/digest on the
Attempt, and fails before credential access when concurrency, bounded dispatch
rate, or assigned budget is exhausted. Retry revalidates the same account;
approved fallback validates the target account independently. The Board
combines these safe configured limits with the existing exact-account observed
usage/cost aggregation. This source slice is implemented with strict replay,
atomic replacement/terminal release, and a concurrent same-account gate that
leaves no partial loser mutation. Claim also CAS-observes an empty policy head,
policy timestamps strictly advance, and selective peer-Run capacity replay
validates deterministic event identity. Production policy configuration,
installed presentation, real Provider accounting, and mixed-Team live
acceptance remain open.

`CURRENT / FALLBACK BOARD PRESENTATION CANDIDATE`: the Team Board now projects
the safe per-Agent fallback state without exposing route or authority digests:
configured, approval required, versioned approval available, and consumed.
Swift strictly validates those combinations and shows `Fallback configured`,
`Fallback approval required`, `Fallback approved vN`, or `Fallback executed`
beside the exact Agent binding. Focused API/race tests, full affected Go
packages, the Go/Swift IPC contract, and the complete Swift suite pass. This is
not in installed build 21 and does not close Agent fallback editing or
interactive approve/reject; those remain in ordered W2D step 5.

`CURRENT / FALLBACK APPROVAL COMPILER SEAM`: the compiler now queries an
optional authority source with the exact Team instance, Plan digest, logical
Agent node, source binding digest, and target binding digest. With no exact
approval, Attempt 2 remains on primary. A valid non-future
`TeamFallbackApproval` with matching frozen binding digests selects the
version-2 fallback RecoveryPolicy and materializes Attempt 2 on the target
Runtime/Profile. Forged target facts fail closed; an approved target that has
become unavailable blocks only the owning Agent and prevents Start. Preflight
projects safe approval availability/version, and Swift rejects contradictory
states.

This completes the compiler consumption seam, not persistent user authority.
Ordered W2D step 5 now starts with a prepared preflight approve/reject command
that writes a versioned Journal fact keyed by the complete query identity,
followed by production source resolution, Decision Sheet actions, Board replay,
and installed fallback execution. No approval may be inferred from RouteSet
configuration or created silently during compilation.

`CURRENT / FIRST W3 VERTICAL SLICE`: the latest source candidate implements an
exact `loom-native + DeepSeek + deepseek-chat` Agent adapter. It consumes the
authoritative per-Agent frozen binding, uses the exact projected credential
reference and revision, resolves and clears secret bytes only around a bounded
fixed-endpoint request, propagates numeric usage, emits safe terminal reasons,
and records an `agent_attempt` operational diagnostic carrying the same
correlation ID plus non-secret Provider Account and Model identity. The daemon
selects a Supervisor by each Agent's Harness and Runtime instance; it never
falls back to a global Pi or Provider client. DeepSeek Team Profiles are
published only while the exact credential projection is verified, and their
IDs include the credential revision so replacement cannot silently mutate a
saved binding.

`CURRENT / OPENAI-COMPATIBLE LOOM-NATIVE EXPANSION`: build 15 generalizes that
strict transport core through closed, code-owned Provider descriptors and adds
Kimi/Moonshot `kimi-k2.6` plus MiniMax `MiniMax-M3`. Each Provider has its own
RuntimeInstance and Supervisor, fixed HTTPS endpoint and endpoint fingerprint,
exact Model, Provider Account and credential revision validation, bounded
response, redirect/proxy rejection, secret clearing, numeric usage mapping,
and Attempt diagnostic. Cross-Provider bindings fail before Keychain access.
Only a verified Provider is admitted into its runtime/Profile catalog; cold
start restores existing verified Providers independently, and rejected or
unconfigured Providers do not create executable Profiles.

`CURRENT / PROVIDER ACCOUNT PERSISTENCE`: exact Provider Account identity now
flows through Broker commands and strict UDS requests. Non-primary accounts
have independent Journal streams and revisions plus an account-keyed
projection; legacy Provider events and new `.primary` mutations retain the
legacy stream while rebuilding as the corresponding primary account. The setup
snapshot and Swift client list account records independently, and Agent runtime
credential resolution requires the exact Provider plus Account before Keychain
access. Account identity is included in safe installed diagnostics without
credential bytes.

`CURRENT / PROVIDER ACCOUNT MANAGEMENT UI`: the latest source Provider
directory shows account counts and opens one native account-local credential
sheet. The sheet always retains `.primary`, lists additional accounts, derives
a validated deterministic account ID from a user name, and applies connect,
verify, replace, revoke, incident, and diagnostic actions only to the selected
account. Primary actions continue through the legacy generic client contract;
only non-primary accounts use account-scoped requests, so existing clients
remain compatible without allowing a named account to degrade to primary.

`CURRENT / NON-PRIMARY EXECUTION PROFILE PUBLICATION`: the dynamic Team catalog
publishes a revision-frozen Coordinator and Worker Profile for every verified
DeepSeek Provider Account. Primary IDs remain migration-compatible;
non-primary internal IDs use a bounded stable account hash, while the Builder
shows the exact account, Model, credential revision, timeout, budget, and
capabilities. The catalog digest binds every sorted account/reference/revision,
and revoking one account removes only its RoleOptions. Team roles can therefore
select different verified accounts independently through the existing
revision-checked Agent Profile editor.

Direct custom Profile authoring for Model, reasoning effort, timeout, and
budget is source-complete. Versioned per-Agent fallback RouteSet authoring,
strict Swift decoding, Builder selection, authoritative persistence, and
saved-Team restart recovery are also source Candidates. Interactive fallback
preflight now independently resolves and displays fallback binding health;
fallback failure does not block the primary Agent. Interactive fallback
approve/reject, Attempt materialization, configured account ceilings, the
remaining adapters, and their complete governance UI remain active Phase 2D
work. Build 13 has passed deterministic packaging and
installation from the retained delivery. No build has yet passed a fresh-key
live acceptance, so P2D-BLOCKER-1 remains open.

`CURRENT / PRIVACY-SAFE DIAGNOSTIC EXPORT`: a Provider Incident opens a native
preview of App/daemon version and hashes, Socket health, console metadata,
non-secret Provider/Profile/Agent binding state, recent closed-schema events,
and skipped unsafe records. Export is user-initiated and writes an owner-only
`0600` JSON file. It reads bounded diagnostic files with `O_NOFOLLOW` and
re-encodes only allowlisted fields; raw console, keys and credential references,
Authorization headers, environment credentials, prompts, conversations, and
Provider response bodies are excluded by default. The bundle remains
non-authoritative support evidence and is never written to the Event Journal.

`TARGET / DISCLOSURE AND ENCRYPTION`: W2D owns versioned DisclosurePolicy and
receipt, target-account retention/region governance, deterministic omission
visibility, and encrypted local conversation state. The current owner-only
`chat-threads.json` is plaintext and is not an encryption-at-rest claim. Phase
2D exit requires a per-Conversation DEK wrapped through the separate
`loom/conversation-wrap/v1` Vault domain; transcript, Context Capsule, and
ExternalSessionHandle are encrypted, missing keys fail closed, and local
deletion supports crypto-erasure. Journal keeps non-secret authority facts and
digests; diagnostics keep safe metadata and stages only.

### P2D-W3 Runtime Client Adapters

`CURRENT / PARTIAL`: governed Provider-aware Agent adapters now cover the
bounded `loom-native` DeepSeek, Kimi/Moonshot, and MiniMax vertical slices
described above. The official protocol sources are
`https://platform.kimi.com/docs/api/chat.md` and
`https://platform.minimaxi.com/docs/api-reference/text-chat-openai.md`.
W3 remains active: add governed adapters for Claude Code + Anthropic,
Codex + OpenAI, and the remaining verified
CC Switch client set where the local executable and protocol can be proven,
including Claude Desktop, Gemini CLI, Grok Build, OpenCode, OpenClaw, and
Hermes. Adapters observe or materialize reviewed configuration; they do not
silently overwrite external files, inherit unbounded credentials, or select a
global Provider client.

## W1 compatibility directory

The v0.5.0 directory includes OpenAI, Anthropic, Google Gemini, DeepSeek, Kimi,
MiniMax, xAI, Zhipu GLM, Alibaba Bailian, Tencent Hunyuan, Baidu Qianfan,
StepFun, ModelScope, OpenRouter, SiliconFlow, NVIDIA NIM, Novita AI, Azure
OpenAI, AWS Bedrock, Google Vertex AI, Ollama, LM Studio, and custom OpenAI- and
Anthropic-compatible entries.

Presence in the directory means a known compatibility shape, not a live or
executable claim. W1 supports fixed API-key verification for 16 entries;
managed-cloud, local Runtime, and custom endpoint connection kinds remain
truthful non-executable states until their dedicated boundaries pass.

## Exit conditions

Phase 2D exits only when W1-W3 are complete and a live Team proves Claude Code
+ Anthropic, Codex + OpenAI, Loom + Moonshot/Kimi, and Loom + MiniMax can pass
preflight, instantiate, dispatch independently, report status, and preserve an
auditable lineage. Revoking one bound credential must block only its Agent.
Conversation and Agent execution must coexist without shared global switching,
credential disclosure, hidden fallback, or lineage loss.

Ordinary conversation acceptance additionally proves one visible Conversation
can transition Codex/OpenAI -> Loom Native/DeepSeek -> Claude/Anthropic through
immutable Segments and policy-admitted Context Capsules without reusing native
handles or promoting old model output to authority. Restart, context-window
omission, prompt-injection trust separation, and encrypted-at-rest storage must
fail closed and preserve the Segment/Capsule/Attempt lineage.

Any Provider import, verification, conversation, or Agent Attempt failure must
be attributable to an explicit stage in one attempt. Users must see a safe
reason and recovery without opening Terminal, and support must be able to trace
the path from an Incident ID and privacy-safe diagnostic bundle without access
to keys or prompts. In a mixed Team, revocation, rate limiting, or timeout on one
account must preserve the other Agents' execution and complete audit chains.

`CURRENT / PERSISTENT FALLBACK DECISION AUTHORITY CANDIDATE`: the exact
Team/Plan/Agent/source/target Scope now owns a revision-CAS Journal stream.
Approve stores a revisioned `TeamFallbackApproval`; Reject advances the same
stream with no approval. Strict projection replay exposes only the latest exact
Scope, and the production compiler source consumes approved records while
treating reject or identity drift as no authority. The prepared backend and
native macOS sheet support explicit Approve fallback, Reject fallback, and
non-writing Not now actions.

Mission preflight now dynamically produces one safe exact-Scope Candidate per
configured Agent fallback and registers it without writing authority. The
projection-backed preparer binds deterministic Approve/Reject commands to the
current Scope revision; replacement removes stale Plan/binding actions only for
the owning Team, and `Not now` remains zero-write. The production daemon uses
the same shared prepared-decision backend and dynamic router controls. The
macOS Store refreshes the same authoritative Snapshot after preflight, making
the new sheet immediately reachable while retaining view-drift invalidation.

The source integration gate now carries an approved exact Scope through
recompilation and controlled two-Agent execution. A failed Codex/OpenAI primary
Attempt schedules and dispatches the approved Loom Native/DeepSeek Attempt on a
different Runtime with its exact Account, credential reference, and revision;
the peer MiniMax Reviewer completes on its original binding and acquires no
fallback state. Recovery Journal facts and Board Projection replay the consumed
approval and both independent bindings.

This closes the source defect that previously forced every Attempt to the
ExecutionPlan's primary Runtime. The semantic authority now freezes the approved
target Runtime, while ordinary retry and legacy same-binding fallback retain the
current Runtime. Work Authority and Projection fail closed if recovery,
scheduled Attempt, target binding, or Runtime disagree. This remains partial W2D
step 5: installed action handling and live Provider fallback dispatch remain
open. The DeepSeek r6 live blocker and Credential Vault CV6 gates are unchanged.

An uninstalled v0.5.2 build 23 candidate packages the earlier decision/Board
slice but predates the cross-Runtime RecoveryPolicy repair. Its
deterministic builder, transactional installer fixture, deep signature,
permissions, symlink boundary, and extracted transport hashes pass. Installed
build 22 remains untouched for the pending DeepSeek r6 attribution gate; build
23 carries no installed or live acceptance claim.

The exact build-23 bundled daemon also passes an isolated-HOME Vault binary
contract: configure, replace, cold-restart automatic unlock and exact
reference/revision restore, then revoke. This uses only a synthetic secret and
no Provider call. Focused race and the full daemon suite pass; real-key verify,
conversation reply, installed fallback, and mixed-Team gates remain open.

An uninstalled v0.5.2 build 24 candidate supersedes build 23 for the current
source-to-binary boundary. It includes the cross-Runtime RecoveryPolicy repair,
passes deterministic build and transactional installer fixtures, deep signature
and owner-only bundle checks, transport extraction with byte-for-byte executable
matching, and the exact bundled-daemon Vault contract in normal and race modes.
The complete repository Go suite, affected-package race/vet checks, and full
macOS Swift suite also pass. This is still a dirty-worktree Candidate, not
release provenance or installed acceptance. Installed build 22 remains
untouched until the user-triggered DeepSeek r6 gate is attributable; build 24
has not been installed or launched, and all real Provider fallback and
four-Provider mixed-Team gates remain open.

`CURRENT / PROVIDER ACCOUNT CEILING AND ATOMIC DISPATCH CANDIDATE`: W2D owns one
revision-CAS policy stream and one distinct capacity stream per exact Provider
Account. A canonical immutable policy binds Provider, account, revision,
maximum concurrent Attempts, bounded dispatch window/start count, maximum
assigned budget, and UTC configuration time. Claim validates the frozen Agent
binding and atomically appends the Run claim plus Runtime and account
reservations before any Adapter or credential lease access. Replacement and
terminal paths release the exact generation in the same authority transaction.
The admitted Run freezes policy revision/digest and assigned budget; strict
authority/Projection replay verifies policy history, capacity limits,
causation, exact account/binding identity, and matched release. The Board API
and Swift models project only safe frozen Agent policy facts and current
account-level limits/capacity. Empty policy heads participate in claim CAS,
revision time strictly advances, and orphan peer-capacity events retain
deterministic identity validation. Focused normal/race tests, complete affected
Go suites, Go vet, strict Swift model tests, the isolated Mission UI suite, and
a 63-test ordered Swift subset pass. The full Swift runner currently has an
independent XCTest `@MainActor` ordering hang before the Mission UI assertion;
no latest-source full-suite pass is claimed. Installed presentation, real
Provider accounting, and four-Provider mixed-Team acceptance remain open.

`CURRENT / PROVIDER ACCOUNT POLICY CONFIGURATION SOURCE CANDIDATE`: the
revision-CAS authority is now reachable through the production setup path:
`LocalProductSetupService/API -> private loomd UDS -> strict Swift client ->
Store -> Runtime & Providers account UI`. loomd injects trusted request
correlation; the command contains only exact Provider/account identity,
expected revision, bounded concurrency/dispatch/budget limits, and operation
ID. Secret, credential reference, policy digest, Prompt, and Provider response
are not command fields and unknown `secret` is rejected.

Swift freezes the same Incident ID before input admission and records only safe
Provider/account, stage, elapsed, result, code, and retryability metadata. The
daemon operational wrapper records the matching terminal event without
operation ID, limit values, credential data, or raw request/result payloads.

The service verifies rebuilt Projection revision/digest before success, while
the account directory exposes only safe policy revision and limits. Swift
strictly validates result shape, account scope, limits, digest, and
RFC3339Nano time. Store then requires a fresh exact-account setup projection
matching the returned revision and limits. The UI summarizes the active ceiling
and offers a dedicated Limits sheet; a policy CAS conflict remains visible and
recoverable on that account without changing global setup readiness.

Focused request/model/Store/UI/diagnostic regressions, the large-type UI render,
complete affected Go tests, focused race, vet, daemon wire, and strict Swift/Go
probes pass. The latest unfiltered Swift suite passes 174 XCTest cases with one
intentional skip and seven Swift Testing contracts; five consecutive
no-rebuild full-suite runs also pass. The controlled four-Provider source matrix
is also complete: Codex/OpenAI, Claude
Code/Anthropic, Loom Native/Kimi, and Loom Native/MiniMax retain distinct frozen
bindings, account-local accounting, and Agent-local rejection/revocation.

An uninstalled v0.5.2 build 25 Candidate supersedes build 24 for the current
source-to-binary boundary. It includes the production Provider Account policy
path, atomic Runtime plus account reservation, frozen policy revision/digest,
account-local Store/UI governance, and safe app/daemon Incident diagnostics.
Deep signature, arm64, owner-only permission, no-symlink, and ZIP extraction
with byte-for-byte App/daemon comparison pass. It was neither launched nor
installed, so installed build 22 remains unchanged and the Candidate does not
close DeepSeek r6, Vault CV6, real Provider accounting, or the mixed-Team live
matrix.

`CURRENT / EXACT CONVERSATION ACCOUNT ROUTING CANDIDATE`: W2A now publishes one
immutable Conversation Profile for every exact verified brokered Provider
Account. Primary Profile IDs remain compatible with persisted conversations;
non-primary IDs freeze the validated account suffix and credential revision.
The daemon no longer resolves an ordinary conversation through a Provider-only
primary credential lookup. It matches the selected Profile to one exact
Provider Account record and leases the Vault by Provider, account, opaque
reference, and revision. Ambiguity, revocation, malformed scope, or revision
drift fails closed before Provider dispatch.

The strict Swift wire model rejects native/brokered auth drift, cross-Provider
accounts, and invalid revisions. The route picker names the exact Provider
Account and Model. Focused Go tests, affected-package suites, race, vet, and the
full Swift suite pass; the latter contains 175 XCTest cases with one intentional
skip and eight Swift Testing contracts. One full repository Go run observed the
existing one-second `internal/localipc` client boundary time out under parallel
load; that target then passed ten consecutive repetitions and the full package
was rerun separately. This evidence does not replace installed DeepSeek r6,
Vault CV6, real accounting, or mixed-Team live acceptance.

An uninstalled v0.5.2 build 26 Candidate now packages this exact-account
Conversation routing slice and supersedes build 25 for source-to-binary
attribution. Deep signing, arm64 identity, owner-only permissions, no-symlink
checks, and ZIP extraction with byte-for-byte App/daemon/plist comparison pass.
It was not launched or installed. Installed build 22 and its managed daemon
remain unchanged, and all live gates above remain open.

`CURRENT / ANTHROPIC CONVERSATION ROUTE SOURCE CANDIDATE`: W2A now publishes
account-scoped Anthropic Messages Profiles and dispatches them through the exact
Provider/Account/reference/revision Loom Vault lease. The fixed official
Messages transport is proxy-free, redirect-free, bounded, secret-free outside
the request header, and accepts only matching assistant text content. The Swift
Store preserves one visible Conversation and creates a new immutable Segment
when switching DeepSeek to Anthropic.

Complete affected Go packages, focused race, vet, strict Swift wire/Store tests,
and the complete 175-XCTest/eight-Swift-Testing suite pass. One full repository
Go run timed out two pre-existing five-second Codex/Pi daemon journeys under
parallel load; the complete daemon package had already passed on the same source
and both targets then passed ten consecutive repetitions. Installed Anthropic
reply and the broader W2A/W2D live gates remain open.

`CURRENT / INSTALLED STATE CORRECTION AND BUILD 28`: the running App changed
outside this task to a separately built v0.5.2 build 26. It is not byte-identical
to this task's build 26 Candidate and no package provenance is inferred. The
read-only setup projection reports an unlocked LocalKeyFile Vault,
`deepseek.primary` verified at revision 6, and the r6 Conversation Profile.
Persisted Segment metadata proves Codex to DeepSeek r3 to DeepSeek r6 continuity
inside one visible Loom Conversation.

The live response gate remains open. The latest correlated r6 Attempt reached
`provider_http` and failed `invalid_response`; it did not fail in credential,
Vault, IPC, Profile publication, or route selection. Build 28 accepts only a
bounded safe Provider-resolved model name as non-authoritative metadata and
adds the closed privacy-safe rejection reasons `response_json`,
`response_model`, `response_choices`, `response_role`, and `response_content`.
No Provider body or conversation content is recorded.

Build 27 remains an uninstalled Anthropic Messages Candidate. Build 28
supersedes it for the current source-to-binary boundary and passes the complete
repository Go suite, Provider race, affected vet, the full 175-XCTest plus
eight-Swift-Testing suite, arm64/deep-signature/permission/no-symlink checks,
and transport byte equality. Neither Candidate was installed. DeepSeek reply,
Anthropic reply, Vault CV6, real accounting, and the mixed-Team live matrix stay
inside the single Phase 2D Goal.

`CURRENT / EXACT BUILD 28 INSTALLED OBSERVATION`: after packaging, another
process installed and restarted the exact build 28 Candidate. This task did not
perform the install. Installed App/daemon/plist hashes match the retained
artifact, deep signature and canonical managed-daemon argv pass, and a read-only
setup request confirms automatic LocalKeyFile Vault unlock plus the verified
DeepSeek revision 6 Profile. No post-install chat Attempt exists. Startup and
state recovery are accepted for this exact bundle; DeepSeek response acceptance,
Anthropic live dispatch, Vault CV6, real accounting, and the mixed-Team matrix
remain open inside Phase 2D.

`CURRENT / BUILD 29 PER-AGENT CATALOG FREEZE REPAIR`: the ongoing Phase 2D
audit found that the Codex/OpenAI product Profile set `high` reasoning but did
not require the observed `reasoning_effort` capability. RED proved the Profile
was not freezable; GREEN adds `reasoning_effort` beside `workspace_edit` and a
shared gate now freezes every published Agent role option against its exact
Runtime/model before packaging.

The controlled four-Provider execution and account-isolation matrices were
rerun successfully: one Codex/OpenAI rejection remains local while Anthropic,
Kimi, and MiniMax peers succeed; DeepSeek revocation leaves the OpenAI role
ready; Provider Account rate and budget policy remain account-local. Complete
Go, focused race/vet, and full Swift suites pass. Build 29 is retained as an
uninstalled Candidate and does not overwrite exact installed build 28 evidence.
All user-triggered DeepSeek/Anthropic, Vault CV6, real accounting, and installed
mixed-Team gates remain in the single Phase 2D Goal.

`CURRENT / BUILD 30 TEAM BOARD STRICT GOVERNANCE WIRE`: Swift previously
accepted contradictory per-Agent binding and accounting facts that the Go
projection never intentionally emits. The W2D client boundary now validates
the exact Harness/Provider/Account/Model/credential tuple, native empty-account
shape, Attempt usage/cost invariants, account-local failure rate, budget and
accounting counts, policy ceilings, and currency aggregates. Malformed data
fails closed instead of being rendered as a plausible Agent row.

The daemon's explicit `aggregation_overflow` marker now appears as
`Accounting incomplete` in the Team inspector, without changing peer Agent
status or converting the Team to offline. Full Swift, API, daemon, package, and
artifact checks pass. Build 30 remains uninstalled; all real Provider and
mixed-Team gates stay under the existing Phase 2D Goal.

`CURRENT / BUILD 31 FROZEN AGENT LIMITS AND CAPABILITIES`: the Board no longer
drops the timeout, optional binding budget, or canonical capability set after
per-Agent preflight. The daemon projects those facts directly from each
Attempt's immutable execution binding, and Swift fail-closes malformed,
contradictory, duplicate, or unsorted values. The Team inspector shows all
three on the exact Agent row and distinguishes binding budget from the Provider
Account policy ceiling.

No credential reference, endpoint fingerprint, secret, Prompt, Provider
response, or hidden reasoning was added to the wire. Complete Go, focused API
race, affected vet, full 176-XCTest/eight-Swift-Testing, deep signature,
owner-only permission, no-symlink, and ZIP byte-equality gates pass. Build 31
remains uninstalled and does not replace the installed build 28 live evidence;
the DeepSeek/Anthropic reply, Vault CV6, real accounting, and four-Provider
mixed-Team gates remain open inside the same Phase 2D Goal.

`CURRENT / BUILD 32 AGENT VAULT AND PROVIDER FAILURE STAGE PRESERVATION`: the
Agent dispatch audit found two diagnostic boundary defects. Vault AAD/decrypt
stages were flattened to `credential_lease_issue`, and every Provider callback
failure was flattened to `credential_unavailable`. The repaired path preserves
only closed non-secret Vault/lease stages while allowing typed Provider and
Harness failures to reach their existing Adapter classifiers.

Envelope, VaultStore, lease manager, exact per-Agent credential access, Loom
Native, and Claude Code Harness regressions prove the stage chain and reject
private error-body disclosure. Complete Go, affected race/vet, full
176-XCTest/eight-Swift-Testing, deep signature, owner-only permission,
no-symlink, and ZIP byte-equality gates pass. Build 32 remains uninstalled and
supersedes build 31 only for source attribution. Installed Provider-stage
observation and all existing Phase 2D live gates remain open.

`CURRENT / BUILD 33 AGENT FAILURE DIAGNOSTIC BOARD`: the W2D read model now
enriches the exact current Agent Attempt from bounded operational diagnostics
without converting those records into Journal authority. Incident ID,
Provider, Provider Account, and Model must all match; unknown, malformed,
duplicate, cross-account, or stale failure records are omitted. Diagnostic
unavailability does not alter Agent or Team status.

The strict Swift wire exposes an explicit availability bit, safe stage/code,
and retryability. The Team inspector shows the stage and recovery posture only
on the affected Agent row and preserves Incident ID copy support. Complete Go,
focused race/vet, full Swift, native reproducibility/smoke, and package gates
pass. Build 33 remains an uninstalled Candidate; installed DeepSeek/Anthropic,
Vault CV6, real accounting, and four-Provider mixed-Team gates remain open in
the single Phase 2D Goal.

`CURRENT / BUILD 36 FOUR-AGENT CARDINALITY REPAIR`: W2B/W2C used the shared
nine-Agent contract in Team definition and execution-plan code, but four legacy
normalizers still imposed a two-sub-Agent ceiling. Saved Team binding,
structured draft content, accepted-draft role seeding, and accepted-plan shape
validation now all use `MaxTeamAgentCount - 1`.

Four-role RED/GREEN tests follow the required mixed-Team shape through frozen
per-role bindings, dormant Saved Team instantiation and record projection,
structured draft acceptance, and accepted-draft instantiation. A max-plus-one
test preserves fail-closed capacity. Each role still owns its exact Harness,
Provider Account, credential revision, Model, limits, capabilities, and binding
digest; no Team-global Provider selection was added.

Complete Go, Teams/Provider race, affected vet, full Swift, reproducible native
build/smoke, and package-integrity gates pass. Build 36 is retained as an
uninstalled Candidate. Another process installed build 35 from the shared
worktree; its daemon is byte-equal to the build 36 daemon, but its App and plist
are different artifacts, so it is not exact build 36 provenance. The real
installed four-Provider Team, account-local failure, Provider accounting,
fallback, DeepSeek restart, and Anthropic reply gates remain in the one Phase
2D Goal.

`CURRENT / CREDENTIAL VAULT PRODUCTION-DEFAULT REVALIDATION`: the accepted
ADR-0020 and P2D-W2D CV1-CV6 contract remain the active Credential Vault
boundary. The production daemon builder now has an explicit regression proving
that its default product path owns `productCredentialVaultRuntime` and projects
an `unlocked` LocalKeyFile Vault without any caller-supplied test flag.
Configure/verify, Conversation routing, and Agent dispatch share the exact
Vault lease source; the Keychain adapter remains only for legacy tests and an
optional explicit one-time migration path.

The focused Vault package and complete daemon package pass. This evidence does
not close CV6 or replace the installed live gates: post-restart DeepSeek and
Anthropic replies, no helper process during repeated calls, mixed-Team
account-local isolation, real accounting, and explicit fallback observation
remain required inside the same Phase 2D Goal.

`CURRENT / P2D-W2B/W2C FOUR-ROLE PRODUCT PATH`: persisted Saved Team
configuration now has direct source evidence through daemon materialization,
Projection Mission binding, preflight, compilation, and the existing
four-Provider controlled dispatcher canary. Codex/OpenAI, Claude
Code/Anthropic, Loom Native/Kimi, and Loom Native/MiniMax keep independent
Runtime, Provider Account, credential reference/revision, Model, limits,
capabilities, Attempt, and binding digest facts.

Revoking `kimi.primary` blocks only the Kimi role and preflight row; the other
three roles remain ready, and start fails closed rather than silently changing
Provider or dropping the role. Focused tests, race, vet, complete App/daemon
packages, and the clean repository Go suite pass. This does not replace the
installed mixed-Team, real Provider, accounting, fallback, or Vault CV6 gates,
which remain open in the single Phase 2D Goal.

`CURRENT / P2D-W2C AUTHORITATIVE ROLE CONTEXT SOURCE ASSEMBLY`: the built-in
Mission compiler now assembles each role's admitted Capsule from the confirmed
Mission objective, exact WorkPackage policy, current task and plan identity,
role governance, and exact Artifact revision bindings. Each item is typed,
scoped, prioritized, content-addressed, and attributed to an authority source.
Required policy and governance omissions fail closed; budget-omitted Artifact
items remain visible in the content-free omission manifest.

Ordinary retry demonstrably reuses the same Capsule digest and disclosure
receipt because attempt identity belongs to the authoritative Attempt record,
not to mutable model context. An explicit approved fallback creates the new
route-bound Capsule and includes only the safe approval identity, version,
time, digest, and source/target binding digests. It does not expose the approval
actor, Provider credential reference, Provider Account, API key, or secret.
The canonical payload reaches Pi, Loom Native, Codex, and Claude Code through
their bounded v2 adapters; Loom Native and Pi retain a 32 KiB hard transport
ceiling for the 2048-token source budget.

Focused RED/GREEN, complete affected packages, the cold-runtime two-Attempt
terminal regression, affected race, `go vet ./...`, and clean serialized
`go test -p 1 ./... -count=1` pass. This remains source-only and advances the
existing P2D-W2C WorkItem. Complete Goal/constraint/decision/workspace assembly,
encrypted Capsule body restart, richer model-specific ContextAdapter output,
installed mixed-Team execution, and real Provider acceptance remain open under
the single Phase 2D Goal.

`CURRENT / P2D-W2B/W2C EXECUTION PROFILE GOVERNANCE PROJECTION`: the complete
role-local Execution Profile was already frozen internally, but Builder and
Mission preflight omitted primary auth mode and several fallback facts. The
wire now carries primary and fallback Harness, Provider, Provider Account,
Model, auth mode, credential revision, reasoning effort, timeout, budget, and
capabilities. Go and Swift independently fail closed on native/account-bound
credential contradictions, invalid limits, noncanonical capabilities,
contradictory readiness, and partial fallback state.

Mission preflight and Builder review show these values on the affected Agent,
while fallback remains explicit and approval-bound. No credential reference,
endpoint fingerprint, secret, Prompt, Provider body, or Team-global Provider
client enters the UI contract. This remains a source-only increment under the
single Phase 2D Goal; all installed mixed-Team, Vault CV6, real accounting,
fallback, failure-isolation, and Provider reply gates remain open.

Verification passes complete Swift, focused cross-language contract probes,
affected Go race, repository vet, and the clean serialized repository Go suite.

`CURRENT / P2D-W2B/W2D INDEPENDENT AGENT ROUTE EDITOR`: the Builder now keeps
Agent role identity separate from Harness, Provider Account, Model, reasoning,
timeout, budget, and explicit fallback controls. Harness and Provider Account
changes compose a new immutable Profile from the current Agent and one approved
compatible route instead of adopting an entire source Role/Profile.

Provider Account edits require the current Harness/runtime, replace the exact
Provider/Account/auth/endpoint/credential tuple plus its compatible Model, and
preserve reasoning, timeout, budget, and capabilities. Harness edits require
the target to publish the current Account, credential revision, and Model, then
replace only Adapter/runtime. Every candidate freezes against the exact Runtime;
cross-Agent sources and incompatible combinations fail before Draft mutation.
The Swift menu identifies account credential revisions, keeps custom Profile
selection state, and exposes no credential reference or secret.

Focused RED/GREEN, Store/API forwarding, complete App/API packages, affected
race, repository vet, clean serialized repository Go, and the complete
177-XCTest/eight-Swift-Testing suite pass with one intentional skip. This is
source-only progress inside the existing W2B/W2D WorkItems. No new Goal was
created; all installed mixed-Team, Vault CV6, real accounting/fallback/failure
isolation, and Provider reply gates remain open in Phase 2D.

`CURRENT / INSTALLED BUILD 39 DEEPSEEK LIVE ACCEPTANCE`: read-only inspection
of the currently running v0.5.2 build 39 installation supersedes the prior
blanket statement that all real Provider replies remain open. The exact
installed App and daemon identities, signature, process start times, owner-only
Vault files, verified DeepSeek account revision 6, encrypted Vault row, and
schema-v2 Conversation metadata were checked without reading credentials,
conversation content, or Provider bodies.

After the current build-39 App and daemon startup, the same visible Conversation
transitioned from a Codex Segment to a DeepSeek r6 Segment and produced five
successful real DeepSeek Attempts. Each Attempt has an immutable binding digest,
Context Capsule digest, and matching successful App/daemon Incident chain.
Earlier Provider timeouts are followed by these successful Calls. This closes
the DeepSeek import/verify/Profile/reply/multi-turn and Segment-continuity portion
of P2D-BLOCKER-1.

Phase 2D remains the single `ACTIVE / PARTIAL` Goal. Anthropic reply, installed
four-Provider mixed-Team execution, account-local live fault isolation and
accounting, explicit fallback, encrypted transcript/Capsule/native-handle
storage, rotation continuity, and instrumented proof that normal Provider calls
do not invoke the optional migration helper remain open. Exact build-39
installed hashes were captured, but no retained build-39 manifest was found;
no retained-Candidate byte-equivalence claim is made.

`CURRENT / P2D-W2C ENCRYPTED TEAM CONTEXT CAPSULE STORE`: the Loom Vault now
stores Team Mission dispatch Capsules under the domain-separated Conversation
key hierarchy. Each Conversation has a random DEK wrapped by the VMK-derived
`loom/conversation-wrap/v1` KEK; each content-addressed Capsule has an
independent AES-GCM data nonce and AAD-bound immutable Authority record and
disclosure receipt. Database uniqueness constraints reject nonce reuse.

Exact restart lookup, tamper/substitution rejection, Conversation-local
crypto-erasure, and atomic VMK rewrap are covered by RED/GREEN tests. The Team
Mission compiler persists every primary/fallback Capsule before dispatch-frame
assembly and fails closed when the Vault is locked, recovering, or unavailable.
There is no plaintext fallback and the Store does not become execution
authority.

Complete affected packages, targeted race, repository vet, and the serialized
repository Go suite pass. The full suite listed every package as `ok`; only a
post-suite shell marker failed because zsh reserves the variable name `status`.
This remains one increment inside P2D-W2C/W2D. Ordinary-conversation transcript
and route-Capsule encryption, ExternalSessionHandle storage, encrypted context
export/restore, installed migration/restart, and the mixed-Team live matrix
remain open under the single Phase 2D Goal.

`CURRENT / P2D-W2A ENCRYPTED ORDINARY CONVERSATION STORE`: the default daemon
source now persists one encrypted document per Loom thread under the
Conversation DEK instead of using plaintext JSON as the normal store. Transcript
and Context Capsule ciphertexts use one Conversation-wide nonce registry,
monotonic AAD-bound revisions, exact idempotency, tamper rejection, atomic VMK
rewrap, and Conversation-local crypto-erasure.

Existing `chat-threads.json` is migration input only. Encrypted documents commit
before an atomic pending rename; pending plaintext must be identity-checked,
overwritten, fsynced, removed, and parent-fsynced before chat starts. Partial
migration resumes, a Vault/legacy mismatch fails closed without deleting the
source, and unsafe filesystem identity is rejected. Recovery mode keeps the
governance surface available while disabling chat rather than falling back to
plaintext.

Complete affected packages, targeted race, repository vet, and the serialized
repository Go suite pass with direct `exit=0`. This is source-only: installed
build 39 still has its prior schema-v2 plaintext store. Installed migration,
encrypted restart/reply, diagnostics/UI, native session handles, encrypted
context backup/restore, and mixed-Team live acceptance remain open under the
single Phase 2D Goal.

`CURRENT / UNINSTALLED BUILD 40 CANDIDATE`: the encrypted ordinary Conversation
Store and resumable legacy migration are packaged as v0.5.2 build 40. Static
arm64, deep-signature, owner-only bundle, no-symlink, and ZIP byte-equivalence
checks pass; exact identities are frozen in the Candidate manifest. The
build-only script passed. Launch smoke was intentionally not run because it
could migrate the current user's installed build-39 transcript before explicit
approval.

The installed App and data remain unchanged. This is packaging evidence only,
not installed migration or Provider acceptance. Encrypted restart/reply,
migration diagnostics/UI, Provider-native handle encryption, no-helper
instrumentation, Anthropic, mixed-Team isolation/accounting, and explicit
fallback remain open under the same Phase 2D Goal.

`CURRENT / ENCRYPTED EXTERNAL SESSION HANDLE SOURCE`: the Vault now stores
opaque Provider-native response, conversation, and prompt-cache handles under
the per-Conversation DEK. Canonical AAD freezes Conversation, Segment,
Provider, exact Account when account-bound, Model, auth mode, credential
reference/revision, kind, and monotonic handle revision. Transcript, Capsule,
and handle ciphertext share one nonce registry; tamper, substitution, route
drift, revision drift, and cross-channel nonce reuse fail closed.

The production daemon exposes a callback lease that zeroizes plaintext after
use. Native auth binds empty account/credential metadata rather than inventing
an account. Existing conversation Providers remain stateless because none
currently advertises a reusable native-session capability. This is unbuilt
post-build-40 source progress, not installed stateful Provider acceptance.
Installed migration/restart/isolation, explicit adapter capability, no-helper
instrumentation, and the mixed-Team live matrix remain in the unified Goal.

## Build 41 and migration-failure governance amendment

Build 41 packages the encrypted transcript, Context Capsule, and
ExternalSessionHandle source. It is an unlaunched, uninstalled Candidate and
does not include the migration-failure observability increment that follows it.
The next package containing that increment must use build 42 or later.

Within existing P2D-W2A and P2D-W2D, encrypted Conversation startup now emits
one correlated, privacy-safe terminal event for each `migration_read`,
`migration_commit`, and `migration_cleanup` stage. A failed migration preserves
the legacy source and no longer takes down Provider setup, Vault recovery, Team
governance, or the rest of loomd. Chat alone remains unavailable and projects a
bounded `state_unavailable` record with stage and Incident ID.

The Swift wire model fail-closes unknown fields, unsafe identifiers,
non-migration stages, and any availability failure paired with `can_reply`.
The Conversation surface renders the existing diagnostic banner with a
stage-specific recovery action, diagnostics preview, and Incident copy. It does
not retransmit a user draft as a migration retry. No transcript, thread
identity, Prompt, Provider response, credential, ciphertext, nonce, or wrapped
DEK enters operational diagnostics.

This source increment strengthens the horizontal observability gate; it does
not change the accepted CV1-CV6 ordering or create another Goal. Installed
migration/restart/reply, optional explicit Keychain migration, no-helper
instrumentation, Anthropic, mixed-Team execution, account-local isolation,
accounting, and fallback acceptance remain open in the single Phase 2D Goal.

## Build 42 Candidate boundary

The migration-failure governance increment is packaged as unlaunched,
uninstalled v0.5.2 build 42. Complete serial Go, targeted race, vet, complete
Swift, release build, arm64, deep-signature, owner-only/no-symlink, and ZIP
byte-equivalence checks pass. The retained manifest freezes exact hashes and
states the dirty source provenance.

Installed build 39 and its data remain untouched. Build 42 does not close the
installed migration/restart/reply, no-helper, Anthropic, mixed-Team,
account-local isolation, accounting, or fallback gates in the unified Goal.

## Vault-only ordinary runtime source gate

P2D-W2D now forbids the ordinary daemon and setup composition path from
constructing `ProductKeychainStore`. The generic daemon defaults to the Loom
Credential Vault when no explicit credential backend is injected, matching the
already-explicit production builder. Direct setup composition fails closed
when neither the Vault mutator nor an explicitly injected legacy test boundary
is present.

The Keychain constructor is reserved only for a future, explicitly named
`credential_migration` component. A Go AST regression and the release build
script scan non-test daemon source and reject any other constructor call. This
is an additive CV3/CV4 gate; it does not reset Conversation Segments, Context
Capsules, W2A/W2B work, or the active DeepSeek live evidence.

Operational diagnostics add the bounded backend fact
`credential_runtime=vault`; `explicit_legacy` exists only for explicitly
injected legacy/test composition. Unknown or drifting values fail closed in Go
and Swift. The field is observational, contains no secret or content, and is
not execution authority.

Complete Go, race, vet, shell/source, diff, and Swift gates pass for this source
increment. Installed build 39 remains untouched. The no-helper live gate stays
open until an approved installed run observes process creation across import,
multi-turn Conversation, Agent Attempt, and restart boundaries. Keychain
remains an optional one-time migration source only; it is not a normal runtime
dependency.

## Build 43 Candidate boundary

The Vault-only ordinary runtime gate is packaged as uninstalled v0.5.2 build
43. Complete source verification, release construction, arm64,
deep signature, owner-only/no-symlink, and ZIP byte-equivalence gates pass; the
retained manifest freezes the exact artifact identities and dirty-worktree
provenance.

The App was not launched. Its exact bundled daemon was run only against a
canonical owner-only temporary state and UDS. Two daemon sessions selected the
Vault, completed lock/unlock, preserved Vault file identity across restart, and
recorded no helper process in bounded 2 ms sampling windows. No credential or
Provider request was used. Installed build 39 remains installed. Build 43 does
not close the
approved installed process-observation, migration/restart/reply, Anthropic,
mixed-Team, account-local isolation/accounting, fallback, or rotation gates.
The unified Phase 2D Goal remains `ACTIVE / PARTIAL`.

## Build 44 helper-attempt counter amendment

The build-43 bounded process sampler is superseded for ordinary isolated Vault
runtime evidence by direct process instrumentation. A process-local monotonic
counter increments immediately before every Darwin `--credential-helper`
`command.Start()` attempt and is snapshotted into operational diagnostics as
`credential_helper_spawn_attempts`. Swift diagnostic preview/export preserves
the optional field for backward compatibility. The metric is non-secret,
session-local, observational only, and is not execution authority.

Build 44 packages this increment as an uninstalled, unlaunched Candidate. Its
exact bundled daemon ran twice against one fresh owner-only Vault; all five
lock, unlock, and content-free invalid-chat records reported
`credential_runtime=vault` and `credential_helper_spawn_attempts=0`, including
after restart. No credential or Provider request was used. This advances W2D
packaged-runtime observability without resetting W2A/W2B/W2C or closing CV6.

Installed import, repeated real Conversation calls, Agent Attempts, restart,
mixed-Team dispatch, and the optional one-time migration helper still require
approved counter-backed observation. Installed build 39 remains untouched and
the unified Phase 2D Goal remains `ACTIVE / PARTIAL`.

## Ordinary Conversation disclosure receipt projection amendment

Within existing P2D-W2A, ordinary Conversation Segments and dispatch Attempts
now freeze the non-secret disclosure receipt digest and shared/omitted item
counts beside the Context Capsule. The Provider responder request carries the
same tuple, and execution binding schema version 2 includes it. Stored current
records recompute that binding and reject receipt or count drift. Pre-receipt
records remain compatible only with an empty receipt and zero counts.

The Swift timeline exposes the tuple as a native expandable Context summary on
the first turn of each Segment. This is a governance projection, not execution
authority, and contains no Prompt, transcript content, Provider response,
credential, ciphertext, nonce, or hidden reasoning.

Complete Go, affected race, vet, 20-run stability, diff, and complete Swift
gates pass. This source is newer than and not contained in the frozen build 44
Candidate. It is packaged as unlaunched, uninstalled build 45 with passing
release, arm64, deep-signature, owner-only/no-symlink, wire-contract, and ZIP
byte-equivalence gates. Installed build 39 remains untouched, W2A/W2B/W2C are
not reset, and the unified Phase 2D Goal remains `ACTIVE / PARTIAL`.

## Agent failure governance action amendment

Within existing P2D-W2D, the Team Pulse inspector now keeps recovery actions on
the exact failed Agent row. A retryable diagnostic exposes Review retry, which
routes to the existing Attention decision surface without dispatching. A
current diagnostic exposes a privacy-safe diagnostic preview, and the existing
Incident action remains available for copy. Healthy peer Agents and Provider
Account accounting rows do not inherit any of these actions.

The action metadata is presentation-only and fail-closed. It does not approve
fallback, mutate a frozen binding, write Journal authority, or expose secret or
content fields. Operational diagnostics remain separate from authoritative
business facts.

A source fixture proves a failed Anthropic Agent can expose all applicable
actions while a successful OpenAI/Codex peer remains visible and action-free.
The complete Swift suite and native wide/compact appearance matrix pass. Build
45 predates this increment; it is packaged in the unlaunched, uninstalled build
46 Candidate with passing release, arm64, deep-signature,
owner-only/no-symlink, action-string, wire-contract, and ZIP byte-equivalence
gates. Installed mixed-Team failure isolation and all existing live gates stay
open under the single Phase 2D Goal.

## Managed workspace and Vault concurrency amendment (2026-08-12)

`CURRENT / P2D-W2C OBSERVED MANAGED SOURCE BASELINE`: the Mission compiler now
observes the managed source once per compile and places a path-free
`workspace-snapshot` in each Role Context Capsule. The item is explicitly
observed rather than authoritative and exposes only tree digest, counts, and
total bytes. Top-level `.git`, names, paths, and file content are excluded.
Ordinary retry retains the same Capsule digest; source drift before adapter
dispatch fails closed with `source_changed`. This does not fabricate Goal,
confirmed-constraint, decision, or observed-test items whose authority source
does not yet exist.

`CURRENT / P2D-W2D CV3 ACCOUNT-LOCAL LEASE CONCURRENCY`: the Vault runtime now
releases its lifecycle mutex before entering the exact short-lived lease
callback. DeepSeek and Anthropic account callbacks can overlap, while the lease
manager continues to own cancellation, revoke, rotation barriers, expiry, and
zeroization. The prior serialized behavior was captured as RED and the focused
normal/race matrix is GREEN.

These are increments inside existing W2C/W2D and ADR-0020; no Goal or WorkItem
was created. Complete source, race, vet, Swift, release, arm64, deep-signature,
permission, no-symlink, contract-string, and ZIP byte-equivalence gates pass.
They are frozen in unlaunched, uninstalled v0.5.2 build 47. Installed build 39,
CV6, real Provider Account accounting, approved fallback, and the mixed-Team
live matrix remain open under the single Phase 2D Goal.

## Canonical Mission Context and exact restart authority amendment (2026-08-12)

Within existing P2D-W2A/W2D, Mission commands now distinguish legacy
objective-only input from Canonical Mission Context v1. The v1 command freezes
the Goal, ordered confirmed constraints, and ordered accepted decisions in the
preflight digest and emits them as separate authoritative Role Capsule items.
Observed workspace state remains separately classified.

Preflight is zero-write. Start encrypts the complete canonical Capsule plus its
derived dispatch payload in the Loom Vault. Journal and operational diagnostics
retain only non-content authority metadata. Restart uses the projected Attempt
authority, the Vault authority manifest, and the complete frozen execution
binding to recover the exact Capsule; it never authorizes current-source
recompilation as a replacement. Missing body, ambiguous route, credential or
limit drift, authentication failure, and source drift fail closed before
Provider dispatch.

This is an increment to the existing Conversation Segment/Context Capsule and
Credential Vault contracts, not a new Goal or WorkItem. Swift now preserves the
same v1 context from preflight through start, while Workbench editing and
confirmation UX remains open. The source is frozen in unlaunched, uninstalled
v0.5.2 build 48 with passing release, arm64, deep-signature, permission,
no-symlink, contract-string, and ZIP byte-equivalence gates. Installed build 39
and all existing live gates remain unchanged under the sole `ACTIVE / PARTIAL`
Phase 2D Goal.

## Mission Context confirmation UX amendment (2026-08-12)

Within existing P2D-W2A/W2D, the Mission Workbench now makes Canonical Mission
Context v1 visible and explicitly confirmable before preflight. The Goal,
ordered confirmed constraints, and ordered accepted decisions remain distinct
authority classes and flow unchanged into the existing preflight/start digest
and encrypted Role Capsules.

Any Objective, Team, work-type, constraint, or decision edit revokes user
confirmation and invalidates the prepared preflight. A generation fence drops
late async preflight success after invalidation. Unchecking confirmation also
revokes Start authority. Canonical validation rejects whitespace drift,
oversized or empty entries, and duplicates within or across authority classes
before IPC.

This is a UI and client-state increment inside the existing WorkItems, not a new
Goal or WorkItem. Complete Swift verification and native light/dark large-type
layout gates pass. The increment is packaged in unlaunched, uninstalled v0.5.2
build 49. Installed interaction/pixel review and all existing Provider,
mixed-Team, fallback, accounting, and CV6 live gates remain open under the sole
`ACTIVE / PARTIAL` Phase 2D Goal.

## P2D-W2C product Attempt Loop Runtime Context v1 (2026-08-13)

The first production Runtime vertical slice is now source verified. Product
composition creates one Attempt Loop authority beside the encrypted Attempt
Payload authority and decorates all configured Runtime Adapters. For an exact
Role Context Capsule, the decorator runs only after `RunStarted`, freezes the
non-secret claim/Incident identity and bounded loop policy, and records the
model request before calling the Runtime.

`loom_read_context` now crosses the common governed Context delivery seam used
by Pi, Codex, Claude Code, and Loom Native. Admission, exact scoped capability
authorization, dispatch, accepted result causation, delivery proof, and
Step/Turn closure are replayable. Route or binding substitution fails before
the Runtime delegate; disclosed content and Prompt remain outside Journal and
diagnostics.

Exact evidence is
`../phase2d/P2D-W2C-product-attempt-loop-runtime-context-v1.md`. This does not
close the general Tool Gateway: local/Web/MCP tools, one-shot approval, sandbox
evidence, uncertain-side-effect recovery, Queue/Steer/Inject, Swift UI,
installed CV6, and live mixed-Team acceptance remain open. Phase 2D remains the
sole `ACTIVE / PARTIAL` Goal.

## Conversation Route transition confirmation amendment (2026-08-12)

Within existing P2D-W2A/W2D, changing an active Conversation's execution route
now requires one explicit Segment review. The sheet shows the old and new
Harness, Provider Account, Model, and credential revision and requires one of
the three accepted Context Capsule disclosure modes. The previous detached
Context mode control is removed so there is one decision point.

The pending transition freezes the exact source and target Profile records,
thread anchor, and client generation. Any drift fails closed before the
selection changes. Blank Conversations retain direct first-Profile selection.
Confirmation uses the existing same-visible-Conversation Segment mechanism;
it does not mutate an old Segment, reuse Provider-native state, approve
fallback, or create execution authority.

Complete Swift and native appearance/accessibility layout gates pass. This is
an increment to the accepted Conversation Route Segment and disclosure design,
not a new Goal or WorkItem. It is frozen in the unlaunched, uninstalled v0.5.2
build 50 Candidate with passing release, arm64, deep-signature,
owner-only/no-symlink, route-contract, and ZIP byte-equivalence gates.
Installed Codex-to-DeepSeek reply, restart, mixed-Team, accounting, fallback,
and CV6 gates remain open under the sole `ACTIVE / PARTIAL` Phase 2D Goal.

## Multi-Agent Team Builder authoring amendment (2026-08-12)

Within existing P2D-W2B/W2D, Team Builder now authors multiple independent
sub-Agents instead of treating `subagent` as one global editable slot. Every
sub-Agent mutation is scoped by an exact stable AgentDefinition identity.
Missing or stale targets, duplicate identities, invalid removal, and
cardinality overflow fail before Draft revision or binding-digest mutation.

The product catalog publishes Bounded Worker, Reviewer, Researcher, and
Verifier identities for every compatible execution route. The App can add and
remove Agent rows and independently edit Harness, Provider Account, Model,
credential revision projection, reasoning, timeout, budget, and explicit
fallback. Secrets and credential references are not projected to Swift.

This closes the App authoring gap for the existing per-Agent Runtime Profile
and FrozenExecutionBinding chain; it does not close execution-start isolation.
The current compiler still refuses the entire start when any required role is
blocked because there is no authoritative initial-blocked plan-node state.
W2D must add that authority before healthy siblings can start while the failed
Agent remains blocked and visible. Installed four-Provider authoring/dispatch,
account-local failure isolation, accounting, approved fallback, and CV6 remain
open. The source is packaged only as unlaunched, uninstalled v0.5.2 build 51.

## Authoritative initial Agent failure-isolation amendment (2026-08-12)

Within existing P2D-W2C/W2D, a single blocked Agent is now represented as
append-only execution authority instead of a Team-global preflight rejection.
The compiler produces a bounded direct failure for that Agent and canonical
`dependency_blocked` facts for declared dependents. Independent siblings stay
eligible for dispatch. If every node is blocked, the Team still receives a
terminal observable plan with zero Attempts.

`TeamExecutionPlanned` freezes a public initial Route summary per Agent:
Harness, Provider, Provider Account, Model, reasoning, timeout, budget,
capabilities, and credential revision. It deliberately excludes credential
reference, endpoint fingerprint, binding digest, secret bytes, Prompt,
Conversation/Capsule content, and Provider response. Attempt 1 must match this
summary. Explicit fallback remains a new approved Attempt and cannot rewrite
the initial Route.

The coordinator creates no Attempt, grant, capacity reservation, or Adapter
call for a blocked node. Capacity admission considers only dispatchable nodes,
so an unavailable Runtime belonging exclusively to an already blocked Agent
cannot stop a healthy sibling. Replay validates failure provenance,
dependency lineage, retryability, UTF-8 safety, and exact stage/code grammar.

Team Pulse shows the frozen Route and safe Incident diagnostic before an
Attempt exists, keeps recovery actions on the affected Agent row, uses `Not
started`, and labels native credentialless routes `Native auth`. Swift Store
readiness uses the same partial-dispatch contract: at least one `ready` Agent
permits Start, while an all-blocked preflight remains fail closed. This closes
the build 51 source gap. Build 52 was rejected by this static product review
and was never installed or launched. Installed four-Provider execution, real
revocation, rate-limit and timeout isolation, account-local cost accounting,
approved live fallback, and Credential Vault CV6 remain open. Build 53 is the
corrected source and static Candidate; installed Loom remains build 39.

## P2D-W2D account-local Provider timeout classification (2026-08-12)

Phase 2D remains the only active product Goal. This increment does not create a
Credential Vault Goal or reset the DeepSeek Conversation, Segment, Capsule, or
per-Agent binding work. ADR-0020 and CV1-CV6 remain inside W2D and supply exact
credential leases to W2A/W2B/W2C; normal runtime is Vault-only, Keychain is an
optional explicit one-time migration source, and no plaintext fallback exists.

Native OpenAI-compatible Agent calls and bounded Claude/Codex Harness gateways
now distinguish request transport timeout (`timeout / provider_connect`) from
response read timeout (`timeout / provider_http`). Both are retryable and
remain bound to the exact Incident, Provider Account, Model, and Attempt. Board
enrichment rejects cross-account and unknown-stage records, so a failed account
cannot mark a healthy sibling or Team as generically offline.

Focused, affected normal/race, full serialized Go, vet, diff, and complete
Swift gates pass. Build 54 is packaged as an unlaunched, uninstalled Candidate
with passing release, arm64, deep-signature, owner-only/no-symlink,
timeout-contract, and ZIP byte/mode-equivalence gates. It does not satisfy
installed revocation/auth/rate-limit/timeout, accounting, fallback, or CV6
acceptance. Installed Loom remains build 39.

## P2D-W2D Provider Account governance visibility (2026-08-12)

The authoritative Board already aggregates Attempt failures, rate limits,
budgets, usage, and currency-specific costs by exact Provider Account. Team
Pulse now exposes that complete account governance summary as a distinct row,
with deterministic error-rate and decimal cost formatting plus separate account
icon/accessibility semantics. It no longer presents only failed count, tokens,
raw microunits, and policy ceilings.

This is W2D UI progress inside the sole Phase 2D Goal. It does not change
W2A/W2B/W2C bindings, Vault leases, or fallback authority, and it does not
claim installed real Provider accounting. Complete Swift and affected Go
verification pass. Build 55 is frozen as an unlaunched, uninstalled Candidate
with passing release, arm64, deep-signature, owner-only/no-symlink,
account-governance source, and ZIP byte/mode-equivalence gates. Installed Loom
remains build 39 and all installed mixed-Team/CV6 gates remain open.

## P2D-W2D Provider Account cost provenance (2026-08-12)

Run terminal accounting now distinguishes Provider-reported and
Harness-reported costs. The authority rejects new cost facts with missing,
legacy, unknown, or estimated sources; historical events with a genuinely
absent field replay as `legacy_unspecified`. Projection and strict Swift wire
preserve that distinction, while account aggregation groups by currency plus
source and Team Pulse names the provenance beside the exact decimal value.
An explicit JSON `null` is not absence and fails closed.

This amendment deliberately does not invent pricing for native
OpenAI-compatible Providers. `rate_card_estimate` remains reserved and rejected
until Loom has a versioned, Attempt-frozen Rate Card identity/revision/digest
and calculation basis. DeepSeek/Kimi/MiniMax currently project token usage with
cost unobserved; Claude Code and Pi RPC project `harness_reported` cost.

Focused RED/GREEN, affected normal and race packages, serialized full Go,
`go vet ./...`, `git diff --check`, and complete Swift verification pass. Build
57 is the current unlaunched, uninstalled static Candidate; build 56 is
superseded historical evidence after the final null-versus-missing decoder
audit. Installed real
Provider Account cost, Rate Card authority, mixed-Team isolation/fallback, and
Credential Vault CV6 remain open under the sole Phase 2D Goal.

## P2D-W2D Rate Card and Credential Vault revalidation (2026-08-12)

Phase 2D remains the only active product Goal. Provider Account and Model Rate
Cards are now append-only authority with exact Provider, account, and Model
identity. Run Claim v2 freezes the complete revision/digest/currency/token
basis/rates/rounding contract per Attempt. `rate_card_estimate` is accepted only
from that frozen card plus observed usage, remains distinct from Provider- and
Harness-reported values, and never relies on hardcoded official pricing.

The accepted Credential Vault architecture is not reopened or replaced. The
ordinary daemon defaults to LocalKeyFile Vault; Provider verification,
Conversation, and Agent dispatch use exact short-lived Vault leases and do not
start a Keychain helper. Keychain remains only an optional explicit one-time
migration source. The compatibility lease adapter requires deliberate
injection and cannot become a plaintext or implicit production fallback.

Complete Go, affected race, vet, diff, and Swift gates pass for the combined
Rate Card and Vault source. This does not close installed acceptance. Build 58
is frozen as an unlaunched, uninstalled Candidate with passing release,
architecture, signature, owner-only/no-symlink, no-implicit-Keychain, and ZIP
byte/mode-equivalence gates; installed Loom remains build 39. CV6 still
requires approved counter-backed no-helper observation, restart and rotation
continuity, per-account revoke/corruption isolation, and the DeepSeek/OpenAI/
Anthropic/MiniMax mixed-Team matrix.

## P2D-W2D four-Provider Vault revoke isolation (2026-08-12)

The source acceptance matrix now composes four role-local frozen execution
bindings with the real `CredentialLeaseManager`: Codex/OpenAI, Claude
Code/Anthropic, Loom Native/Kimi, and Loom Native/MiniMax. Each identity uses a
different Provider Account, credential reference/revision, Model, Harness,
limits, and capability set.

Revoking the exact MiniMax lease identity blocks only that Agent. The other
three independent Agents remain dispatchable and succeed. Board enrichment
requires the same Incident, Provider, Provider Account, and Model before it may
show `credential_lease_revoke / credential_unavailable`; it cannot annotate a
healthy peer or create execution authority.

This is source-level evidence for the W2D failure-isolation contract and does
not replace CV6. The installed four-Provider matrix, real account revoke and
corruption observation, no-helper counter, restart/rotation continuity,
accounting, approved fallback, and Provider replies remain open. Exact evidence
is `../phase2d/P2D-W2D-four-provider-vault-revoke-isolation.md`.

The matrix now also uses a real encrypted LocalKeyFile VaultStore. After a
close/reopen boundary, only the MiniMax ciphertext is corrupted. That Agent
fails at `vault_decrypt`; the independent OpenAI, Anthropic, and Kimi Agents
still acquire exact leases and succeed. Board enrichment preserves the same
account-local boundary, and neither the Vault database nor Team Journal
contains the plaintext test secrets. Installed CV6 remains open.

The corruption classification is now proven through the real product Agent
path, not only through coordinator and Board fixtures. A corrupted DeepSeek
record traverses VaultStore, CredentialLeaseManager, exact frozen binding
validation, the Loom Native Adapter, persistent operational diagnostics, and
an exact Board query. It produces
`vault_decrypt / credential_unavailable / not retryable`, does not call the
Provider, and persists no credential reference, secret, Prompt, ciphertext, or
private error text.

One shared retry policy now governs Loom Native and Claude/Codex Harness
diagnostics. Durable recovery stages are not directly retryable; lease
issue/expire/revoke remain retryable after authoritative state changes. This
production increment is packaged as unlaunched, uninstalled v0.5.2 build 60;
installed Loom remains build 39 and CV6 remains open under the same Phase 2D
Goal.

The W2D source matrix now also composes explicit fallback with the real Vault.
A damaged primary OpenAI record creates no implicit route switch. A second
Attempt exists only when an accepted fallback decision binds the exact primary
and target binding digests, and the target acquires a distinct encrypted
Provider Account/reference/revision. The fallback carries its own Role Context
Capsule and audit linkage.

The four-Provider Team variant keeps Claude Code/Anthropic, Loom Native/Kimi,
and Loom Native/MiniMax successful while Codex/OpenAI primary fails and an
approved `openai.main-backup` revision 5 succeeds. Five complete Vault
identities are observed exactly once and no secret reaches the Journal. This
closes source-level approved fallback plus mixed-Team isolation, while installed
approved fallback, real Provider replies, accounting, and CV6 remain open.

The same test freezes accounting per Attempt: the failed primary account has no
usage/cost, the successful approved backup receives only its own controlled
Provider-reported accounting, and all three peer accounts preserve independent
facts. Real installed Provider accounting remains open; no synthetic fixture is
treated as an invoice or live limit observation.

The same four exact records now pass production-equivalent rotation and restart:
the rotation barrier revokes an old lease, rewraps DEKs, promotes and adopts the
new LocalKeyFile, and then a reopened Vault issues fresh leases to all four
Agents. Their binding digests and credential revisions do not change. This
closes the source continuity composition only; installed real-Provider
post-rotation observation remains open.

## P2D-W2D Agent-scoped Vault recovery UI (2026-08-12)

Durable Vault failures now carry a direct recovery action on only the affected
Agent row. The action opens Runtime & Providers at the existing Credential
Vault surface. A closed stage allowlist prevents ordinary Provider failures or
retryable lease transitions from being mislabeled as Vault recovery work.
Retry review, diagnostics, and Incident ID copy remain separate governance
actions.

This production Swift increment is frozen as unlaunched, uninstalled v0.5.2
build 61 after complete Swift/Go/race/vet/diff/non-disclosure and reproducible
bundle/ZIP verification. Installed Loom remains build 39 and CV6 live gates
remain open under the same Phase 2D Goal.

## P2D-W2A/W2D Conversation Vault failure governance (2026-08-12)

Conversation dispatch now preserves the shared credential-stage retry policy
from the Vault lease through Router failure details, IPC `recoverable`, and the
frozen failed Attempt. Durable Vault decrypt or binding-integrity failures no
longer masquerade as directly retryable Conversation failures.

The chat failure banner exposes an explicit Credential Vault recovery action
only for a closed set of non-recoverable Vault stages. Provider auth, rate
limit, timeout, route conflict, and retryable lease transitions retain their
own recovery semantics. Diagnostics and Incident ID copy remain independently
available.

Source, focused race, complete Swift, full Go, and vet gates pass. This
production increment is build 62 and remains an unlaunched, uninstalled
Candidate. Installed Loom remains build 39; installed DeepSeek conversation
recovery and CV6 remain open under the sole Phase 2D Goal.

## P2D-W2A/W2D Provider Account disclosure policy v2 (2026-08-12)

Provider Account policy now versions and digests trust domain, retention mode,
and data region alongside capacity and budget. Historical v1 facts replay
unchanged and remain explicitly unspecified for disclosure. New App writes are
complete v2 policies; partial or unknown disclosure values fail closed.

The exact account policy identity and values project through setup and each
Conversation Profile. Swift requires mutation response plus authoritative
snapshot agreement, and the native account editor uses closed Pickers while
stating that Loom does not independently certify Provider guarantees.

Full source, Swift, Go, focused race, vet, and strict-wire gates pass. The
production increment is build 63 and remains unlaunched and uninstalled.
Conversation Segment/Attempt policy-digest freezing and all installed CV6 gates
remain open under the sole Phase 2D Goal.

## P2D-W2A/W2D Conversation policy binding v3 (2026-08-12)

Conversation Segments and Attempts now freeze the daemon-resolved Provider
Account disclosure policy identity and values. A policy or route transition
uses one explicit Context review and carries the exact reviewed non-secret
binding through Swift and private UDS. The daemon re-resolves current authority
and rejects any mismatch before message persistence, Attempt creation,
credential lease acquisition, or Provider dispatch.

The App keeps one visible Conversation across route conflicts and preserves the
draft, selected target, authoritative old thread, and Incident ID. Retry returns
to Segment/context review rather than silently opening a different Conversation
or adopting a changed policy. Existing schema-2 records are preserved and
upgrade by adding a new schema-3 Segment on the next turn.

Complete Swift, full Go, focused race, vet, strict-wire, UI rendering, and diff
gates pass. Exact evidence is
`../phase2d/P2D-W2A-W2D-conversation-policy-binding-v3.md`. The production
increment is frozen as unlaunched, uninstalled v0.5.2 build 64 with two
byte/mode-identical production builds and an equivalent ZIP extraction. Build
63 is historical. Installed Loom remains build 39 and all CV6 live gates remain
open under the sole Phase 2D Goal.

## P2D-W2A/W2C structured ordinary Conversation Capsule v1 (2026-08-12)

Ordinary Conversation Provider dispatch now consumes the same structured,
content-addressed Role Context Capsule and encrypted Conversation-DEK Store as
Team Agent dispatch. User turns are authoritative; prior model output is
untrusted or an explicit policy-filtered omission according to the selected
context mode. Exact Provider Account policy and route identity are resolved by
the daemon from the immutable Conversation Profile.

Capsule persistence precedes thread metadata and Provider dispatch. Encryption
failure leaves no message, Segment, Attempt, or Provider request. A subsequent
thread-document failure performs an exact Authority-bound compensation delete;
identity substitution fails closed. Real LocalKeyFile close/reopen tests recover
the Capsule and raw Vault database scans find no plaintext context.

Exact evidence is
`../phase2d/P2D-W2A-W2C-structured-conversation-capsule-v1.md`. Build 64
predates this source and remains historical. Installed Loom remains build 39;
no installed App, credential, or live Provider was touched. W2A/W2C remain
`ACTIVE / PARTIAL` for tokenizer-aware packing, Provider-specific adapters,
scoped retrieval, parallel aggregation, encrypted export, and installed CV6.

## P2D-W2C scoped Context retrieval broker v1 (2026-08-12)

The existing P2D-W2C Context Capsule boundary now includes an encrypted,
Attempt-bound source for retrieving only items omitted by deterministic token
budget. Envelope v2 keeps retrieval material under the same Conversation DEK
without changing the canonical Capsule digest, disclosure receipt, or dispatch.
Policy-filtered, access-denied, credential, hidden-reasoning, and Provider-body
material is permanently ineligible.

The daemon broker freezes exact Capsule Authority plus WorkItem, Run, claim,
Runtime, execution-binding digest, Agent, Role, item/content digest, and artifact
scope. It records a content-free operational result before disclosure and
zeroizes mutable results. Production Vault store and diagnostics are injected
as a pair; explicit compatibility runtimes receive no read capability.

Exact evidence is
`../phase2d/P2D-W2C-scoped-context-retrieval-broker-v1.md`. The source now has a
bounded Loom Native Harness/model-callable protocol, but not the generic Pi,
Codex, or Claude Code transports, persisted result delivery, or crash resume.
It is therefore not presented as end-user or Tool-Loop complete. Build 64
predates the source, installed Loom remains build 39, and no
package/install/live Provider action occurred. Phase 2D remains the sole
`ACTIVE / PARTIAL` Goal.

The source increment now also exposes one bounded Loom Native
OpenAI-compatible result wire for DeepSeek, Kimi, and MiniMax. It permits one
strict scoped read and one second Provider round, with exact digest and
classification revalidation, content-free diagnostics, and combined Attempt
usage. It does not yet provide Pi, Codex, or Claude Code transport, persistent
delivery/restart recovery, or a general multi-tool loop. The Phase and installed
boundaries above remain unchanged.

The wire is governed by a frozen `context_retrieval` Runtime capability. New
Loom Native discovery observations publish it and new DeepSeek/Kimi/MiniMax
Execution Profiles require it. Daemon broker construction and Adapter request
validation both use the exact frozen capability; mismatched broker/capability
states fail before credential access. Exact historical empty-capability runtime
records receive one append-only idempotent rediscovery event, while unknown
capabilities or other identity drift are rejected. Previously frozen Team
bindings are not rewritten or silently upgraded.

The affected Runtime, prompting, Capsule, Vault, Supervisor, Native Adapter,
app, API, and daemon race matrix passes three runs. Full repository Go tests,
vet, targeted format, diff, and module verification also pass. These are source
gates only; installed CV6 remains open.

## P2D-W2C Pi Context retrieval transport v1 (2026-08-13)

The exact locked Pi 0.82.1 Harness now has one bounded private extension/UDS
transport for the existing scoped Context broker. The Attempt must freeze
`context_retrieval` and carry matching Capsule Authority; otherwise the Adapter
fails before process start. The extension accepts only the exact child PID, one
random per-Attempt capability, one strict item/digest/artifact proposal, and one
validated result. A second tool request or any binding, digest, trust, scope,
source, peer, or secret-classification drift fails closed.

The permission is published through an explicit Runtime conformance port, not a
version-string inference. Only the metadata runner bound to the locked Pi
executable digest and unchanged filesystem identities offers the conformance.
Ordinary and historical Pi 0.82.1 observations retain their prior capability
sets and no-write semantics.

Exact evidence is
`../phase2d/P2D-W2C-pi-context-retrieval-transport-v1.md`. The locked local Pi
component passed 20 consecutive race-enabled two-round runs against a loopback
fake endpoint; full Go, affected race, vet, diff, and module gates pass. No
bundle or installed App was touched. Codex/Claude MCP, persisted result/crash
resume, general multi-tool behavior, parallel aggregation, encrypted export,
and CV6 remain open under the sole `ACTIVE / PARTIAL` Phase 2D Goal.

## P2D-W2C Codex and Claude Context MCP transport v1 (2026-08-13)

The existing scoped Context broker is now callable by Codex and Claude Code
through one private Attempt-scoped loopback MCP service. The transport is
created only after the frozen execution binding, Capsule Authority, dispatch
Capsule/receipt digests, Agent, Provider Account, Model, and auth mode agree.
The child receives one random bearer capability only through its minimal
environment. The service exposes exactly one read-only `loom_read_context`
tool, allows one successful call, revalidates digest/scope/trust/source, and
zeroizes the mutable result.

Capability publication follows the Pi fail-closed rule. Runtime discovery
publishes `context_retrieval` only for exact executable SHA-256 identities whose
real CLI component gates passed. Hash drift retains ordinary Harness operation
but no Context retrieval permission. New Profiles require the observed
capability and freeze it in the Attempt binding; only exact historical prior
capability sets are eligible for one append-only upgrade.

Each Context-enabled Attempt must re-hash and re-attest the exact executable
after Authority and frozen-binding validation but before opening the MCP
listener, acquiring a credential, or starting the Harness. A binary replaced
after discovery therefore fails closed and cannot inherit the projection's
Context permission.

Real Codex 0.144.1 and Claude Code 2.1.196 completed their native MCP flows with
isolated temporary roots, loopback fake Providers, and controlled fake keys.
The combined component matrix and the directly affected daemon Runtime/Profile
matrix each passed 20 race-enabled runs. Full Go, vet, module, and diff gates
pass. A broader daemon race matrix still exposes an unrelated existing
real-Pi-timeout/IPC flaky test and is not claimed green.

Exact evidence is
`../phase2d/P2D-W2C-codex-claude-context-mcp-transport-v1.md`. Build 64 predates
this source; no bundle, installed App, real credential, or live Provider was
touched. Persisted tool-result/crash resume, general multi-tool loops, parallel
Aggregation Attempts, encrypted export, and CV6 remain open under the sole
`ACTIVE / PARTIAL` Phase 2D Goal.

## P2D-W2C encrypted Attempt Payload Store v1 (2026-08-13)

The Credential Vault now provides an encrypted, restart-readable payload queue
for one frozen Attempt. Canonical AAD binds the exact Conversation, WorkItem,
Run, generation, Runtime, execution binding, Capsule, call, sequence, content
digest, and authenticated pending/delivered state. Payloads share the existing
per-Conversation DEK and nonce registry; delivery transition re-encrypts under
a fresh nonce. Drift, tamper, sequence reuse, status edit, and stale generation
fail closed, while rotation and Conversation-local crypto-erasure preserve
peer isolation.

Exact evidence is `../phase2d/P2D-W2C-attempt-payload-store-v1.md`. This is the
storage prerequisite for crash resume, not the completed delivery protocol.
Journal result facts and common Pi/Codex/Claude/Loom Native acknowledgements
remain open, along with general multi-tool loops, Aggregation Attempts,
encrypted export, installed CV6, and live mixed-Team acceptance. Phase 2D
remains the sole `ACTIVE / PARTIAL` Goal.

## P2D-W2C Journal-backed Attempt Payload delivery v1 (2026-08-13)

The encrypted Attempt Payload Store is now composed with the scoped Context
Retriever and a dedicated Work Authority fact stream. Payload commit precedes
`ToolResultAccepted`; only a valid Loom Native Provider continuation or a
validated Pi/Codex/Claude final Harness output can append
`ToolResultDelivered`. MCP HTTP success, Pi UDS success, and Provider request
write are explicitly insufficient.

Restart recovery reuses the same generation-, binding-, Capsule-, call-, and
digest-bound encrypted result without rerunning retrieval. Provider-native
tool-call IDs may drift without changing Loom's semantic call identity. Journal
facts contain no result body, Prompt, credential, or Provider response.

Exact evidence is
`../phase2d/P2D-W2C-journal-backed-attempt-payload-delivery-v1.md`. This is an
at-least-once bounded context-result transport with no side-effecting tool
re-execution, not a general exactly-once Tool Loop. General multi-tool
authorization/execution, terminal reconciliation, parallel Aggregation,
encrypted export, installed CV6, and live mixed-Team acceptance remain open.
No bundle, installed App, real credential, or Provider was touched. Phase 2D
remains the sole `ACTIVE / PARTIAL` Goal.

## P2D-W2C/W2D terminal Attempt Payload reconciliation (2026-08-13)

Phase 2D remains the only active product Goal. The execution runtime startup
path now reconstructs exact Attempt Payload authorities from dedicated Journal
streams and repairs the narrow `ToolResultDelivered`-committed / Vault-still-
`pending` crash window even after the Run reached terminal state. The replay is
fenced by current Run identity, claim generation, Agent, frozen execution
binding, Capsule, semantic call, sequence, content digest, causation, Incident
ID and one of the two accepted strong proofs.

One corrupt Vault row produces an Attempt-local blocked reconciliation and a
privacy-safe `context_delivery_reconcile` diagnostic while independent rows
continue. A real LocalKeyFile Vault close/reopen plus SQLite Journal test proves
terminal repair and plaintext-negative files. The complete affected normal and
race matrices pass. Exact evidence is
`.loom-evidence/phase2d/P2D-W2C-W2D-attempt-payload-terminal-reconciliation-v1.md`.

This closes only the bounded Context payload terminal reconciler. General
side-effecting multi-tool execution, accepted-but-unproved terminal expiry,
TTL/compaction, Aggregation, installed CV6, real Provider accounting/fallback,
and live mixed-Team acceptance remain open under the same Phase 2D Goal.

## P2D-W2B/W2D frozen Agent disclosure policy (2026-08-13)

Phase 2D remains the sole Goal. Every account-governed Agent Run/Attempt now
freezes Provider Account policy version, revision, digest, trust domain,
retention mode, and data region from the exact historical policy event selected
at claim/dispatch. Current account-policy edits affect only future claims and
cannot rewrite existing or terminal Agent rows.

The complete authority-to-App path is closed: Work replay, Run projection,
Team Attempt overlay, Board wire, strict Swift decoding, and Agent governance
UI. Legacy v1 is rendered as disclosure unspecified; v2 requires a closed tuple
and substitution fails closed. No secret, credential reference, Prompt, or
Provider response is added to Board state.

Exact evidence is
`.loom-evidence/phase2d/P2D-W2B-W2D-agent-disclosure-policy-freeze-v1.md`.
Installed CV6, real Provider policy observation, parallel Aggregation, and live
mixed-Team acceptance remain open under the same Goal.

## P2D-W2A/W2C parallel RouteSet aggregation v1 (2026-08-13)

Phase 2D remains the sole Goal. The generic Team execution path now supports
one stable Agent with multiple explicit Provider route sibling Attempts and a
separate Aggregation Attempt. Plan digest, Journal replay, dispatch waves,
Projection, Board, and Swift preserve the route group. Every Attempt freezes
one independent Execution Binding and Provider Account accounting identity.

Aggregation reads only exact succeeded Attempt Evidence. Authorized output
events are supplied as untrusted model output beside authoritative source
metadata; private runtime frames, Prompt, credential, and scratchpad are not
admitted. Substitution and partial dependency sets fail closed, and a failed
sibling leaves healthy sibling execution intact while holding Synthesis.

Exact evidence is
`.loom-evidence/phase2d/P2D-W2A-W2C-parallel-route-aggregation-v1.md`. This is
source authority and governance infrastructure only. Product RouteSet authoring,
ordinary Conversation parallel UI, installed CV6, real Harness aggregation,
and live Provider acceptance remain open under the same `ACTIVE / PARTIAL`
Phase 2D Goal.

## Accepted Harness interaction and governance increment (2026-08-13)

Phase 2D remains the sole product Goal. DeepSeek Harness is accepted as design
input, not added as a production dependency or execution authority. Loom keeps
its immutable authority core, encrypted payload/Capsule storage, Credential
Vault, exact Provider Account revisions, and content-free Journal/diagnostic
boundaries.

The accepted increment extends the existing WorkItems:

- `P2D-W2A`: Context Meter, immutable Conversation Fork/Segment continuity,
  explicit compaction disclosure, and durable Queue/Steer/Inject input UX.
- `P2D-W2C`: authoritative Attempt -> Turn -> Step -> Model Request -> ToolCall
  lifecycle, `parallel`/`exclusive` tool execution modes, durable runtime input
  admission, and crash-safe consumption/delivery authority.
- `P2D-W2D`: exact one-shot ToolCall approvals, component-level sandbox
  requirements and enforcement completeness, tool-level Incident correlation,
  and explicit recovery governance for uncertain side effects.

These additions are `TARGET / ACCEPTED / NOT YET IMPLEMENTED`. They do not
change the status of current RouteSet aggregation, DeepSeek Conversation work,
Credential Vault, or per-Agent binding slices. They also do not permit full
Prompt logging, hidden-reasoning persistence, unredacted telemetry, plaintext
tool payloads, credential exposure, silent Provider fallback, or model-authored
execution authority.

The implementation order inside the current Goal is:

1. W2C authority schemas and RED transition/replay/concurrency tests.
2. W2D exact approval, sandbox report, Incident, and recovery contracts.
3. W2A projections and conversation UX for Context Meter, Fork, compaction,
   Queue, Steer, and Inject.
4. Restart, race, privacy-negative, installed-App, and mixed-Team acceptance.

The detailed acceptance semantics live in the existing W2A, W2C, and W2D
contracts. No new Goal, WorkItem family, or completion claim is created by this
amendment.

## P2D-W2A/W2B/W2C product parallel RouteSet authoring v1 (2026-08-13)

The product Team Builder now composes the previously accepted generic sibling
and Aggregation authority. A role-owned `parallel_route_set` persists the exact
primary route, one or two additional execution routes, and an explicit
Synthesis route. It never becomes a Team-level Provider default. Every route
retains its own Harness, Provider Account, credential revision, model, limits,
and capabilities.

Fallback and parallel execution remain separate governed meanings and are
mutually exclusive in this slice. State, Projection, Builder reopen, Mission
preflight, and Mission reconstruction reject substitution, duplicate routes,
missing Synthesis, unknown versions, and topology drift. Swift exposes compact
per-Agent add/remove controls and strict account-scoped route decoding.

The source fixture expands four stable Agents into six physical nodes and
reconstructs the same plan after restart. Kimi credential revocation blocks the
Kimi sibling while the DeepSeek sibling remains ready; Synthesis is held and no
silent route transition occurs. Exact evidence is
`../phase2d/P2D-W2A-W2B-W2C-product-parallel-routeset-authoring-v1.md`.

This closes product Builder RouteSet authoring, not ordinary Conversation
parallel UX, real Harness Synthesis, installed CV6, or live mixed-Team
acceptance. Phase 2D remains the sole `ACTIVE / PARTIAL` Goal.

## Loom Harness Platform Core absorption decision (2026-08-13)

The Product Owner clarified that Loom is the Harness Platform. The adapter
dimension is Runtime, not a peer Harness authority. Phase 2D therefore absorbs
the selected DeepSeek Harness Core semantics into Loom Harness Core and replaces
target `HarnessAdapter` terminology with `RuntimeContract` and
`RuntimeAdapter`. Existing v0.5.x fields and evidence retain their historical
bytes until a controlled migration.

The selected semantics are Agent lifecycle ownership, Attempt/Turn/Step loop,
unified Queue/Steer/Inject inbox, scoped capability composition, guarded tool
pipeline, bounded parallel calls with exclusive barriers, cancellation
convergence, resume, and replay invariants. The reference is pinned to
`deepseek-ai/deepseek-harness@47f943859bef60e4160492346772ded9b24f765a`
under MIT; copied substantial code must retain the required notice and source
provenance.

This is selective semantic/code porting, not whole-core vendoring. Cordis,
DeepSeek SessionEvent authority, plaintext model-visible logs, credentials,
runtime self-modification, and ungoverned tools do not enter Loom's trusted
core. Loom Event Journal, encrypted Payload/Capsule stores, Credential Vault,
Provider Account binding, approvals, sandbox, recovery, and terminal authority
remain canonical.

Runtime ownership is now explicit:

- Loom Native implements `RuntimeContract` in daemon;
- Pi, Codex, Claude Code, and future DeepSeek Harness use bounded
  `RuntimeAdapter` implementations;
- Provider adapters are lower model-protocol components for Loom Native;
- no Runtime may bypass Loom Tool Gateway or become a second Harness authority.

This decision refines W2B/W2C/W2D inside the current Phase 2D Goal. It does not
reset current DeepSeek conversation work, Credential Vault, RouteSet
aggregation, or per-Agent binding implementation.

## P2D-W2C Attempt Loop authority v1 (2026-08-13)

The first generalized Loom Harness Core authority slice is now source verified.
It repairs the bounded single-result Attempt Payload stream with a compatible
call-scoped v2 and atomic sequence index, then adds strict Turn, Step,
ModelRequest, ToolCall, dispatch, result-lineage, Step-end, and Turn-end facts.
Every command is fenced by the current Run head, claim generation, Agent,
Runtime, Execution Binding, Context Capsule, permission/capability/tool-schema
digests, budget version, and Incident ID.

Parallel and exclusive modes use explicit conflict-scope digests. The result
fact must cite the exact dispatch event before delivery or Step completion can
be accepted. Restart and terminal reconstruction, deep-copy isolation, replay
tamper, sequence races, stale generation, binding/capability substitution, and
content-negative Journal tests pass. Exact evidence is
`../phase2d/P2D-W2C-attempt-loop-authority-v1.md`.

This advances ATL1/ATL2 only. RuntimeContract/Tool Gateway integration,
encrypted approval inspection, sandbox evidence, Queue/Steer/Inject, UI projection,
installed CV6, and live mixed-Team acceptance remain open under the sole
`ACTIVE / PARTIAL` Phase 2D Goal.
## P2D-W2D one-shot approval and content-free Journal increment (2026-08-13)

The existing W2D governance scope now has a source-verified one-shot approval
consumption fact and content-free schema-v2 execution/permission facts. Approval
reuse is fenced by exact approval/call/consumer/operation identity and races can
produce at most one executor invocation. New Journal payloads omit tool
arguments, free-form denial text, and raw executor errors while preserving old
schema-v1 replay.

The authenticated encrypted proposal-detail channel was still open at this
checkpoint. An approval without inspectable details cannot be allowed; it may
only be rejected. General Attempt Tool Gateway composition, sandbox and
diagnostic evidence, explicit interrupted-side-effect recovery, Swift
governance, and installed CV6 remain open. This increment does not create a new
Goal and does not change the completed status of Phase 2C.

`CURRENT / SOURCE VERIFIED (2026-08-14)`: the encrypted proposal-detail
channel is now implemented for the existing Pi ask path. It uses the
Conversation DEK and exact Attempt/call/approval AAD, and permission attention
revalidates the authoritative continuation digest before displaying detail.
Missing or corrupt detail disables Allow. General Tool Gateway composition,
native approval controls, sandbox/diagnostic/recovery evidence, other Runtime
adapters, and installed acceptance remain open.

`CURRENT / SOURCE VERIFIED (2026-08-14)`: the next Pi local execution slice now
commits exact Attempt ToolCall admission and dispatch before Bash/Edit side
effects. Dispatch failure is zero-side-effect; approved resume carries its
one-shot approval identity. Digest-only results use the encrypted Attempt
Payload Store, and the result frame omits command/path. A separate
`run_stream_tool_result` receipt is intentionally not a Provider-continuation
claim. Ask pause/resume, actual Pi model continuation, sandbox/diagnostics/
recovery governance, other tools/Runtimes, and installed acceptance remain
open under the same Phase 2D Goal.

`CURRENT / SOURCE VERIFIED (2026-08-14)`: Pi Bash/Edit now records a closed,
content-free ToolCall stage chain with exact Attempt, Provider Account, Model,
Run, Agent, Execution Binding, Capsule, operation, call digest, and Incident
identity. Edit preflight precedes dispatch, and raw executor errors are reduced
to controlled codes before result delivery.

Daemon startup no longer leaves an authorized unknown side effect ambiguous or
replays it. `Allowed` without a terminal fact becomes the fixed schema-v2
`recovery_required / side_effect_unknown` authority state, while a
Proposed-only ask remains pending. Swift decodes and shows this state and its
Incident ID without inventing an executable recovery control. Authoritative
Retry/Skip/Cancel/New-Attempt commands, exact Attempt-bound startup recovery
diagnostics, component sandbox reports, other tools/Runtimes, and installed
acceptance remain open. Evidence:
`../phase2d/P2D-W2D-tool-diagnostics-recovery-required-v1.md`.

`CURRENT / SOURCE VERIFIED (2026-08-14)`: Pi Bash/Edit now complete the native
Harness continuation boundary. A private PID-attested extension holds Ask on
the same deterministic operation, returns only a digest-only Pi `toolResult`,
and records delivery only after a validated second-turn final output using
`harness_final_output`. Context retrieval and the governed Tool extension can
coexist; the first exact ToolCall locks the route and substitution fails
closed. Approval IDs/digests, command/path, payload binding, Prompt and
Provider body do not enter the child result or Bridge frames.

The locked Pi runtime advertises `governed_tool_loop` only through explicit
0.82.1 conformance and the frozen capability must match the daemon Hook before
process start. Read/Grep, multiple calls, Web/MCP, other Runtime adapters,
sandbox/recovery/UI and installed acceptance remain open under the same Phase
2D Goal. Evidence:
`../phase2d/P2D-W2D-pi-native-tool-continuation-v1.md`.

`CURRENT / SOURCE VERIFIED (2026-08-14)`: governed Read/Grep now execute after
the exact Attempt dispatch commit, store result bytes only in the encrypted
Conversation-DEK Attempt Payload, and return them through the private Pi native
tool result. Journal, Evidence, diagnostics, RunStream and Adapter cache retain
only digest/identity metadata. Invalid Grep patterns fail before dispatch;
result-commit failures zeroize content; read-only replay must re-read and match
the committed digest.

The strict Pi transcript rejects result-content substitution and records
delivery only after `harness_final_output`. A real managed Pi child Read canary
now consumes the private result and completes the second turn without leaking
content/path into Bridge or audit. Web/MCP remains closed because a remote result cannot
be safely reconstructed by repeating the call, ATL3 must encrypt content before
terminal execution state. Multiple calls, other Runtime adapters, sandbox/
recovery/UI and installed acceptance remain open under the same Phase 2D Goal.
Evidence: `../phase2d/P2D-W2D-pi-governed-read-grep-content-v1.md`.

`CURRENT / SOURCE VERIFIED (2026-08-14)`: ATL3 now persists WebSearch,
WebFetch and MCPTool result bytes in the encrypted Conversation/Attempt Payload
before the execution terminal fact. The exact Attempt Loop acceptance is
therefore crash-safe without repeating a remote call. Persistence failure
zeroizes content and terminalizes with a controlled code.

The injected Broker publishes an exact immutable capability set consumed by
the Adapter, daemon Hook, Pi extension schema, dynamic prompt and strict
transcript parser. Unconfigured MCP is not advertised; undeclared tools and
result-content substitution fail closed. Production Search/MCP composition,
multiple sequential calls, other Runtime adapters and installed live
acceptance remain open under the same Phase 2D Goal. Evidence:
`../phase2d/P2D-W2C-W2D-crash-safe-web-mcp-result-commit-v1.md`.

## P2D-COMP1/COMP2 governed composition insertion (2026-08-14)

The Product Owner accepted the Cordis-informed Profile, Bundle, scoped service
context, Effect, and lifecycle concepts as Loom-owned composition behavior.
This is an insertion into the sole Phase 2D Goal, not a new Goal and not an
adoption of Cordis as Loom authority. ADR-0021 and the following contracts are
normative:

- `../phase2d/contracts/P2D-COMP1-composition-kernel.md`;
- `../phase2d/contracts/P2D-COMP2-product-daemon-strangler.md`.

P2D-COMP1 compiles desktop/headless/test Profiles and versioned built-in
Bundles into an immutable Composition Snapshot. Canonical Bundle order,
capability graph, RouteDescriptor table, lifecycle plan, schema, and digest are
deterministic. Capability scopes are exactly `Root -> Product -> Conversation
-> Team -> Agent -> Attempt -> Turn`; children may narrow but never widen
authority. Lifecycle is `Register -> Validate -> Start -> Ready -> Stop ->
Dispose`, with strict reverse cleanup of temporary Effects after failure.

`CapabilityContext` is a typed process-local service container. It is not Go
`context.Context`, Attempt Context, Context Capsule, Agent memory, or a shared
data bag. Prompt, transcript, tool-result content, Provider response, plaintext
credential, VMK, Journal append authority, policy/grant authority,
authoritative StateWriter, and terminal decision authority cannot be exposed as
ordinary capabilities. A running Attempt binds both its snapshot digest and
Frozen Execution Binding; recomposition cannot rewrite Provider Account,
credential revision, model, policy, tool schema, budget, or disclosure state.

P2D-COMP2 is a behavior-preserving strangler migration:

1. COMP2-A wraps current services in built-in compatibility Bundles.
2. COMP2-B replaces the fifteen-parameter composition handler and conditional
   method availability with a compiled typed route/capability registry.
3. COMP2-C moves service construction into the accepted built-in Bundles.
4. COMP2-D opens and owns scoped resources at existing lifecycle boundaries.
5. COMP2-E removes legacy product reachability only after complete parity.

Initial Bundle ownership is `loom-core`, `loom-vault`, `loom-conversation`,
`loom-agent-runtime`, `loom-governance`, `loom-work`, `loom-assets`,
`loom-observability`, and `loom-local-ipc`. Phase 2D does not open arbitrary
third-party dynamic code loading, hot replacement, or remote Bundle download.

The ATL3 crash-safe Web/MCP result slice remains source verified and is not
reset. The next implementation order is COMP1, COMP2-A, COMP2-B, then bounded
COMP2-C/D before broader sequential tools, more Runtime adapters,
Queue/Steer/Inject, or further product wiring. COMP2-E waits for parity,
restart, race, privacy, and shutdown gates. Installed CV6 and mixed-Team ATL9
remain the Phase 2D completion boundary.

`CURRENT / P2D-COMP1 SOURCE VERIFIED (2026-08-14)`: the governed Composition
Kernel now compiles exact built-in Profile/Bundle inputs into deterministic
snapshot bytes and digest, validates graph/route/factory contracts before
Start, exposes only descriptor-admitted ports to Bundle lifecycle hooks, masks
protected Core authority below Root, owns exact operational scopes and Effects,
and records metadata-only lifecycle/scope diagnostics. Attempt scope freezes the
Composition Snapshot and independent Execution Binding digests.

The focused suite passes at 83.0% statement coverage and under 20 race-enabled
runs; full repository Go and vet checks pass. COMP2-A/B now consume the kernel
at the product admission boundary. The next implementation gate is bounded
COMP2-C/D migration. Evidence:
`../phase2d/P2D-COMP1-governed-composition-kernel-v1.md`.

`CURRENT / P2D-COMP2-A SOURCE VERIFIED (2026-08-14)`: the production daemon now
activates the desktop compatibility Bundle snapshot after the legacy services
and wrapped handler are assembled and before local IPC server creation.
Desktop/headless/test compile the same nine versioned Bundles. The facade
delegates byte-equivalent responses, gates every request on composition Ready,
waits for admitted requests during Close, and revokes old handler references.

All ten composition lifecycle/scope stages persist through the existing bounded
owner-only operational JSONL store before IPC admission; diagnostic failure
fails startup closed. Full repository Go, vet, format/diff, and focused 10-run
race checks pass. Monolithic service construction remains the compatibility
oracle. COMP2-B declarative route migration is source verified. Evidence:
`../phase2d/P2D-COMP2-A-compatibility-bundle-facade-v1.md`.

`CURRENT / P2D-COMP2-B SOURCE VERIFIED (2026-08-14)`: the compatibility
snapshot now compiles 47 exact sorted local product RouteDescriptors across
the assets, Conversation, governance, Vault, Agent Runtime, and work Bundles.
The aggregate legacy dispatch route is absent. Route metadata freezes the
typed handler capability, required capabilities, unavailable code, Incident
policy, and privacy class without retaining request or service content.

Product startup directly constructs typed `productRouteServices` and the
registry; the positional fifteen-parameter entry is retained only for old
tests. AST, exact manifest, unavailable-code, Profile digest, focused 20-run
race, daemon package, full repository, vet, and diff gates pass. The existing
dispatch switch remains the behavior oracle for COMP2-C/D; COMP2-E and all
installed/live Phase 2D gates remain open. Evidence:
`../phase2d/P2D-COMP2-B-declarative-route-registry-v1.md`.

`CURRENT / P2D-COMP2-D PRODUCT SCOPE SOURCE VERIFIED (2026-08-14)`: after
Bundle Ready and before IPC admission, the compatibility composition now opens
and owns the exact Product Capability Context. The scope inherits only the
snapshot digest, masks protected Core capability keys, emits persistent
metadata-only open/close diagnostics, and closes before activation/root after
request quiescence. Concurrent Close is idempotent and production startup proves
the scope is open before Run.

Deeper scopes remain open. Agent Runtime construction now runs behind
Composition, while Conversation and remaining services still predate activation;
post-construction mutable injection is prohibited. Further bounded COMP2-C
inversion must not carry Journal/Vault/policy/terminal authority or content
through Capability Context. Evidence:
`../phase2d/P2D-COMP2-D-product-scope-v1.md`.

`CURRENT / P2D-COMP2-C ASSETS ROUTE CONSTRUCTION SOURCE VERIFIED
(2026-08-14)`: the `loom-assets` Bundle Start hook now constructs and binds the
typed Assets service/API route. Failed construction aborts activation before
Product scope or IPC admission; its reversible Effect waits for in-flight calls
and revokes the route. The product builder no longer directly constructs these
route objects, while exact existing socket and Agent execution behavior remains
green.

Assets authority/Evidence, skill materialization, and startup recovery have
subsequently moved into `loom-assets`; Agent Runtime receives only a bounded
Team materializer port. This is a partial C slice, not completion of COMP2-C/D.
Evidence:
`../phase2d/P2D-COMP2-C-assets-route-construction-v1.md`.

`CURRENT / P2D-COMP2-C AGENT RUNTIME CONSTRUCTION SOURCE VERIFIED
(2026-08-14)`: `loom-agent-runtime` Start now creates Mission Execution,
Handoff, and Team materialization routes and its Effect owns the old execution
closer. Product startup only supplies a trusted factory and typed slot; direct
execution-builder construction and duplicate runner ownership are gone.

Assets-before-Runtime order, failed-Start rollback, startup reconciliation,
safe error identity, Agent binding, mission/handoff behavior, and shutdown pass
focused 20-run race, post-review 10-run race, and full repository/vet gates.
Remaining service families and deeper scopes remain open. Evidence:
`../phase2d/P2D-COMP2-C-agent-runtime-construction-v1.md`.

`CURRENT / P2D-COMP2-C ASSETS AUTHORITY OWNERSHIP SOURCE VERIFIED
(2026-08-14)`: `loom-assets` now constructs and owns its Evidence store, Asset
authority, API, Pi skill materializer, Team materializer, and startup recovery.
The product root no longer retains concrete Assets owners; `loom-agent-runtime`
consumes only `app.TeamAssetMaterializer` after Assets Ready. Reverse cleanup
closes Runtime before Assets, revokes both ports, and preserves request
quiescence. Focused 10-run race, daemon, repository, vet, AST ownership,
rollback, shutdown, and diff gates pass. Remaining COMP2-C/D, COMP2-E, CV6,
ATL9, and live gates remain open. Evidence:
`../phase2d/P2D-COMP2-C-assets-authority-ownership-v1.md`.

`CURRENT / P2D-COMP2-C WORK QUEUE CONSTRUCTION SOURCE VERIFIED
(2026-08-14)`: `loom-work` now constructs Queue in Bundle Start and exposes it
through an extensible typed Work slot. Failed Start prevents Product scope/IPC;
cleanup quiesces and revokes the port. Production no longer constructs Queue.
Assets -> Work -> Agent Runtime order, focused 10-run race, daemon, repository,
vet, AST, Queue journey, shutdown, and diff gates pass. Remaining Work services,
deeper scopes, COMP2-E, CV6, ATL9, and live gates remain open. Evidence:
`../phase2d/P2D-COMP2-C-work-queue-construction-v1.md`.

`CURRENT / P2D-COMP2-C WORK ROUTES CONSTRUCTION V2 SOURCE VERIFIED
(2026-08-14)`: the same `loom-work` Start now atomically constructs Queue,
Workers, and Integration and publishes three typed ports through one slot.
Product construction no longer creates their services/APIs. All ports gate
Ready; one Effect quiesces and revokes all six methods. Exact Queue/Workers/
Integration build-stage identity, focused 10-run race, full daemon/repository,
vet, AST ownership, and three real socket journeys pass. Execution, Production,
work/governance authority, setup/read routes, deeper scopes, COMP2-E, CV6, ATL9,
and live gates remain open. Evidence:
`../phase2d/P2D-COMP2-C-work-routes-construction-v2.md`.

`CURRENT / P2D-COMP2-C WORK EXECUTION OWNERSHIP V3 SOURCE VERIFIED
(2026-08-14)`: `loom-work` now also constructs and owns governed Execution:
the owner-only Evidence store, decision recorder, sandbox gate, Adapter,
pending recovery, service, and API. The Bundle cannot become Ready until
recovery succeeds. Its typed slot exposes the local Execution route and one
private bounded tool-execution port; Agent Runtime receives only that port after
Work Ready, with no concrete Adapter or content carried in Capability Context.

Exact `build_execution` and `build_execution_recovery` failures survive the
Composition boundary. Reverse shutdown closes Agent Runtime before Work,
quiesces and revokes both Execution ports, and closes Evidence ownership exactly
once. Focused tests, 10-run composition/daemon race, full daemon/repository Go,
vet, AST ownership, privacy, shutdown, and diff gates pass. Production,
governance/work authority, setup/read routes, Conversation/observability
construction, deeper scopes, COMP2-E, CV6, ATL9, and installed live acceptance
remain open. Evidence:
`../phase2d/P2D-COMP2-C-work-execution-ownership-v3.md`.

`CURRENT / P2D-COMP2-C WORK PRODUCTION OWNERSHIP V4 SOURCE VERIFIED
(2026-08-14)`: `loom-work` now also constructs and owns the Production core,
local service, and API. Production Snapshot, Command, and the read-only
Degraded admission gate are required by the atomic Work route set before Ready;
product construction no longer resolves Production paths or invokes its three
constructors.

The prior sandbox-root/user-Library path behavior, daemon executable,
Journal-replayed AdminLock, authoritative projection, and degraded-write policy
are preserved. Construction failures remain `build_production`; reverse cleanup
quiesces and revokes Production and then fails closed as degraded. Focused
Composition and real socket journeys, 10-run race, full daemon/repository Go,
vet, AST ownership, shutdown, privacy, and diff checks pass. Governance/work
authority, setup/read routes, Conversation/observability construction, deeper
scopes, COMP2-E, CV6, ATL9, and installed live acceptance remain open. Evidence:
`../phase2d/P2D-COMP2-C-work-production-ownership-v4.md`.

`CURRENT / P2D-COMP2-C GOVERNANCE AUTHORITY OWNERSHIP V5 SOURCE VERIFIED
(2026-08-14)`: `loom-governance` now constructs and owns the Permission approval
port and Rules authority, Permission API, Customer Rule API, and Standing Order
authority/API. The product root no longer constructs these objects. Rules
authority stays inside the trusted built-in factory; typed IPC proxies expose
only route operations, and Work acquires only Request/Consume approval methods
after Governance Ready.

Governance failure prevents Work, Product scope, and IPC admission. Reverse
cleanup closes Work before Governance and revokes the approval port. Exact
`build_permissions`, `build_customer_rule`, and `build_standing_order` stages,
Permission ask/decision, Customer Rule, Standing Order, Execution approval
consumption, focused 10-run race, full daemon/repository Go, vet, AST ownership,
shutdown, privacy, and diff checks pass. Mission decision, Provider policy,
setup/read routes, Conversation/observability construction, deeper scopes,
COMP2-E, CV6, ATL9, and installed live acceptance remain open. Evidence:
`../phase2d/P2D-COMP2-C-governance-authority-ownership-v5.md`.

`CURRENT / P2D-COMP2-C GOVERNANCE DECISION OWNERSHIP V6 SOURCE VERIFIED
(2026-08-14)`: `loom-governance` now also constructs and owns the Prepared
Mission Decision backend, fallback decision StateWriter/preparer, Decision API,
and Mission Execution decision router. Backend and writer remain private to the
trusted built-in factory. Read receives only command listing; Agent Runtime gets
only execution-control and explicit fallback interfaces; IPC gets only the
typed Decision route.

Governance failure blocks downstream activation and closing the slot revokes all
decision interfaces. Exact `build_decision`, strict Decision IPC, prepared-view
rebind, mission preflight/control, fail-closed registry, four-Provider explicit
fallback, focused 10-run race, full daemon/repository Go, vet, AST ownership,
shutdown, privacy, and diff checks pass. Provider policy, setup/read route
construction, Conversation/observability construction, deeper scopes, COMP2-E,
CV6, ATL9, and installed live acceptance remain open. Evidence:
`../phase2d/P2D-COMP2-C-governance-decision-ownership-v6.md`.

`CURRENT / P2D-COMP2-C GOVERNANCE PROVIDER ACCOUNT POLICY V7 SOURCE VERIFIED
(2026-08-14)`: `loom-governance` now also constructs and owns Provider Account
Policy and Provider Model Rate Card authority. Setup receives only the two exact
configuration interfaces and no longer creates this authority. It stays inside
the trusted factory and is never exposed through Capability Context.

Account-local identity, revision, concurrency, dispatch rate, budget,
disclosure policy, token basis and cost freezing remain exact. Revocation,
`build_setup_policy`, trusted correlation, projection refresh, account
isolation, policy/price drift, privacy-safe diagnostics, focused 10-run race,
full daemon/repository Go, vet, AST ownership, shutdown, and diff checks pass.
Setup/read and Conversation/observability construction, accounting UI, deeper
scopes, COMP2-E, CV6, ATL9, and installed live acceptance remain open. Evidence:
`../phase2d/P2D-COMP2-C-governance-provider-account-policy-v7.md`.

`CURRENT / P2D-COMP2-C OBSERVABILITY BOOTSTRAP HANDOFF V8 SOURCE VERIFIED
(2026-08-14)`: the operational store remains pre-composition only for
credential-runtime selection, encrypted Conversation migration, and
Composition lifecycle failures. `loom-observability` Start now binds the
revocable bounded ports used by Read, Agent Attempt, Context retrieval, tool,
and IPC diagnostics after Ready; downstream product construction no longer
retains the store directly.

The slot exposes no path, file handle, Journal writer, credential, Prompt,
Provider body, or terminal authority. Atomic activation failure,
`build_diagnostics`, close revocation, privacy-safe records, UDS parity,
focused 10-run race, full daemon/repository Go, vet, AST ownership, shutdown,
and diff checks pass. V10/V11 below subsequently move Vault/Conversation and
persistent store construction. Setup/read construction, UI/export/accounting,
deeper scopes, COMP2-E, CV6, ATL9, and installed live acceptance remain open.
Evidence:
`../phase2d/P2D-COMP2-C-observability-bootstrap-handoff-v8.md`.

`CURRENT / P2D-COMP2-C CONVERSATION ROUTE HANDOFF V9 SOURCE VERIFIED
(2026-08-14)`: Read now consumes `LocalProductChatSource` and
`loom-conversation` Start owns the existing chat route's bind/revoke lifecycle.
Failure prevents Agent Runtime and Product/IPC admission and preserves
`build_setup_provider`. Profile, Segment, Attempt, encrypted migration, Context
Capsule, Provider Account binding, UDS, focused 10-run race, AST ownership, full
daemon/repository Go, vet, shutdown, privacy, and diff checks pass.

The port carries no Prompt, transcript, Provider body, credential, Journal
writer, or terminal authority. V10 below subsequently moves the Provider,
router, migration, and Chat construction. Setup/read construction, deeper
scopes, UI/accounting, COMP2-E, CV6, ATL9, and installed live acceptance remain
open. Evidence:
`../phase2d/P2D-COMP2-C-conversation-route-handoff-v9.md`.

`CURRENT / P2D-COMP2-C VAULT AND CONVERSATION CONSTRUCTION V10 SOURCE VERIFIED
(2026-08-14)`: `loom-vault` now constructs and privately owns the real Vault or
fail-closed recovery runtime, exposing only bounded exact-revision credential,
encrypted state, Context Capsule, Attempt payload, Tool Proposal, and external
session ports. `loom-conversation` constructs Provider clients, the
account-aware Profile router, migration recorder, and persistent encrypted Chat
after Vault Ready. Reverse cleanup revokes Conversation before Vault; recovery
reset preserves the same bounded slot; exact build stages and encrypted
migration behavior remain stable. The monolithic builder no longer calls those
constructors, and unavailable writes clear sensitive content buffers.

Focused lifecycle/ownership/recovery/privacy tests, ten-run race, full daemon
and repository Go, vet, static constructor inspection, and diff checks pass.
The lazy Pi Conversation runtime, Setup/read and diagnostics bootstrap
construction, deeper scopes, COMP2-E, CV6, ATL9, and installed live acceptance
remain open. Evidence:
`../phase2d/P2D-COMP2-C-vault-conversation-construction-v10.md`.

`CURRENT / P2D-COMP2-C OBSERVABILITY CONSTRUCTION V11 SOURCE VERIFIED
(2026-08-14)`: the persistent owner-only diagnostic store is now constructed in
`loom-observability` Start. A no-I/O, fixed-bound bootstrap sink buffers only
validated privacy-safe Composition records emitted before that Start, then
binds the credential-runtime identity and flushes in order before route
publication. Construction, overflow, or flush failure blocks admission as
`build_diagnostics`; stop/dispose records remain durable after operational port
revocation. The bootstrap is not Capability Context and carries no content,
credential, ciphertext, VMK, Journal writer, or authority.

Focused pre-start flush/filesystem failure/AST tests, ten-run race, full daemon
and repository Go, vet, static constructor checks, and diff checks pass. Lazy Pi
Conversation ownership, Setup/read construction, deeper scopes, COMP2-E, CV6,
ATL9, and installed live acceptance remain open. Evidence:
`../phase2d/P2D-COMP2-C-observability-construction-v11.md`.

`CURRENT / P2D-COMP2-C SETUP CONSTRUCTION V12 SOURCE VERIFIED
(2026-08-14)`: the final built-in `loom-local-ipc` Start now constructs the
existing Setup aggregate only after its Observability, Vault, Conversation,
Governance, Work, Assets, and Agent Runtime dependencies are Ready. The product
handler receives a revocable typed slot; Vault/Provider Policy roots remain
behind their exact bounded interfaces. Credential input is cleared on
unavailable routes, original Setup build reasons survive rollback, and reverse
cleanup avoids double-close. The product builder no longer calls the Setup
constructor.

Focused Setup/Vault/policy/UDS/AST tests, ten-run race, full daemon and
repository Go, vet, static constructor checks, and diff checks pass. Read
construction, lazy Pi Conversation ownership, deeper scopes, COMP2-E, CV6,
ATL9, and installed live acceptance remain open. Evidence:
`../phase2d/P2D-COMP2-C-setup-construction-v12.md`.

`CURRENT / P2D-COMP2-C READ CONSTRUCTION V13 SOURCE VERIFIED
(2026-08-14)`: Read now constructs atomically with Governance and publishes a
revocable typed snapshot/timeline/chat/observer port. Agent Runtime receives
only its observer and SideTask integration surface, not the concrete Read
service. Journal, Projection, cached views, stream subscriptions, and observer
state remain private; construction failure rolls Governance back as
`build_state`, and reverse cleanup closes observers before Governance. The
product builder no longer calls the Read constructor.

Focused Governance/Read/Agent Runtime/UDS/AST tests, ten-run race, full daemon
and repository Go, vet, static constructor checks, and diff checks pass. Lazy
Pi Conversation ownership, deeper scopes, COMP2-E, CV6, ATL9, and installed
live acceptance remain open. Evidence:
`../phase2d/P2D-COMP2-C-read-construction-v13.md`.

`CURRENT / P2D-COMP2-C SHARED LOCAL MODEL OWNERSHIP V14 SOURCE VERIFIED
(2026-08-14)`: `loom-conversation` now constructs and owns the optional shared
Pi local-model runtime used by both Pi Conversation and Agent Runtime. Agent
Runtime acquires only its bounded port through the existing Conversation
dependency and closes first; Conversation closes the server exactly once. The
product builder no longer constructs the shared model/responder or retains a
separate closer.

Focused shared identity/lazy-load/rollback/AST tests, ten-run race, full daemon
and repository Go, vet, static constructor checks, and diff checks pass. The
required constructor ownership audit is completed by V15 below. Deeper
scopes, COMP2-E, CV6, ATL9, and installed live acceptance remain open. Evidence:
`../phase2d/P2D-COMP2-C-shared-local-model-ownership-v14.md`.

`CURRENT / P2D-COMP2-C PROTECTED CORE OWNERSHIP V15 SOURCE VERIFIED
(2026-08-14)`: `loom-core` now constructs and uniquely owns SQLite, Journal,
Projection, replay, controlled fixture bootstrap, and verified built-in Runtime
records. Trusted built-in factories obtain them only during Start through a
private revocable construction slot that never enters Capability Context. A
later Bundle failure rolls back through Core and closes the database.

The explicit legacy/test SecretStore adapter also constructs inside
`loom-vault` behind a revocable lease slot. The production builder is now
limited by AST allowlist to launch/test configuration, executable validation,
no-I/O diagnostics bootstrap, Bundle factory/route assembly, Composition
activation, IPC server creation, and lifecycle handoff. Focused RED,
rollback/error-stage, legacy credential, route parity, ten-run race, full daemon
and repository Go, vet, and diff checks pass. COMP2-C's source construction exit
is complete. Deeper COMP2-D scopes, COMP2-E, CV6, ATL9, and installed live
acceptance remain open. Evidence:
`../phase2d/P2D-COMP2-C-protected-core-ownership-v15.md`.

`CURRENT / P2D-COMP2-D CONVERSATION ATTEMPT SCOPE V2 SOURCE VERIFIED
(2026-08-14)`: ordinary Chat dispatch now opens Product-owned Conversation,
Team, stable Loom Agent, Attempt, and Turn scopes before Provider dispatch. The
Attempt freezes the active Composition Snapshot and immutable Conversation
execution binding digests. Scope-open failure rolls back Message, Segment,
Attempt, and stored Capsule before Provider access; Attempt/Turn close on every
return while higher scopes are reused until Product close.

The Chat service receives only a one-time-bound revocable manager port. Scope
diagnostics use domain-separated SHA-256 opaque IDs and never raw thread IDs.
Focused RED, rollback/reuse/production wiring, Conversation parity, ten-run
race, full daemon and repository Go, vet, and diff checks pass. Team mission
identities/full Frozen Execution Binding, multi-turn resources, lease/session/
tool Effects, COMP2-E, CV6, ATL9, and installed live acceptance remain open.
Evidence:
`../phase2d/P2D-COMP2-D-conversation-attempt-scope-v2.md`.

`CURRENT / P2D-COMP2-D TEAM AGENT ATTEMPT SCOPE V3 SOURCE VERIFIED
(2026-08-14)`: the real mission runner now opens an opaque internal execution
Conversation and Team scope before executor construction. Every source and
verifier execution freezes the exact per-Agent `FrozenExecutionBinding` and
opens a stable Agent, Attempt, and Turn before delegate execution. Mixed-Team
Agents therefore retain separate snapshot/binding ownership, while any scope
admission failure blocks the Runtime/Provider delegate.

Attempt/Turn close per call; executor close precedes Team-scope revocation.
Production Agent Runtime receives the one-time-bound scope slot, and all source
Team/Agent/Run/correlation identities are replaced by domain-separated opaque
diagnostic IDs. Focused RED/isolation/fail-closed/wiring tests, related parity,
ten-run race, full daemon/repository Go, vet, and diff checks pass. Credential,
Provider session, Tool Loop channel, component process, temporary-root, worker,
COMP2-E, CV6, ATL9, UI/accounting, and installed-live gates remain open.
Evidence:
`../phase2d/P2D-COMP2-D-team-agent-attempt-scope-v3.md`.

`CURRENT / P2D-COMP2-D ATTEMPT CANCELLATION OWNERSHIP V4 SOURCE VERIFIED
(2026-08-14)`: every Team Agent Attempt now owns a cancellable Go execution
context as an idempotent Composition Effect. The delegate receives only that
context after binding/scope admission. Normal return and Team/Product
revocation both cancel it with the stable scope-close cause, so an active
delegate cannot outlive its governed Attempt.

This does not merge Capability Context, Go context, Attempt Context, or Context
Capsule and carries no content, secret, or authority. RED, normal-return and
active-revocation tests, ten-run race, full daemon/repository Go, vet, and diff
checks pass. Credential/session/tool/process/filesystem Effects, multi-turn
Turn ownership, COMP2-E, CV6, ATL9, UI/accounting, and installed-live gates
remain open. Evidence:
`../phase2d/P2D-COMP2-D-attempt-cancellation-ownership-v4.md`.

`CURRENT / P2D-COMP2-D SEQUENTIAL TOOL TURN SCOPE V5 SOURCE VERIFIED
(2026-08-14)`: the mission Attempt lease now owns deterministic Turn
transitions through a private controller in the Attempt execution context.
Resolved governed tool results advance N to N+1 only after required encrypted
payload persistence; ask polling remains in N and stale/duplicate/out-of-order
transitions fail closed. Every Turn retains the immutable Attempt snapshot and
full per-Agent binding digests.

RED, Turn 1→2→3, ask, duplicate, real WBridge allow, mission wiring, tool
protocol, ten-run race, full daemon/repository Go, vet, and diff checks pass.
This is not the complete sequential ToolCall exit. Tool channel/process,
credential, Provider session, temporary-root, other Runtime, COMP2-E, CV6,
ATL9, UI/accounting, and installed-live gates remain open. Evidence:
`../phase2d/P2D-COMP2-D-sequential-tool-turn-scope-v5.md`.

`CURRENT / P2D-COMP2-D CREDENTIAL LEASE ISOLATION V6 SOURCE VERIFIED
(2026-08-14)`: the existing Vault hot path is cross-component verified under
real per-Agent Attempt scopes. Independent DeepSeek and MiniMax bindings resolve
exact Provider Account/reference/revision leases. Revoking DeepSeek cancels and
zeroizes only that lease; MiniMax remains active until Team Scope close cancels
and zeroizes its own lease. No Team-wide offline collapse or Provider-only
credential lookup occurs.

Focused isolation/zeroization, Vault and COMP2-D parity, ten-run race, full
daemon/repository Go, vet, and diff checks pass. Provider native sessions,
tool/process/filesystem Effects, other Runtime adapters, COMP2-E, CV6, ATL9,
UI/accounting, and installed-live gates remain open. Evidence:
`../phase2d/P2D-COMP2-D-credential-lease-isolation-v6.md`.

`CURRENT / P2D-COMP2-D CONVERSATION PROVIDER SESSION LIFECYCLE V7 SOURCE
VERIFIED (2026-08-14)`: ordinary Conversation dispatch now receives only its
Attempt-owned cancellable execution context after scope admission. Product
close cancels the responder with `composition.ErrScopeClosed`; a missing or
cancelled context fails closed, rolls back state, closes the scope, and never
reaches the Provider. A real encrypted ExternalSessionHandle callback inherits
the same context and exact Provider Account/model/Segment/credential revision
binding, then zeroizes plaintext after callback exit.

Focused RED/GREEN, Vault/session parity, ten-run race, full daemon/repository
Go, vet, and diff checks pass. This closes the lifecycle boundary only;
currently stateless Provider adapters have not yet adopted native handle reuse.
Tool/process/filesystem Effects, other Runtime adapters, COMP2-E, CV6, ATL9,
UI/accounting, and installed-live gates remain open. Evidence:
`../phase2d/P2D-COMP2-D-conversation-provider-session-lifecycle-v7.md`.

`CURRENT / P2D-COMP2-D PI TOOL PROCESS RESOURCE CLEANUP V8 SOURCE VERIFIED
(2026-08-14)`: a real Pi 0.82.1 child now enters the private governed Tool UDS
and remains in `ask` until its execution context is cancelled. Cancellation
returns a content-free denial, completes native abort, reaps the process group,
closes the socket/connection, and removes extension and fallback socket roots.

Focused process/Tool parity, ten-run race, full Pi Adapter/repository Go, vet,
and diff checks pass. Other Runtime adapters, arbitrary MCP processes,
crash/restart residue, COMP2-E, CV6, ATL9, UI/accounting, and installed-live
gates remain open. Evidence:
`../phase2d/P2D-COMP2-D-pi-tool-process-resource-cleanup-v8.md`.

`CURRENT / P2D-COMP2-D HARNESS PROCESS AND PRIVATE PROMPT CLEANUP V9 SOURCE
VERIFIED (2026-08-14)`: Claude Code and Codex now join private system-prompt
cleanup into every Harness result. A blocked cleanup path fails closed instead
of returning success. Real local child tests prove Attempt-context cancellation
reaps both process groups and removes the prompt file/private directory.

Focused RED/GREEN, ten-run race, full Harness/repository Go, vet, and diff
checks pass. Live CLI/Provider, gateway/Context MCP crash cleanup, other Runtime
adapters, COMP2-E, CV6, ATL9, UI/accounting, and installed gates remain open.
Evidence:
`../phase2d/P2D-COMP2-D-harness-process-private-prompt-cleanup-v9.md`.

`CURRENT / P2D-COMP2-D ATTEMPT LOOPBACK SERVICE REVOCATION V10 SOURCE
VERIFIED (2026-08-14)`: the Attempt credential gateway and Harness Context MCP
now receive the Attempt execution context directly. Cancellation revokes both
loopback listeners before a deliberately blocked trusted callback or Harness
runner returns. Context MCP also clears its bearer token under synchronized
lease/authorization access; normal completion remains graceful and idempotent.

Focused RED/GREEN, full protocol groups, ten-run race, full Harness/repository
Go, vet, and diff checks pass. This is normal-process cancellation only. The
credential callback must still return before its bounded plaintext copy is
cleared; daemon crash/restart, `kill -9` residue, arbitrary MCP component
processes, other Runtime adapters, COMP2-E, CV6, ATL9, UI/accounting, and
installed gates remain open. Evidence:
`../phase2d/P2D-COMP2-D-attempt-loopback-service-revocation-v10.md`.

`CURRENT / P2D-COMP2-D PI CONTEXT EXTENSION REVOCATION V11 SOURCE VERIFIED
(2026-08-14)`: the production Pi RPC Context extension now receives the Agent
Attempt execution context when it is constructed. Cancelling the Attempt
terminates a blocked delivery, closes the private UDS and accepted connection,
and removes the generated extension file, socket, optional short socket root,
and private extension root before the outer Adapter returns.

Focused RED/GREEN, Pi Context protocol tests, ten-run race, full Pi Adapter and
repository Go, vet, and diff checks pass. The selected locked Pi 0.82.1 test was
skipped by its existing environment gate and is not counted as live binary
acceptance. Crash/restart, `kill -9` residue, arbitrary MCP component processes,
other Runtime adapters, COMP2-E, CV6, ATL9, UI/accounting, and installed gates
remain open. Evidence:
`../phase2d/P2D-COMP2-D-pi-context-extension-revocation-v11.md`.

`CURRENT / P2D-W2C/W2D PI SEQUENTIAL TOOL CONTINUATION V2 SOURCE VERIFIED
(2026-08-14)`: one real local managed Pi-compatible child now performs two
bounded sequential governed ToolCalls through separate peer-attested private
UDS connections in one RPC invocation. Each call freezes a distinct sequence,
execution ID, payload/call lineage, and result digest. The final transcript is
accepted only after both results match, then both receive
`harness_final_output` acknowledgement; commands and payload authority stay out
of Bridge frames and audit records.

The initial managed-child run RED on a unit-only prompt constant; the parser
correctly failed closed. The fixture now uses the exact rendered child prompt,
with no production protocol relaxation. Focused protocol, ten-run managed-child
race, full Pi Adapter/repository Go, vet, and diff checks pass. COMP2-D V5
separately proves sequence-driven Turn 1→2→3 authority. Official Pi 0.82.1,
product Tool-hook dual-call live, other Runtime transports, Queue/Steer/Inject,
CV6, ATL9, UI/accounting, and installed gates remain open. Evidence:
`../phase2d/P2D-W2C-W2D-pi-sequential-tool-continuation-v2.md`.

`CURRENT / P2D-W2C/W2D LOOM NATIVE AGENT INPUT CONSUMPTION V5 SOURCE VERIFIED
(2026-08-14)`: the encrypted durable Agent Inbox now reaches one real product
Runtime. Loom Native consumes Steer/Inject into the exact next Step and Queue
into the exact next Turn while one Execution Binding, Route Segment, Provider
Account, credential revision and credential lease remain frozen. Three Provider
rounds produce one final Bridge result and combined accounting.

Runtime input batches, message wire buffers and HTTP request bytes are mutable
and cleared after use. Journal facts contain only non-content bindings, digests
and consumption status. Focused ten-run race and full repository Go pass.
Authenticated daemon ingress, Swift controls, crash-mid-transition recovery,
Pi/Codex/Claude conformance, CV6 and ATL9 remain open. Evidence:
`../phase2d/P2D-W2C-W2D-loom-native-agent-input-consumption-v5.md`.

`CURRENT / P2D-W2C/W2D AUTHENTICATED AGENT INPUT IPC AND SWIFT GOVERNANCE V6
SOURCE VERIFIED (2026-08-14)`: the durable Inbox now has a strict authenticated
`agent_input` UDS route. The client supplies exact public active-Agent identity,
mode, scope and bounded UTF-8 bytes; daemon uses the shared mission-scoped
active-Attempt registry and freezes all internal IDs, ordering, Turn/Step,
Execution Binding and Capsule lineage. Request ID is the Incident and
idempotency anchor. Replay is exact, substitution/stale generation fail closed,
and request plaintext is cleared on every path.

Swift Mission center now exposes per-active-Agent Queue/Steer/Inject controls
with Harness, Provider Account, Model and status. Drafts, progress, receipts and
errors are Agent-local. Failures retain the draft and show a safe stage,
recovery action and copyable Incident ID. App and daemon diagnostics share the
closed `agent_input_admission` stage and contain no input body.

Focused authenticated UDS/race, complete Swift, serial full repository Go, vet,
strict cross-language probe and privacy-negative gates pass. Crash-mid-
transition recovery, Pi/Codex/Claude consumption, installed CV6, mixed-Team
ATL9 and COMP2-E remain open. Phase 2D remains the sole `ACTIVE / PARTIAL` Goal.
Evidence:
`../phase2d/P2D-W2C-W2D-authenticated-agent-input-ipc-swift-v6.md`.

`CURRENT / P2D-W2C/W2D AGENT INPUT PRE-MODEL CRASH RECOVERY V7 SOURCE
VERIFIED (2026-08-14)`: the exact consumed input can now be recovered after its
Turn/Step Journal transition but only before `ModelRequestAdmitted`. Coordinator
and product Runtime recovery align Vault status, bind the previous output
checkpoint, recompute model-input identity, restore the deterministic cursor and
return zeroizable content. Wrong checkpoint and post-ModelRequest replay fail
closed.

RED, focused twenty-run race, vet, diff and frozen-source serial full repository
Go pass. Daemon restart checkpoint reconstruction and uncertain Provider
recovery after ModelRequest admission remain open; this is not a general
exactly-once claim. Evidence:
`../phase2d/P2D-W2C-W2D-agent-input-pre-model-crash-recovery-v7.md`.

`CURRENT / P2D-W2C/W2D PI AGENT INPUT CONTINUATION V8 SOURCE VERIFIED
(2026-08-14)`: after strict `agent_settled`, Pi now checks the daemon-owned
Inbox with the accepted assistant-output digest and can send the resulting
Queue/Steer/Inject bytes as a second native RPC prompt to the same managed
child. One Execution Binding/process, monotonic Bridge sequence, dispatch ACK,
terminal authority and combined accounting cover every prompt.

The shared renderer and RPC writer use owned mutable bytes and clear Inbox,
prompt-wire, round-state and accounting copies. Managed-child exact-wire,
checkpoint, content-negative, focused race, affected package, vet, diff and
serial full repository gates pass. Follow-up prompts are text-only and reject a
new Context/Tool event. Official Pi 0.82.1, Codex/Claude, restart recovery,
installed CV6, ATL9 and COMP2-E remain open. Evidence:
`../phase2d/P2D-W2C-W2D-pi-agent-input-continuation-v8.md`.

`CURRENT / P2D-W2C/W2D HARNESS MUTABLE PROMPT PREREQUISITE V9 SOURCE VERIFIED
(2026-08-14)`: Codex/Claude prompt and command stdin are now owned mutable bytes
and cleared on all production return paths. The current Codex
`exec --ephemeral` and Claude `--print --no-session-persistence` transports are
still one-shot, so neither Adapter advertises Agent Input capability and Loom
does not restart them to simulate continuation.

Version-locked persistent app-server/stream-json conformance, exact native
session handles, installed CV6, ATL9 and COMP2-E remain open. Harness package,
focused race, vet, diff and serial repository gates pass. Evidence:
`../phase2d/P2D-W2C-W2D-harness-mutable-prompt-prerequisite-v9.md`.

`CURRENT / P2D-W2C/W2D DAEMON RESTART ATTEMPT RECONSTRUCTION V10 SOURCE
VERIFIED (2026-08-14)`: startup now replays strict Attempt Loop and Inbox
authority, revalidates current Run/generation and frozen bindings, reconstructs
pre-model Queue/Steer/Inject checkpoints, and classifies an admitted open
ModelRequest as uncertain without repeating it. Historical generations and
terminal Runs are not active.

Per-Agent encrypted Inbox failure is isolated as `recovery_blocked`; protected
Journal/Run corruption remains fail-closed. Safe `agent_attempt_reconcile`
diagnostics retain Incident and non-content Provider/Agent binding identity.
Explicit resume authority/UI, persistent Codex/Claude protocols, installed CV6,
ATL9 and COMP2-E remain open. Evidence:
`../phase2d/P2D-W2C-W2D-daemon-restart-attempt-reconstruction-v10.md`.

`CURRENT / P2D-W2C/W2D CODEX APP-SERVER CONTINUATION V11 SOURCE VERIFIED
(2026-08-14)`: exact Codex 0.144.1 bytes now gate a bounded persistent
app-server runner. Queue/Steer/Inject starts repeated Turns on one ephemeral
thread only after prior final checkpoints. Response/thread/Turn identity and
per-Turn token usage are validated and accounting is combined. Ordinary
requests keep the one-shot exec path. Native thread persistence, restart
reattach and installed live remain open. Evidence:
`../phase2d/P2D-W2C-W2D-codex-app-server-agent-input-v11.md`.

`CURRENT / P2D-W2C/W2D CLAUDE STREAM-JSON CONTINUATION V12 SOURCE VERIFIED
(2026-08-14)`: exact Claude Code 2.1.196 bytes now gate one bounded
`--no-session-persistence` stream-json process. A stable emitted session ID,
exact replayed inputs, typed internal ToolCall/ToolResult closure, compaction,
final result, usage and reported cost are validated across all rounds. Ordinary
requests keep the one-shot JSON path. Native session persistence, restart
reattach and installed live remain open. Evidence:
`../phase2d/P2D-W2C-W2D-claude-stream-json-agent-input-v12.md`.

V11/V12 do not complete Phase 2D. CV6, ATL9 mixed-Team, COMP2-E, installed
Provider replies, explicit restart recovery and final governance UI/accounting
remain under the sole `ACTIVE / PARTIAL` Phase 2D Goal.

`CURRENT / P2D-COMP2-D LOCAL IPC BUNDLE OWNERSHIP V12 SOURCE VERIFIED
(2026-08-14)`: the final production IPC construction boundary now follows the
accepted strangler direction. `newProductDaemonRunnerWithPreparedDecisions`
provides only a typed revocable handler slot and no-I/O factory. The built-in
`loom-local-ipc` Bundle constructs Setup and the typed route handler during
Start, gates both during Ready, and revokes/quiesces the handler before Setup
cleanup. Observability and controlled-journey decorator order is unchanged.

The COMP2-A direct handler remains a test/parity oracle, but production passes
no legacy handler and exactly-one-source validation rejects ambiguity. RED,
full daemon, focused race, Composition race, serial repository Go, vet, AST
ownership and diff gates pass. This does not authorize COMP2-E deletion:
restart, crash-window, privacy, shutdown and installed App startup parity are
still required. CV6, ATL9, recovery/UI, accounting and real Provider live gates
remain open under the sole Phase 2D Goal. Evidence:
`../phase2d/P2D-COMP2-D-local-ipc-bundle-ownership-v12.md`.

`CURRENT / P2D-W2C/W2D AGENT ATTEMPT RECOVERY PROJECTION V13 SOURCE VERIFIED
(2026-08-14)`: the safe V10 `agent_attempt_reconcile` outcomes now survive the
Team board stage allowlist and are projected only onto the exact matching
Incident/Provider Account/Model Agent row. Swift recognizes the closed stage;
Mission governance distinguishes resume approval required, uncertain Provider
outcome, unavailable encrypted input, and recovery-state conflict. View
diagnostics and Copy incident ID remain available.

This is deliberately read-only governance. V10 records stay non-retryable, the
UI provides no invented Retry or Resume command, recovered state is not added to
the active registry, and no Runtime/Provider dispatch or native-session
reattachment occurs. Focused RED/GREEN, ten-run API race, complete Swift, full
API/daemon, vet and diff gates pass. Explicit recovery authority, restart-safe
native session persistence, installed CV6, mixed-Team ATL9 and COMP2-E remain
open. Evidence:
`../phase2d/P2D-W2C-W2D-agent-attempt-recovery-projection-v13.md`.

`CURRENT / P2D-W2C/W2D AGENT ATTEMPT RECOVERY DECISION AUTHORITY V14 SOURCE
VERIFIED (2026-08-14)`: one explicit `resume_pre_model` user decision now binds
the exact domain-separated V10 candidate digest and an independently
domain-separated Runtime restart-capability digest. Capability is resolved by a
trusted port rather than accepted from caller data. Authorization rebuilds the
candidate around a consistent snapshot and atomically fences recovery, Attempt
Loop and Run heads. A concurrent distinct decision has one winner.

`provider_outcome_uncertain`, candidate/binding/capability drift and Run/Loop
progress fail closed. The Journal fact contains only non-secret identity,
digests and controlled transition metadata. V14 intentionally issues no
dispatch lease and adds no IPC/UI action, active registry entry, native-session
reattachment or Provider call. Focused RED/GREEN, ten-run race, complete
`internal/work`, serial repository Go, vet and diff gates pass. Evidence:
`../phase2d/P2D-W2C-W2D-agent-attempt-recovery-decision-authority-v14.md`.

`CURRENT / P2D-W2C/W2D AGENT ATTEMPT RECOVERY CONSUMPTION V15 SOURCE VERIFIED
(2026-08-14)`: one exact V14 approval can now append one content-free
`AgentAttemptRecoveryConsumed` fact. The authority rebuilds the candidate,
re-resolves the trusted Runtime capability, and fences recovery, Attempt Loop
and Run heads. An internally generated lease ID prevents caller-controlled
idempotency from giving concurrent consumers duplicate success.

Only the CAS winner receives a process-local dispatch lease, and its frozen
grant can be taken once. Capability drift, competing consumption and every
later call return no lease. This still performs no active-Attempt registration,
native-session reattachment, IPC or Runtime/Provider call. Focused RED/GREEN,
ten-run race, complete `internal/work`, serial repository Go, vet and diff gates
pass. Evidence:
`../phase2d/P2D-W2C-W2D-agent-attempt-recovery-consumption-v15.md`.

`CURRENT / P2D-W2C/W2D AGENT ATTEMPT RECOVERY RUNTIME ATTACHMENT V16 SOURCE
VERIFIED (2026-08-14)`: the recovered `pre_model_resume` candidate now freezes
the exact Route Segment from its consumed Inbox authority. Missing or mixed
Segment metadata blocks only that Attempt. The v2 candidate digest,
authorization metadata and consumed grant bind the Segment, and the grant
freezes the recovery operation Incident ID.

Trusted composition validates the one-use grant, reattaches an identity-only
Runtime session, verifies exact Runtime/session digests and only then registers
the active Attempt. Any post-attachment failure closes the session; normal
close revokes the registry entry first. The port cannot issue a Provider call.
Production still has no restart-safe resolver/reattacher, recovery IPC/Swift
action or native-session persistence. Evidence:
`../phase2d/P2D-W2C-W2D-agent-attempt-recovery-runtime-attachment-v16.md`.

`CURRENT / P2D-W2C/W2D LOOM NATIVE RESTART CAPABILITY V17 SOURCE VERIFIED
(2026-08-14)`: only the built-in Loom Native adapter now advertises the explicit
`loom-owned-checkpoint/v1` conformance contract. The trusted resolver validates
the exact frozen Runtime/Provider Account/credential revision/Model binding and
content-free encrypted Capsule authority, then freezes Segment, checkpoint and
input identity in a domain-separated session digest.

Unsupported or duplicate Runtime identity, duplicate exact Capsule authority,
binding substitution and post-consumption Capsule drift fail closed. An
unrelated corrupt Capsule remains isolated. Resolution and V16 identity
attachment perform no Capsule-content read, credential lease, Runtime Execute,
HTTP or Provider call. Production continuation reconstruction, daemon wiring,
authenticated IPC and Swift Resume remain open. Evidence:
`../phase2d/P2D-W2C-W2D-loom-native-restart-capability-v17.md`.

`CURRENT / P2D-W2C/W2D ENCRYPTED LOOM NATIVE CONTINUATION V18 SOURCE VERIFIED
(2026-08-14)`: a dedicated Agent Checkpoint Store now encrypts each Loom Native
previous-output checkpoint with the Conversation DEK and authenticates the
complete frozen route, Attempt position, Capsule and content digest. It does
not reuse Tool Result storage and writes no content to the Journal. Exact
resolution rejects both missing and ambiguous candidates; Conversation deletion
crypto-erases the records.

Checkpoint persistence precedes Agent Input consumption. The controlled local
recovery path consumes the one-use grant, attaches and registers the exact
Attempt, restores Capsule/checkpoint/current consumed input, admits the next
ModelRequest, acquires the exact Vault credential lease, dispatches Loom Native,
terminalizes Step/Turn and deletes the checkpoint. Substitution fails before
credential or HTTP access, and uncertain Provider outcome remains fail closed.

Focused RED/GREEN, ten-run cross-package race, serial full repository Go, vet,
format and diff gates pass. This is source verification with a simulated
Provider fixture. Production daemon construction, authenticated recovery IPC,
Swift governance, installed CV6, mixed-Team ATL9, accounting and COMP2-E remain
open under the sole Phase 2D Goal. The next route must reconstruct authorized
frame validation, commit Run terminal/accounting facts and close prior execution
authorization together with continuation; direct IPC dispatch is not an
acceptable intermediate production path. Evidence:
`../phase2d/P2D-W2C-W2D-encrypted-agent-checkpoint-continuation-v18.md`.

`CURRENT / P2D-W2C/W2D RECOVERY FRAME AND TERMINAL CLOSURE V19A SOURCE VERIFIED
(2026-08-14)`: recovered Loom Native output now passes through exact Bridge
frame, dispatch ACK, Incident and original-grant authority before Step/Turn and
Run terminal state can advance. Completion re-resolves frozen accounting,
commits terminal Run state and revokes the exact original grant without
reconstructing its token. Evidence:
`../phase2d/P2D-W2C-W2D-recovery-frame-terminal-closure-v19a.md`.

`CURRENT / P2D-W2C/W2D PRODUCTION RECOVERY LIFECYCLE V19B SOURCE VERIFIED
(2026-08-14)`: the production mission Bundle now owns recovery composition,
restores the exact Team evidence observer and repairs the Run-terminal-to-grant-
revoke crash window at startup without any Provider redispatch capability.
Successful repair writes only content-free operational audit. Evidence:
`../phase2d/P2D-W2C-W2D-production-recovery-lifecycle-v19b.md`.

`CURRENT / P2D-W2C/W2D AUTHENTICATED RECOVERY IPC AND SWIFT GOVERNANCE V19C
SOURCE VERIFIED (2026-08-14)`: one typed authenticated private-UDS route now
separates read-only Preview, principal-bound Confirm and one-use Resume. The
Swift Store requires fresh authoritative state, exact confirmation and a
separate Resume action, rejects malformed wire state, suppresses duplicate
dispatch and discards uncertain local authority after failure.

Mission Inspector projects recovery onto the exact Agent and shows Harness,
Provider Account, Model and credential revision. App and daemon share the
request Incident while diagnostics exclude candidate/capability digests,
principal identity, content and secrets. Focused RED/GREEN, ten-run race, real
Go-to-Swift contract, complete Swift, serial repository Go, vet, format,
privacy and diff gates pass. Installed App recovery and real Provider
continuation were not run. Evidence:
`../phase2d/P2D-W2C-W2D-authenticated-recovery-ipc-swift-governance-v19c.md`.

V19A-V19C do not complete Phase 2D. Broader Runtime restart support, installed
CV6, mixed-Team ATL9, remaining ATL3-ATL8 work and COMP2-E stay open under the
sole `ACTIVE / PARTIAL` Phase 2D Goal.

`CURRENT / P2D-W2C/W2D TOOLCALL RECOVERY DECISION GOVERNANCE V20 SOURCE
VERIFIED (2026-08-14)`: `side_effect_unknown` ToolCalls now expose a separate
authenticated content-free Preview/Resolve authority. Candidate identity is
domain separated and CAS fenced; Abort, trusted observed-effect acceptance and
trusted replacement-Attempt authorization all terminalize the original
execution without replaying it.

Production currently advertises only Abort because no trusted observation or
replacement resolver is composed. Swift consumes only the exact daemon action
allowlist, requires confirmation, suppresses duplicate Resolve and invalidates
uncertain local state after failure. ToolCall Inspector and operational
diagnostics expose only safe Tool/Job/Incident and stage/result metadata.
Evidence:
`../phase2d/P2D-W2C-W2D-tool-recovery-decision-governance-v20.md`.

V20 does not complete Phase 2D. Trusted observed-result delivery,
replacement-Attempt startup, installed CV6, mixed-Team ATL9, remaining
ATL3-ATL8 work and COMP2-E stay open under the sole `ACTIVE / PARTIAL` Phase 2D
Goal.

`CURRENT / P2D-W2C/W2D LOOM NATIVE SEQUENTIAL CONTEXT TOOLS V21 SOURCE
VERIFIED (2026-08-14)`: Loom Native now performs up to four strictly sequential
`loom_read_context` continuations in one frozen Provider exchange. Each call has
an independent monotonic ToolCall and encrypted payload lineage; accepted-only
state does not release the exclusive admission slot, and only an authoritative
delivery proof permits the next call.

All Provider-round usage is accumulated into the same frozen Run/Account
accounting result. The request after the fourth call omits the tool schema and a
Provider that nevertheless emits another call fails closed. Real product
composition tests cover two Provider continuations through the Attempt Loop,
including Provider proof, terminal Step/Turn facts, payload delivery and
content-free Journal/Bridge boundaries. Evidence:
`../phase2d/P2D-W2C-W2D-loom-native-sequential-context-tools-v21.md`.

V21 is not the general Tool Loop or a live acceptance. Read/Grep/Web/MCP,
parallel tools, additional Runtime transports, installed CV6, mixed-Team ATL9
and COMP2-E remain open under the sole `ACTIVE / PARTIAL` Phase 2D Goal.

`CURRENT / P2D-W2C/W2D CODEX AND CLAUDE BOUNDED MULTI-CONTEXT V22 SOURCE
VERIFIED (2026-08-14)`: Codex and Claude Code now support up to four distinct
governed Context reads through one Attempt-scoped private MCP service. Each
result stays accepted/pending until validated Harness final output; later MCP
traffic is not treated as delivery proof. Final output acknowledges the exact
bindings in order, and partial failure resumes at the first undelivered binding.

Product policy assigns four parallel pending Context slots only to Codex and
Claude. Loom Native, Pi and default Runtime types retain exclusive/one and V21
continues to require Provider-continuation proof per result. Duplicate, fifth,
concurrent, post-seal and Prepare/ACK races fail closed. Complete affected
packages, ten-run race, vet and formatting gates pass. Evidence:
`../phase2d/P2D-W2C-W2D-codex-claude-bounded-multi-context-v22.md`.

V22 does not complete the general Tool Loop or Phase 2D. General tools,
arbitrary parallel effects, Runtime restart reattachment, installed CV6,
mixed-Team ATL9, UI/accounting and COMP2-E remain open under the sole
`ACTIVE / PARTIAL` Phase 2D Goal.

`CURRENT / P2D-W2A/W2D BUILD 108 INSTALLED CONVERSATION AND GOVERNANCE SLICE
VERIFIED (2026-08-22)`: the installed App starts its bundled daemon without a
manual service step or a transient unavailable publication, retains the exact
25-Provider/7-Runtime/4-Profile setup catalog, and completes a real OpenCode
conversation. The focused DeepSeek/OpenCode Context Capsule gate passes all
four explicit route transitions; `summary_only` and later `start_clean` both
freeze distinct disclosure receipts with explicit omission counts.

Mission pagination now checks streams discovered after projection rebuild
before publishing `has_more=false`. Mission remains a title-to-one-workflow
drill-in, and Team pickers collapse same-source same-name historical
configurations while retaining every authoritative execution in Mission
history. Full Go, vet, diff, Swift, strict contract, reproducible build and
installed signature gates pass. Builds 104-107 were intermediate candidates;
build 108 is the current installed slice. Evidence:
`../phase2d/P2D-W2A-W2D-runtime-governance-context-build108.md`.

This does not complete Phase 2D. The complete shipped-account Vault lifecycle,
trust-domain/restart/encrypted-Capsule coverage, installed four-Provider Team,
remaining account-local failure cells, installed MCP mid-flight revocation,
explicit fallback and complete accounting visual matrix remain open.

`CURRENT / P2D-W1E/W1F/W1G + P2D-W2D-FL1/FL2 BUILD 121 SOURCE + INSTALLED
VERIFIED (2026-08-23)`: custom Provider candidates now freeze deterministic
candidate, endpoint-fingerprint and policy bindings before review. W1E enforces
HTTPS, public DNS/address resolution, no URL credentials, no redirects and no
proxy use. W1F records a 15-minute, versioned Journal approval separately from
credential import. W1G exposes the exact non-secret review surface and supports
account-scoped custom OpenAI-compatible verification; approval itself neither
imports nor sends a credential.

FL1/FL2 add a credential-free loopback Failure Lab with separate temporary
accounts. The installed private-UDS and Swift UI matrix now passes auth, rate
limit, timeout, insufficient balance, corrupt Vault record and credential-
revision conflict. Each target receives its exact safe stage/code/retryability
and Incident ID while the healthy peer succeeds. Controlled account-local
failure coverage is therefore closed rather than inferred from source tests.

Build 121 also fixes two installed-loop regressions: local socket probing is
non-blocking, and secure CC Switch database growth no longer invalidates source
identity while path replacement and unsafe filesystem identity still fail
closed. The installed contract sees 25 Providers, 7 Runtimes, 4 Conversation
Profiles and 5 safe candidates, and a real OpenCode Conversation returns exact
`E2E-OK`.

Phase 2D remains the sole `ACTIVE / PARTIAL` Goal. Live import/verify of an
operator-approved custom endpoint plus its executable frozen Agent binding, and
the four-distinct-Provider installed Team, remain open. Existing W2A/W2B,
Credential Vault, Composition and Attempt Loop work is preserved.

## Phase 2D external live-gate scope deferral (2026-08-24)

Status: `ACCEPTED / DEFERRED / NO FUTURE PHASE ACTIVATED`.

This amendment supersedes earlier statements that made the following three
external live scenarios mandatory Phase 2D completion gates:

1. Codex/OpenAI + Claude/Anthropic + Loom/Kimi + Loom/MiniMax in one installed
   independently bound Team.
2. Real Provider Account revoke or rate-limit isolation while peer Agents
   continue.
3. Approved custom-endpoint credential import, verification, exact frozen
   binding and real Conversation.

They are preserved as future-Phase acceptance candidates. No Phase number or
parallel product Goal is activated by this deferral. Existing implementations,
controlled tests and historical live evidence remain valid and are not reset.
Phase 2D retains the underlying per-Agent binding, Vault, diagnostics, failure
isolation, explicit fallback, accounting and governance contracts; only the
three external live completion requirements move.

## Phase 2D active-scope closure — Build 127 (2026-08-24)

Status: `ACCEPTED / COMPLETE FOR ACTIVE SCOPE`.

Build 127 closes the three completion cells that remained after the external
live-gate deferral:

1. Trust-domain confirmation compares the immutable source Segment binding
   with the target route and requires acknowledgement for each changed policy
   dimension.
2. Context Capsule capacity handling freezes exact, estimated or unavailable
   authority, deterministic budget packing, counter identity, safe contribution
   totals and explicit omissions. Unknown capacity remains unavailable.
3. Route Transition acceptance covers all disclosure modes plus missing/stale
   binding rejection through the authenticated private UDS boundary, with no
   mutation on failure.

Complete Go, relevant race, vet, Swift, package/sign/install and installed
App/daemon restart gates pass. No credential was changed and no new external
Provider request was made. The four-distinct-Provider Team, real-account
revoke/rate-limit matrix and custom-endpoint real Conversation remain deferred
exactly as recorded above; no future Phase is activated here.

Evidence:
`../phase2d/P2D-W2A-W2D-trust-capacity-route-build127.md`.

## Phase 2D HG1 continuation after Build 127 (2026-08-24)

Status: `ACTIVE / PARTIAL / SOURCE VERIFIED / INSTALLED PENDING`.

ADR-0022 adds the unified Harness Gateway and Segment Session lifecycle to the
same Phase 2D Goal. It does not reopen the three Build 127 governance cells:
trust-domain confirmation, complete Context Capsule capacity admission and the
authenticated Route Transition matrix remain accepted. It does add a new
completion boundary for persistent Harness Sessions and exact Response
cancellation.

The current source registers all five built-in Harness identities through one
Gateway. Codex now owns a persistent App Server Session per immutable Segment,
freezes reasoning effort with the rest of its authority, reuses its native
thread across same-Segment turns and supports exact `turn/interrupt` through a
metadata-only Swift/private-UDS cancellation route. Full Go, cross-layer race,
vet and current Swift source suites pass.

HG1 is not complete. Claude Code, OpenCode, Pi and Loom Native still enter
registered compatibility Backends; remaining lifecycle/event findings and the
installed Codex reuse/switch/concurrency/cancel matrix remain open. Build 127 is
the installed predecessor, and this source checkpoint did not read or modify a
credential, contact an external Provider, package or install the App.

Evidence:
`../phase2d/P2D-HG1-persistent-codex-cancel-source.md`.

## Phase 2D HG1 authority/evidence hardening (2026-08-25)

Status: `SOURCE VERIFIED / INSTALLED PENDING`.

The Build 127 trust-domain confirmation, complete Context Capsule capacity and
authenticated Route Transition cells remain accepted. Their exact authority is
now also carried through HG1: Segment Session, Response, privacy-safe event and
operational diagnostic all freeze the same optional Route Transition review
digest. Native-auth Harnesses preserve explicit absent credential authority;
brokered routes still require exact Provider Account and positive credential
revision.

All five built-in Harnesses now use governed Segment Backends. Codex alone owns
a persistent native App Server process/thread Session; Claude Code resumes a
stable native CLI session ID across bounded response processes; OpenCode, Pi
and Loom Native retain persistent Loom-owned Segment authority without claiming
unsupported native persistence.

The installed G7 gate is tightened: raw daemon JSONL may satisfy source
verification only. Installed evidence must be the App-exported privacy-safe
diagnostic bundle, must originate from `$HOME/Applications/Loom.app`, and must
embed App/daemon build hashes matching the exact bundle under acceptance. This
prevents an old trace from being combined with a newly stamped bundle.

Evidence remains:
`../phase2d/P2D-HG1-persistent-codex-cancel-source.md`.

## Phase 2D Build 136 installed HG1 acceptance (2026-08-25)

Status: `ACCEPTED / ACTIVE SCOPE COMPLETE`.

Loom `0.5.3` Build 136 is installed at the canonical owner-controlled user App
path and passes the ADR-0022 G7 installed matrix. The App-exported, privacy-safe
v3 trace is bound to the exact installed App and daemon hashes. It proves source
Codex Session reuse, reviewed same-visible-Conversation switching to Loom
Native/DeepSeek, two target responses with independent Attempt Capsules,
cross-Conversation overlap, exact Response cancellation and post-cancel Session
reuse. The diagnostic exporter and verifier use the production Codex backend
identity, freeze Profile authority and fail closed on historical aliases,
unknown fields or binding substitution.

The trust-domain confirmation, full Context Capsule capacity and Route
Transition cells remain accepted and are now exercised through the installed
Gateway path. The four-distinct-Provider Team, real-account revoke/rate-limit
matrix and custom-endpoint real Conversation remain deferred exactly as decided
on 2026-08-24; no future Phase or parallel Goal is activated here.

Evidence:
`../phase2d/P2D-HG1-build136-installed-g7.md`.
