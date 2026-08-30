# Current State

## Phase 7

Status: `ACCEPTED / COMPLETE / INSTALLED BUILD 324 / CLAUDE LIVE N/A BY USER`

The unified full model-driven Loom Harness Goal is accepted. P7-HT2 covers
Conversation and Session, Mission, Agent Team, RoundTable, governance, setup
diagnostics and every built-in Conversation Runtime without creating a
separate product Goal. Loom `0.5.6` Build 324 is the current installed
acceptance. Its exact installed matrix passes for Codex, OpenCode/DeepSeek, Pi
and Loom Native/MiniMax; Claude Code remains `N/A` under the user's explicit
installed-login and paid-live waiver. The waiver does not remove Claude source
parity, Profile admission, cancellation, privacy or failure-isolation gates,
all of which remain green and fail closed while no executable Claude Profile
exists.

Build 324 closes the two final installed blockers. Codex native authentication
now interprets the official App Server `account/read` result according to its
real protocol: an authenticated account whose active provider requires OpenAI
authentication is executable, while an unauthenticated account remains
actionably unavailable. Codex and OpenCode subprocesses now use a canonical,
owner-only per-call temporary directory, remove it on success and failure, and
repair legacy private scratch trees to directory mode `0700` and file mode
`0600`. The previous Build 323 matrix reached its final privacy gate but failed
there because old CLI-created directories were `0755`; that failed result is
preserved in
`.loom-evidence/phase7/verification/P7-HT2-installed-build323-failed-privacy.md`
rather than rewritten as a pass.

The Build 324 source gate passes the complete Go repository, `go vet`, the
affected Provider race gate, 496 XCTest cases with two conditional skips and
21 strict Swift contract cases. Deterministic double-build and transactional
installer fixtures pass. Candidate and installed bytes match, strict signing
passes, Build 323 is retained as `.previous`, and cold startup uses the bundled
managed daemon. The post-restart preflight reports 24 Providers, seven
Runtimes, six executable Conversation Profiles and the exact Phase 7
Mission/Team governance anchor.

The explicitly authorized installed matrix passed in 407.66 seconds. Across
Codex, OpenCode/DeepSeek, Pi and Loom Native/MiniMax it exercised all 28
model-facing tools through ordinary-language turns, exact read/Proposal tool
selection, confirmation, cancellation, digest-bound expiry, replay rejection,
RoundTable action targeting, managed App restart, restored decision authority
and a bounded plaintext/privacy scan. A separate Build 324 Codex ordinary reply
and Mission Proposal probe also passed. The acceptance harness did not inspect
or print credentials and did not change or migrate them; normal Provider calls
used the daemon's existing short-lived credential leases.

Build 261 introduced the Pi executable-Route gate without publishing a
false-positive entry: Setup requires both an
online Pi Runtime and an integrity-checked local model backend, the managed App
automatically discovers the established locked local-model location, and the
macOS model catalog preserves the exact executable Qwen model. At the Build 264
checkpoint the machine did not contain those assets, so that historical install
correctly kept Pi out of the executable Route menu and did not count it as live
acceptance.

Build 262 also closes a final integrity gap found after Build 261 installation.
Build 261 fixed the Qwen model digest but only observed the llama.cpp executable
digest.
Build 262 pins both the official `llama.cpp b10107` executable SHA-256 and the
Qwen GGUF SHA-256 before discovery or process start, while retaining exact file
identity revalidation immediately before execution. Its affected Pi/daemon
packages, focused race gate, complete repository suite and `go vet` pass. Its
deterministic package, transactional installation and two cold starts also pass.

Build 263 closes the installed UX ambiguity exposed by Build 262. Runtime
discovery can truthfully show Pi online even while its locked model backend is
absent, but the macOS setup and quick inspector now render `Local model required
for Conversation` instead of generic capacity. The Route menu remains unchanged
and fail closed. Focused rendering tests and the complete macOS suite pass, and
installed visual inspection confirms the actionable status without removing or
duplicating any Provider or Runtime.

Build 264 closes the final source authority gaps found during the full review.
Every terminal schema-v2 Proposal decision now has an immutable receipt bound
to the exact Proposal digest, decision time and Incident ID. The App requires
that receipt before executing a restored confirmation, shows the Incident ID
and keeps historical confirmed Proposals without a receipt read-only. A
content-free daemon diagnostic records the decision stage. Build 264 also fixes
later Turns in an immutable Segment: the opening Execution Binding remains
frozen, while Route validation correctly uses the current Attempt's Context
Capsule digest.

Build 265 closes a Pi local-Runtime supply-chain gap found while preparing the
five-Runtime live gate. The official llama.cpp release archive SHA-256 is now
the Runtime trust root. Loom derives a canonical complete tree from that exact
archive and binds every executable, sibling dylib and metadata file, in
addition to the server and GGUF. Extra files, changed dependencies, unsafe
archive paths or links, symlinks, hard links, ownership/mode drift and archive
substitution fail closed. App bootstrap and daemon argv now carry one complete
four-part local-model binding: private root, official archive, server and model.
The full Go repository, `go vet`, affected race gates and all 461 XCTest plus
20 strict Swift tests pass. Deterministic double-build, signing, transactional
installer fixture and a dry-run against the then-current install passed; Build
265 remained a candidate and did not replace Build 264 at that checkpoint.

Build 315 closes the installed Claude native-auth recovery gap. Runtime &
Providers now offers an exact `Sign In` action that launches the official
bounded Claude login path through the daemon. Setup re-observes auth on every
bounded Snapshot and can publish the native Claude Conversation Profile without
an App or daemon restart. UDS admission, product composition, executable
identity, minimal environment, process-group cleanup, staged errors and
Claude-local failure isolation are covered by source contracts. The installed
UI distinguishes a detected CLI from an executable Conversation Route with an
orange `Sign In Required` state instead of contradictory `Online` copy. The
user did not start login during this gate, so no credential changed and no
Claude Profile was invented.

Build 316 closes the post-install review gaps without changing that user-owned
authentication boundary. Every production Setup refresh now passes through one
admission gate: a structurally valid but empty projection preserves the last
known-good Provider and Runtime inventory, publishes an `empty_setup` recovery
state and never becomes ready. Claude sign-in now carries one exact Incident ID
from the App through UDS and daemon diagnostics, retains staged retryability,
supports explicit cancellation and records only bounded non-secret metadata.
Home and temporary-directory identity replacement plus descendant process-group
cleanup are fail-closed test contracts. The current user still has not started
Claude browser login, so Build 316 does not invent a Profile or rewrite the
Build 313 paid result.

Build 317 closes the remaining cancellation-ordering gap in that recovery
path. Swift UDS cancellation now shuts down the exact live descriptor so a
blocked Setup poll cannot delay the daemon cancel request. Claude sign-in owns
an explicit generation and cancellation state, sends `claude_code_cancel`
before awaiting the old task and prevents stale work from clearing or
overwriting the result. Runtime & Providers renders separate waiting and
cancelling states only on the Claude row. The real Go-to-Swift protocol probes
retain strict decoding but use debug compilation for source-test latency; the
release App still passes deterministic double-build packaging. This build did
not start Claude login, touch credentials or make paid Provider calls.

Build 313 was the first complete paid four-Runtime matrix and closed the
tool-selection
gaps found after Build 265. OpenAI-compatible and Anthropic-style loops now
repair malformed arguments within a strict bound, force fresh terminal
reselection after repair exhaustion and reject tools omitted from the current
least-privilege round. Read-only metadata receives one bounded retry only for a
transient Gateway outage; Proposal calls never auto-retry. The installed runner
uses an explicit governance anchor and synthetic `p7-` RoundTables whose Pause,
Steer, Retry, Skip and Replace targets satisfy current action preconditions.
Historical or concluded RoundTables are never reused as writable targets.

The versioned Registry admits exactly 28 tools. Thirteen are bounded metadata
reads for Session, Mission, Team, RoundTable, Needs You, Runtime, Provider,
Incident, Workspace, Route and Library state. Fifteen can only prepare typed
Proposals for Session alignment; Mission create/continue; Team create/edit;
RoundTable open/pause/steer/retry/skip/replace; Route, model and reasoning
changes; and workspace selection. There is no model-facing confirm, cancel,
execute or generic mutation tool. Stop remains an immediate local operator
control and is intentionally not model-invocable.

Codex, OpenCode, Claude Code, Pi and Loom Native expose the same frozen Registry
and pass cross-Runtime read/Proposal parity in source. Transport differs by
adapter, but each Turn freezes the exact Route, Workspace, Registry digest,
Segment, Attempt and Incident identity before dispatch. The product metadata
Gateway returns only bounded non-secret projections and requires the current
frozen turn.

All mutation Proposals now use schema v2. The digest binds Route, Workspace,
Registry, Incident, target content, Segment, Attempt and expiry. Confirmation
rechecks the current daemon and encrypted thread projections before issuing a
one-time immutable receipt; expiry, replay, workspace/route drift, Registry
substitution, credential-shaped input and tool/action mismatch fail closed.
Schema v1 remains readable and cancellable for migration but cannot be
confirmed or executed.

The macOS source presents one compact review card for both alignment and the 14
generic action types, preserves exact Mission/Team/RoundTable targets and opens
the existing governed review only after daemon confirmation. Confirmation alone
does not create a Mission, Team, RoundTable Session, Run or Attempt. Confirmed
Proposals restore across restart only with an exact matching decision receipt;
stale and historical no-receipt Proposals remain non-executable.

The complete Go repository suite passes under default package concurrency;
`go vet ./...` and the affected control, API, RoundTable, Gateway, Provider and
Harness adapter race gates also pass. Real subprocess
contract tests use bounded fixture deadlines that tolerate repository load
while retaining their short cancellation and process-group cleanup deadlines.
The latest completed macOS source gate passes 496 XCTest cases with two
conditional skips and 21 strict Swift contract cases. The v2 exact-Mission card
and confirmed decision-receipt card are visually checked at 560px and 360px
accessibility size.

Build 318 installs the post-Build 317 hardening that closes three final-gate
overclaim risks without changing credentials. Setup now publishes the native
Claude Conversation Profile only when one current Runtime projection matches the
canonical instance ID, adapter, positive capacity, exact model and
`workspace_edit` capability. The installed live selector independently requires
the exact Profile ID, protocol, auth mode, Provider Account and credential
revision contract for Codex, OpenCode, Claude Code, Pi and Loom Native. Its
privacy gate injects a content-only marker and scans the private state and
diagnostic roots for plaintext after restart. A real Swift client now proves
that cancelling a blocked Setup exchange closes the live descriptor and reaches
the product daemon's `claude_code_cancel` route with the same Incident ID. The
default-concurrency Go repository suite, `go vet`, affected race gates, 496
XCTest cases and 21 strict Swift contract cases pass. Deterministic double-build,
strict signing, installer fixtures, transactional replacement, cold startup and
exact installed identity all pass; Build 317 remains the rollback bundle.

Historical Loom `0.5.6` Build 318 passed strict signing, exact App/daemon digest
identity, managed startup and private UDS identity gates. Its current Snapshot
retains 24 Providers, seven online Runtimes and six executable Conversation
Profiles. The locked Pi Runtime and Qwen model are executable, Provider and
Runtime rows remain distinct, and the App plus managed daemon remain running.
Deterministic double-build, transactional installer and full macOS regression
pass. Build 317's three installed 720px visual inspections remain representative
because Build 318 changes no UI production source. The installed preflight also fails
closed on an ambiguous governance anchor and an incorrect daemon digest, while
an explicit exact Mission and Team anchor passes. Build 264, Build 265, Build
313, Build 315, Build 316 and Build 317 remain historical milestones rather
than the current install.

The explicitly authorized historical Build 313 installed available-Runtime matrix made real paid
model calls without reading or changing credentials. Codex, OpenCode, Pi and
Loom Native completed their decision lifecycle and collectively selected all 28 tools
through ordinary-language turns. Confirmation, cancellation, five-minute
expiry, replay rejection, exact Tool audit and App restart restoration passed.
Three synthetic RoundTable streams ended in `RoundtableConcluded`. Build 324
subsequently reran and completed the same matrix against the final installed
bytes.

The explicitly authorized Build 318 rerun started against the exact installed
bundle and stopped safely on the first Codex confirmation turn. App and daemon
diagnostics carried Incident `loom-client-46a24eb3e9a0e564-13`; a narrower
read-only Codex probe reproduced the same failure. A direct bounded native
Codex check then proved `invalid_refresh_token`: the CLI's legacy `login status`
still returned success even though a real account refresh required login. No
later Runtime or governance mutation ran. The subsequent source correction
replaced that false-positive observer with an attested, bounded App Server
`account/read` refresh probe that
returns only closed authentication booleans. It also classifies a mid-Session
`unauthorized` notification as actionable `provider_auth`, invalidates cached
state on reconnect and never projects account identity or Provider error text.

The installed gate defaults to skip and requires separate paid-request,
managed-restart and exact bundle identity flags. Its governance fixture now
requires one unambiguous P7 Mission or explicit exact Mission/Team IDs. It
never selects the first fuzzy historical match. Session creation
registers cleanup immediately, read-only turns require an exact Tool set and
RoundTable action targets are revalidated from current state. The four-Runtime
Build 313 gate passed in 431.45 seconds. The historical five-Runtime gate stopped at profile
admission with `installed Phase 7 matrix missing executable claude-code
profile`, before any additional model request or state mutation. The discovered
Claude Code `2.1.196` Runtime is online with capacity three, but its native auth
status is `loggedIn=false`, `authMethod=none`; Setup contains no Anthropic
Provider Account or Anthropic import candidate. Build 318 retains the optional
recovery action in Runtime & Providers. The formal available-Runtime gate now
requires `LOOM_PHASE7_CLAUDE_LIVE_WAIVER=1`, rejects the waiver if an executable
Claude Profile exists, and otherwise verifies Codex, OpenCode, Pi and Loom
Native without inventing Claude availability.

The accepted architecture is
`docs/adr/0023-model-driven-loom-control-tools.md`; the first installed slice is
`.loom-evidence/phase7/contracts/P7-HT1-model-driven-harness-tools.md` with
`.loom-evidence/phase7/verification/P7-HT1-installed-build259.md`. The active
full-surface contract is
`.loom-evidence/phase7/contracts/P7-HT2-full-model-driven-capability-surface.md`;
its source verification is
`.loom-evidence/phase7/verification/P7-HT2-source.md`; current installed
evidence is `.loom-evidence/phase7/verification/P7-HT2-installed-build324.md`.
Build 317 cancellation evidence remains preserved at
`.loom-evidence/phase7/verification/P7-HT2-installed-build317.md`.
Build 316 recovery evidence remains preserved at
`.loom-evidence/phase7/verification/P7-HT2-installed-build316.md`.
Build 313 real-model evidence remains preserved at
`.loom-evidence/phase7/verification/P7-HT2-installed-build313.md`.
Build 264 remains historical local evidence in its original record.
Phase 2D, Phase 4, Phase 5 and Phase 6 remain closed.

## Phase 6

Status: `ACCEPTED / COMPLETE / HISTORICAL INSTALLED BUILD 257`

Phase 6 is an accepted historical product Goal. Loom now has a
Codex-inspired unified action entry in the Conversation composer while
retaining Loom's stronger governance boundary. Typing `/` or choosing Commands
opens a searchable, keyboard-operable catalog of 18 actions for Conversation,
Mission, Agent Team, RoundTable, Route/Model/Reasoning, status, diagnostics,
stop, recovery and setup. Explicit Chinese and English requests resolve through
the same typed catalog. Questions and incidental nouns remain ordinary chat.

Unknown Slash commands remain in the composer and never reach a Provider.
Credential-shaped command arguments are rejected locally. Mission and Team
commands create prefilled review drafts, RoundTable opens Mission-linked
governance, and Route trust transitions plus Stop retain their existing review
or confirmation. Model text still has no execution authority.

The source gate passes 442 XCTest cases with two conditional skips and 20
strict Swift contract cases. Installed Loom `0.5.4` Build 257 passes strict
bundle verification and candidate/install digest equality. Real desktop and
compact interaction covered search, keyboard selection, contextual disabled
reasons, unknown commands, credential rejection, Mission/Team drafts,
RoundTable, Model, Status, diagnostics, restart and managed-daemon recovery.
Live testing found and fixed an asynchronous diagnostics presentation race that
could otherwise open an empty sheet.

The accepted contract is
`.loom-evidence/phase6/contracts/P6-UA1-conversation-unified-action-entry.md`;
installed evidence is
`.loom-evidence/phase6/verification/P6-UA1-installed-build257.md`. Phase 6
remains closed.

## Phase 5

Status: `ACCEPTED / COMPLETE / INSTALLED BUILD 253`

Phase 5 is an accepted historical product Goal. It evolves Loom into a
conversation-first desktop workspace inspired by Codex without copying Codex's
product boundary or weakening Loom's governance authority. Opening Loom must
land in a usable conversation. Mission, Agent Team and RoundTable capabilities
must appear progressively around the current task instead of requiring the user
to understand Loom's internal object model before sending a message.

The first vertical slice is P5-UX1/P5-UX2, followed by the first P5-UX3
in-context Mission slice. Local inspection of Codex
`26.818.31338` and the installed Loom Build 211 found the decisive difference:
Codex keeps the thread as the stable center and places execution location,
workspace state and tools around the composer or current thread, while Loom
exposed seven peer destinations, more than ninety historical conversations and
three governance actions before the first message. The source candidate now
uses a single `New task` entry, separates primary work from governance and
tools, bounds the visible recent-conversation list, and groups folder, Agent
Team and Mission actions behind the composer's `+` menu.

P5-UX1 and P5-UX2 are source accepted. The complete macOS suite passes 432
XCTest cases with two conditional skips and 20 Swift Testing contract cases.
Desktop and compact SwiftUI rendering pass; visual inspection also closed the
compact rail's wrapped section-label defect.

P5-UX3 is installed accepted. Clicking a linked Mission keeps the
conversation mounted and opens an in-context Mission panel instead of the
full-sheet workbench. The panel follows the authoritative visible Mission,
shows progress and bounded per-Agent output with Harness, Provider Account and
Model, exposes the concrete failure reason, and lets a blocked, failed or
cancelled Mission collect guidance for a new audited Attempt. Team, Decision
and Needs You paths resolve to the same context. A Mission-linked RoundTable
receives its exact Conversation, Mission and Team relationship. RoundTable now
also restores as a conversation-side governance context: it shows every
Agent's frozen route, visible output and Attempt status, supports
capability-gated Steer, explicit Retry guidance, Pause and Moderator conclusion
acceptance, and retains the full workbench only as a secondary setup/details
path. Full Mission details remain a secondary action.

P5-UX4 is installed accepted. Mission node failures expose their safe
stage, code, terminal reason, retryability, recovery action and copyable
Incident ID next to Agent output. RoundTable exposes the same failure language
per seat and fails closed when a restored Session does not match the selected
Mission. Source rendering covers Mission and RoundTable contexts at 1,280 x
760 and overlay-width 900 x 760 while preserving the conversation beneath.

Loom `0.5.3` Build 253 is the accepted installed Phase 5 build. Strict bundle
verification passes, packaged and installed executable digests match, and the
bundled daemon runs as the App's managed child. Long RoundTable results now use
a 420-rendered-character preview that keeps the conclusion beginning and latest
action, parses Markdown before slicing, and cuts only at complete word
boundaries. Both Agent identities, frozen routes and statuses remain visible
before detail, with explicit accessible `Show full result` and `Show less`
actions. Installed visual and accessibility inspection exercised both actions,
then a full App/daemon restart restored the same Mission-linked RoundTable,
both collapsed summaries and composer focus without raw Markdown or partial
words. Build 253 also gives every RoundTable and Mission operation a structured,
correlation-aware recovery state in both surfaces; diagnostic-preview and
folder-selection failures no longer disappear silently.

Build 242 moved external Codex, Claude Code and OpenCode discovery off the
Conversation startup path, filters terminal Attempt history before exact Run
replay and reuses one exact frozen-authority validation for sibling delivered
payload facts. Two
installed cold runs measured Agent Runtime aggregates of 18,128 ms and 12,732
ms. Their external Runtime catalog work took 6,199 ms and 3,868 ms, leaving
about 11.9 and 8.8 seconds of authority/history work. The composer remained
focused and Route/Model controls became available before governed Runtime
restoration completed.

Build 240 was rejected by the installed UX loop: opening the persisted
RoundTable during startup exposed a raw Swift `LocalProductClientError error 4`
and required a manual Retry even though authoritative state was healthy. Build
241 treats local unavailability and timeout during this bounded restore as
transient preparation, keeps the Mission-linked inspector mounted and retries
automatically. Installed visual and accessibility acceptance observed
`Preparing the Agent Team` transition to the concluded two-Agent RoundTable
without Retry, navigation or restart. Build 242 preserves that behavior across
two complete App/daemon cold restarts: Conversation remained mounted and the
same concluded two-Agent RoundTable, both Agent results and both frozen routes
restored without Retry. Build 241 remains the accepted predecessor and its
39,931 ms and 56,274 ms Agent Runtime samples remain preserved as baseline
evidence rather than being rewritten.

Build 227 closes the installed new-Mission identity and result-continuity gap.
Starting another bounded Mission from an already-used saved Team now
materializes a fresh TeamInstance through the private product service, opens a
distinct Mission and keeps the user-entered Mission title. The real two-Agent
MiniMax Mission `mission/team-instance-c11a01b82dab3919c50861582fa0a1cc`
completed successfully. After a full App/daemon cold restart, the same Mission
restored both accepted Agent outputs from digest-checked terminal Evidence,
with no Provider response copied into Journal facts. The right inspector now
uses the saved Mission presentation title consistently and distinguishes live
tentative output from accepted restart-safe output.

Build 229 closes two RoundTable continuity defects found by continuing the
installed workflow. Retry no longer replaces the original discussion prompt
with the user's retry guidance. The daemon restores the exact authoritative
prompt from the prior Attempt's encrypted Context Capsule, creates a fresh
Attempt with the same frozen execution binding, and adds the guidance as a
separate role-scoped confirmed constraint. Prompt and guidance remain absent
from Journal facts. The full workbench now shows the same safe failure stage,
recovery action and copyable Incident ID as the conversation inspector.
Concluding a Session also returns the projected Agent outputs immediately
instead of briefly clearing them until refresh. The real Build 228 retry and
Build 229 cold restart restored the concluded two-Agent Session and both
outputs. The run also exposed the next honest gap: later synthesis receives the
original question but not the relevant prior Agent result artifacts. That
untrusted, provenance-bound context handoff and a substantive fresh synthesis
remain open rather than being represented as completed deliberation.

Build 235 closes that open RoundTable synthesis loop. Every follow-up Agent
Attempt receives prior successful Agent outputs as bounded, provenance-bound
`untrusted_model_output` Context Capsule items while retaining the exact
original discussion prompt and frozen execution binding. The production daemon
also injects a separately classified authoritative Mission context item resolved
from the current Mission projection, including the exact Mission, Team Instance,
objective and plan digest. Agent Provider failures preserve their safe HTTP and
Provider classification, and the MiniMax Agent response boundary now matches
the 30 KiB RoundTable payload limit.

Installed live acceptance used two independently frozen Loom Native/MiniMax
Agents across an initial round, two synthesis rounds and one guided retry. One
Agent timed out while its peer result remained visible; the UI exposed the exact
failure stage and Incident ID, accepted bounded retry guidance, and then
completed both seats. The Lead produced a substantive synthesis using the
authoritative Mission item and untrusted peer contributions. The user explicitly
accepted that Lead synthesis. A full App and daemon cold restart restored the
concluded Session and both latest Agent results. Build 235 also distinguishes a
first synthesis from a follow-up refinement and names the exact Lead result or
Lead synthesis being accepted.

Build 236 closes the first P5-UX5 keyboard consistency slice. The macOS New
Item command is now Loom's `New Task` command instead of a second-window action.
`Command-N`, the navigation rail and both Conversation header entries share one
path that creates an isolated Conversation task, returns primary navigation to
Conversation, closes an unpinned governance inspector and restores composer
focus. Installed acceptance started from a restored Mission-linked RoundTable
and verified the exact transition into a fresh focused Conversation.

Build 220 accepted the installed 900 x 760 compact layout. The compact rail no
longer repeats twelve indistinguishable historical conversation buttons; full
history remains in the header conversation switcher. Mission and RoundTable
open beside the same conversation without clipping, Escape closes the overlay,
and the blocked Mission exposes a labeled guidance input and recovery action.
The composer and all in-context Mission/RoundTable guidance inputs have stable
accessibility labels and identifiers. A largest-accessibility-text Mission
render also passes without horizontal overflow. Build 253 temporarily enabled
macOS Full Keyboard Access, completed the full installed traversal without a
focus trap and restored the original system setting afterward.

Build 221 closes two RoundTable correctness defects found during the next
workflow review. A seat's latest Attempt is now selected within the latest
round, so an older retry with a higher attempt number cannot incorrectly block
a newer successful round. After governed Replace, the seat row resolves the
current Agent name, role and route from the authoritative frozen binding while
retaining the stable seat lineage. Both daemon and Swift regressions failed
before the fixes and pass afterward. The installed App restored the exact
concluded two-Agent RoundTable after a full App/daemon restart at 900 x 760.

P5-UX3 now has installed acceptance for a new real Mission, restart-safe Agent
results and a substantive multi-round RoundTable synthesis with prior Agent
output carried as untrusted provenance. P5-UX4 has installed Conversation,
Mission and RoundTable fault-injection acceptance with safe stage,
retryability, recovery action, diagnostics and copyable Incident IDs. P5-UX5
closes desktop/compact visual, accessibility, keyboard, restart and real
Mission/RoundTable continuity gates. Build 253 also closes the rapid-restart
service-adoption race. Repeated startup distribution and further Agent Runtime
authority/history optimization may continue as later performance work; they do
not block this interaction contract. Phase 5, Phase 4 and Phase 2D are closed.

## Phase 4

Status: `ACCEPTED / COMPLETE / INSTALLED BUILD 211`

Phase 4 is the most recent accepted product Goal. It completes RoundTable as a governed
multi-Agent deliberation node inside a Mission. The completion boundary is a
Mission-linked, restart-safe RoundTable with two to six independently bound
Agent seats, real Agent Attempts and visible discussion progress, seat-local
failure isolation, user intervention, a Moderator candidate conclusion and an
explicit user decision before an AlignmentSummary can continue the Mission.

RT2-1 is accepted. It adds authoritative Conversation, Mission and Team
identity to a RoundTable Session, freezes the exact non-secret Execution
Binding for every Agent seat, enforces two to six active Agent seats and locks
membership when the first round opens. The production daemon resolves only a
minimal role selection from IPC against the projected saved Team; clients
cannot author Provider, account, credential or model binding fields. Mission
Room now opens a linked RoundTable and renders the frozen route after restart.
Existing schema-v1 streams remain readable as legacy ledger-only sessions and
are not silently granted execution authority. Installed Build 208 cloned a
Mission-linked Team through the real UDS using only role selections and restored
the exact server-resolved bindings after restart. The contract is
`.loom-evidence/phase4/contracts/RT2-1-mission-linked-frozen-seats.md`.

RT2-2 replaces the manually clicked message ledger with real per-seat Agent
Attempts, role-scoped Context Capsules and a visible, incremental discussion
stream while preserving one immutable Execution Binding per Attempt. Opening a Mission-linked
round compiles an isolated RoundTable execution through the existing governed
Team Coordinator, Supervisor, Attempt Loop and Runtime Adapter path. Every seat
freezes its own execution Team, WorkItem, Run, runtime, Agent, seat binding,
execution binding and Role Context Capsule identity. Authorized text deltas are
projected to the macOS workbench while running; successful terminal text is
stored in the encrypted Attempt Payload store and restored by exact binding.
The RoundTable Journal contains only lifecycle metadata and digests.

The installed mixed-seat gate is green: one seat succeeds and retains its
visible encrypted output while a peer records a seat-local
`agent_attempt_dispatch` Incident. The prompt does not enter Journal facts.
The Mission-linked UI uses one `Start discussion` action and live Agent
conversation cards instead of the canned propose/relay/ack journey. Dedicated
RoundTable error codes are part of the Swift closed wire set.

RT2-3 is accepted. RoundTable now exposes governed Pause, Steer, Retry,
Skip and Replace controls. Each intervention is an idempotent content-negative
Journal fact; steering guidance enters the active Agent inbox but only its
digest and input identity enter the RoundTable stream. Retry creates a fresh
Attempt, Run, Segment and Context Capsule while preserving the exact frozen
seat route. Pause and daemon shutdown cancel only running seat Attempts. On
restart, stale running Attempts become cancelled and retryable through explicit
user action; Provider work is never resumed silently. Dynamic Round IDs avoid
the former `round-1` collision. Build 208 fixed Runtime-wrapper capability loss:
Loom Native admits and consumes Steer into a second governed model Step while
unsupported Runtimes return an immediate `capability_gap`. Build 211 projects
the exact active Attempt capability to the App, so Codex, Claude, OpenCode and
future Runtimes are not inferred from their Harness names.

RT2-4 is accepted. The latest successful main-seat Attempt is the
Moderator candidate conclusion. A Mission-linked session cannot conclude while
another active seat is failed, cancelled or unresolved; the user must retry,
replace or explicitly skip it. `Accept conclusion` is the sole publish action.
The resulting AlignmentSummary binds the Conversation, Mission, Team, accepted
Attempt payload digest, frozen execution binding and Context Capsule digest,
without copying Provider output into Journal metadata.

RT2-5 is accepted. The macOS workbench exposes local Import and Export
actions. A concluded RoundTable exports a one-day, one-megabyte maximum,
digest-bound document; import validates schema, expiry, top-level/embedded
identity consistency and provenance before opening a read-only view. The App
stops reading at the one-megabyte boundary. Mission context survives the
portable record. Invalid or changed files produce the actionable
`export_invalid` error.

Verification passes the complete Go repository, `go vet ./...`, the focused
RoundTable race matrix and the complete macOS suite (396 XCTest cases, two
conditional skips, plus 20 Swift Testing cases). Installed acceptance covers
Mission/Team binding, live output, seat-local failure, Pause/Steer/Retry,
candidate acceptance, AlignmentSummary, restart and local Export/Import.
Contracts and Build 208 live plus Build 211 hardening evidence are recorded in
`.loom-evidence/phase4/`.

Remote A2A seats and collaboration/Web surfaces remain later extensions. They
are not prerequisites for the local installed Phase 4 completion matrix.

## Phase 2D

Status: `ACCEPTED / COMPLETE`

Phase 2D remains accepted and complete. Phase 2C remains closed. ADR-0022
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

Status: `CURRENT / PHASE 5 COMPLETE / BUILD 253 INSTALLED / PHASE 4 COMPLETE / PHASE 2D COMPLETE`

Loom `0.5.3` Build 253 is the accepted installed Phase 5 build on branch
`codex/phase4-roundtable-governed-deliberation`. The App launches its bundled
daemon as one managed product. Phase 5 hashes, setup projection, installed UX,
keyboard, recoverable-failure and rapid-restart evidence are recorded in
`.loom-evidence/phase5/live/P5-BUILD253-CONVERSATION-FIRST-UX-ACCEPTANCE.md`.
Build 247 recoverable-error and result-summary evidence remains in
`.loom-evidence/phase5/live/P5-BUILD247-RECOVERABLE-ERRORS-AND-RESULT-SUMMARIES.md`.
Build 243 result-hierarchy evidence remains in
`.loom-evidence/phase5/live/P5-BUILD243-ROUNDTABLE-RESULT-HIERARCHY.md`. Build 242
Agent Runtime recovery evidence remains in
`.loom-evidence/phase5/live/P5-BUILD242-AGENT-RUNTIME-RECOVERY.md`. Build 241
Conversation-ready startup evidence remains in
`.loom-evidence/phase5/live/P5-BUILD241-CONVERSATION-READY-STARTUP.md`. Build 236
New Task keyboard evidence remains in
`.loom-evidence/phase5/live/P5-BUILD236-NEW-TASK-KEYBOARD.md`. Build 235
RoundTable synthesis evidence remains in
`.loom-evidence/phase5/live/P5-BUILD235-ROUNDTABLE-SYNTHESIS.md`. Build 229
RoundTable continuity evidence remains in
`.loom-evidence/phase5/live/P5-BUILD229-ROUNDTABLE-CONTINUITY.md`. Build 227
Mission continuity evidence remains in
`.loom-evidence/phase5/live/P5-BUILD227-MISSION-CONTINUITY.md`. Build 221
RoundTable correctness evidence remains in
`.loom-evidence/phase5/live/P5-BUILD221-ROUNDTABLE-CORRECTNESS.md`. Build 220
compact and accessibility evidence remains in
`.loom-evidence/phase5/live/P5-BUILD220-COMPACT-ACCESSIBILITY.md`. Phase 4 live
Session IDs and content-negative acceptance details remain recorded in
`.loom-evidence/phase4/live/P4-BUILD208-ROUNDTABLE-LIVE-ACCEPTANCE.md` and
`.loom-evidence/phase4/live/P4-BUILD211-ROUNDTABLE-REVIEW-HARDENING.md`.

RoundTable is now a Mission-linked execution surface rather than a manual
message ledger. The user drags two to six configured Team Agents into seats,
starts one discussion, watches each independently frozen Attempt and can Pause,
Steer, Retry, Skip or Replace at the seat boundary. A candidate conclusion only
continues the Mission after explicit acceptance. Concluded records can be
exported locally and reopened read-only.

Build 208 closes the last installed blocker. The product Runtime wrapper now
preserves its delegate's live Agent-input capability. A real Loom Native/MiniMax
Attempt froze an 8-turn/16-step budget, admitted a Steer through the UDS,
consumed it from the encrypted Agent Inbox and opened model Step 2. The peer
seat succeeded while the steered seat later recorded a Provider HTTP failure,
proving seat-local isolation. OpenCode, which does not yet support continuation,
returns `capability_gap` in seconds; the macOS workbench no longer presents live
Steer as an available OpenCode action. Build 211 completes the follow-up review:
the App now consumes the exact running Attempt capability instead of inferring
from the Harness name, rejects internally inconsistent portable provenance,
enforces one 4,096-byte Steer boundary and reads imports with a hard one-megabyte
cap. The installed UI also gives terminal state precedence and distinguishes
Agent seats from the system Moderator.

The complete macOS suite passes 396 XCTest cases with two conditional skips and
20 Swift Testing contract cases. `go vet ./...`, the complete Go repository,
focused race coverage, strict bundle verification and transactional installed
launch are the Phase 4 completion gates.

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
  own Agent Team orchestration and execution detail. RoundTable is a governed
  Mission node with drag-and-drop seating, real independently bound Attempts,
  visible delivery, seat-local intervention, explicit candidate acceptance,
  restart recovery and local portability. Team rows expose independent Harness,
  Provider Account, model and Agent-local status.
- Provider failures use the closed Phase 2D stage vocabulary and Incident IDs.
  Failure Lab is the accepted Phase 2D controlled isolation proof and preserves
  the path for broader real-account acceptance in a later Phase.

## Accepted WorkItems

- `RT2-1`: Mission/Conversation/Team-linked Sessions and two-to-six frozen Agent
  seats resolved by daemon authority.
- `RT2-2`: real per-seat Agent Attempts, Role Context Capsules, live authorized
  delivery and encrypted terminal payloads.
- `RT2-3`: governed Pause, Steer, Retry, Skip and Replace with restart-safe,
  content-negative intervention facts and capability-aware recovery.
- `RT2-4`: Moderator candidate conclusion and explicit user acceptance before a
  digest-bound AlignmentSummary can continue the Mission.
- `RT2-5`: bounded local Export and strict read-only Import.

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

- Phase 4 installed acceptance and final review hardening:
  [P4-BUILD208-ROUNDTABLE-LIVE-ACCEPTANCE.md](../.loom-evidence/phase4/live/P4-BUILD208-ROUNDTABLE-LIVE-ACCEPTANCE.md)
  and [P4-BUILD211-ROUNDTABLE-REVIEW-HARDENING.md](../.loom-evidence/phase4/live/P4-BUILD211-ROUNDTABLE-REVIEW-HARDENING.md)
- Phase 4 contracts:
  [RT2-1](../.loom-evidence/phase4/contracts/RT2-1-mission-linked-frozen-seats.md),
  [RT2-2](../.loom-evidence/phase4/contracts/RT2-2-real-seat-attempts-and-live-delivery.md),
  [RT2-3](../.loom-evidence/phase4/contracts/RT2-3-governed-intervention-and-restart.md),
  [RT2-4/RT2-5](../.loom-evidence/phase4/contracts/RT2-4-RT2-5-acceptance-and-portability.md)

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
