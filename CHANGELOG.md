# Changelog

All notable changes to Loom are documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Loom uses semantic release labels, while the macOS bundle also carries an
incrementing build number.

## [Unreleased]

### Added

- An installed, paid-call Phase 7 acceptance matrix for exact model-selected
  Loom Tools, Proposal decisions, expiry, replay rejection, RoundTable
  interventions, App restart restoration and a final privacy scan. Build 324
  passes this matrix for Codex, OpenCode/DeepSeek, Pi and Loom Native/MiniMax.
- Action-specific synthetic RoundTable fixtures for Pause, Steer, Retry, Skip
  and Replace. Each fixture uses current state, is `p7-` prefixed and concludes
  after the Proposal is cancelled.

### Changed

- Loom `0.5.6` Build 324 is the current installed acceptance. Its private UDS
  inventory retains 24 Providers, seven Runtimes and six executable
  Conversation Profiles; the Pi local Route is now executable.
- Installed governance acceptance requires one unambiguous P7 Mission or exact
  Mission/Team IDs. It no longer selects the first fuzzy historical match.
- Claude Code installed login and paid live use are explicitly N/A for this
  user. The App continues to discover the Runtime without publishing an
  executable Profile, and source parity plus fail-closed admission remain
  required.

### Fixed

- OpenAI-compatible and Anthropic-style tool loops now repair malformed tool
  arguments within a strict bound, force a fresh terminal selection after
  exhaustion and reject a Tool omitted from the current least-privilege round.
- A transient metadata Gateway outage receives one bounded retry only for
  read effects. Proposal effects remain non-retrying.
- The installed RoundTable matrix no longer asks a concluded historical
  Session to prepare Pause or other current-state interventions.
- Partial synthetic Session creation registers cleanup immediately, and exact
  single-Tool turns reject any unrequested read Tool instead of counting it as
  a pass.
- Codex native auth now follows the official App Server `account/read`
  semantics instead of treating `requiresOpenaiAuth=true` as proof that an
  authenticated account is logged out.
- Codex and OpenCode Provider subprocesses now use canonical owner-only
  per-call temporary directories, clean them up on every exit path and repair
  validated legacy scratch trees to directories `0700` and files `0600`.

### Security

- The exact official llama.cpp b10107 archive is now the Pi Runtime trust root.
  Loom derives and freezes its complete file tree, including sibling dynamic
  libraries, and rejects extra files, changed dependencies, unsafe archive
  paths or links, symlinks, hard links, ownership/mode drift and identity
  replacement before Route publication or process start.
- MiniMax and other models may repeat a Tool name remembered from prompt or
  history even when it is omitted from the current wire inventory. Loom keeps
  runtime allowlist validation authoritative, so remembered names cannot
  expand the frozen Tool grant.
- Installed diagnostics and evidence record only non-secret identity, stage,
  result and Tool IDs. They exclude Prompt, transcript, Provider body,
  Authorization values, credentials and hidden reasoning.
- The final installed privacy gate now proves subprocess scratch roots are
  owner-only after restart and that the content-only acceptance marker does not
  appear in bounded state or diagnostic storage.

### Known Limitations

- Claude Code has no executable Conversation Profile on this machine, so its
  installed login and paid live call are N/A under the user's explicit waiver.
  The matrix rejects that waiver if a Claude Profile appears; it is not counted
  as a pass or silently replaced by another Runtime.
- A four-real-Provider Team, live revoke/rate-limit isolation against user
  accounts and a newly imported custom-endpoint Conversation remain explicitly
  deferred to a later Phase.

## [0.5.6] - 2026-08-29

Installed candidate Build 264. Source, deterministic package, transactional
install and two local cold-start snapshots pass; the locked Pi model asset and
real-model acceptance remain pending. Build 263 is retained for rollback.

### Added

- A 28-tool, versioned Loom Harness Registry: 13 bounded metadata reads and 15
  governed mutation previews spanning Conversation/Session, Mission, Agent
  Team, RoundTable, Route/Model/Reasoning, Workspace, Provider, Runtime,
  diagnostics, Needs You and Library state.
- Cross-Runtime control-tool adapters for Codex, OpenCode, Claude Code, Pi and
  Loom Native with the same frozen Registry and authority boundary.
- A typed Conversation Action Proposal channel and one compact macOS review
  experience for Session alignment plus 14 Mission, Team, RoundTable, Route,
  model, reasoning and workspace actions.
- Bounded product metadata projections for model-selected read tools, including
  Incident lookup, without exposing transcripts or writable state authority.
- A fail-closed installed acceptance runner for the five Runtime adapters. It
  requires separate paid-call, App-restart and exact-build authorization gates,
  uses only synthetic Mission Proposals, never changes credentials, and checks
  confirmation, replay rejection, cancellation, expiry and restart recovery.
- Immutable decision receipts for confirm, cancel, expiry and supersede. Each
  receipt binds the exact Proposal digest, decision time and Incident ID; the
  macOS review displays the copyable Incident ID.

### Changed

- Ordinary language can now lead the selected Conversation model to choose a
  matching Loom tool from its schema instead of relying on fixed trigger
  phrases. Slash commands remain an explicit discovery shortcut.
- Proposal schema v2 freezes the exact Route, Workspace, Registry digest,
  Segment, Attempt, Incident, target content, expiry and Proposal digest.
- Confirmed action proposals open the existing governed review with the exact
  Mission, Team or RoundTable target; they do not create execution state by
  themselves.
- Stop remains an immediate local user control and is deliberately absent from
  the model-facing tool registry.

### Fixed

- Confirmation now rejects Workspace drift in addition to Route, content,
  Segment, Attempt and Registry drift.
- Pi is now published as a Conversation Route only when both its Runtime and
  its integrity-checked local model backend are executable. The managed App
  discovers the existing locked local model automatically, and the macOS model
  picker preserves its exact Qwen model instead of exposing an empty Route.
- Pi now pins both the official `llama.cpp b10107` executable SHA-256 and the
  Qwen GGUF SHA-256 before publishing or starting the local Route; changing
  either file fails closed.
- Real subprocess contract tests no longer confuse repository scheduling
  pressure with JSONL, Runtime metadata or process-group cleanup failures;
  production timeouts remain unchanged.
- Codex login and Pi Tool cancellation fixtures now separate a bounded
  high-load process-start window from their still-short cancellation and
  resource-reaping deadlines.
- Runtime surfaces now distinguish a detected Pi Harness from an executable Pi
  Conversation Route. When the locked local model is absent, the inspector
  says `Local model required for Conversation` instead of showing only generic
  online capacity.
- Second and later Turns in one immutable Segment now validate the current
  Attempt's Context Capsule digest while retaining the Segment-opening
  Execution Binding. Legitimate continuation no longer fails on Capsule drift.
- Historical confirmed Proposals created before decision receipts remain
  visible but read-only instead of regaining execution authority after restart.

### Security

- The model has no confirm, cancel, execute, credential, policy, Journal,
  StateWriter or terminal Run/Attempt tool. Expiry, replay, Registry
  substitution, Workspace/Route drift, unknown fields, excess tool calls,
  credential-shaped arguments and tool/action mismatch fail closed.
- Metadata reads require the current frozen turn and Incident identity and
  return bounded non-secret projections. Prompt, transcript, Provider response,
  credential and hidden reasoning stay outside the tool plane and diagnostics.
- Legacy v1 Proposals remain readable and cancellable for migration but cannot
  be confirmed or executed.
- Daemon `chat_control_decision` diagnostics contain only bounded stage,
  outcome and Incident metadata. They exclude Conversation text, Prompt,
  Provider response and credentials.
- The final installed live runner now freezes the exact build plus App and
  daemon SHA-256, verifies strict signing before and after restart, retries
  asynchronous private-Socket startup and rejects ambiguous daemon identity.

## [0.5.5] - 2026-08-28

### Added

- A versioned Loom Control Tool registry that lets the Conversation model
  discover typed, capability-scoped product tools instead of relying only on
  fixed natural-language phrases.
- A private, Segment-scoped Codex MCP adapter with metadata-only Conversation
  search and a governed Conversation-alignment preview tool.
- An in-conversation alignment review showing exact source titles, context
  scope, status and separate Confirm/Cancel actions.

### Changed

- Confirmed Conversation alignment now keeps one visible Conversation while
  creating a new immutable Segment on the user's next message. The Segment and
  Harness Session freeze the alignment receipt digest.
- Summary-only alignment imports authoritative user goals, constraints and
  decisions through Loom's Context Capsule rules without trusting prior model
  output as system history.

### Fixed

- The Runtime inspector now identifies Loom Native DeepSeek, Kimi and MiniMax
  instances separately. Existing exact runtime records migrate in place instead
  of rendering three indistinguishable `Loom Native` rows.

### Security

- Model-visible tools can prepare a Proposal but cannot confirm, cancel or
  execute it. Confirmation remains a user-owned, digest-bound local IPC action
  with expiry, drift detection and replay rejection.
- The model receives a bounded Conversation metadata catalog, not transcripts.
  MCP tokens remain turn-scoped child-environment values and never enter argv,
  Prompt, Journal, diagnostics or evidence.

## [0.5.4] - 2026-08-28

Installed acceptance is Loom `0.5.4` Build 257. This release adds the Phase 6
Conversation unified action entry and includes the accepted Phase 5 Mission and
RoundTable experience developed after `0.5.3-rc.1`.

### Added

- A searchable 18-action command catalog in the Conversation composer. Type
  `/` or choose Commands, then filter and select with mouse or keyboard.
- Typed actions for New Task, Mission, Missions, Agent Team, RoundTable,
  Continue Mission, Route, Model, Reasoning, Status, Diagnostics, Stop, Needs
  You, Library, Runtime & Providers, Folder and Copy Conversation.
- Conservative Chinese and English natural-language intent routing through the
  same action catalog, without treating questions or incidental nouns as
  actions.
- Contextual unavailable reasons for actions such as Stop, Continue Mission,
  Route, Model and Copy Conversation.
- Mission-linked RoundTables with two to six independently bound Agent seats,
  real Agent Attempts and visible contribution streams.
- Governed Pause, Steer, Retry, Skip and Replace controls for active or failed
  RoundTable seats, including seat-local Incident details and restart recovery.
- Moderator conclusion candidates that require explicit user acceptance before
  an Alignment Summary is bound back to the Mission.
- Local, bounded and digest-verified RoundTable export/import with expiry and
  correlation-aware audit facts.

### Changed

- Conversation remains ordinary chat by default; Slash commands are an
  optional discovery layer rather than a second interaction mode.
- Mission and Team commands now prepare bounded, prefilled review drafts.
  RoundTable opens governed Mission context, and Route transitions retain
  disclosure and trust-boundary review.
- The macOS workspace now keeps Conversation as the stable center while Mission
  and RoundTable open as progressive governance context beside it.
- Mission RoundTable seating now uses Lead and Participant roles instead of the
  legacy two-seat Writer/Target vocabulary.
- The standalone RoundTable entry now routes users into Mission selection or
  imports a concluded record; it no longer creates an unbound legacy ledger.
- A failed Agent seat no longer hides successful peer output or blocks the user
  without an intervention surface.
- RoundTable now distinguishes the first result round from follow-up synthesis.
  The Lead receives an explicit synthesis responsibility, Participants receive
  an independent critique responsibility, and acceptance names the exact Lead
  candidate being governed.
- `Command-N`, the navigation rail and both Conversation header actions now use
  one New Task path: they return to Conversation, close an unpinned governance
  inspector, create an isolated task and restore composer focus.
- External Codex, Claude Code and OpenCode discovery now starts only after the
  local Conversation service is ready. Mission preparation reuses that single
  refresh instead of probing the same Harnesses again.
- Agent Runtime restart recovery now filters terminal Attempt history before
  expensive exact-Run replay while still validating every frozen binding.
  Multiple delivered tool results under one frozen Attempt reuse the same
  exact authority validation within that reconciliation pass.
- RoundTable's Conversation-side inspector now keeps each Agent's identity,
  frozen route and status scannable before long result detail. Results over the
  compact boundary expose accessible `Show full result` and `Show less` actions
  instead of silently discarding the beginning of the result.
- Compact RoundTable previews now parse the complete inline Markdown result
  before taking a bounded conclusion-and-latest-action summary. Preview cuts
  stay on complete rendered words and never expose partial Markdown syntax.
- Mission preflight and start failures now preserve one correlation identity,
  safe stage, retryability, recovery action and Incident ID through the App and
  daemon instead of collapsing to a generic unavailable reason.

### Fixed

- Publishing diagnostics preview data and its exporter through one atomic
  presentation state prevents an empty white sheet during fast local preview.
- Return now submits a selected local command without the text field's native
  edit cycle immediately clearing the resulting action feedback.
- Unknown commands preserve their draft and show `Command not found`; locally
  rejected credential-shaped arguments remain visible for correction.
- RoundTable follow-up Attempts now receive prior Agent results as bounded,
  provenance-bound untrusted Context Capsule items instead of receiving only
  the original discussion question.
- Mission-linked RoundTable Attempts now receive the authoritative Mission ID,
  objective, Team Instance and plan digest without copying Mission content into
  the operational Journal.
- MiniMax Agent responses now use the same 30 KiB result boundary as the
  RoundTable payload contract, avoiding successful Provider responses being
  replaced by a generic empty-result placeholder.
- Agent Provider failures now preserve privacy-safe HTTP status, Provider error
  code, retry delay and exact failure stage through operational diagnostics.
- A real failed RoundTable Agent can now be retried with bounded user guidance,
  while successful peer output remains visible; the accepted conclusion and
  both Agent results restore after an App and daemon restart.
- Opening a persisted RoundTable during cold startup now stays on the selected
  Mission, presents Agent Team preparation as a recoverable state and restores
  the discussion automatically. Temporary local connection failures and
  timeouts no longer leak a raw Swift `error 4` or require a second navigation.
- Installed Build 242 reduced the observed Agent Runtime startup aggregate from
  Build 241's 39.9/56.3 second samples to 18.1 and 12.7 seconds. The remaining
  post-catalog authority/history work measured about 11.9 and 8.8 seconds while
  Conversation stayed usable and the persisted RoundTable restored without
  user intervention.

- RoundTable Retry now restores the original authoritative discussion prompt
  from the prior encrypted Context Capsule and adds retry guidance as a separate
  role-scoped confirmed constraint. It no longer substitutes the guidance for
  the question being retried.
- Concluding a RoundTable now returns the same projected Agent outputs as a
  Snapshot, avoiding a temporary blank result until the next refresh.
- Full RoundTable details now preserve the safe failure stage, recovery action
  and copyable Incident ID already shown beside the Conversation.
- RoundTable terminal state now takes precedence over stale intervention history,
  so an accepted Mission discussion shows `Discussion concluded` instead of the
  contradictory `Input needed` status.
- RoundTable session statistics now distinguish participating Agent seats from
  the system Moderator instead of presenting two conflicting seat counts.
- RoundTable live Steer controls now follow the exact running Attempt capability
  projected by the daemon, including a pending state, instead of inferring support
  from the Harness name.
- RoundTable import now rejects top-level and embedded Summary provenance
  mismatches even when a packet digest is recomputed, and the macOS reader stops
  at the one-megabyte boundary without loading an oversized file in full.
- The daemon now enforces the same 4,096-byte guidance limit for RoundTable Steer
  as the App, Open Round and Retry paths.
- Preserved Runtime live-input capability through the Mission adapter wrapper,
  allowing Loom Native RoundTable Steer to reach and continue the exact running
  Attempt instead of failing against a one-step budget.
- Return an immediate, actionable `capability_gap` for Runtimes such as
  OpenCode that cannot accept live guidance, and present Retry after completion
  instead of a Steer action that waits until timeout.
- Cancelled any already-admitted seat Attempts when a later seat fails Round
  admission, preventing orphaned `running` state before Provider dispatch.
- Made repeated Replace requests idempotent at the product route boundary.
- Preserved the export operation Correlation ID through the Journal audit fact.
- RoundTable operations now keep one request Correlation ID through local
  validation, UDS transport and daemon failures, and present the safe stage,
  retryability, recovery action and Incident ID in both RoundTable surfaces.
- Diagnostic preview and workspace-folder chooser failures no longer disappear
  into silent navigation or dismissal; both remain visible with an in-place
  retry or an explicit setup recovery action.
- Mission diagnostics now open over the existing review flow, retain the draft
  and avoid showing a second raw internal failure reason beside the governed
  recovery state.
- Rapid App restart now monitors a briefly adopted daemon Socket and starts a
  new bundled daemon if the previous App's managed service exits, preventing an
  indefinite `Starting local service` state.

### Security

- Unknown Slash commands and rejected credential-shaped arguments never enter
  Conversation dispatch or Provider requests.
- Governed actions resolve to typed local requests and existing review or
  confirmation surfaces; model text does not gain execution authority.
- Command feedback and diagnostics exclude API keys, Authorization headers,
  Prompts, transcripts, Provider response bodies and hidden reasoning.

## [0.5.3-rc.1] - 2026-08-26

Release candidate based on Loom `0.5.3` Build 188. This release aligns the
Conversation-first App with the accepted Phase 2D multi-Provider Agent Team
architecture and the subsequent Mission usability work.

### Added

- Conversation Profiles with independent Harness, Provider Account, credential
  revision, model and reasoning selection.
- Immutable Conversation Route Segments, governed Route Transitions and
  Context Capsules with disclosure modes, capacity accounting and omissions.
- Loom-owned Credential Vault with envelope encryption and short-lived exact
  credential leases for Conversation and Agent dispatch.
- Independent per-Agent Execution Bindings, Role Context Capsules, explicit
  fallback decisions and Provider Account accounting.
- Harness Gateway and Segment Sessions for Codex, Claude Code, OpenCode, Pi and
  Loom Native, including response-scoped cancellation and bounded cleanup.
- Conversation-linked Missions with titled workflow rooms, Team orchestration,
  live per-Agent activity, result publication and local artifact opening.
- A fixed `Continue this Mission` composer for Blocked, Failed and Cancelled
  Missions. Continuation creates a new reviewed Attempt without rewriting the
  previous result or Incident.
- Actionable operational diagnostics with safe stages, retryability, recovery
  actions and Incident IDs.
- Governed RoundTable sessions, session resume and drag-and-drop seating.
- OpenCode runtime and dynamic model discovery without projecting OpenCode as a
  duplicate model Provider.

### Changed

- The macOS App is now Conversation-first. Mission and Team governance remain
  available without forcing users through setup boards before chatting.
- The App owns the bundled daemon lifecycle; users no longer start `loomd`
  manually.
- Mission is presented as one titled workflow related to a Conversation, rather
  than as every project object flattened into one board.
- Mission Room now displays each Agent's frozen Harness, Provider Account,
  model, status and bounded visible output while work is running.
- Provider Directory, Runtime inventory and executable Route selection are
  separate projections. Composite route labels no longer become Provider rows.
- Provider/model switching uses a reviewed Segment transition instead of
  mutating the existing thread's Profile.
- Credentials use Loom Vault during normal execution. macOS Keychain remains an
  explicit one-time migration source only.
- User-facing failures now prefer a specific stage and recovery action over the
  generic `Unavailable` state.

### Fixed

- Prevented malformed CC Switch import candidates from making the complete
  Setup snapshot fail strict decoding and hiding all Providers and Runtimes.
- Moved bounded, non-blocking daemon Socket health probes off the main actor to
  prevent App startup and polling from hanging.
- Corrected the installed daemon bootstrap identity so the Credential helper
  sees canonical state, isolation and Socket arguments without weakening
  attestation.
- Preserved credential failure stages across setup and IPC instead of reducing
  helper authorization and Vault errors to `daemon_admission`.
- Normalized pasted credential boundary whitespace without changing bytes
  inside the secret, including the trailing-newline case.
- Fixed stale Conversation anchors and asynchronous load races when switching
  from an existing Codex Segment to another Provider Profile.
- Removed silent Conversation send failures and retained the draft with an
  actionable Incident state.
- Prevented `opencode-deepseek` and `opencode-minimax` route combinations from
  appearing as duplicate Providers.
- Prevented OpenCode Agent prompts and governed Mission context from entering
  child-process arguments.
- Reconciled stale Mission projections, surfaced blocked reasons and preserved
  successful peer Agents when another Agent fails.
- Preserved Mission activity during cursor races and exposed visible Agent
  output before the final Mission result is available.
- Added a direct recovery path when a Mission is blocked instead of leaving the
  user without an input surface.
- Made controlled Mission projection readers return independent nested
  snapshots, matching the production read-model contract and removing a race
  between continuation dispatch and start-lineage inspection.

### Security

- Provider secrets remain outside source, process arguments, global
  environment, Conversation, Prompt, Team, Journal, Evidence and diagnostics.
- Credential, Conversation and export encryption use separate key domains;
  integrity, identity and revision drift fail closed.
- Runtime dispatch resolves credentials by Provider Account, opaque reference
  and exact revision rather than by Provider ID alone.
- Diagnostic export remains user initiated, privacy safe and content free by
  default.
- External tool use is governed by scoped capabilities; OpenCode Conversation
  routes deny tools and sharing, while Agent routes receive only explicit Loom
  context capabilities.

### Known Limitations

- The installed four-Agent matrix using real Codex/OpenAI, Claude/Anthropic,
  Loom/Kimi and Loom/MiniMax accounts is deferred to a later Phase.
- Real-account revoke and rate-limit isolation remains narrower installed and
  controlled Failure Lab evidence, not a completed external-account matrix.
- Custom compatible endpoints have source contracts but not a completed
  user-approved import-to-real-Conversation live loop.
- Tentative live Agent output is bounded in the App and is not reconstructed
  after restart unless it became accepted terminal Evidence.
- Provider catalog presence does not guarantee that a credential, compatible
  Harness or live route is available on the current machine.

## [0.5.2] - 2026-08-16

### Added

- Initial installed Phase 2D multi-Provider Agent Team acceptance boundary.
- Installed G1-G6 evidence for App launch, real Conversation, mixed-Team
  execution, controlled failure isolation, diagnostics and accounting UI.

### Changed

- Advanced the product from the earlier Phase 2A Mission preview to the
  Conversation and Provider foundations used by Phase 2D.

[Unreleased]: https://github.com/tttboy123/loom/compare/v0.5.6...HEAD
[0.5.6]: https://github.com/tttboy123/loom/compare/v0.5.5...v0.5.6
[0.5.5]: https://github.com/tttboy123/loom/compare/v0.5.4...v0.5.5
[0.5.4]: https://github.com/tttboy123/loom/compare/v0.5.3-rc.1...v0.5.4
[0.5.3-rc.1]: https://github.com/tttboy123/loom/compare/v0.5.2...v0.5.3-rc.1
[0.5.2]: https://github.com/tttboy123/loom/releases/tag/v0.5.2
