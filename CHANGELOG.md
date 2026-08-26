# Changelog

All notable changes to Loom are documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Loom uses semantic release labels, while the macOS bundle also carries an
incrementing build number.

## [Unreleased]

No changes yet.

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

[Unreleased]: https://github.com/tttboy123/loom/compare/v0.5.3-rc.1...HEAD
[0.5.3-rc.1]: https://github.com/tttboy123/loom/compare/v0.5.2...v0.5.3-rc.1
[0.5.2]: https://github.com/tttboy123/loom/releases/tag/v0.5.2
