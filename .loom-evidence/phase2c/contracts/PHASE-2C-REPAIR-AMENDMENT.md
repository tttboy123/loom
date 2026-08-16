# Phase 2C Repair Amendment - Chat-First Product Integrity

**Status**: Repair 20 Source/Status Review 1 passed; replacement source lock generated and review pending  
**Date**: 2026-08-08  
**Baseline**: v0.4.1 whole-slice (`7e24ec29`) on `codex/loom-platform-slice2`  
**Parent**: `PHASE-2C-EXIT-CONTRACT.md`  
**Decision**: ADR-0015  

## 1. Why Phase Acceptance Is Reopened

The P2C-W1 and P2C-W2 commits remain part of the historical Candidate lineage,
but a whole-product review found that several accepted claims do not match the
shipped behavior. Phase 2C is therefore not accepted. Existing evidence is not
rewritten; the affected journeys must be rerun after the repairs below.

The product currently looks chat-first, but ordinary chat is an in-memory echo,
every workspace task can resolve to the same `start` thread, `Open Folder...`
does not open a picker, and the governance area embeds a wide Mission Board in
a narrow side column. Connection failures are also described in two competing
places with contradictory copy.

## 2. Repair Matrix

| Priority | Problem | Root cause | Required solution | Owner | Proof |
|---|---|---|---|---|---|
| P0 | Chat is not a pair-programming conversation | `LocalProductChatAPI` returns `Loom received:` from process memory and has no model/runtime port | Replace echo behavior with a bounded conversation service port. A configured conversational runtime may produce tentative assistant output; unavailable runtime returns an explicit recoverable state. Ordinary chat remains non-authoritative and never creates Team, Mission, or Run facts. | W2 | Service contract tests, no-Journal-write test, real configured-runtime journey |
| P0 | Tasks can share one conversation | `threadAnchor` defaults to `start`, so the UUID fallback is unreachable | Derive a stable, opaque thread ID from task identity; preserve an explicitly supplied anchor; prove two tasks never share a thread. | W2 | Swift unit tests and cross-task journey |
| P0 | Conversation disappears when the daemon restarts | Threads live only in a Go map | Add a daemon-owned bounded conversation projection/store with atomic persistence and strict decoding. It is replaceable conversation state, not Event Journal authority. Reload it after daemon restart. | W2 | Go restart test and J8 |
| P0 | `Open Folder...` is a false affordance | The action only calls refresh | Open the native directory picker, retain only a safe display handle in client state, and defer scoped read access to explicit preflight. Add the equivalent bounded TUI chooser. | W1 | Picker model test, native J2, PTY J2 |
| P0 | Service state is contradictory and exposes transport vocabulary | Shell and embedded workbench render separate states; the latter prints raw reasons such as `invalid_socket` | Make the shell the single connection-status presenter. Use `Starting local service`, `Local service unavailable`, `Showing last loaded state`, and `Local service rejected the connection`; keep sanitized diagnostics behind developer details. | W1 + W3 | Presentation tests, offline/reconnect screenshots, J3/J8 |
| P0 | Governance is a squeezed dashboard, not a contextual inspector | A full `MissionWorkbench` with a 560pt minimum is inserted into a 520pt side pane | Add independent `hidden`, `visible`, and `pinned` panel state; use a compact 320-420pt inspector for overview, team, decisions, evidence, runtime, and attention. Keep the full Mission Board as an explicit expanded workspace while conversation state is preserved. | W3 | Panel-state tests, compact screenshots, J5-J9 |
| P1 | Compact windows clip rail labels, composer actions, and board columns | Fixed rail/panel minimums exceed the tested window width | Introduce a pure layout policy: compact icon rail, protected composer minimum, inspector collapse/overlay fallback, and stable control dimensions. | W1 + W3 | Width-policy tests and J10 at 900/1080/1440pt |
| P1 | Providers opens placeholder UI | Shell owns a non-functional sheet instead of the existing management surface | Remove the placeholder. Route to the real provider/runtime surface when reachable; otherwise show a truthful unavailable state with no enabled setup action. | W3 | Click-path test and J7/J9 |
| P1 | Board tabs imply unavailable views | `Topology`, `Timeline`, and `Capacity` are static text | Replace them with the governance view switcher or remove them. Every visible tab must change content and expose a unique accessibility label. | W3 | View-switch tests and keyboard journey |
| P1 | Empty chat still feels setup-first | Large welcome and Recent cards compete with the composer | Use a quiet conversation empty state; keep recent tasks in the left rail and make folder/team actions secondary composer context controls. | W1 + W2 | Fresh-launch screenshot and J1 |
| P1 | TUI has no equivalent governance inspector | Screens are flat and `View()` renders one workspace | Render chat plus governance summary at wide terminal widths; use a stacked/switchable fallback for narrow terminals without losing selected panel state. | W3 | TUI width tests and PTY J5-J10 |
| P1 | Phase documentation overstates completion | ADR remains proposed, W1/W2 contracts remain pending, while `CURRENT` calls them accepted | Preserve historical commits, mark Phase 2C `PARTIAL`, record the reopened gates here, and accept the Phase only after all repaired journeys and independent reviews pass. | Phase | Documentation review and whole-Phase Result |
| P0 | Visible approval actions do not reach authoritative Mission Decision IPC | The TUI opened a synthetic read-only approval page and native Evidence actions could be inert | Expose only daemon-prepared actions, read the exact Mission Decision sheet, show expected Evidence, default to the first prepared action rather than the highest-authority action, and submit only the selected action through `mission_decision`. Hide the action when no prepared command or capable client exists. | W3 | Native click-path tests, TUI typed-client tests, J6 |
| P1 | Agent Team builder starts but is not visible | `builderSession` existed only in store state | Render the daemon-authored builder question, options, free-text answer, preview, compatibility issues, and explicit confirm/cancel controls. Confirmation must never auto-create a Mission. | W2 + W3 | Native structural tests and J4/J5 |
| P1 | New Mission is not reachable from the chat shell | Mission creation existed only inside the expanded workbench | Add New Mission to both the left rail and composer context controls, then open the existing governed Mission composer. | W3 | Native structural tests, TUI interaction tests, J5 |
| P1 | Governance destinations are incomplete or misleading | Panel labels and TUI hints exposed views/actions that had no distinct content or client capability | Provide distinct Overview, Board, Teams, Topology, Timeline, Decisions, Evidence, Runtimes, Attention, and Library content where supported; hide unsupported action shortcuts and keep full Board explicit. | W3 | View-switch tests, no-inert-action scan, J5-J9 |
| P1 | Full race gate can read an empty cancellation-fixture PID file | The provider test waits for file existence, while shell redirection creates the file before writing its contents | Publish the fixture PID through a temporary file and atomic rename so visibility means complete content. Do not change the production Provider runner. | Phase verification prerequisite | Failed lock-bound race attempt, focused repetition, full race rerun |
| P0 | TUI Home advertises message and Agent Team actions that do nothing | The Home key hints and renderer were added without matching `i` and `u` branches in the key handler | `i` must enter the existing bounded chat draft and submit through the typed chat client; `u` must open Team Builder and perform only the existing setup read. Neither path may write Team, Mission, or Run facts without the later explicit confirmations. | W2 + W3 | Real PTY failure, focused RED/GREEN interaction tests, replacement J1/J4 |
| P0 | Real PTY text input drops every space | Bubble Tea emits a physical space as `tea.KeySpace`, while the shared entry handler accepts only `tea.KeyRunes`; synthetic tests hid the mismatch | Accept `tea.KeySpace` as one ASCII space through the shared bounded entry path. Preserve every mode-specific byte ceiling, trimming rule, cancellation path, and typed client boundary. | W1 + W2 + W3 | Real PTY failure, physical-key RED/GREEN, replacement J1/J2/J4/J5/J9 |
| P0 | First-use chat loses a successful local-model response at five seconds | `chat_message` uses the ordinary five-second server and TUI connection deadlines even though local-model cold start completed in 5.270493 seconds; Swift also assigns the method only five seconds | Classify `credential_verify`, `mission_execution`, and `chat_message` as bounded long operations with a ten-second server/client window. Keep all other methods at five seconds, preserve cancellation, and never reinterpret a timeout as success. | W2 | Attempt 003 controlled IPC timing, Go server/client RED/GREEN, Swift timeout RED/GREEN, replacement J1 |
| P0 | Native chat sends an invalid wire key and silently restores the draft | `LocalProductChatMessageRequest` encodes Swift's default `threadID`, while the daemon's strict request contract requires `thread_id`; the store does not surface the typed remote error | Encode exactly `thread_id` and `content`, reject accidental key drift with an exact-shape Swift regression test, and preserve strict daemon decoding. Error presentation remains a separate UI review item unless the corrected request still fails. | W2 | Attempt 004 GUI/TUI IPC contrast, Swift encoding RED/GREEN, replacement native J2 |

## 3. Existing WorkItem Ownership

Phase 2C still contains exactly three WorkItems. Repairs are acceptance rounds
inside the existing ownership boundaries:

- **P2C-W1 repair round**: real folder selection, one truthful service status,
  quiet empty conversation, adaptive shell layout, J1/J2/J3/J10 rerun.
- **P2C-W2 repair round**: per-task thread identity, bounded restart-safe
  conversation storage, real conversational runtime port, J4/J5/J8 rerun.
- **P2C-W3 repair round**: compact panel state and switcher, first-class
  Decisions/Evidence/Runtime/Attention, truthful provider route, expanded Board,
  TUI split view, J5-J9 rerun.

Two whole-repository verification prerequisites are admitted without creating a
fourth WorkItem:

- `internal/app/local_queue.go` and `internal/app/local_queue_test.go` may
  serialize same-daemon admission for one DAG node so the existing
  `ErrDuplicateWork` invariant is deterministic under concurrency. This is a
  test-discovered correctness repair, not a Scheduler, Journal, policy, Grant,
  or authority change.
- `internal/provider/codex_native_auth_test.go` may atomically publish its child
  PID fixture so the race gate cannot observe an empty synchronization file.
  No Provider product source or behavior is admitted by this test-only repair.

All three files must be reviewed and included in the repaired source lock. No
other Queue, Scheduler, or Provider work is admitted by these exceptions.

Repair 3 remains inside the existing W2/W3 TUI ownership. It reopens only
`internal/tui/model.go` and `internal/tui/model_test.go`; it adds no IPC method,
Event type, authority writer, or fourth WorkItem.

Repair 4 remains inside the same two TUI files. The shared entry parser serves
W1 folder input, W2 chat/builder input, and W3 Mission/search input, so the
physical-space closure is jointly owned by W1/W2/W3 without adding a WorkItem.

Repair 5 remains inside W2 conversation transport ownership. It may change only
the bounded local IPC deadline policy, the TUI client configuration, and the
matching native/Go tests. It does not add an IPC method, model capability,
credential path, Event type, Journal write, Team/Mission authority, or fourth
WorkItem.

Repair 6 remains inside W2's existing native conversation wire ownership. It
may change only `LocalProductChatMessageRequest` encoding and its Swift client
regression test. It adds no IPC method, daemon behavior, model capability,
filesystem access, Event type, Journal write, authority, or fourth WorkItem.

## 4. Contract Reconciliation

This amendment supersedes conflicting wording in the original W1-W3 contracts
and Journey Manifest as follows:

1. Folder selection establishes a client-only display and conversation context.
   Only a sanitized folder name and opaque thread identity are retained. Loom
   performs no file read in Phase 2C; a daemon-authorized workspace identity and
   scoped access preflight are required before any future file operation.
2. Recent tasks are shown in the rail. Skills are governed assets inside
   Library, not a second duplicate top-level navigation item.
3. `Starting local service` is non-blocking and truthful. Retry and setup are
   exposed for recoverable states; a cancel action is required only after the
   daemon exposes a cancellable connection operation.
4. Agent Team confirmation creates only the confirmed Team state allowed by the
   existing builder contract. Starting a Mission always requires the separate,
   explicit New Mission flow.
5. Structured Agent content is rendered by the governed Team builder and
   Mission/Decision surfaces. Free-form model text remains tentative and is not
   parsed into authority-bearing cards by the client.
6. In the TUI, “full-width composer” means the complete center conversation
   lane at the active terminal width; a wide terminal may simultaneously reserve
   a stable governance lane.

## 5. Status Vocabulary

- `CURRENT`: chat-first shell and IPC chat scaffolding exist.
- `PARTIAL`: the current reply implementation, conversation continuity, folder
  journey, governance panel, compact behavior, and TUI parity.
- `TARGET`: a Codex-like pair-programming conversation with explicit Agent Team
  governance beside it, subject to Loom's policy and confirmation boundaries.
- `EXPERIMENTAL`: any conversational runtime adapter until a configured-runtime
  journey proves its output, cancellation, failure, and no-authority behavior.

The UI must never present `PARTIAL` or `EXPERIMENTAL` behavior as a connected,
working assistant when the required service or runtime is unavailable.

## 6. Repair Exit Gates

1. Every matrix row has a RED test or an explicit cross-client journey before
   implementation.
2. No visible button, tab, or menu item is inert or placeholder-only.
3. Native and TUI clients render one service state derived from the same daemon
   response; raw transport errors are developer details only.
4. J1-J10 are rerun from a clean daemon fixture, including daemon restart,
   compact width, keyboard traversal, and light/dark screenshots.
5. Go full/race/vet/tidy/gofmt and Swift full/TSAN/Release are green.
6. Independent implementation, contract, and whole-Phase reviews pass before
   ADR-0015 or Phase 2C is marked accepted.

## 7. Authority Boundary

This amendment does not allow chat text or model output to write authoritative
facts. A model response is tentative output. Team Draft, Mission, Run, approval,
Evidence, policy, Grant, and state transitions continue through their existing
daemon-authorized contracts and explicit confirmation flows.

## 8. Repair Candidate Checkpoint - 2026-08-08

Implemented in the current Candidate and covered by focused regression tests;
these observations are not lock-bound acceptance evidence:

- W1: native folder picker and `File > Open Folder...`, safe folder display
  handle, one user-facing service strip, quiet chat empty state, and adaptive
  900/1080/1440pt layout.
- W2: opaque per-task thread identity, bounded strict daemon-owned persistence,
  restart restoration, and a `LocalProductConversationResponder` port that
  removes fake echo behavior and reports an unavailable runtime truthfully.
- W3: independent hidden/visible/pinned inspector state, compact Overview,
  Board, Teams, Topology, Timeline, Decisions, Evidence, Runtimes, Attention,
  and Library views, direct Runtime & Providers routing, an explicit full Board,
  real prepared Mission Decision actions, and wide/compact TUI parity.
- Cross-client click paths: the native Team builder is visible and confirmable,
  New Mission is reachable from rail and composer, Evidence opens the exact
  Mission context, unsupported `edit_scope` is hidden until a real editor exists,
  and visible TUI actions are capability-gated.

The concrete configured conversation responder P0 is implemented in the
current repair Candidate as a dedicated tool-disabled Pi RPC
conversation adapter selected from the validated runtime/provider
configuration. It may reuse process binding and framed-response validation from
`internal/runtime/piadapter`, but it must not call the execution adapter, expose
tools, write Journal facts, or inherit execution authority. It must enforce
cancellation, response and transcript bounds, sanitized failures, and return
only tentative assistant text through `LocalProductConversationResponder`.

### Configured responder gate

The repair Candidate closes this blocker only when all of the following are
true:

1. The daemon owns one lazily started loopback local-model process. Ordinary
   conversation and governed Agent execution share that process and its exact
   configured model binding; neither path may start a competing process on the
   catalog port.
2. The conversation adapter runs Pi in offline RPC mode with sessions,
   extensions, skills, prompt templates, themes, context files, approval, and
   tools disabled. Any tool-call transcript event fails closed.
3. Conversation mode reuses strict Pi RPC lifecycle, identity, UTF-8, size, and
   terminal-closure validation, but it does not construct a Supervisor request,
   receive a Grant, emit bridge frames, or write Event Journal facts.
4. The bounded daemon-owned thread projection is serialized into an explicitly
   untrusted conversation prompt. Oldest messages are removed first when the
   prompt budget is reached; the latest user message is never removed.
5. Cancellation aborts and reaps the Pi child. Runtime, protocol, output, and
   cleanup failures cross the API boundary only as the existing sanitized
   recoverable conversation state.
6. Daemon shutdown is the sole owner of shared local-model cleanup. A mission
   executor may release its own adapter resources, but it cannot stop the shared
   model needed by another conversation or mission.

RED coverage must prove the tool-disabled argument vector, strict accepted
transcript, tool-call rejection, bounded history, cancellation cleanup, shared
single-start behavior, daemon responder injection, tentative output, and zero
new Journal facts for an ordinary configured chat exchange.

The configured adapter has focused daemon-level controlled-runtime coverage.
Phase acceptance remains pending until that coverage and J1-J10 are rerun
against the regenerated lock and the independent review gates pass. Runtime
absence continues to be a supported recoverable state, not simulated success.

Historical pre-lock verification runs on 2026-08-08 informed this repair but do
not authorize acceptance because source bytes changed afterward. Fresh Go,
Swift, Release, visual, and J1-J10 results must be recorded in new append-only
evidence bound to the regenerated source lock.

Non-blocking debt observed during verification: `QueueCommand<Input>` emits the
existing Swift 6 Sendable warning. Tighten the generic constraint to
`Input: Encodable & Sendable` in its owning Phase before Swift 6 mode becomes a
release gate.

Historical acceptance-harness risk observed during the first verification:
`TestPrepareSocketConcurrentReclaimHasSingleWinnerFiftyTimes` failed once with
zero winners because same-process contenders were relying only on `flock`.
Section 9 closes the risk with contracted exact-path same-process
serialization, while retaining `flock` for cross-process ownership.

## 9. Configured Responder Repair Checkpoint - 2026-08-08

Implemented in the current Candidate; fresh lock-bound verification remains
required:

- A dedicated `PiRPCConversationAdapter` runs Pi offline with approvals,
  sessions, extensions, skills, prompt templates, themes, and context files
  disabled. It shares strict Pi RPC lifecycle and assistant identity validation
  while rejecting every tool-call event and emitting no Supervisor frames.
- Conversation history is encoded as bounded untrusted JSON. Oldest messages
  are removed first and the latest user message remains mandatory.
- The daemon lazily owns one loopback local-model server shared by conversation
  and governed mission execution. Mission executor cleanup cannot stop the
  shared process; daemon shutdown is its owner and failed cleanup remains
  retryable at the runtime boundary.
- The configured IPC journey returns tentative assistant text through the real
  adapter, then constructs the governed Mission executor against the same
  runtime. The model starts exactly once, is not closed by the Mission executor,
  closes exactly once with the daemon, and ordinary chat records no Event
  Journal facts.
- TUI conversation identity is a stable opaque digest of the physical workspace
  and changes when a different folder is selected. Tests prove restart
  stability, cross-workspace isolation, and that neither a raw path nor the
  former constant `tui-thread` is exposed.
- Daemon shutdown tracks completion per owner. A failed model cleanup can be
  retried without re-closing successful owners, while `Run` remains unavailable
  from the moment shutdown starts.
- The serial repository matrix exposed an older Queue admission race: two jobs
  for one DAG node could both read the pre-admission projection and then depend
  on incidental SQLite lock timing. `LocalQueueService` now serializes the
  single-daemon admission critical section. One hundred normal and twenty
  race-enabled repetitions produce one admitted job and one typed
  `ErrDuplicateWork` rejection.
- Same-process socket preparation is serialized per exact socket path while
  cross-process ownership remains guarded by the advisory lock. The former
  flaky reclaim test passes 20 repetitions (1,000 contests) and 5 race-enabled
  repetitions (250 contests), each with exactly one winner.

Pre-lock test observations include focused Queue, socket, runtime, daemon, TUI,
and Swift UI regressions. They are diagnostic history only. The deterministic
record must rerun the complete matrix after the repaired lock and must preserve
any failed or superseded attempt.

The first lock-bound full race attempt failed in the provider cancellation test
after the fixture exposed an empty PID file. The failure and focused diagnosis
are preserved in `repair-deterministic-verification.md`. Repair 2 owns only the
fixture's atomic PID publication; it changes no Provider runtime behavior.

Independent cold reviews found the TUI constant identity, top-level cleanup
retry, shared-runtime integration proof, historical-status wording, stale lock,
Queue ownership, normative-input inventory, and evidence-mutation issues. The
Candidate contains repairs for those findings, but a fresh Contract Review must
judge the exact bytes before a new source lock is frozen. Remaining work is:

1. Run J1-J10 on native macOS and a real PTY TUI against one clean daemon
   fixture, including daemon restart and event/projection comparison.
2. Capture the required light, dark, 900pt, 1080pt, and 1440pt screenshots plus
   TUI transcripts and redacted IPC summaries.
3. Complete keyboard and accessibility traversal. Computer-Use automation stays
   skipped until explicitly reauthorized; manual VoiceOver evidence is valid.
4. Complete independent implementation, contract, dual-Result, whole-WorkItem,
   and whole-Phase reviews, then request Product Owner sign-off.

## 10. Repair 3 - Real PTY Home Action Closure

The first Repair 2 source-lock-bound cross-client attempt used a production
native App, a real PTY TUI, and one clean daemon fixture. Native opened
chat-first, but the TUI's advertised `i message` and `u Agent Team` keys were
inert. The attempt stopped before acceptance and is preserved at
`journeys/repair-2026-08-08/attempt-001/FAILURE.md`. Its Journal remained at
the three expected initialization/runtime facts with zero Team or Mission
facts, zero duplicate identities, and clean shutdown.

Mandatory Repair 3 RED reproduced both failures through `Model.Update`:

1. Home `i` did not enter `entryChatDraft`, so no bounded message could reach
   `ChatClient.SendChatMessage`.
2. Home `u` did not switch to `ScreenTeamBuilder` or call the existing
   `SetupClient.SetupSnapshot` read.

The minimal Candidate adds only those two Home branches. Focused RED/GREEN,
the adjacent New Mission regression, the full TUI package, formatting, and
diff checks pass. These are diagnostic results, not acceptance evidence.

Repair 3 invalidates the Repair 2 source lock for further acceptance runs.
Before any replacement journey, the exact Repair 3 contract and boundary must
receive an independent P0/P1/P2 review, a new source lock must be generated,
and the complete lock-bound deterministic matrix must pass again. Attempt 001
is never overwritten or promoted to a partial pass.

## 11. Repair 4 - Real PTY Space-Key Closure

The first Repair 3 replacement journey proved that Home `i` now opened a real
draft, then exposed a second terminal-only defect: every typed space vanished.
Attempt 002 is preserved at
`journeys/repair-2026-08-08/attempt-002/FAILURE.md`. It stopped before message
submission with the same three initialization/runtime Journal facts, zero
Team/Mission facts, zero duplicates, and clean shutdown.

Mandatory Repair 4 RED replaced the synthetic multi-rune sentence with the
physical Bubble Tea sequence `KeyRunes`, `KeySpace`, `KeyRunes`. Current source
rendered `Pairontheentryflow`, proving the shared entry parser ignored spaces.

The minimal Candidate computes the existing mode-specific byte ceiling first,
accepts `tea.KeySpace` as one byte only when capacity remains, and leaves every
other key, trim, cancellation, typed-client, and authority path unchanged.
Focused RED/GREEN, adjacent Home actions, the full TUI package, focused race
repetition, formatting, and diff checks pass. These are diagnostic results.

Repair 4 invalidates the Repair 3 source lock for further acceptance runs. A
fresh independent review, new source lock, and complete deterministic matrix
are required before attempt 003. Attempts 001 and 002 remain failed history.

## 12. Repair 5 - First-use Chat Deadline Closure

Attempt 003 is preserved at
`journeys/repair-2026-08-08/attempt-003/FAILURE.md`. The real PTY proved Repair
4 by rendering every physical space, then `chat_message` request
`loom-client-2` crossed the local IPC boundary at monotonic offset `89819010`
microseconds. The daemon produced an otherwise successful response at
`95089503`, after `5270493` microseconds. The server connection/handler and TUI
client had already reached their ordinary five-second deadlines, so the TUI
reported `timeout` and did not publish the returned thread.

Mandatory Repair 5 RED proves three independent gaps: the Go client lacks a
method-specific extended timeout, the Go server gives `mission_execution` and
`chat_message` only five seconds, and the Swift client gives `chat_message`
only five seconds. The bounded GREEN classifies exactly
`credential_verify`, `mission_execution`, and `chat_message` as ten-second
operations. Every other method remains five seconds. Context cancellation,
socket ownership, strict framing, response identity, and error mapping are
unchanged.

Focused Go client/server tests, affected Go package normal/race tests, and the
focused Swift client test are GREEN. These are diagnostic results, not
acceptance evidence.

Repair 5 invalidates the Repair 4 source lock for further acceptance runs. A
fresh independent review, expanded source lock, and complete deterministic
matrix are required before a fresh replacement journey. Attempts 001, 002, and
003 remain failed history and may not be reused or promoted.

## 13. Repair 6 - Native Chat Wire-Key Closure

Attempt 004 is preserved at
`journeys/repair-2026-08-08/attempt-004/FAILURE.md`. Repair 5 passed its full
source-lock-bound matrix. In the replacement fixture, both real-PTY TUI chat
requests passed, including first-use latency, while five native
`chat_message` requests reached the same daemon and failed with
`invalid_request`. Native folder selection itself passed and exposed only the
safe `Documents` display name.

The strict wire mismatch is deterministic. `LocalProductChatThreadRequest`
defines `threadID = "thread_id"`; the adjacent
`LocalProductChatMessageRequest` does not, so `JSONEncoder` emits `threadID`.
The daemon correctly rejects that unknown key and missing `thread_id`. The
native store then restores the draft, which explains the apparently inert send
action without introducing any authority fact.

Mandatory Repair 6 RED must encode a real `LocalProductChatMessageRequest` and
prove the key set is exactly `thread_id` plus `content`, with no `threadID`.
GREEN adds only the missing `CodingKeys`. Existing strict request decoding,
identifier/content bounds, long-operation deadline, tentative response model,
and zero-Journal ordinary-chat rule remain unchanged.

Repair 6 invalidates the Repair 5 source lock for further acceptance runs. A
fresh independent review, expanded source lock, complete deterministic matrix,
and new clean shared fixture are required before replacement attempt 005.
Attempts 001-004 remain failed history and may not be reused or promoted.

## 14. Repair 7 - Authoritative Start, Cold Runtime, and Cross-client Closure

Attempt 005 is preserved at
`journeys/repair-2026-08-08/attempt-005/FAILURE.md`. It passed J1-J4 and the
J5 Team confirmation/preflight boundary. The explicit Mission start then
returned `state_unavailable` after `5.054680` seconds while its daemon-owned
flight remained live. The same flight later committed `TeamExecutionPlanned`
through `TeamReadySetDispatched` and left the Team projected as `running`
without a Grant, terminal Run, finalized Evidence, or Team terminal/recovery
fact. The client response and authoritative state therefore disagreed about
whether execution had started.

### P0 authoritative acceptance contract

Mission start is not a best-effort request. Before a client can receive a
retry-shaped error, exactly one of these outcomes must be true and observable:

1. No execution authority fact was committed, the daemon-owned flight is
   cancelled and joined, and it cannot write a later fact for that command.
2. An authoritative execution lineage for the command is projected, and the
   response carries its Mission ID, Team ID, execution digest, view version,
   and truthful non-terminal or terminal status.
3. A typed authoritative recovery receipt is committed and returned. Repair 7
   does not introduce such a new receipt, so the Candidate must satisfy item 1
   or item 2 using the existing contract.

`state_unavailable`, IPC timeout, caller cancellation, or projection timeout
may not be returned while the same daemon-owned flight can later create or
advance authority facts invisible to that response. A repeated start with the
same preflight/plan lineage must converge on the same execution digest and
projected state; it must not create a second Team execution, WorkItem, Run,
Grant, or runtime reservation.

### P0 cold-runtime solution boundary

The production executor must be constructible without synchronously starting
or health-waiting on the local model. `TeamCoordinator.Run` must be able to
commit and project the existing plan/dispatch lineage before model startup or
another bounded runtime initialization step begins. Slow initialization then
runs inside the already-authorized Supervisor execution lifecycle.

Runtime initialization failure or cancellation after dispatch must be handled
by that lifecycle: the Run reaches a truthful `failed` or `cancelled` terminal
state, its Grant is revoked, Evidence is finalized, and the Team reaches an
existing terminal or governed recovery state. It may not stop at
`TeamReadySetDispatched` with an empty, unterminated capture. Daemon shutdown
continues to own a shared model; Mission executor cleanup must not stop it.

The repair must preserve all existing model binding, loopback-only, private
root, offline, no-tool, exact runtime identity, strict bridge, Grant, Evidence,
and one-writer constraints. It must not manufacture a client-side `running`
state before an Event Journal fact exists.

Mandatory RED/GREEN coverage must prove all of the following:

1. A controlled local-model starter remains blocked longer than the projection
   visibility timeout. Mission start still returns the existing authoritative
   `running` result before the starter is released. Before invoking any local-
   model starter or health wait, the Journal/projections and Evidence store
   must expose exactly one complete start lineage for the execution digest:
   `TeamExecutionPlanned`, ready-set dispatch facts through
   `TeamReadySetDispatched`, the current WorkItem/Run/capacity reservation, a
   bound attempt capture, `AgentGrantIssued`, and `RunStarted` where required
   by the existing terminalization lifecycle. `StartMission` may return
   `running` only from that projected lineage, not from
   `TeamExecutionPlanned` alone.
2. Releasing the starter allows the same lineage to execute and terminalize;
   no duplicate plan, WorkItem, Run, reservation, Grant, or Evidence identity
   appears.
3. A starter failure after dispatch becomes a terminal failed/recovery lineage
   with revoked Grant and finalized Evidence, never an empty capture plus a
   permanently running Team.
4. Caller timeout/cancellation and daemon shutdown cannot leave an unreported
   flight that writes a later first authority fact.
5. Existing warm-runtime vertical execution, ordinary chat zero-authority,
   single shared model ownership, and restart recovery tests remain green.

Merely raising `VisibilityTimeout`, widening IPC deadlines, returning a local
in-memory flight as `running`, or retrying in the client does not satisfy this
contract.

### P1 stale cross-client draft contract

When one client confirms a Team, another client's draft at the old view version
is stale. The first stale confirm may return the existing typed `conflict`, but
the client must then refresh authoritative setup state, mark or dismiss the
stale draft, show one actionable explanation, and disable confirmation of that
same stale revision. Repeated render/update cycles may not resubmit it. Starting
a new draft remains an explicit user action. No conflict path writes authority
facts.

Mandatory Swift Store/UI coverage must reproduce a stale confirm against a
newer setup snapshot and prove one submit, one refresh, no automatic retry, no
remaining enabled stale-confirm action, and a truthful recovery message. TUI
coverage must prove equivalent conflict recovery when its draft becomes stale.

### P2 tool-shaped tentative output contract

Ordinary conversation may contain untrusted text that resembles a tool call.
Tool-shaped tentative prose is any tentative assistant content whose trimmed
body is JSON, fenced JSON, or a JSON-like object/array containing `tool`,
`tool_name`, `arguments`, `command`, `path`, `function_call`, or `tool_call`.
Native and TUI must replace that body in the main conversation with the fixed
non-actionable warning: `The conversation runtime returned tool-shaped text.
Loom did not execute it.` No button, shortcut, permission row, or execution
control may be rendered from it. Tests must prove zero tool execution, zero
Journal facts, and no actionable tool control for that output.

Repair 7 invalidates the Repair 6 source lock for all future acceptance work.
The repair sequence is: causal RED, bounded GREEN, focused and adjacent tests,
independent P0/P1/P2 contract review, regenerated expanded source lock,
complete deterministic matrix, and a new clean attempt 006. Attempts 001-005
remain failed history and may not be reused or promoted.

## 15. Repair 8 - Per-Attempt Terminal Commit Budget

The first Repair 7 lock-bound serial race matrix failed in
`TestProductMissionExecutionVerticalLoopbackClosesAuthorizedLineage`. A Team
outcome reached terminal Run state, but `commitTeamTaskOutcomes` shared one
five-second detached context across every outcome. Under race instrumentation
and SQLite contention, an earlier outcome consumed the shared budget and a
later Evidence commit returned `context deadline exceeded`. The resulting
Side-task reconciliation correctly failed closed with `Side-task product
unavailable`; the matrix is preserved and no Journey was authorized.

Terminalization after execution cancellation must remain detached from the
cancelled execution context, bounded, and complete for every attempt. Each
attempt therefore receives its own 15-second commit context covering capture
finalization and its matching authoritative Team receipt. One attempt may not
consume another attempt's cleanup budget. The timeout remains finite; Repair 8
does not introduce an unbounded shutdown wait, a retry loop, or a new Event.

Mandatory verification is the causal failed race test followed by focused
normal/race Team and daemon tests, independent P0/P1/P2 review, a regenerated
51-path lock, and the entire deterministic matrix. Repair 8 invalidates the
Repair 7 lock. Attempt 006 remains unauthorized until the replacement matrix
passes.

## 16. Repair 9 - Exact Decision Submission and Restart Recovery

Repair 8 passed its independent review, regenerated 51-path source lock, full
serial Go normal/race/vet/tidy/format matrix, Swift and Thread Sanitizer suites,
and signed Release build. Attempt 006 then proved chat-first entry, tentative
ordinary conversation, explicit Team confirmation, explicit Mission start,
complete governed execution terminalization, and Journal-backed restart
recovery. It failed before Phase acceptance because the real TUI submitted a
prepared authoritative Mission decision with operation `decide`, while the
closed service contract accepts authoritative actions only with operation
`submit`. The daemon correctly rejected the request as `invalid_request` and
wrote no decision fact.

The attempt also exposed two bounded recovery defects. A Team Draft held by a
client across daemon restart can be absent from the daemon's in-memory draft
registry. Confirmation then returns typed `not_found`; both clients must treat
that response like an expired stale draft, discard it, refresh authoritative
setup once, explain that a new draft is required, and never resubmit
automatically. Separately, after Mission start returns an authoritative
`running` result, the TUI refreshes against a newer view while retaining the
consumed preflight. Its snapshot handler clears the successful result and
reports `preflight_expired`, contradicting the accepted start.

Repair 9 has three closed requirements:

1. TUI authoritative Mission actions use operation `submit` with the exact
   prepared action, digest, identity, view version, and fresh correlation ID.
   Native Store must also reject any action absent from the authoritative
   sheet's action and prepared-action sets. A Native authoritative submit may
   dismiss the sheet only after a matching authoritative Mission/Decision
   result is returned. `read` and `defer` remain unchanged. Tests must reject
   any reintroduction of `decide`, unprepared Native action, or non-authoritative
   or identity-drifted submit result; the service remains the authority for
   validation and replay.
2. Native and TUI Team confirmation recover from both typed `conflict` and
   typed `not_found` by issuing exactly one confirmation request, discarding
   the local draft, refreshing setup exactly once, presenting one actionable
   recovery message, and requiring an explicit new draft. No authority fact is
   written and no automatic retry is permitted.
3. A successful Mission start consumes and clears its preflight before the
   snapshot refresh in both clients. A newer authoritative snapshot may update
   the board but must not replace the accepted `running` result with
   `preflight_expired`. A repeated Store/TUI start call after consumption sends
   no request and does not overwrite accepted running state. A genuinely stale
   preflight before Start must continue to expire normally.

The 500-millisecond discovery interval briefly used during Attempt 006 was a
journey-harness misconfiguration; the product contract and historical fixture
use ten seconds. Its observer timeout is retained as diagnostic evidence only
and does not authorize product source changes. Native system-picker and native
builder interaction, the controlled runtime-offline path, dual-client decision
actions, accessibility, and visual review remain unverified and must be rerun
in Attempt 007. Computer-Use automation remains skipped unless explicitly
reauthorized.

Repair 9 invalidates the Repair 8 source lock. The required sequence is causal
RED tests, bounded GREEN changes in the already-listed TUI and Swift Store
paths, focused and adjacent tests, independent review, regenerated source lock,
the complete deterministic matrix, and a new clean Attempt 007. Attempt 006 is
immutable failed history and no partial result may be promoted.

## 17. Repair 10 - Controlled Runtime Offline Authority

Repair 9 passed final independent re-review, a regenerated source lock, the
complete deterministic matrix, and a signed Release. The replacement journeys
then proved exact TUI decision submission, restart-expired Team Draft recovery,
successful Mission-start continuity, governed terminalization, and exact
Journal-backed restart projection. J7 remained blocked: an empty Pi search path
correctly produced no observation and no status transition because the durable
Runtime contract forbids inferring offline from absence. The Pi metadata probe
can author only online observations, so Phase 2C has no deterministic
production-shaped path that can create an authoritative offline fact for the
cross-client journey.

Repair 10 adds one test-only, environment-gated controlled Runtime status
fixture beside the existing controlled Mission fixture. It has the following
closed requirements:

1. The fixture is accepted only from a private current-owner `0600` manifest
   at the exact `<attempt-root>/manifest/runtime-status-fixture.json` path. The
   attempt root and state/manifest directories remain canonical current-owner
   `0700` paths. Strict decoding rejects unknown fields, symlinks, path drift,
   invalid source-commit identity, future or non-monotonic time, unsupported
   status pairs, and identity mismatch before any write. For this Phase 2C-only
   fixture, `source_commit` must equal the frozen Repair 10 baseline HEAD
   `651f156afda37a8e703cbc0396f9f38b7912600b`; the daemon does not discover or
   trust repository state at runtime.
2. The only Repair 10 transition is an explicit expected `online` to target
   `offline` transition for one already-projected Runtime identity. The fixture
   must build a real Runtime discovery snapshot, canonical projected baselines,
   the existing Runtime status reconciliation Candidate, and the existing
   prepared status committer over the Event Journal. Direct SQL mutation,
   absence inference, a new Event type, client-side state, or a synthetic UI
   status is forbidden.
3. Exactly one `RuntimeInstanceStatusChanged` fact is committed with
   deterministic fixture-scoped reconciliation, Event, and idempotency
   identities and exact-next stream sequence. The projection must rebuild to
   offline before IPC becomes ready. Mission/Team status and all unrelated
   facts remain unchanged. An exact fixture encountered again after its exact
   fact is already projected is a completed replay and succeeds with zero
   append. Any same-identity envelope drift, unmatched offline projection, or
   other stale fixture fails closed and writes nothing.
4. J7 recovery omits the controlled fixture and restores the real Pi search
   paths. The ordinary observer must then reconcile the same Runtime identity
   from offline to online through a second authoritative status fact. Runtime
   health remains distinct from local-service availability and Mission state.

Mandatory RED/GREEN coverage must prove the private-manifest boundary, exact
Journal transition, projection-before-IPC behavior, exact restart replay with
zero duplicate write, stale/drift zero-write rejection, Mission-state
independence, and ordinary observer recovery. Repair 10 changes
only the already-listed `cmd/loomd/product_daemon.go` and
`cmd/loomd/product_daemon_test.go` product paths plus this contract/status
record. It invalidates the Repair 9 source lock. A fresh independent review,
regenerated lock, complete deterministic matrix, and a new clean replacement
journey are required before Phase acceptance.

## 18. Repair 11 - Coherent Attention Recovery and Final Client Evidence

Repair 10 passed final independent source review, regenerated its source lock,
passed the complete deterministic matrix, and produced a signed Release.
Attempt 009 then proved J1-J6, a controlled authoritative `online -> offline`
Runtime fact, ordinary observer `offline -> online` recovery, and byte-identical
restart rebuild with `408/408/408` unique Event and idempotency identities. J7
did not pass as one coherent client journey, and independent journey/UI review
returned `FAIL`.
The TUI Attention page rendered authoritative product `snapshot.Attention` plus
permission attention but refreshed only the permission source, so a recovered
Runtime continued to display `restore_runtime` until the user refreshed the
Runtimes page. The same review found that final native J9 evidence was static
rather than a live accessibility/keyboard traversal. Its live J10 gap was
subsequently closed by real signed-Release light/dark screenshots at 900, 1080,
and 1440 points, but those images predate Repair 11 source bytes and must be
recaptured after the next lock.

Attempt 009 also exposed two P2 quality issues: the journey harness omitted the
existing `LOOM_JOURNEY_ID`, so TUI permission calls used a generated UUID, and
native Recent items collapsed every authoritative attention item to `Needs your
attention` despite already carrying safe `action_required` context.

Repair 11 has four closed requirements:

1. Entering or refreshing TUI Attention must refresh the exact two sources the
   page renders: the authoritative local-product snapshot and, when supported,
   permission attention. Runtime recovery must remove stale
   `restore_runtime` in place after one `r`; the user must not navigate to
   another screen. A client failure preserves the last loaded authoritative
   view and presents the existing truthful service state. No client may invent,
   clear, or write an attention fact.
2. Native Recent attention titles use the existing sanitized
   `action_required` value as the primary context when non-empty, with a
   deterministic humanized `kind` fallback. Raw internal IDs, paths, prompts,
   outputs, credentials, or new daemon fields are forbidden. Governance remains
   hidden by default and conversation remains the primary lane.
3. Every replacement fixture explicitly supplies the manifest Journey UUID via
   the existing `LOOM_JOURNEY_ID` input before TUI model construction. All main
   TUI typed requests and correlations must use that UUID. Intentional decision
   or other subfixtures retain separate UUIDs only when listed in the final
   evidence map. This is a harness/evidence correction, not a new product flag
   or authority source.
4. J9 is rerun live against the signed Release without Computer Use. Accepted
   alternative evidence is a direct macOS accessibility traversal that records
   visible actionable AX roles and unique human labels, plus real keyboard Tab,
   Shift-Tab, activation, and Escape behavior across chat and governance. J10
   is recaptured from the same source-locked signed Release in light and dark at
   900, 1080, and 1440 points. Static tests remain supporting evidence only.

Mandatory RED coverage must prove that Attention `r` requests both rendered
sources and that native Recent attention titles expose safe action context with
the documented fallback. Repair 11 changes only the already-listed
`internal/tui/model.go`, `internal/tui/model_test.go`,
`apps/macos/Sources/LoomLocalAppCore/LocalProductStore.swift`, and
`apps/macos/Tests/LoomLocalAppTests/LocalProductStoreTests.swift` product/test
paths plus this contract, Candidate boundary, and current-status record. It adds
no IPC method, Event type, authority, filesystem access, model behavior,
execution trigger, or persistence format.

Repair 11 invalidates the Repair 10 source lock. The required sequence is an
independent review of these exact contract/boundary bytes, causal RED tests,
minimal GREEN changes, focused and adjacent verification, final independent
source review, a regenerated lock, the complete deterministic matrix, a fresh
signed Release, and a new clean J1-J10 replacement journey with independent
journey/UI and whole-Phase reviews. Attempt 009 remains immutable failed
history and may not be promoted.

## 19. Repair 11 Matrix Remediation - Standalone Store Compilation

The first Repair 11 lock was structurally valid, but its lock-bound Go normal
matrix failed in the real Go-to-Swift contract probe. That probe intentionally
compiles an explicit standalone Store source set. `LocalProductStore.swift`
referenced `SafeText`, while `SafeText.swift` was not part of that source set,
so the Store could not compile at the cross-language contract boundary even
though the Swift Package build passed.

The remediation remains inside the four Repair 11 source/test paths. The Store
uses a private, bounded attention-text sanitizer with the same relevant closed
properties: CSI and OSC escape sequences, remaining control characters, and
bidi controls are removed; newline and tab become one visible space; and the
result is bounded to 48 characters before humanization. Tests must cover CSI,
OSC, bidi, the bound, action-first context, and kind fallback. The existing
real cross-language probe must compile and pass without adding `SafeText.swift`
to its source list or changing the probe, IPC, daemon, Event Journal, authority,
filesystem, or persistence boundary.

The failed lock and matrix remain immutable evidence. Focused cross-language
and Swift GREEN, independent exact-byte source/status review, a new replacement
lock, and the complete fresh deterministic matrix are required before any
replacement journey is authorized.

## 20. Repair 12 - Native Governance Escape

Repair 11 passed its replacement lock-bound matrix and produced a signed
Release. Attempt 011 then proved J1-J8, including coherent TUI Attention
recovery and byte-identical restart rebuild. Live native J9 identified one new
P1 interaction defect: keyboard Space opened the visible governance inspector
with a unique AX label and visible focus, but Escape left the inspector open.
That violates ADR-0015 and J9; Attempt 011 is failed history and is not promoted.

Repair 12 is limited to the existing native shell state, shell view, and shell
state test paths. The governance state exposes one deterministic dismissal
transition: hidden is a no-op, while visible or pinned closes without changing
the selected governance destination. The shell routes the native
`onExitCommand` to that transition. The repair adds no IPC method, Journal fact,
authority, execution trigger, persistence, filesystem access, or model behavior.

Focused tests must cover hidden, visible, and pinned transitions. An independent
source review, regenerated source lock, complete Go/Swift/Thread Sanitizer and
fresh signed-Release matrix, then a clean J1-J10 replacement journey are
required. Live J9 must prove AX labels, visible focus, Tab, Shift-Tab, keyboard
activation, and Escape closure; J10 must come from that same signed Release.

## 21. Repair 13 - Native Action Labels

Repair 12 passed independent source review, its replacement source lock, the
complete deterministic matrix, and produced a signed Release. Attempt 012 then
proved J1-J8, live governance Space/Escape behavior, and the full signed-Release
J10 matrix. Final live J9 AX enumeration identified three enabled product
buttons with `AXPress` but no human-readable AX help: service `Try Again`,
empty-chat `Open Folder...`, and empty-chat `Use Agent Team`. Standard window
buttons are distinguished by their native AX subroles and are not product
actions. Attempt 012 is failed history and is not promoted.

Repair 13 is limited to `LoomWorkspaceShell.swift` and its existing
`LoomGraphiteViewTests.swift` regression path. Each of the three product actions
must expose a stable, concise help label matching its visible command. The
repair changes no action closure, navigation, IPC, Journal fact, authority,
execution trigger, persistence, filesystem access, model behavior, layout, or
visual token.

Causal RED must fail on all three missing-help conditions. Focused GREEN and a
fresh signed-Release AX enumeration must prove every enabled product `AXPress`
button has a non-empty human label, while separately identifying the standard
close, minimize, and full-screen controls by AX subrole. An independent source
review, regenerated source lock, complete Go/Swift/Thread Sanitizer/Release
matrix, and a clean J1-J10 replacement journey remain required.

## 22. Repair 14 - Long-operation Response Grace

Repair 13 passed independent review, its replacement lock, the complete
deterministic matrix, a signed Release, and direct AX enumeration with zero
unlabeled enabled product actions. Attempt 013 then passed J1 and folder
selection before the first post-restart folder chat exposed an equal-deadline
race. The daemon received `chat_message` and emitted its bounded safe response
`10.066780s` later, while the TUI client's long-operation deadline was exactly
the same nominal 10 seconds. The client therefore rendered raw `timeout` before
the daemon's safe tentative unavailable message arrived and was persisted.
Attempt 013 is failed history and is not promoted.

The server retains its existing 10-second bounded handler deadline for
`credential_verify`, `mission_execution`, and `chat_message`. Its connection
gets a 12-second response-I/O deadline for those methods. Go and Swift clients
use a 15-second receive deadline while retaining 5 seconds for ordinary
methods. An explicitly configured Go extended client deadline less than or
equal to the server's 12-second response deadline is invalid. This creates
response-delivery grace; it does not extend handler work, reinterpret a timeout
as success, retry an operation, or weaken cancellation.

Repair 14 changes only the already-listed Go local IPC client/server/tests,
CLI/TUI composition, Swift IPC client/test, contract, Candidate Boundary, and
current status paths. It adds no IPC method, response code, Event, authority,
execution trigger, persistence, filesystem access, model behavior, or retry
loop.

Causal RED must prove that equal explicit Go deadlines are currently accepted
and that Swift long operations currently use 10 seconds. GREEN must prove the
handler remains 10 seconds, response I/O is 12 seconds, Go and Swift clients use
15 seconds, ordinary methods remain 5 seconds, earlier context cancellation
still wins, and the client receives a controlled response after the 10-second
handler boundary. Independent source review passed with `P0=P1=P2=0`; a
replacement source lock, complete lock-bound matrix, signed Release, and clean
J1-J10 replacement journey remain mandatory.

## 23. Repair 15 - Native Recent Action Help

Repair 14 passed independent source review, its replacement source lock, the
complete deterministic matrix, and produced a signed Release. Attempt 015 then
proved J1-J8, including the first post-restart folder chat returning a bounded
safe response without a raw client timeout, exact prepared approval facts,
authoritative Runtime offline/recovery transitions, in-place Attention refresh,
and byte-identical restart rebuilds. Live signed-Release J9 exposed a narrower
native accessibility defect. Six visible Recent rows were enabled `AXButton`
elements with `AXPress`, but each reported empty `AXHelp`, `AXTitle`, and
`AXValue`. Directly pressing the first such element selected the visible
`Inspect Failure` Recent row, proving these are product actions rather than the
separately identified `AXScrollBar` or standard-window controls. Attempt 015 is
failed history and is not promoted.

Repair 15 is limited to the already-listed
`apps/macos/Sources/LoomLocalAppUI/LoomWorkspaceShell.swift` and
`apps/macos/Tests/LoomLocalAppTests/LoomGraphiteViewTests.swift` paths. Each
Recent row must expose a stable concise help string derived from its visible
title. A single native UI helper must sanitize escape, control, newline, tab,
and bidi content through existing `SafeText`, bound the title component to 48
characters, trim it, and fall back to `Open recent task` when empty. Both the
accessibility label and help must consume the exact helper output. The repair changes no action
closure, selection behavior, layout, visual token, IPC, Event, authority,
execution trigger, persistence, filesystem access, or model behavior.

Causal RED failed because the Recent-row helper had an accessibility label but
no `.help` modifier. Source Review 1 then found that directly interpolating the
task title was not safe for raw saved-team names and that the structural test
did not prove sanitization or bounding. Remediation must test hostile and empty
titles through the shared helper, then bind its result to both the existing
label and the new help string. A fresh signed-Release AX
enumeration must classify product buttons separately from `AXScrollBar` and
standard-window subroles and prove zero enabled product `AXPress` buttons with
empty human help. Independent contract and source reviews, a replacement
source lock, the complete deterministic matrix, and a clean J1-J10 replacement
journey remain mandatory before Phase 2C or ADR-0015 acceptance.

## 24. Repair 16 - Product-daemon Test Readiness Barrier

Repair 15 passed source/status reviews and its replacement source-lock review.
The first lock-bound full Go normal matrix then failed only in
`TestProductDaemonProjectsControlledRuntimeOfflineBeforeIPCAndReplaysOnRestart`:
its snapshot call returned `local product unavailable`. The exact frozen test
reproduced once in 20 consecutive runs. This is not classified as an
environmental transient and invalidates the Repair 15 lock.

The product-daemon integration helper currently treats an `Lstat`-visible Unix
socket as complete readiness. The accepted local IPC Server already exposes an
exact `Ready()` channel that closes only after the listener, ownership,
permission, identity, lock, and server state have been installed. The daemon
tests launch `runner.Run` asynchronously but bypass that signal, allowing a
filesystem-only observation to race this generation's readiness, especially
across rapid reuse of one socket path.

Repair 16 is limited to the already-listed
`cmd/loomd/product_daemon_test.go`. Its shared wait helper must require the exact
runner server's `Ready()` signal with the existing bounded test timeout before
confirming the socket path, and all 16 product-daemon call sites must pass their
own runner. The helper must fail closed unless that exact value is a
`*productDaemonRunner`, then await its server. It must never accept a stale path or a different
runner generation. Production daemon, local IPC client/server, retry policy,
deadlines, protocol, Event Journal, authority, persistence, filesystem access,
model behavior, and executable bytes remain unchanged.

Causal RED is the preserved full-normal failure plus the exact 20-run
reproduction. GREEN must pass the exact test at least 100 consecutive times,
the complete `cmd/loomd` package normal/race, independent source review, a new
replacement source lock, and the complete fresh Go/Swift/TSAN/signed-Release
matrix. Only then may a clean Attempt 016 rerun J1-J10.

## 25. Repair 17 - Quiescent Human-review Restart

Repair 16 passed its replacement source lock, complete deterministic matrix,
and signed Release. Attempt 016 was rejected because ordinary Runtime recovery
omitted the original local-model catalog arguments. Attempt 017 corrected that
envelope and passed J1-J8 behaviorally, but two J7 TUI permission-attention
reads used a generated Journey UUID because the harness omitted
`LOOM_JOURNEY_ID`; it is failed traceability history. Attempt 018 corrected the
UUID binding and passed J1-J6. Its Mission legitimately reached human review
with one succeeded work Run and one `insufficient_evidence` verifier Run.

Attempt 018 J7 appended the exact controlled `online -> offline` Runtime fact,
rendered `restore_runtime`, and replayed byte-identically. Ordinary recovery
then failed closed in `build_execution`. A read-only diagnostic build against a
temporary source copy exposed `mission execution conflict`. The authoritative
TeamExecution aggregate remains `running` while its only node is
`ready_for_review`; `ResumeProjectedMissions` attempts to reconstruct that
quiescent lineage before Runtime observation can restore the offline binding.
This prevents daemon restart while a Mission awaits human review and violates
J7/J8.

Repair 17 is limited to the already-listed
`internal/app/local_product_execution.go` and
`internal/app/local_product_execution_test.go` paths plus this contract,
Candidate Boundary, current status, review, lock, and append-only evidence
records. Restart reconciliation must treat a projected execution as quiescent
only when its aggregate status is `running`, it has at least one node, at least
one node is `ready_for_review`, and every node is either `succeeded` or
`ready_for_review`. A quiescent review lineage must remain projected and must
not reconstruct, launch, retry, or append authority. Executions with active,
pending, or recovery nodes retain the existing exact resume path. Terminal
executions remain skipped and unknown nonterminal states remain fail-closed.

The causal RED is
`TestAuthoritativeMissionRestartKeepsReadyForReviewQuiescent`: before GREEN it
failed with `mission execution conflict`. GREEN must prove zero runner calls
for that topology while the existing expired-running-lineage resume and unknown
status rejection remain green. The complete `internal/app` package, focused
daemon restart coverage, independent contract/source reviews, a replacement
source lock, the complete Go normal/race/vet/format and Swift/TSAN matrix, a new
signed Release, and a clean J1-J10 replacement journey are mandatory. The live
diagnostic recovery of failed Attempt 018 is supporting causal evidence only;
it cannot promote that mixed-source attempt.

Repair 17 adds no Event type, authority, execution trigger, retry, timeout,
IPC field, persistence format, filesystem access, Provider behavior, model
behavior, or autonomous action. Phase 2C and ADR-0015 remain unaccepted.

## 26. Repair 18 - Deterministic Cross-language Test Readiness

Repair 17 passed source/status/lock review. Its first lock-bound full Go normal
matrix then failed. `cmd/loomd` reached the package's 12-minute timeout while
`TestProductMissionExecutionVerticalLoopbackClosesAuthorizedLineage` waited in
`buildProductDaemonSwiftContractProbe`; that invocation had spent `4m48s` in an
external Swift build. The same exact test passed alone in `99.926s` (wall
`105.26s`). The helper is called twice in one package process but assigns each
call a different scratch path, forcing two cold Release builds of the same
`LoomLocalAppContractProbe` inside an already long integration package.

The same matrix also failed
`TestSystemCodexLoginControllerStartsExactSingletonAndJoinsOnClose` with `EOF`
while parsing its PID fixture. The test waits only for path existence; the shell
creates the file before `printf` completes. An exact 100-run repetition failed
twice with the same `EOF`. This is a test synchronization defect, not a Codex
login product failure.

Repair 18 is limited to the already-listed
`cmd/loomd/product_daemon_test.go` and
`internal/provider/codex_native_auth_test.go` plus this contract, Candidate,
current status, review, replacement lock, and append-only evidence. The daemon
test package must build `LoomLocalAppContractProbe` at most once per test
process through `sync.Once`, use a private process-owned temporary scratch root,
return the same executable path to both call sites, preserve complete build
error output, and remove the root after `m.Run()` through one package
`TestMain`. It must not reuse repository `.build`, share mutable artifacts
between test processes, skip either client journey, or change Swift product
source.

The provider test must poll within its existing five-second bound until the PID
file both reads successfully and parses as one positive decimal PID. Only then
may it call `Close` and assert `ESRCH`. It must not add production retry, weaken
singleton/join assertions, extend the product timeout, or ignore malformed
fixture output.

Causal RED is the preserved full-normal failure, the exact Swift test's
single-run timing, and the provider count-100 `2/100` reproduction. GREEN must
prove one shared Swift executable path in-process, both Swift-probe client tests
in one invocation, provider count-100, complete `cmd/loomd` and
`internal/provider` normal/race, independent source/status reviews, a
replacement source lock, and the complete fresh matrix. Repair 18 changes no
production executable, Event, authority, retry, IPC, persistence, filesystem,
Provider, model, credential, or UI behavior. Phase 2C and ADR-0015 remain
unaccepted.

## 27. Repair 19 - Distinct Native Recent-row Actions

Repair 18 passed its replacement source-lock review, complete deterministic
matrix, and signed Release. Attempt 022 then passed J1-J8 authority and restart
boundaries and the J10 visual matrix. Its direct signed-Release J9 action audit
found four visually distinct Recent rows whose VoiceOver actions were not
distinguishable. Two rows shared `Open recent task Recent work`, and two shared
`Open recent task Attempt 022 Review Team`, because the accepted helper included
only the repeated visible title. Attempt 022 is failed history and is not
promoted.

Repair 19 is limited to the already-listed
`apps/macos/Sources/LoomLocalAppUI/LoomWorkspaceShell.swift` and
`apps/macos/Tests/LoomLocalAppTests/LoomGraphiteViewTests.swift` paths plus this
contract, Candidate Boundary, current status, review, replacement lock, and
append-only evidence. The one shared Recent-row helper must produce a bounded,
sanitized label from the row's one-based visible position, title, and subtitle.
It must sanitize title and subtitle through `SafeText`, bound them to 48 and 32
characters respectively, clamp a non-positive position to one, omit empty
components, and bind the exact result to both `.accessibilityLabel` and `.help`.
The visible position makes every displayed Recent action distinct even when
both visible title and subtitle repeat. Raw IDs must not enter the label.

The causal RED is the preserved Attempt 022 AX audit plus the focused test's
compile failure against the old title-only helper signature. Focused GREEN must
prove hostile content remains sanitized and bounded, the position fallback is
one-based, the subtitle is announced, and duplicate title/status rows receive
distinct labels. A fresh signed-Release AX audit must prove all displayed Recent
actions have unique, non-empty human labels and help while separately
classifying native window chrome.

Repair 19 changes no action closure, selection behavior, visual layout, IPC,
Event, authority, execution trigger, retry, persistence, filesystem access,
Provider, credential, or model behavior. An independent review may authorize
carrying Attempt 022 J1-J8 authority evidence only after proving the exact
source delta is confined to this label helper and its regression test. In that
case, a replacement J9/J10 run against a newly locked and signed Repair 19
Release is still mandatory. Otherwise a complete replacement J1-J10 attempt is
required. Phase 2C and ADR-0015 remain unaccepted.

## 28. Repair 20 - Concurrent Recovery Loser Classification

Repair 19 passed exact-byte review and replacement source-lock review. Its
first complete lock-bound Go normal matrix then failed only in
`TestPhase1EngineeringDemoApprovalRestartReconnectAndRecovery`. One of two
competing recovery coordinators returned `ErrTeamExecutionIncomplete`, which
the test helper rejected even though the authoritative winner completed. The
matrix finished all other packages and failed in `753.73s`. The exact test then
passed 100 consecutive repetitions in `167.398s`, confirming a low-probability
scheduling window rather than permission to ignore the frozen failure.

`TeamCoordinator.Run` intentionally returns `ErrTeamExecutionIncomplete` when
its final authoritative Team record is nonterminal. During the competing
recovery test, a loser may rebuild after the winner has acquired the only
ready attempt but before the winner commits its terminal fact. That loser has
no ready node, executes no node, and returns the incomplete result. The helper
already accepts several authoritative conflict forms but omitted this bounded
non-dispatching outcome.

Repair 20 admits only
`internal/app/phase1_engineering_demo_test.go` plus this contract, Candidate,
current status, review, replacement lock, and append-only evidence. A shared
test helper must classify existing conflict errors as expected losers and may
classify `ErrTeamExecutionIncomplete` only when `ExecutedNodeIDs()` is empty.
It must reject an incomplete result that executed any node and every unrelated
error. `runCompetingDemoRecovery` must continue to require exactly one
successful coordinator that executed only `main`; its existing final Event,
source/verifier/effect call-count, idempotent restart, and evidence assertions
remain unchanged.

Causal RED is the preserved Repair 19 full-matrix failure. A deterministic
focused RED must require the closed loser-classification table before its helper
exists. GREEN must prove every accepted and rejected class, the exact scenario
at least 100 consecutive times, complete `internal/app` normal/race, independent
contract/source/status reviews, a replacement source lock, and the complete
fresh matrix. Repair 20 changes no production executable, coordinator behavior,
Event, authority, dispatch, retry, timeout, IPC, persistence, filesystem,
Provider, credential, model, or UI behavior. Phase 2C and ADR-0015 remain
unaccepted.
