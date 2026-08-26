# Current State

## Phase 2D

Status: `ACCEPTED / COMPLETE`

Phase 2D is the sole completed product Goal. Phase 2C remains closed. ADR-0022
formed the final accepted Harness Gateway and Segment Session boundary; Build
136 supplies its installed completion evidence, with Build 127 retained as the
verified predecessor.

The Goal is to make Loom usable immediately after the App opens, support real
conversation, and govern multi-Provider, multi-model Agent Teams. Each Agent
must independently freeze its Harness, Provider Account, credential revision,
model, limits and capabilities. Phase 2D completion uses the installed core
conversation, existing mixed-Team slices, controlled failure matrix and
actionable governance diagnostics. The externally credentialed four-Provider
Team, real-account revoke/rate-limit matrix and custom-endpoint live loop are
deferred to a later Phase by the 2026-08-24 scope decision.

## Current checkpoint

Status: `CURRENT / BUILD 188 INSTALLED / BLOCKED MISSION INTERVENTION / PHASE 2D COMPLETE`

Release publication status: `v0.5.3-rc.1 / SOURCE + PACKAGE VERIFIED`. The
canonical source branch is `codex/release-v0.5.3-rc.1`; installed Build 188
remains the behavioral reference. The RC gate passed the complete Go repository,
`go vet`, the focused and package race matrix, all 379 XCTest cases with two
conditional skips, all 20 Swift Testing cases, reproducible two-build packaging
and the transactional installer fixture. The race gate found and closed a
controlled Mission projection fixture that returned shared slices instead of
the production read model's independent snapshot; the focused race test, the
complete `internal/app` race package and the remaining API/IPC/execution/runtime
race matrix now pass.

The root README, product entry guide and Changelog use the Conversation ->
Mission -> Agent Team mental model and explicitly separate Runtime, Provider
Account, Model and executable Route. This publication checkpoint does not
expand the accepted Phase 2D boundary or activate the three deferred external
scenarios.

Loom `0.5.3` Build 188 is the current canonical user installation. A Blocked,
Failed or Cancelled Mission now keeps a fixed `Continue this Mission` composer
in Mission Room. The user can enter bounded guidance and open a prefilled
`Continue Mission` review for the same Team. The continuation is a fresh,
separately audited Attempt: the previous Attempt, visible output, failure and
Incident remain unchanged, and no Provider work begins until the user confirms
Mission context, reviews preflight and explicitly starts the new Attempt. Empty,
oversized and active-Mission drafts fail closed; an unavailable Team routes the
user to Runtime & Providers. Closing review preserves the draft, while a
successful start clears it.

Installed UI acceptance used the existing Blocked MiniMax Mission. It proved
the empty disabled state, bounded draft admission, exact Team/title/objective
prefill, explicit context confirmation gate, no-write close path and draft
retention. The acceptance stopped before preflight, Provider dispatch or new
Attempt creation, then removed the test draft. The complete macOS suite passes
379 XCTest cases with two conditional skips and 20 Swift Testing contract cases;
the reproducible native App build fixture, strict signature check and managed
daemon launch pass. Evidence is recorded in
`P2D-W2C-W2D-build188-blocked-mission-intervention.md`.

Build 187 introduced the preceding Mission live-activity checkpoint. Mission Room
follows a visible nonterminal Mission once per second and renders an `Agent
conversation` section while the Team is still working. Each Agent row shows the
frozen Harness, Provider Account, model and status; bounded `node_output_delta`
records are grouped by logical node and Attempt so partial model output appears
as it is observed instead of waiting for the final Mission result. The view
keeps at most 24,000 visible characters per Agent, collapses to the latest 2,400
characters by default and provides `Show full` for longer output.

The live stream is deliberately labelled tentative until terminal Evidence is
accepted. Only the authorized text delta is rendered: prompts, credentials,
Provider bodies and hidden reasoning remain excluded. Output observed by the
App is retained across the running-to-terminal refresh, but an App/daemon
restart does not reconstruct earlier tentative output that was never committed
as authoritative Evidence. The terminal result, accepted Evidence, accounting
and actionable failure state remain available.

Installed live acceptance created a new two-Agent Team using only Loom Native,
`minimax.primary` and `MiniMax-M3`. The Mission moved from 0/2 to 1/2 while the
second Agent's complete visible update appeared before the first Agent had
finished. The remaining Agent later ended at `Provider Http`, so the Mission
correctly became Blocked while preserving the successful peer, per-Agent
binding, accounting and Incident diagnostics; this acceptance does not claim a
successful final Provider response. Build 187 also preserves the last complete
activity snapshot when a follow-up page races a newly appended event instead of
blanking the Mission on a cursor conflict.

The complete macOS suite passes 377 XCTest cases with two conditional skips and
20 Swift Testing contract cases. The reproducible native App build fixture,
strict deep signature verification, managed bundled-daemon launch and installed
UI inspection pass. Evidence is recorded in
`P2D-W2C-W2D-build187-mission-live-conversation.md`.

Build 183 was the preceding canonical result-publication checkpoint. A pure
OpenCode + MiniMax Agent Team completed the two-node Neon Snake Mission and
published the playable single-file result to
`/Users/lune/Documents/Loom-Projects/snake-game/index.html`; no Luna route or
fallback was used. The installed Mission list shows `Neon Snake · MiniMax` as
Succeeded. Its Mission Room presents `Web result ready`, `Open result` and
`Show in Finder`; installed UI acceptance clicked `Open result` and Chrome
opened the exact local `index.html`.

Build 182 closes the publication loop behind that result. Build 183 additionally
normalizes malformed Mission IPC payloads to the actionable
`invalid_response` code instead of a generic internal error. A user-approved
terminal restart is consumed after the first successful Team dispatch wave, so
later waves can finish and publish instead of repeatedly reopening the terminal
Team and ending in a dispatch conflict. Completed-flight errors are surfaced
only for terminal projections; active concurrent callers continue joining the
single visible execution. Mission presentation metadata now preserves a
validated output workspace across App restart and opens `index.html` when
present, otherwise the workspace folder.

Browser acceptance covers desktop and 390x844 mobile, keyboard/WASD, touch
controls, pause/restart, `snake_best_score_v1` persistence across reload, zero
console errors, no viewport overflow and no external resources. The game is
served locally at `http://127.0.0.1:8766/` for the current acceptance session.
The complete macOS suite passed 373 XCTest cases with two conditional skips and
20 Swift Testing cases; the Mission result tests, 30-round concurrent-start
regression, affected Go packages, complete Swift/Go IPC contract package and
installed bundle/UI checks pass.

This Build 183 checkpoint closes the final active user-facing follow-up: a
Mission can run with only the selected MiniMax route, publish its workspace,
surface the result in Mission Room and open the playable artifact without Luna
or an implicit fallback. Phase 2D is complete for its accepted scope. Residual
internal Composition and Tool Loop extensions remain ordinary backlog and do
not reactivate this Goal.

Build 138 was the preceding canonical user installation. The App and
bundled daemon are validly signed, launch as one managed product and pass the
installed HG1 G7 matrix inherited from Build 136. In one visible Conversation, two Codex responses
complete before a reviewed Loom Native/DeepSeek Segment opens; two DeepSeek
responses then reuse the target Segment Session with independent Attempt
Capsules. A separate Conversation overlaps the cancelled Response, the exact
cancelled Attempt remains durable, and the same Session completes a later
Response. The App-exported privacy-safe diagnostic bundle passes the closed G7
machine verifier and is bound to the installed App and daemon hashes.

Build 138 raises the `chat_message` local IPC budget to 30 minutes while keeping
setup operations bounded independently. Installed live use proved that the old
605-second boundary was real, then produced a successful GPT-5.6 Sol / Max
architecture response in Conversation 203. Later Sol turns still failed inside
the Codex app-server runtime with `provider_unavailable`; the larger Loom IPC
budget therefore closes the transport timeout but does not claim Provider
runtime stability.

An earlier MiniMax-only live attempt exposed a separate P0 privacy defect:
the OpenCode Agent runner placed the complete governed Mission Context in the
child process argv. The affected Attempt was stopped before any workspace file
was committed. Build 139 is the source candidate that removes all model-visible
content from OpenCode argv and supplies it only through the child stdin; focused
tests freeze argv non-disclosure, stdin delivery and post-run prompt zeroization.
That correction is included in Build 161 and the resumed MiniMax Mission passed
the process-argv non-disclosure boundary and produced the game candidate.

Build 136 also closes the installed disclosure-detail decoder drift and the
diagnostic route-identity drift. The Context inspector renders capacity,
omissions and disclosure receipt fields from the real daemon projection. The
exporter and verifier recognize the production Codex backend
`backend.codex.app-server`, freeze the exact Profile ID and continue to reject
the historical alias, unknown fields and authority substitution. Full macOS,
daemon, core Go, vet, live-harness and diff verification is green. Evidence is
recorded in `P2D-HG1-build136-installed-g7.md`.

The following paragraphs preserve the source-hardening history leading to this
installed checkpoint.

Build 127 (`0.5.3`) supersedes Build 126 and closes the earlier Route/Context
governance gates. Trust-boundary review compares the last immutable Segment binding
with the target route and requires a separate acknowledgement when trust
domain, retention mode or data region changes. Context Capsule admission now
freezes exact, estimated or unavailable capacity authority, deterministic
input budget, reserved output, adapter/tool overhead, privacy-safe
contributions and explicit omissions. The authenticated private-UDS Route
Transition matrix covers all three disclosure modes plus stale/missing binding
rejection with zero mutation.

Post-Build-127 parallel review found and closed three source gaps. Stale route
conflicts no longer auto-retry around explicit confirmation; missing legacy
Segment authority is shown as unavailable and requires acknowledgement; model
and reasoning changes use the same reviewed Transition path. Capacity counting
now occurs only after policy/scope admission, and Pi freezes explicit
`unavailable` capacity authority instead of using the legacy capacity-free path.
These changes are source-verified and are not yet installed evidence.

The latest source checkpoint closes the remaining cross-layer confirmation
gap. A trust-boundary acknowledgement now carries one canonical review digest
from Swift through the private UDS and is revalidated by `LocalProductChatAPI`
while holding the Conversation lock. The digest binds the thread, immutable
source Segment and binding, exact target binding, disclosure mode and changed
trust dimensions; accepted Segments and Attempts freeze the same review digest.
Tamper, drift and replay fail before mutation. Context Capsule capacity now
completes policy/scope admission and canonical ordering before tokenization,
with overflow, unknown-capacity, required-priority and privacy-negative bounds.

A second and third parallel hardening pass closed the remaining combined-authority
gaps. Swift consumes a Route Transition generation at confirmation, and changing
the target model or reasoning effort invalidates that confirmation before send.
The canonical cross-layer acknowledgement is now version 3 and binds the exact
target Profile ID, reasoning effort, Execution Binding and context mode; the
daemon recomputes it under the Conversation lock before mutation. Incomplete
source or target policy authority, including blank-to-blank legacy policy, always
requires review. Persisted v1/v2 Segment digests remain read-compatible, but new
requests require v3 authority. A legacy Segment without frozen binding authority
fails closed and offers a real Start new conversation recovery action.

Capacity handling now performs policy/scope admission before tokenization,
revalidates TokenCounter identity after counting and bounds cumulative authority,
contribution and extension totals. Every Attempt receives an independent
capacity-governed Capsule and disclosure receipt while the Segment retains its
immutable opening Capsule. Production Team role, dependency and aggregation
Capsules use the same frozen capacity authority and counter rather than the
legacy capacity-free extension path.

The HG1 source slice now routes all five built-in Harness identities through one
registered Gateway. Codex uses a persistent App Server Session per immutable
Conversation Segment; same-Segment turns reuse its thread and exact frozen
binding, while a new Segment opens a new Session. Response cancellation crosses
the Swift App, private UDS, product API and Gateway to the exact Incident and
Codex turn. The cancelled Attempt is retained as `cancelled`, the user message
and Segment remain authoritative, and the Codex Session is reusable after a
clean interrupt. The four non-Codex identities now enter governed Segment
Backends instead of compatibility Backends. Those Backends own immutable
Session authority, per-Response validation, cancellation and cleanup. Claude
Code now freezes and resumes one native CLI session ID across same-Segment turns
using `--session-id` followed by `--resume`; each response still uses a bounded
process, so only Codex has proven persistent native process/thread ownership.
OpenCode, Pi and Loom Native remain one-shot native protocols inside persistent
Loom-owned Segment authority. The native-auth Claude Code Conversation Profile
and production Gateway path do not acquire a Provider credential lease.

HG1 lifecycle hardening now versions every content-free event with its event
schema, Configured Harness version and Backend version. An unhealthy exact
Session cannot reopen until the old native Session has completed cleanup;
`SessionFailed` is emitted only after that cleanup, and production Session
cleanup has an independent four-second upper bound. Ordinary Provider response
failure leaves the exact Session reusable. Source acceptance now proves two
Codex turns reuse one Session, a reviewed Codex-to-Loom-Native/DeepSeek switch
creates a second frozen Segment inside the same visible Conversation, and two
different Conversations overlap through the Gateway. A second DeepSeek turn
then reuses the target Gateway Session while freezing a new Attempt Capsule and
binding digest against the unchanged Segment opening Capsule. Operational diagnostics
preserve the complete content-free Gateway envelope, including event time,
sequence, Harness/Backend versions and Session/Segment/Workspace/Response
identities; the stored projection is revalidated against the authoritative
Gateway event contract.

Build 127 passed the complete Go repository, relevant race tests, vet, all 342
macOS tests with 2 conditional skips, release packaging, signing, transactional
installation, installed accessibility inspection and App-managed daemon
restart. Its App executable hash is
`fb4ebbce2487dbc19a92fb18c6ca63fae25d0ea8cdc25d1bb514d7ed9072ec3b`;
the bundled daemon hash is
`3011561ec9175e9e162f4e611868f7ccc6dcaf92a38275dd8807b642fdd77a2f`.

After Build 127, the HG1 source passed `go test -p 4 ./... -count=1`, the
cross-layer race suite, `go vet ./...`, and all 351 current macOS tests with 2
conditional skips and no failures. These are source results only. Build 127 is
still the installed predecessor; the persistent Codex Session and Stop response
flow have not yet been packaged or accepted in the installed App.

For the latest lifecycle/backend delta, focused Gateway, adapter, daemon and
ADR-0022 acceptance tests passed, as did the cross-layer race suite and
`go vet ./...`. A full `-p 4` repository run passed every package except
`internal/localipc`: concurrent Swift probe builds caused one transient UDS
failure and the package-level ten-minute timeout. The failed UDS test then
passed alone, the strict Swift/Go setup probe passed alone, and the entire
`internal/localipc` package passed serially in 385.697 seconds. No product
failure was reproduced outside that parallel build-pressure run.

For the trust/capacity/Route Transition revalidation, the affected Go packages,
private-UDS matrix, focused race suite, full vet and diff checks passed. The
complete macOS suite passed 352 tests with two conditional gates skipped.
The repository-wide Go run again passed every package except the default
ten-minute `internal/localipc` package budget while its strict Swift probe was
still compiling. That exact test passed alone in 165.277 seconds, and the full
`internal/localipc` package passed with an explicit fifteen-minute budget in
435.475 seconds. No credential, installation or external Provider request was
used; this remains source evidence after installed Build 127.

For the latest five-Harness production-reachability and diagnostic-envelope
delta, the complete repository passed with
`go test -p 4 ./... -count=1 -timeout=15m`; `cmd/loomd` completed in 233.832 seconds and
`internal/localipc` in 368.159 seconds. Focused Gateway/daemon race tests,
`go vet ./...` and `git diff --check` also passed. No Swift source changed in
this delta, so the preceding complete 352-test macOS result remains the current
Swift source evidence. No credential, network, package or installation action
was performed.

For the trust-domain, complete Capsule-capacity and Route Transition parallel
closure, all affected Go packages and combined daemon acceptance tests passed,
including the second target-route turn and production Team role/dependency/
aggregation capacity path. Focused race tests, `go vet ./...` and
`git diff --check` passed. The complete macOS suite passed 355 tests with two
conditional skips and no failures. A repository-wide Go run passed every
product package; one Pi cancellation test missed its child-start marker only
under the full parallel load, then passed alone and the complete Pi adapter
package passed in 57.685 seconds. `cmd/loomd` passed in 306.851 seconds and
`internal/localipc` in 599.481 seconds. No credential, network, package,
installation or installed-App action was performed, so Build 127 remains the
installed predecessor.

The latest HG1 reliability and acceptance pass closes four additional source
gaps. Gateway event schema v3 now carries a random, non-secret daemon-instance
boundary plus the complete privacy-safe frozen
Session and Response authority through daemon diagnostics and the App's
redacted diagnostic export. Cancellation after Codex has accepted
`turn/start`, but before the native turn ID response is observed, performs a
bounded exact `turn/interrupt` and only returns the Session to service after the
interrupted terminal event. A daemon restart reconciles orphaned
`dispatching` Conversation Attempts to durable, content-free `cancelled`
Attempts before serving traffic. A private-UDS acceptance test proves exact
cancellation, peer-Conversation progress and same-Session recovery through the
real IPC, API, Gateway and Codex fixture path. Production composition tests now
cover exact registered Gateway reachability for Claude Code, OpenCode, Pi and
Loom Native as well as Codex.

The G7 machine verifier can inspect either the App's privacy-safe diagnostic
bundle or the owner-only daemon operational trace for source development. It
validates complete frozen
authority, same-Session Codex reuse, same-visible-Conversation Codex-to-
DeepSeek Segment transition, cross-Conversation overlap, exact cancellation and
post-cancel Session reuse inside one exact Gateway instance; records from
different daemon lifecycles cannot be combined. Unknown or content-bearing
fields fail closed. Historical v1/v2 records remain readable but cannot satisfy
G7. The acceptance fixture, key package race suite, `go vet ./...` and
`git diff --check` pass. The complete macOS run passes 359 XCTest cases plus 20
Swift Testing cases, with two conditional visual-export skips and no failures.
The controlled repository-wide `go test -p 4 ./... -count=1 -timeout=15m` run
passes every package, including `cmd/loomd` in 403.236 seconds,
`internal/localipc` in 673.519 seconds and the Pi adapter in 146.751 seconds.
No credential, network, package, installation or installed-App action was
performed. G7 is source-verifier green and installed-open.

The latest parallel trust/capacity/Route Transition review closed three
additional source bypasses. Dispatch-safe Capsule rebuilding now preserves the
exact frozen Capacity Projection and requires its matching TokenCounter;
controlled Mission, Conversation and Team role/dependency/aggregation paths no
longer admit a capacity-free production Capsule. Every existing-thread new
Segment with binding authority now requires one canonical v3 Route Transition
review, including same-trust model, reasoning, Provider Account and credential
revision changes. Swift generates and consumes the same review for these
transitions, while the daemon recomputes it under the Conversation lock before
any message, Segment, Attempt, Capsule or responder mutation. Unknown Harness
and same-Provider multi-account acceptance additionally prove zero executor
calls and exact account/reference/revision lease selection.

The corrected source passes the complete Go repository with a fifteen-minute
package budget, including `cmd/loomd` in 273.638 seconds,
`internal/localipc` in 460.562 seconds and the Pi adapter in 111.539 seconds.
Focused cross-layer race tests and `go vet ./...` pass. The complete macOS suite
passes 359 XCTest cases plus 20 Swift Testing cases, with two conditional visual
export skips and no failures. No credential, network, package, installation or
installed-App action was performed; Build 127 remains the installed predecessor.

The 2026-08-25 parallel HG1 authority pass closes the final source-evidence
gaps around those three cells. Native Harness Sessions now preserve exact
absent credential authority rather than synthetic accounts. The immutable Route
Transition review digest is frozen through Session, Response, event and
operational diagnostic authority; the reviewed DeepSeek target carries it and
the initial Codex Segment does not. Swift performs closed-key validation before
decoding v3 events, and Go explicitly serializes native empty account, revision
zero and empty policy so missing fields cannot masquerade as native authority.

Production-path tests now prove cancellation before the Codex native turn ID is
observable and two distinct Codex Conversations overlapping through separate
Segment Sessions. G7 rejects same-Session Response overlap, Capsule reuse,
non-Codex cancellation, incomplete Session lifecycle, unknown/content-bearing
fields and Route review drift. Raw daemon JSONL remains usable for source
verification only; installed G7 evidence must come from the App diagnostic
bundle, match the exact App/daemon hashes and target the canonical user install.

The affected tests, ten repeated IPC/concurrency runs, focused race tests, the
G7 negative harness, `go vet ./...` and `git diff --check` pass. The full Swift
suite passes 363 XCTest cases with two conditional skips plus 20 Swift Testing
cases. A controlled repository Go run passed every non-daemon package and found
one stale Claude native-auth fixture; after correction the complete daemon
package passed twice, most recently in 154.233 seconds. No credential, network,
package, installation or installed-App action occurred. Build 127 remains the
installed predecessor and G7 remains open.

The follow-up three-agent acceptance pass found and closed two additional
source-evidence gaps. Dispatch-safe Context Capsule rebuilding now preserves
every existing policy/access/budget omission, reconstructs retrievable budget
omissions, retains the exact frozen Capacity Projection and TokenCounter
identity, and validates cumulative contribution plus omission totals before
tokenization. The G7 verifier now requires two completed Loom Native/DeepSeek
responses in one target Session with independent Attempt Capsule digests. Its
result exposes the exact non-secret Provider Account, credential revision,
model and Route Transition review digest, while negative fixtures prove that a
missing second turn or per-event authority substitution fails closed.

Production composition now also exercises Codex through the real factory and
specialized `backend.codex.app-server` path alongside the other four built-in
Harnesses. The complete repository passes
`go test -p 4 ./... -count=1 -timeout=15m`, including `cmd/loomd` in 209.050
seconds, `internal/localipc` in 335.647 seconds and the Pi adapter in 81.282
seconds. Focused cross-layer race tests, the expanded G7 source harness,
`go vet ./...` and `git diff --check` pass. The complete macOS suite passes 364
XCTest cases with two conditional skips plus 20 Swift Testing cases. No
credential, network, package, installation or installed-App action occurred;
Build 127 remains the installed predecessor and installed G7 remains open.

The subsequent completion audit found one remaining Gateway lifecycle gap:
Session opening was owned by the daemon lifecycle but had no independent
deadline, so a non-responsive Backend could retain an opening slot until daemon
shutdown. Gateway opening is now bounded to 30 seconds in production. A timed
out or late-success Backend Session reaches a terminal content-free lifecycle,
is cleaned before the same Segment can reopen and releases its exact Harness
slot. Direct contract tests also prove Configured Harness registry authority is
copied across construction/resolution and a completed Response ID cannot reach
the Backend twice. Focused Gateway and five-Harness production composition
tests, including race runs, pass; this delta remains source-only.

The completion-audit full verification is green under a controlled repository
run. The first `-p 4` run exposed one transient `internal/localipc` UDS result
under parallel build pressure; that exact test then passed 20 repeated runs and
the complete package passed serially in 240.500 seconds. The controlled
`go test -p 2 ./... -count=1 -timeout=15m` rerun passed every package, including
`cmd/loomd` in 199.682 seconds, `internal/localipc` in 270.873 seconds and the Pi
adapter in 69.219 seconds. The final macOS run passed 364 XCTest cases with two
conditional skips plus 20 Swift Testing cases. G7's source fixture,
`go vet ./...` and `git diff --check` also passed. No credential, network,
package, installation or installed-App action was performed; Build 127 remains
the installed predecessor and installed G7 remains open.

The installed-evidence audit then closed three verifier and export gaps. G7
installation evidence now accepts only the App's complete, closed diagnostic
bundle schema; raw daemon JSONL remains source-only, and missing or nested
unknown bundle metadata fails closed. The App retains a bounded 512-event
privacy-safe window so a full Gateway lifecycle is not displaced by ordinary
diagnostics. The machine matrix now proves two Codex completions occur before
the DeepSeek Session opens, and persisted gate documents revalidate the full
versioned summary, including route ordering, overlap and cancellation, instead
of trusting a stored `pass` label. The full macOS suite passes 365 XCTest cases
with two conditional skips plus 20 Swift Testing cases; the G7 harness passes
20 repeated runs, `go vet ./...` and `git diff --check` pass. This remains
source-only; no credential, network, package, installation or installed-App
action was performed.

The HG1 installation candidate is now versioned as Loom `0.5.3` Build 128 in
source. The runbook and G7 harness use the same owner-controlled canonical path,
`$HOME/Applications/Loom.app`; the previous system `/Applications` instruction
would fail the installer's ownership boundary and has been removed. A
source-only regression requires the candidate build to advance beyond installed
Build 127 and rejects any return to the system path. Build 128 has not been
packaged or installed, while the read-only bundle check still identifies the
installed predecessor as Build 127 with its previously recorded hashes.

The installed App retains the authoritative Provider, Runtime and Conversation
Route separation verified by Builds 125-126. OpenCode appears only
in Runtime inventory; searching Model Providers for `opencode` returns no
match. OpenCode + DeepSeek/MiniMax remain executable Routes with separate
Harness, Provider Account and Model fields. The installed Route menu presents
those combinations under Loom Native, OpenCode and Codex headings rather than
as duplicate Providers. Build 124's real OpenCode plus
`deepseek.primary` reply/restart evidence remains valid dispatch evidence;
Build 127 did not make a new external Provider request.

## Implemented source boundaries

- The App owns the bundled daemon lifecycle. Socket probes are non-blocking,
  bounded and run outside the main actor; Swift never removes the daemon Socket
  or lock without lock authority.
- Setup isolates invalid import candidates, so one malformed CC Switch record
  cannot hide all Providers and Runtimes. Runtime values also pass a closed,
  bounded Swift projection contract before entering the UI.
- The Loom-owned Credential Vault is the normal credential path. Conversation
  and Agent dispatch acquire exact short-lived leases by Provider Account,
  credential reference and revision; Keychain is migration-only.
- Conversation execution bindings freeze Harness, Provider Account, credential
  revision and model. Account-scoped OpenCode profiles do not scan for a
  first-available credential at dispatch.
- Provider Directory is built from the Model Provider catalog only. Native
  Harnesses are discovered from Runtime inventory, and Conversation Profile
  display names are normalized from Provider identity at the daemon boundary;
  composite names such as `opencode-deepseek` cannot become Provider rows.
- OpenCode Conversation subprocesses deny tools and sharing. Agent subprocesses
  deny all tools except explicitly scoped Loom context MCP capabilities.
- Private App registries use `0700` directories and `0600` files. A narrowly
  validated owner-owned, single-link legacy `0644` registry is migrated in
  place; unsafe modes, links, owners and object types fail closed.
- Mission is a titled, navigable workflow related to a Conversation, with its
  own Agent Team orchestration and execution detail. RoundTable supports
  drag-and-drop seating. Team rows expose independent Harness, Provider Account,
  model and Agent-local status.
- Provider failures use the closed Phase 2D stage vocabulary and Incident IDs.
  Failure Lab is the accepted Phase 2D controlled isolation proof and preserves
  the path for broader real-account acceptance in a later Phase.

## Accepted WorkItems

- `P2D-HG1`: versioned Configured Harness, Backend Registry, Segment Session,
  unified events, reliability lifecycle, cross-Conversation concurrency and
  Response-scoped cancellation. All five Harness identities enter production
  through registered Gateway paths; the persistent Codex Segment Session,
  exact native interrupt, App Stop-response route, governed non-Codex Segment
  Backends, versioned event stream and unhealthy cleanup/reopen ordering are
  source verified. All five registered Harness identities have source-reachable
  Conversation paths with exact frozen binding/event acceptance. Claude Code
  native session resume is source verified. Build 136 closes installed G7 with
  Codex reuse, reviewed Codex-to-DeepSeek switching, cross-Conversation overlap,
  exact cancellation and post-cancel Session reuse. The G7 verifier consumes the
  user-exported privacy-safe diagnostic bundle without terminal access to the
  daemon state directory and requires its App/daemon hashes to match the exact
  canonical installed bundle. Raw operational JSONL is source-verification
  input only.

- `P2D-W2A`: Conversation Profiles, immutable Route Segments, Context Capsules,
  account/model switching and installed real-conversation matrix. Trust review,
  full capacity handling and the private-UDS Route Transition matrix are source
  and installed verified through v3 target-Profile authority, independent
  per-Attempt Capsules and target-Session reuse.
- `P2D-W2B`: per-Agent RouteSet and exact Execution Profile authoring.
- `P2D-W2C`: per-Attempt frozen dispatch, Role Capsules and mixed-Team runtime.
- `P2D-W2D`: Credential Vault, diagnostics, failure isolation, explicit
  fallback, account-level accounting, governance UI and encrypted storage.
- `P2D-BLOCKER-1`: DeepSeek Vault import and verification are live; the Build
  124 OpenCode/DeepSeek restart and reply path is resolved. The broader custom
  endpoint import loop is deferred and no longer blocks Phase 2D.

Composition contracts `P2D-COMP1` and `P2D-COMP2` remain part of Phase 2D, not
separate Goals. Protected core authority, CapabilityContext boundaries and the
Strangler migration rules remain in force.

## Completion boundary

All pre-HG1 Phase 2D gates are accepted. Build 127 closes trust-domain
confirmation, full Context Capsule capacity handling and the approved Route
Transition matrix for that installed boundary. Historical installed
real-conversation, mixed-Team,
controlled failure-isolation, fallback, accounting and governance evidence
remains part of the accepted composite Phase boundary.

ADR-0022 is now part of the accepted active boundary. Build 136 passes HG1
source, production-composition and installed acceptance. The three explicitly
deferred external scenarios below remain preserved for a later Phase and do not
reopen Phase 2D.

## Deferred to a later Phase

Status: `DEFERRED / NO FUTURE PHASE ACTIVATED`

- Run one installed Team with Codex/OpenAI, Claude/Anthropic, Loom/Kimi and
  Loom/MiniMax as four independently bound real Agents.
- Revoke or rate-limit one real Provider Account and prove only its Agent is
  blocked while peers continue.
- Import and verify a user-approved custom endpoint, freeze its exact binding
  and complete a real Conversation through it.

Existing source, controlled Failure Lab and narrower installed evidence remain
valid. Deferral changes the completion gate, not the product architecture or
the fail-closed execution contract.

## Authoritative references

- Phase amendment:
  [2026-08-09-phase2d-provider-compatibility.md](../.loom-evidence/plan-amendments/2026-08-09-phase2d-provider-compatibility.md)
- Conversation contract:
  [P2D-W2A-conversation-route-segments.md](../.loom-evidence/phase2d/contracts/P2D-W2A-conversation-route-segments.md)
- Governance contract:
  [P2D-W2D-observability-governance.md](../.loom-evidence/phase2d/contracts/P2D-W2D-observability-governance.md)
- Installed-live matrix:
  [PHASE-2D-LIVE-ACCEPTANCE.md](../.loom-evidence/phase2d/acceptance/PHASE-2D-LIVE-ACCEPTANCE.md)
- Historical volatile status:
  [CURRENT-history-through-build123.md](archive/CURRENT-history-through-build123.md)
- Build 124 evidence:
  [P2D-W2A-W2D-full-review-remediation-build124.md](../.loom-evidence/phase2d/P2D-W2A-W2D-full-review-remediation-build124.md)
- Build 125 Provider/Runtime/Route separation evidence:
  [P2D-W2A-provider-runtime-route-separation-build125.md](../.loom-evidence/phase2d/P2D-W2A-provider-runtime-route-separation-build125.md)
- Build 126 Route-menu information architecture evidence:
  [P2D-W2A-route-menu-information-architecture-build126.md](../.loom-evidence/phase2d/P2D-W2A-route-menu-information-architecture-build126.md)
- Build 127 trust/capacity/Route Transition evidence:
  [P2D-W2A-W2D-trust-capacity-route-build127.md](../.loom-evidence/phase2d/P2D-W2A-W2D-trust-capacity-route-build127.md)
- Post-Build-127 governance hardening evidence:
  [P2D-W2A-W2D-trust-capacity-route-hardening-post-build127.md](../.loom-evidence/phase2d/P2D-W2A-W2D-trust-capacity-route-hardening-post-build127.md)
- Harness Gateway and Segment Session contract:
  [P2D-HG1-harness-gateway-segment-session.md](../.loom-evidence/phase2d/contracts/P2D-HG1-harness-gateway-segment-session.md)
- Harness Gateway ADR:
  [0022-harness-gateway-and-segment-session.md](adr/0022-harness-gateway-and-segment-session.md)
- HG1 persistent Codex and cancellation source evidence:
  [P2D-HG1-persistent-codex-cancel-source.md](../.loom-evidence/phase2d/P2D-HG1-persistent-codex-cancel-source.md)
- Build 136 installed HG1 G7 evidence:
  [P2D-HG1-build136-installed-g7.md](../.loom-evidence/phase2d/P2D-HG1-build136-installed-g7.md)
