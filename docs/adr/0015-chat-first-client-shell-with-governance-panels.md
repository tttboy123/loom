
 # ADR-0015: Chat-First Client Shell with Governance Panels
 
 **Date**: 2026-08-06
 **Status**: proposed
 **Deciders**: Product Owner, Loom Architecture Controller
 
 ## Context
 
 ADR-0011 established the TUI-first local product over versioned daemon IPC,
 and ADR-0012 established the native app host over the same shared daemon IPC.
 Phase 2A/2B and v0.4.0/v0.4.1 proved the Go authority substrate, cross-client
 journeys, and deterministic matrix.
 
 The installed v0.2.0 product, however, opens directly into a five-lane Mission
 Board. The Phase 2C Client Experience Audit
 (`.loom-evidence/phase2c/CLIENT-EXPERIENCE-AUDIT.md`) found this to be the
 largest P0 interaction gap: the user is forced to understand Missions, lanes,
 Teams, Runtime, Providers and authoritative state before stating the work they
 want done. The audit also found no normal Open Folder journey and that new
 tasks are blocked by prior Team administration.
 
 `PRODUCT-PLAN.md` already states that ordinary chat is the default entry and
 that Agent mode is explicit. The current client implementation contradicts
 that principle.
 
 ## Decision
 
 Loom's primary client surface is a chat-first three-pane shell:
 
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
 
 The following rules are binding:
 
 1. **Chat is the default empty-state surface.** Fresh launch shows a blank
    composer plus bounded recent work. No Mission Board is forced.
 2. **Left rail is navigation and library.** It blends into the canvas, shows
    active state, and gives every navigation action a unique accessible name.
 3. **Center is the conversation timeline.** It supports plain chat, task
    context, agent messages, tentative output, and milestone cards.
 4. **Right panel is governance.** It can display Mission Board, Team
    Topology, Timeline, Decisions, Evidence, Runtime health, and Attention.
    It is collapsible and can be opened contextually when an explicit Agent
    journey starts.
 5. **No state authority in the client.** Clients hold only replaceable copied
    view state. Team creation, Run start, approval, and Evidence acceptance
    remain daemon-authorized Journal facts.
 6. **Cross-client parity.** Every user-facing journey must work in both the
    Bubble Tea TUI and the native macOS app, using the same daemon IPC methods
    and the same deterministic fixture.
 7. **Semantic visual system.** All surfaces use a shared token system
    (canvas, rail, surface, raised, text-primary/secondary/muted, accent,
    success, warning, danger, offline). Platform-native frameworks implement
    the tokens; ad-hoc system colors are not mixed.
 8. **Truthful connection states.** `connecting`, `offline`, `reconnecting`,
    `stale`, and `fatal` are distinct and carry an actionable recovery path.
 9. **Accessibility and input.** Every navigation action has a unique
    accessible name; keyboard traversal, focus visibility, Escape, text
    scaling, and reduced-motion are verified.
 10. **No automatic execution from chat.** Ordinary conversation never creates
     a Team Draft or Mission. A clear, intentional trigger is required.
 
 ## Alternatives Considered
 
 ### Alternative 1: Keep the Board-first layout
 
 - **Pros**: Reuses existing `MissionWorkbench` and recent Multica-style
   styling.
 - **Cons**: Contradicts the accepted product principle; audit found it the
   largest P0 gap.
 - **Why not**: Rejected by Product Owner and audit.
 
 ### Alternative 2: Chat-first with a separate full-screen Board
 
 - **Pros**: Simpler panel layout.
 - **Cons**: Forces context switch between conversation and governance.
 - **Why not**: User explicitly wants side-by-side operation, referencing
   Codex’s split-pane design.
 
 ### Alternative 3: Web-first UI
 
 - **Pros**: Familiar layout, easier cross-platform.
 - **Cons**: Violates Phase 2A/2B local-first, no-public-network boundary;
   adds a second stack before native parity is proven.
 - **Why not**: Web remains explicitly outside the current Phase. The native
   SwiftUI and TUI clients stay authoritative.
 
 ## Consequences
 
 ### Positive
 
 - Matches the user mental model of pair-programming companions (Codex,
   Claude Code).
 - Reduces first-run friction; the user can state work before choosing a Team.
 - Governance surfaces remain reachable without blocking task entry.
 - The existing daemon, Journal, Projection, and authority model are unchanged.
 
 ### Negative
 
 - Large client refactor; native SwiftUI and Bubble Tea TUI both need new
   screens.
 - Must preserve v0.4.1 daemon/authority boundaries and cross-client parity.
 - The improved Mission Board styling becomes panel content instead of the
   primary surface.
 
 ### Risks
 
 - **Chat composer becomes a second command authority.** Mitigation: plain text
   only routes to chat/proposal surfaces; mutating commands require explicit
   confirmation and daemon authorization.
 - **Native app and TUI drift.** Mitigation: shared daemon IPC, shared
   deterministic fixtures, cross-client journeys for every WorkItem.
 - **Old sidebar code leaks back in.** Mitigation: delete or migrate remnants
   from `ContentView.swift`; static check that only one shell frame exists.
 
