
 # P2C-W1 Contract — Workspace Shell & Entry
 
 **Status**: frozen — pending RED and implementation  
 **Date**: 2026-08-06  
 **Owner**: Phase 2C Candidate writer  
 **Parent**: `PHASE-2C-EXIT-CONTRACT.md`
 
 ## Contract
 
 Replace the Board-first launch with a task-first workspace shell. Establish
 the three-pane frame, the visual token system, and truthful connection states.
 Provide a clear, non-blocking path to open a folder, continue without a
 folder, or pick a recent workspace.
 
 ## Scope
 
 - macOS: `apps/macos/Sources/LoomLocalAppUI/ContentView.swift`,
   `MissionWorkbench.swift`, new shell/rail/composer views, `LoomGraphite.swift`
   (or renamed `LoomDesignTokens.swift`), `LoomLocalApp.swift` File menu.
 - TUI: `internal/tui/model.go`, `internal/tui/style.go`, welcome/composer
   screen, bounded directory chooser.
 - Shared: daemon IPC client surfaces, connection state machine, accessibility
   labels.
 
 ## Required Behavior
 
 1. Fresh launch shows a blank composer as the central surface, bounded recent
    work below it, and a prominent `Open Folder…` affordance.
 2. `Open Folder…` is in the File menu and welcome chrome; desktop uses
    `fileImporter`/`NSOpenPanel`; TUI uses a bounded chooser.
 3. A no-folder knowledge or chat task is a valid first-class journey.
 4. Folder selection is read-only until explicit preflight grants scoped access.
 5. Recent folders store a safe display handle + daemon-authorized workspace ID.
 6. Left rail shows Recents, Teams, Runtimes, Skills, Library, with a clearly
    selected active item and unique accessibility names.
 7. Connection states are distinct: `connecting` (cancellable), `offline`
    (retry / open setup), `reconnecting` (preserve view), `fatal` (daemon
    rejected).
 8. Visual token system is enforced: canvas, rail, surface, raised,
    text-primary/secondary/muted, accent `#5E6AD2`, success, warning, danger,
    offline, border, shadow.
 9. macOS uses Linear Governance floating panels adapted to SwiftUI; TUI uses
    Warm Dark palette with semantic ANSI fallbacks.
 10. No persistent client cache, no second Journal, no SQLite reads from the
     client.
 
 ## Acceptance
 
 - `swift test` and `go test ./internal/tui/...` pass.
 - Fresh native app launch screenshot shows composer-first, not Board-first.
 - Real PTY TUI transcript shows the same welcome/composer journey.
 - Accessibility tree identifies every rail action uniquely.
 - Contrast automation: normal text ≥ 4.5:1, large text/UI ≥ 3:1.
 - Offline state shows actionable recovery, not indefinite “Connecting…”.
 
 ## Verification
 
 - Deterministic matrix: Go full/race/vet/tidy/gofmt; Swift full/TSAN/Release.
 - Journeys J1, J2, J3, J8, J10.
 - Screenshot matrix light/dark/compact.
 - Independent implementation review, dual-Result review, whole-WorkItem review.
 
 ## Explicitly Out
 
 - Changing daemon authority, Journal, or Projection.
 - New credential capture or Provider negotiation.
 - Automatic Team/Mission/Run creation.
 - Web UI or Tauri.
 
