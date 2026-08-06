# Phase 2C Client Experience Baseline Audit

Status: `CURRENT BASELINE AUDIT` — read-only product/code inspection plus a
user-level installation. This is not an implementation PASS or a Phase freeze.

Date: 2026-08-03

## Baseline and method

- Repository baseline: `6d380233b5b89309a1a7ce3919aa611654e0f4ee`
- Installed app: `/Users/lune/Applications/Loom.app`
- Bundle version: `0.2.0`
- Executable SHA-256:
  `7133a7c3e2000e798e1135148107b065de45b58fcc472fd3c37d64691a5952ed`
- arm64 UUID: `FD48B6E5-1260-342B-A070-78ED674B42EB`
- Signature: ad-hoc, strict `codesign --verify` PASS
- Swift verification: 80 tests PASS, 1 visual-export-only test SKIP, 0 FAIL
- Native reproducible-build fixture: PASS
- Native atomic install/rollback fixture: PASS
- User-level launch: PASS; real Loom window observed through macOS
  accessibility Computer Use
- Daemon/Provider: intentionally not started; the app truthfully reached its
  offline path

The audit used the UI/UX Pro Max design-system workflow, source inspection,
real installed-window screenshots, the accessibility tree, and recent unified
logs. It did not modify product code or the active Phase 3A Candidate.

Screenshots:

- [Board-first offline baseline](screenshots/current-board-offline.jpeg)
- [New Mission baseline](screenshots/current-new-mission.jpeg)

## Findings

### P0 — The product opens in the wrong mental model

`ContentView.body` renders `MissionWorkbench` directly, while
`MissionWorkspaceState` defaults to `.board`. The installed first screen is
therefore an operational Mission Board, not a blank task/chat entry.

This makes Loom ask a new user to understand Missions, lanes, Teams, Runtime,
Providers and authoritative state before stating the work they want done. It is
the largest interaction gap from Codex-, Claude- and Multica-like task-first
products.

Evidence:

- `apps/macos/Sources/LoomLocalAppUI/ContentView.swift:50`
- `apps/macos/Sources/LoomLocalAppCore/MissionOrchestration.swift:87`

### P0 — There is no normal Open Folder journey

The active navigation offers Mission, Team, Library and Runtime/Provider
surfaces but no `Open Folder…`, recent folder, drag/drop or no-folder choice.
The application declares no File-menu commands, and the macOS source contains
no `fileImporter`, `NSOpenPanel`, `@FocusState` or drop handler.

The current installed build does not literally force a repository connection;
instead it omits the ordinary workspace entry entirely. A future implementation
must not replace that omission with mandatory Git setup or a manually typed
path.

Evidence:

- `apps/macos/Sources/LoomLocalApp/LoomLocalApp.swift:37`
- `apps/macos/Sources/LoomLocalAppUI/MissionWorkbench.swift:382`

### P0 — A new task is blocked by prior Team administration

The New Mission sheet requires a confirmed executable Team. With no Team, the
only choice is `No confirmed Team available`, and `Review preflight` stays
disabled even after the objective is entered.

The user should be able to state a task first. Team selection, recommendation
or a Team Draft should follow progressively without weakening confirmation or
execution authority.

Evidence:

- `apps/macos/Sources/LoomLocalAppUI/MissionWorkbench.swift:444`
- `apps/macos/Sources/LoomLocalAppUI/MissionWorkbench.swift:464`
- `apps/macos/Sources/LoomLocalAppUI/MissionWorkbench.swift:536`

### P1 — The white/gray inconsistency is structural

`LoomGraphite` maps four separate AppKit system surfaces to canvas, rail,
surface and raised roles. The workbench additionally uses `.bar`, `.secondary`,
raw `.orange` and opacity variants. Those colors resolve differently across
appearance, material, window activity and OS versions.

The installed screenshot shows an overly dark gray rail next to a white canvas,
a different white toolbar, a pink error strip and another white content area.
This is not one coherent graphite hierarchy.

Evidence:

- `apps/macos/Sources/LoomLocalAppUI/LoomGraphite.swift:15`
- `apps/macos/Sources/LoomLocalAppUI/MissionWorkbench.swift:1212`
- `apps/macos/Sources/LoomLocalAppUI/MissionWorkbench.swift:1634`

### P1 — Offline and loading states contradict each other

When the socket is invalid and no snapshot exists, the header says
`Loom is offline · invalid_socket`, while the empty state says
`Connecting to Loom`. There is no recovery call to action in that center state.

This creates an indefinite-looking loader for a known terminal connection
failure and makes it impossible for a user to know whether to wait, retry or
open setup.

Evidence:

- `apps/macos/Sources/LoomLocalAppUI/MissionWorkbench.swift:615`
- `apps/macos/Sources/LoomLocalAppUI/MissionWorkbench.swift:629`

### P1 — Static copy looks interactive

`Board`, `Topology`, `Timeline` and `Capacity` are presented as one tab row, but
only Board has styling and the other three are plain `Text`. They have no action
or unavailable-state explanation. This is a false affordance.

Evidence:

- `apps/macos/Sources/LoomLocalAppUI/MissionWorkbench.swift:599`

### P1 — Accessibility collapses distinct navigation actions

The real accessibility tree reports multiple rail buttons as the same
`Mission navigation` action. The container-level accessibility label masks the
individual button identity, so VoiceOver and Computer Use cannot reliably tell
New Mission, My Missions, Teams, Needs You, Library and Runtime & Providers
apart.

Evidence:

- `apps/macos/Sources/LoomLocalAppUI/MissionWorkbench.swift:428`
- installed accessibility tree: buttons 6–12 all announce
  `Mission navigation`

### P1 — Client behavior is not observable at product level

The launched process remained healthy and produced no crash report, but recent
unified logs contained only AppKit, input, XPC and system framework activity.
There were no structured Loom events for launch, refresh, offline detection,
opening New Mission, closing the sheet or user-visible state transitions.

Without a redacted client event stream, a GUI/TUI/Web E2E can prove pixels or
text but cannot explain why the behavior was reasonable, delayed, duplicated,
stale or rejected.

### P1 — Two competing interaction systems remain in one source file

`ContentView.body` now delegates to `MissionWorkbench`, but the remainder of
the approximately 2,100-line `ContentView.swift` still contains an older
task-first sidebar/composer/inspector implementation with a separate palette
and interaction vocabulary. This inactive surface is compiled alongside the
new workbench and is a continuing source of visual and behavioral drift.

Evidence:

- `apps/macos/Sources/LoomLocalAppUI/ContentView.swift:50`
- `apps/macos/Sources/LoomLocalAppUI/ContentView.swift:69`

### P2 — Navigation state is weak and the Board is needlessly dense

The rail does not visibly distinguish the current route. The first screen also
spends primary space on a five-lane horizontally scrollable Board and exposes
filter/topology/timeline/capacity concepts before any task exists. Horizontal
scroll is explicitly required to understand the complete state.

Evidence:

- `apps/macos/Sources/LoomLocalAppUI/MissionWorkbench.swift:562`
- `apps/macos/Sources/LoomLocalAppUI/MissionWorkbench.swift:642`

### P2 — Fixed geometry has no demonstrated scaling contract

The design fixes rail width, inspector width, toolbar/lane heights, sheet sizes
and minimum content widths. There is no explicit focus model or verified
large-text/compact-window journey. Existing minimum-target tests are useful but
do not prove keyboard traversal, focus visibility, VoiceOver naming, text
scaling or compact layout.

Evidence:

- `apps/macos/Sources/LoomLocalAppUI/LoomGraphite.swift:4`
- `apps/macos/Sources/LoomLocalAppUI/MissionWorkbench.swift:318`
- `apps/macos/Sources/LoomLocalAppUI/MissionWorkbench.swift:349`

## What should be preserved

- one Go application/authority path and strict local IPC;
- explicit preflight and explicit Start;
- authoritative Journal/Projection status and preserved prior view on refresh
  failure;
- bounded safe-text rendering and strict wire decoding;
- decision sheets, Evidence inspection and recovery semantics;
- current reproducible build, atomic install/rollback and fail-closed client
  composition; and
- a 44-point minimum action-target primitive.

The Phase should change information architecture and presentation without
weakening these safety properties.

## Audit verdict

`NEEDS DEDICATED PHASE`

The app is installable and launchable, but its ordinary-user entry flow is not
yet product-ready. The defects are coupled across information architecture,
workspace selection, Team progression, theme semantics, accessibility,
cross-client parity and observability. Treating them as isolated color or
folder-picker patches would recreate the micro-splitting problem; they belong
in one vertical Phase 2C contract.
