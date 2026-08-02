# P2A-W3 Final Walkthrough Safe-Name Repair Verification

Date: 2026-08-03

## Behavior closed

- Mission titles equal to `mission_id` or `team_instance_id` resolve to the
  confirmed Team display name or `Mission`.
- Board, Mission detail, Team pulse, topology, authorization sheet, Home,
  stale view, Runtime, Team, Evidence, History, and Compare surfaces no longer
  render raw authority IDs as ordinary product copy.
- TUI Compare is a reachable read-only screen and labels the two selected Runs
  as Previous and Current with Runtime display names and Evidence counts.
- Exact IDs remain inside model selection, IPC commands, view-version checks,
  generation fencing, and CAS bindings. No Journal, Projection, writer,
  scheduler, authority, schema, Runtime, Provider, Grant, or Evidence behavior
  changed.

## Causal and focused checks

PASS:

```text
go test -count=1 ./internal/tui -run 'TestMissionWorkbenchStartsOnBoardAndUsesExactNavigation|TestModelPresentsRecentWorkWithoutRawInternalIdentifiers'
go test -count=1 ./internal/tui
swift test --package-path apps/macos --filter LocalProductExperienceViewTests/testMissionRailRendersTeamsAttentionAndHistoryComparePages
swift test --package-path apps/macos --filter 'LocalProductExperienceViewTests|MissionOrchestrationTests'
```

## Complete matrix

PASS:

```text
go test -count=1 -p=1 ./...
go test -count=1 -race -p=1 ./...
go vet ./...
swift test --package-path apps/macos
swift build -c release --package-path apps/macos
git diff --check
```

Swift result: 63 XCTest tests, 1 visual-export-only skip, plus 4 Swift Testing
tests. All executed tests passed. Both complete Go matrices exited `0`.

