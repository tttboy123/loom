
 # Phase 2C — Chat-First Client Experience Plan
 
 **Status**: proposed — pending Product Owner review and Exit Contract freeze  
 **Date**: 2026-08-06  
 **Author**: Loom Client Experience Audit follow-up  
 **Scope**: native macOS app (`apps/macos/`), Bubble Tea TUI (`internal/tui/`), shared daemon IPC, cross-client journey parity  
 **Baseline**: accepted v0.4.1 whole-slice (`7e24ec29`), branch `codex/loom-platform-slice2`  
 **Audit input**: `.loom-evidence/phase2c/CLIENT-EXPERIENCE-AUDIT.md` (2026-08-03)  
 **Design input**: `docs/ui/visual-direction-options.md` (2026-08-05)
 
 ---
 
 ## 1. Phase Goal
 
 Turn Loom's default surface from a **Board-first operations dashboard** into a **Chat-first pair-programming companion** where ordinary work starts with a sentence, and Agent-team governance is an **optional, always-available side panel** rather than a prerequisite.
 
 Outcome: a new or returning user can open Loom, type a task, optionally pick a folder, and continue in a conversation. The Mission Board, Team roster, Runtime health, Decisions, and Evidence live in a right-hand governance panel that can be expanded, collapsed, or pinned. The left sidebar holds navigation, recent work, Teams, Runtimes, Skills, and Library. The center is the conversation timeline plus a composer.
 
 This does not weaken any existing authority boundary: ordinary chat still **does not** create a Team, Mission, or Run. Only an explicit user action (e.g., “use Agent Team,” select a Team, confirm a Draft) creates a Journal fact.
 
 ---
 
 ## 2. Product Decisions Already Accepted
 
 From `PRODUCT-PLAN.md` and prior conversation:
 
 1. **Chat is the default entry.** Ordinary tasks do not auto-create a Team.  
 2. **Agent mode is explicit.** Only user intent (button, command, explicit request) triggers Team Draft / Team load.  
 3. **Chat-first and governance are not mutually exclusive.** The conversation is the primary lane; the Mission Board / Team / Timeline / Evidence are the secondary governance panel.  
 4. **No-folder and Open Folder are both valid first-class journeys.** Git is optional; typing a path is never the only route.  
 5. **Offline means “cannot connect to the local Loom daemon,” not “no internet.”** It is distinct from fatal (connected but rejected).  
 
 ---
 
 ## 3. Proposed ADR-0015: Chat-First Client Shell with Governance Panels
 
 ### Context
 
 ADR-0011 (TUI-first) and ADR-0012 (native app host) established the one-authority, daemon-IPC client boundary. Phase 2A/2B proved the technical substrate. The current installed v0.2.0 product, however, opens directly into a five-lane Mission Board, which contradicts the accepted product principle that ordinary chat is the default entry. A dedicated ADR is needed to capture the new client information architecture without changing state authority or execution policy.
 
 ### Decision
 
 Loom's primary client surface is a **chat-first three-pane shell**:
 
 ```text
 +---------------------------------------------------+
 |  Left Rail    |  Center Conversation   |  Right  |
 |  (Navigation, |  (Timeline + Composer)   |  (Team/ |
 |   Recents,    |                         |  Mission|
 |   Teams,      |                         |  /Deci- |
 |   Runtimes,   |                         |  sions/ |
 |   Skills,     |                         |  Eviden-|
 |   Library)    |                         |  ce)    |
 +---------------------------------------------------+
 ```
 
 Binding rules:
 
 1. **Chat is the default empty-state surface.** Fresh launch shows a blank composer + bounded recent work. No Mission Board is forced.  
 2. **Left rail is navigation and library.** It blends into the canvas, shows active state, and never masks individual actions with a generic accessibility label.  
 3. **Center is the conversation timeline.** It supports plain chat, task context, agent messages, tentative output, and milestone cards.  
 4. **Right panel is governance.** It can display Mission Board, Team Topology, Timeline, Decisions, Evidence, Runtime health, and Attention. It is collapsible and can be opened contextually when an explicit Agent journey starts.  
 5. **No state authority in the client.** Clients hold only replaceable copied view state. Team creation, Run start, approval, and Evidence acceptance remain daemon-authorized Journal facts.  
 6. **Cross-client parity.** Every user-facing journey must work in both the Bubble Tea TUI and the native macOS app, using the same daemon IPC methods and the same deterministic fixture.  
 7. **Semantic visual system.** All surfaces use a shared token system (canvas, rail, surface, raised, text-primary/secondary/muted, accent, success, warning, danger, offline). Platform-native frameworks implement the tokens; ad-hoc system colors are not mixed.  
 8. **Truthful connection states.** `connecting`, `offline`, `reconnecting`, `stale`, and `fatal` are distinct and carry an actionable recovery path.  
 9. **Accessibility and input.** Every navigation action has a unique accessible name; keyboard traversal, focus visibility, Escape, text scaling, and reduced-motion are verified.  
 10. **No automatic execution from chat.** Ordinary conversation never creates a Team Draft or Mission. A clear, intentional trigger is required.
 
 ### Alternatives
 
 #### A. Keep the Board-first layout
 - **Pros**: Reuses existing `MissionWorkbench` and recent Multica-style styling.  
 - **Cons**: Contradicts the accepted product principle; audit found it the largest P0 gap.  
 - **Why not**: Rejected by Product Owner and audit.
 
 #### B. Chat-first with a separate full-screen Board
 - **Pros**: Simpler panel layout.  
 - **Cons**: Forces context switch between conversation and governance.  
 - **Why not**: User explicitly wants side-by-side operation, referencing Codex’s split-pane design.
 
 #### C. Web-first UI
 - **Pros**: Familiar layout, easier cross-platform.  
 - **Cons**: Violates Phase 2A/2B local-first, no-public-network boundary; adds a second stack before native parity is proven.  
 - **Why not**: Web remains explicitly outside the current Phase. The native SwiftUI and TUI clients stay authoritative.
 
 ### Consequences
 
 - **Positive**: matches user mental model; reduces first-run friction; governance surfaces remain reachable without blocking task entry.  
 - **Negative**: large client refactor; must preserve v0.4.1 daemon/authority boundaries; native SwiftUI and TUI both need new screens.  
 - **Risks**: chat composer can become a second command authority; mitigated by only routing explicit typed commands through daemon IPC and never creating resources from plain text.
 
 ---
 
 ## 4. WorkItem Decomposition
 
 This plan proposes **three vertical WorkItems** reviewed under a single Phase 2C Exit Contract. They are intentionally sequential in implementation risk but are accepted as one coherent client surface. If the combined Contract Review finds them too tightly coupled or thin, they shall be consolidated into a single `P2C-W1` with the three sub-deliveries below.
 
 | ID | Title | Core question | Vertical journey |
 |---|---|---|---|
 | **P2C-W1** | Workspace Shell & Entry | Where does the user start? | Fresh launch → workspace selection / no-folder → offline recovery → navigation rail. |
 | **P2C-W2** | Chat-First Conversation | How does the user state and follow work? | Composer → message timeline → explicit Agent intent → task context cards → tentative output. |
 | **P2C-W3** | Governance Side Panel | How does the user inspect and steer Agent teams? | Toggle/open panel → Mission Board / Team / Timeline / Decisions / Evidence / Runtime health → approval actions. |
 
 ---
 
 ## 5. P2C-W1 — Workspace Shell & Entry
 
 ### Contract
 Replace the Board-first launch with a task-first workspace shell. Provide a clear, non-blocking path to open a folder, continue without a folder, or pick a recent workspace. Establish the visual token system and the three-pane frame.
 
 ### Required interaction behavior
 1. Fresh launch shows a blank composer as the central surface, bounded recent work below it, and a prominent `Open Folder…` affordance.  
 2. `Open Folder…` is in the File menu and in the welcome/chrome; desktop uses `fileImporter`/`NSOpenPanel`; TUI uses a bounded directory chooser.  
 3. A no-folder knowledge or chat task is a valid first-class journey.  
 4. Folder selection is read-only until an explicit preflight grants scoped access.  
 5. Recent folders store only a safe display handle + daemon-authorized workspace ID; no credentials or raw paths.  
 6. The left rail shows navigation: Recents, Teams, Runtimes, Skills, Library, plus a clearly selected active item.  
 7. Connection states are distinct: `connecting` (with cancellation), `offline` (retry / open setup), `reconnecting` (preserve view), `fatal` (daemon rejected).  
 8. The visual token system is implemented and enforced: canvas, rail, surface, raised, text-primary/secondary/muted, accent, success, warning, danger, offline, border, shadow.  
 9. macOS uses Option C (Linear Governance floating panels) adapted to SwiftUI; TUI uses Option A (Warm Dark palette) with semantic ANSI fallbacks.  
 10. No persistent client cache, no second Journal, no SQLite reads from the client.
 
 ### Acceptance criteria
 - `swift test` and `go test ./internal/tui/...` pass.  
 - A fresh native app launch screenshot shows composer-first, not Board-first.  
 - Real PTY TUI transcript shows the same welcome/composer journey.  
 - Accessibility tree identifies every rail action uniquely (not a generic “Mission navigation”).  
 - Contrast automation proves normal text ≥ 4.5:1, large text/UI ≥ 3:1.  
 - Offline state shows an actionable recovery path, not an indefinite “Connecting…” loader.
 
 ### Verification
 - Deterministic matrix: Go full/race/vet/tidy/gofmt; Swift full/TSAN/Release.  
 - Cross-client journey: fresh launch → no-folder → open folder → offline → reconnect.  
 - Screenshot matrix: light/dark/compact.  
 - Independent implementation review (P0/P1/P2).  
 - Whole-WorkItem review.
 
 ---
 
 ## 6. P2C-W2 — Chat-First Conversation
 
 ### Contract
 Implement the center conversation surface: a composer, message timeline, and explicit transition from ordinary chat to Agent mode. The conversation must not become a command authority; it surfaces daemon-provided messages and records user intent.
 
 ### Required interaction behavior
 1. The composer is always visible and focused by default; it supports multi-line input and a clear “send” action.  
 2. The timeline shows user messages, Loom responses, tentative output, milestones, and error/recovery cards.  
 3. Ordinary messages stay in chat mode; no Team, Mission, or Run is created.  
 4. An explicit trigger transitions to Agent mode: e.g., user clicks “Use Agent Team,” selects a Team from the left rail, or types a recognized explicit command that the daemon confirms with a dialog.  
 5. In Agent mode, the right governance panel auto-opens (or remains open) and shows the Team Draft / Mission / Timeline.  
 6. Main Agent messages in the center are clearly labeled as proposals/drafts; structured fields (Team members, Runtime, budget, tasks) are rendered as cards.  
 7. User confirmation is required before any Journal fact is created; the confirmation surface is a modal or pinned decision card.  
 8. Streaming/tentative output is visibly marked and does not overwrite authoritative state until the daemon commits it.  
 9. The conversation history is bounded; long-running tasks collapse older milestones.  
 10. TUI uses a full-width composer band with ANSI semantic colors; macOS uses the panel design system.
 
 ### Acceptance criteria
 - A plain chat exchange creates no Mission or Team facts in the Journal.  
 - An explicit “use Agent” flow creates a confirmed Team Draft / Mission after user confirmation.  
 - TUI and native app render the same message timeline from the same daemon fixture.  
 - Composer handles empty/whitespace input gracefully.  
 - Keyboard shortcut for send and cancel are discoverable and consistent across clients.  
 - Accessibility labels distinguish user messages, Loom messages, proposals, and confirmations.
 
 ### Verification
 - Cross-client journey: plain chat → explicit Agent trigger → Team Draft proposal → user confirm → Mission starts → center shows milestones → right panel shows Board.  
 - Deterministic matrix.  
 - RED tests proving ordinary chat does not create Team/Mission facts.  
 - RED tests proving explicit Agent trigger requires confirmation.  
 - Independent implementation and whole-WorkItem reviews.
 
 ---
 
 ## 7. P2C-W3 — Governance Side Panel
 
 ### Contract
 Refactor the existing Mission Board and related surfaces into a collapsible, context-aware right governance panel. It must reuse the improved Multica/Linear visual language already started in `MissionWorkbench.swift` but be reachable from the chat-first surface rather than replacing it.
 
 ### Required interaction behavior
 1. The right panel can be opened, collapsed, and pinned from the left rail or from an active Agent task.  
 2. Panel views include: Mission Board (lanes), Team Topology, Timeline, Decisions, Evidence, Runtime/Provider health, Attention.  
 3. The Mission Board is a **secondary** view, not the default first screen.  
 4. Board lanes are widened and use the improved card styling (status pill, priority, progress, attention indicator).  
 5. Static tabs (`Board`, `Topology`, `Timeline`, `Capacity`) are either fully interactive or removed; no false affordances.  
 6. `+ New Mission` is available from the left rail and from the composer area, but it does not block simple chat entry.  
 7. Team, Runtime, and Provider management surfaces are reachable from the left rail or panel switcher, not from the center conversation.  
 8. Decisions and Evidence are first-class views in the panel, not buried inside mission cards.  
 9. The panel preserves the improved visual tokens from P2C-W1 and does not reintroduce the old `LoomGraphite` mixing.  
 10. The panel is replaceable view state; collapsing or switching views does not cancel or mutate running work.
 
 ### Acceptance criteria
 - The same Mission Board rendered as the first screen in v0.2.0 is now only reachable through the panel.  
 - All panel views are reachable via keyboard and have unique accessibility labels.  
 - Panel state (open/collapsed/selected view) is restored after reconnect but is not authoritative.  
 - `Board`, `Topology`, `Timeline`, `Capacity` tabs are either implemented or truthfully disabled.  
 - The old inactive `taskSidebar`/`sidebar` code in `ContentView.swift` is removed or migrated to avoid drift.  
 - Native app and TUI show the same Mission Board fixture.
 
 ### Verification
 - Cross-client journey: open Mission Board from panel → inspect lanes → open a Mission → view Evidence → make a decision.  
 - Deterministic matrix.  
 - RED tests proving panel views are read-only projections.  
 - Independent implementation and whole-WorkItem reviews.
 
 ---
 
 ## 8. Audit Finding → WorkItem Mapping
 
 | Audit item | Severity | Owner WorkItem | How it closes |
 |---|---|---|---|
 | P0 — opens in wrong mental model (Board-first) | P0 | W1 + W3 | Default launch is composer; Board is only the right panel. |
 | P0 — no normal Open Folder journey | P0 | W1 | Welcome/chrome includes `Open Folder…`; File menu registers it; TUI chooser. |
 | P0 — new task blocked by prior Team admin | P0 | W2 | Composer is active before Team selection; Team only after explicit trigger and confirmation. |
 | P1 — white/gray inconsistency structural | P1 | W1 | Shared semantic token system replaces ad-hoc `LoomGraphite` mixing. |
 | P1 — offline/loading contradict each other | P1 | W1 | Distinct `connecting/offline/reconnecting/fatal` states with actionable recovery. |
 | P1 — static copy looks interactive (tabs) | P1 | W3 | Tabs are implemented or truthfully disabled; no false affordances. |
 | P1 — accessibility collapses navigation | P1 | W1 | Left rail gives each button a unique action/name; static checks. |
 | P1 — client behavior not observable | P1 | W1–W3 | Structured, redacted client events emitted; E2E journeys correlate events with Journal facts. |
 | P1 — two competing interaction systems in `ContentView.swift` | P1 | W3 | Remove/migrate the old `taskSidebar`/`sidebar` remnants; single shell frame. |
 | P2 — navigation state weak, Board too dense | P2 | W1 + W3 | Rail shows active selection; Board is secondary and can be collapsed. |
 | P2 — fixed geometry no scaling contract | P2 | W1 + W3 | Minimum targets, text scaling, compact-window, and keyboard tests added. |
 
 ---
 
 ## 9. Cross-Client Journey Set (Mandatory Exit Evidence)
 
 Each journey must run on both the native macOS app and the Bubble Tea TUI against the same Go daemon and fixture, using the alternative-verification method already accepted (production Swift client over real socket, real PTY TUI, real native window, `screencapture` checkpoints, restart/reconnect rebuild). Computer-Use-driven window automation remains skipped unless explicitly reauthorized.
 
 | # | Journey | Key assertions |
 |---|---|---|
 | J1 | Fresh launch → no-folder chat | No Team/Mission facts created; composer visible; offline state truthful. |
 | J2 | Open Folder → first chat | Native folder picker used; path not typed; scoped preflight before file access. |
 | J3 | Offline → reconnect → continue | Distinct states; recovery action visible; view rebuilds from Journal on reconnect. |
 | J4 | Plain chat → explicit “use Agent” → Team Draft | Main Agent proposes a structured Draft; no fact created until user confirms. |
 | J5 | Confirm Team Draft → Mission starts → governance panel | Right panel shows Board; center shows milestones; Journal contains the facts. |
 | J6 | Mission running → inspect Evidence → approve decision | Right panel switches to Evidence/Decisions; approval creates a Journal fact. |
 | J7 | Runtime offline | Panel shows Runtime health; status distinct from Mission state; recovery actionable. |
 | J8 | Restart daemon → reconnect → view preserved | Journal rebuilds the same views; no duplicate facts; no client-side state authority. |
 | J9 | Keyboard/accessibility traverse every action | VoiceOver/Computer Use identifies every button uniquely; focus visible; Escape works. |
 | J10 | Light/dark/compact screenshot matrix | Contrast passes; no ad-hoc system colors; tokens consistent across views. |
 
 ---
 
 ## 10. Visual & Token System
 
 Adopt the **mixed recommendation** from `docs/ui/visual-direction-options.md`:
 
 - **TUI**: Option A (Warm Authority) — warm dark canvas, indigo-violet accent `#5E6AD2`, semantic status colors, theme variants (dark/light/ANSI).  
 - **macOS**: Option C (Linear Governance) — canvas + floating panels, left rail blends in, property rows, status pills, 13px metadata, sticky headers, 10–12px radius, `#5E6AD2` accent.  
 - **Shared**: 44pt minimum action targets, 0.2s motion, restrained hover/pressed feedback, no gradient orbs, no decorative blocking motion.
 
 Tokens must be centralized before W3:  
 - `internal/tui/style.go` for TUI (already started).  
 - `apps/macos/Sources/LoomLocalAppCore/LoomGraphite.swift` (or a renamed `LoomDesignTokens.swift`) for native.
 
 The recently added `MissionWorkbench.swift` Multica-style cards are preserved and moved into the panel context.
 
 ---
 
 ## 11. Risk Register
 
 | Risk | Owner | Mitigation |
 |---|---|---|
 | Chat composer becomes a second command authority | W2 | Plain text only routes to chat/proposal surfaces; mutating commands require explicit confirmation. |
 | Native app rewrite drifts from TUI | All | Shared daemon IPC, shared fixtures, cross-client journeys for every WorkItem. |
 | Old `ContentView.swift` sidebar code leaks back in | W3 | Delete or migrate remnants; static check that only one shell frame exists. |
 | Board still steals focus as default | W1 | Screenshot/RED test proving first launch is composer. |
 | Accessibility labels remain collapsed | W1 | Add automated accessibility dump comparison to the journey script. |
 | Offline and fatal states confused | W1 | Explicit state machine with negative tests for each transition. |
 | Visual tokens inconsistent across clients | W1 | Centralized token files + screenshot diff matrix. |
 | Phase scope expands into Web or full redesign | PM | Explicitly excluded; Web/cloud/multi-user remain out of scope. |
 | User expects Claude/Codex exact UX | Design | Borrow feelings, not copy identities; keep Loom indigo-violet accent and governance vocabulary. |
 | v0.4.1 scheduler/canary work is disrupted | Process | Phase 2C is inserted after v0.4.1 acceptance; product code locked until Exit Contract is frozen. |
 
 ---
 
 ## 12. Dependencies & Sequencing
 
 ```text
 v0.4.1 whole-slice acceptance (DONE)
     |
     v  Phase 2C Gate 0: cwd/Git/CURRENT.md identity check
     |
     v  Gate 1: ADR-0015 + Phase 2C Exit Contract + WorkItem contracts
     |          (frozen as one Candidate; contract review → repair → re-review)
     |
     v  P2C-W1 RED / implementation / reviews / atomic commit
     |
     v  P2C-W2 RED / implementation / reviews / atomic commit
     |
     v  P2C-W3 RED / implementation / reviews / atomic commit
     |
     v  Cross-client journeys J1–J10
     |
     v  Whole-Phase review + user sign-off
     |
     v  Refresh Phase 3A source lock and entry review
 ```
 
 ---
 
 ## 13. Exit Contract Checklist
 
 Before any product code is edited, the following must be frozen and reviewed:
 
 - [ ] ADR-0015 proposed and reviewed.  
 - [ ] `P2C-W1` contract: workspace shell + entry + visual tokens + offline states.  
 - [ ] `P2C-W2` contract: chat-first composer + conversation timeline + explicit Agent intent.  
 - [ ] `P2C-W3` contract: governance side panel + Mission Board migration + removed competing code.  
 - [ ] Combined Phase 2C Exit Contract reconciles `PRODUCT-PLAN.md`, `TECH-PLAN.md`, and ADR-0011/0012.  
 - [ ] Mandatory RED captured for each WorkItem.  
 - [ ] Cross-client journey manifest J1–J10 frozen.  
 - [ ] Source-lock exclusions updated (old `ContentView.swift` remnants, `LoomGraphite` mixing, etc.).  
 - [ ] Independent contract review PASS with P0=P1=P2=0.  
 - [ ] Product Owner sign-off.
 
 ---
 
 ## 14. Exclusions
 
 Phase 2C explicitly does **not** include:
 
 - Changing the Event Journal, one-writer, Projection, or Scheduler authority model.  
 - New credential capture, OAuth parsing, or Provider negotiation in the client.  
 - Public network binding, remote multi-user hosting, or cloud sync.  
 - WebSocket/notification state authority.  
 - Automatic Team/Mission/Run creation from ordinary chat.  
 - Hiding policy, permission, budget, Evidence, or approval gates for visual simplicity.  
 - Full Web UI, cross-platform Tauri, or browser-based desktop shell.  
 - Cross-platform delivery beyond macOS native + Bubble Tea TUI.  
 - Pushing, merging, or releasing to public channels without explicit approval.
 
 ---
 
 ## 15. Next Step
 
 1. Product Owner reviews this plan and confirms the three-pane chat-first direction.  
 2. Author freezes ADR-0015 + Phase 2C Exit Contract + WorkItem contracts as a single Candidate.  
 3. Independent Contract Review.  
 4. Repair/re-review until PASS.  
 5. Capture Mandatory RED and begin P2C-W1 implementation.
 
 No product code is edited until the combined Exit Contract is accepted.
 
