
 # P2C-W1 Implementation Evidence — Workspace Shell & Entry
 
 **WorkItem**: P2C-W1 — Workspace Shell & Entry  
 **Contract**: `.loom-evidence/phase2c/contracts/P2C-W1-CONTRACT.md`  
 **Date**: 2026-08-06  
 **Status**: implementation complete — pending whole-WorkItem review and atomic commit
 
 ## What changed
 
 1. **New `LoomWorkspaceShell`** (`apps/macos/Sources/LoomLocalAppUI/LoomWorkspaceShell.swift`):
    - Three-pane layout: left navigation rail, center chat workspace, right governance panel.
    - Left rail: Home, Work, Teams, Attention, Library, Runtimes, Providers.
    - Center: header, truthful connection strip, "Start with a task" card, Recent card, composer.
    - Right panel: `MissionWorkbench(store: showRail: false)` for Work/Teams/Attention/Library; a runtime panel for Runtimes.
    - Navigation syncs with `store.workbench.route` on init.
 
 2. **`ContentView` body updated** to use `LoomWorkspaceShell` with `minWidth: 1080, minHeight: 680`.
 
 3. **`MissionWorkbench` embeddable** (`apps/macos/Sources/LoomLocalAppUI/MissionWorkbench.swift`):
    - Added `showRail: Bool` parameter.
    - Rail is hidden when `showRail == false` for panel embedding.
    - Accessibility label reflects panel mode.
 
 4. **Visual token system** preserved from `LoomGraphite` in `LoomLocalAppCore`:
    - Indigo-violet accent `#5E6AD2`, semantic surfaces, status colors, 44pt action targets, 12px card radius, 192px rail width.
 
 5. **Test update** (`apps/macos/Tests/LoomLocalAppTests/LocalProductExperienceViewTests.swift`):
    - `testMissionRailRendersTeamsAttentionAndHistoryComparePages` now renders `MissionWorkbench(showRail: false)` for the page-distinctness assertion, matching the new panel architecture.
 
 ## Verification
 
 | Check | Command | Result |
 |---|---|---|
 | Swift build | `cd apps/macos && swift build` | PASS |
 | Swift tests | `cd apps/macos && swift test` | 101 tests, 0 failures, 1 skipped |
 | Go TUI tests | `go test -timeout 60s ./cmd/loom/... ./internal/tui/... ./internal/localipc/...` | PASS |
 | Native app install | `scripts/build-loom-local-app.sh --output /tmp/loom-p2c-w1/Loom.app` | PASS |
 | Fresh launch screenshot | `open /tmp/loom-p2c-w1/Loom.app --args --socket /tmp/nonexistent-loom.sock` + `screencapture` | PASS — chat-first three-pane layout visible |
 
 ## Screenshot
 
 `/tmp/loom-p2c-w1/screenshot.png` shows:
 - Left rail with Home selected.
 - Center workspace with "Start with a task" card, "Open Folder…", "Use Agent Team", composer.
 - Truthful offline strip: "Loom is not reachable" with Retry.
 - No Mission Board forced as the default first screen.
 
 ## Known remaining work (P2C-W1 → P2C-W2/W3)
 
 - `Open Folder…` is a visible affordance but not yet wired to `NSOpenPanel` / scoped preflight (marked `TODO(P2C-W1)`).
 - The center chat timeline is a welcome/composer placeholder; P2C-W2 will implement the message stream and explicit Agent intent transition.
 - The right panel reuse of `MissionWorkbench` is a structural migration; P2C-W3 will fully consolidate navigation and remove the old `ContentView` sidebar remnants.
 
 ## Exclusions honored
 
 - No Event Journal, one-writer, Projection, Scheduler, policy, Grant, or daemon authority change.
 - No new credential capture or Provider negotiation in the client.
 - No public network, cloud sync, Web UI, or Tauri.
 - No automatic Team/Mission/Run creation from chat.
 
