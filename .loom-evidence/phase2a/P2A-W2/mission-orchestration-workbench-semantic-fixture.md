# GUI/TUI Mission Semantic Fixture Comparison

Date: 2026-07-30

Status: PASS

The shared deterministic fixture resolves to:

| Semantic | Go facade | Native workbench | TUI |
|---|---|---|---|
| Mission | `mission/team-1` | Ship reviewed change | Ship reviewed change |
| Lane | Orchestrating | Orchestrating column | Orchestrating |
| Status | `human_required` | Human Required + warning | Human required |
| Team role | `main` | Main | Main |
| Node | `main` | Team Pulse / Plan | Team Pulse `main` |
| Attempt | 1 | Attempt 1 | Attempt 1 |
| State | waiting | Waiting | Waiting |
| Attention | 1 | Needs You | prepared decision required |
| Prepared decision | `authorization` | Open Authorization | Authorization prepared · `a` open |

The shared visual fixture contains the exact prepared Authorization command.
Its `prepared_actions` value is the exact per-action capability set. The native
sheet disables every visible mutation action absent from that set; `Not now`
and `Edit scope` remain presentation-only. The separately tested missing-
Evidence Review fixture contains no prepared command and opens a read-only gate
with all authority mutations disabled.

Evidence:

- `internal/api/local_product_mission_test.go`
- `apps/macos/Tests/LoomLocalAppTests/MissionOrchestrationTests.swift`
- `apps/macos/Tests/LoomLocalAppTests/LoomGraphiteViewTests.swift`
- `internal/tui/model_test.go`
- `mission-workbench-wide-light.png`
- `mission-room-wide-light.png`
- `tui-mission-board.png`

No client invents a Run, Evidence record, progress percentage, Provider result
or authoritative decision.
