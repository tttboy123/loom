
 # P2C-W3 Contract — Governance Side Panel
 
 **Status**: frozen — pending RED and implementation  
 **Date**: 2026-08-06  
 **Owner**: Phase 2C Candidate writer  
 **Parent**: `PHASE-2C-EXIT-CONTRACT.md`
 
 ## Contract
 
 Refactor the existing Mission Board and related surfaces into a collapsible,
 context-aware right governance panel. Reuse the improved Multica/Linear visual
 language already started in `MissionWorkbench.swift`, but make it reachable from
 the chat-first surface rather than replacing it.
 
 ## Scope
 
 - macOS: right panel container, panel switcher, Mission Board lanes, Team
   Topology, Timeline, Decisions, Evidence, Runtime/Provider health, Attention.
 - TUI: side panel rendering, panel tabs, dense list rows, status pills, sticky
   headers.
 - Shared: panel state, view switching, read-only projections from daemon.
 
 ## Required Behavior
 
 1. The right panel can be opened, collapsed, and pinned from the left rail or
    from an active Agent task.
 2. Panel views include: Mission Board (lanes), Team Topology, Timeline,
    Decisions, Evidence, Runtime/Provider health, Attention.
 3. The Mission Board is a **secondary** view, not the default first screen.
 4. Board lanes are widened and use the improved card styling (status pill,
    priority, progress, attention indicator).
 5. Static tabs (`Board`, `Topology`, `Timeline`, `Capacity`) are either fully
    interactive or removed; no false affordances.
 6. `+ New Mission` is available from the left rail and composer area, but does
    not block simple chat entry.
 7. Team, Runtime, and Provider management surfaces are reachable from the left
    rail or panel switcher, not from the center conversation.
 8. Decisions and Evidence are first-class views in the panel, not buried inside
    mission cards.
 9. The panel preserves the visual tokens from P2C-W1 and does not reintroduce
    old `LoomGraphite` mixing.
 10. The panel is replaceable view state; collapsing or switching views does not
     cancel or mutate running work.
 
 ## Acceptance
 
 - The same Mission Board rendered as the first screen in v0.2.0 is now only
   reachable through the panel.
 - All panel views are reachable via keyboard and have unique accessibility
   labels.
 - Panel state (open/collapsed/selected view) is restored after reconnect but
   is not authoritative.
 - `Board`, `Topology`, `Timeline`, `Capacity` tabs are either implemented or
   truthfully disabled.
 - The old inactive `taskSidebar`/`sidebar` code in `ContentView.swift` is removed
   or migrated.
 - Native app and TUI show the same Mission Board fixture.
 
 ## Verification
 
 - Deterministic matrix: Go full/race/vet/tidy/gofmt; Swift full/TSAN/Release.
 - Journeys J5, J6, J7, J8, J9.
 - RED tests proving panel views are read-only projections.
 - Independent implementation review, dual-Result review, whole-WorkItem review.
 
 ## Explicitly Out
 
 - Changing the Mission Board data model or daemon authority.
 - New credential capture or Provider negotiation.
 - Web UI or Tauri.
 - Making the panel the default first screen.
 
