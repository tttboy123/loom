# Phase 2C Client Experience Continuity — Queued Insertion

Status: `QUEUED` — authorized roadmap input, non-authoritative and
non-executing.

Date: 2026-08-03

Authority input: the Product Owner explicitly authorized inserting a dedicated
Phase to repair and improve the client experience, including the inconsistent
white/gray visual system, task-first onboarding, native folder selection,
cross-client E2E, and behavior-oriented log observation.

This file does not freeze an implementation contract, modify current product
code, change credentials, start the daemon, call a Provider, or authorize a
live canary. The current Phase 3A Candidate remains untouched.

## Placement and sequencing

Insert `Phase 2C — Client Experience Continuity` between the accepted Phase 2B
product boundary and Phase 3A product implementation.

Phase 3A contract repair is already active, so insertion must occur without
polluting that Candidate:

1. finish or formally stop the currently frozen Phase 3A contract-repair gate;
2. at that safe checkpoint, review one roadmap amendment that reconciles
   `PRODUCT-PLAN.md`, `TECH-PLAN.md`, and the Phase 3A entry contract;
3. freeze and execute the single vertical `P2C-W1` contract below;
4. complete whole-Phase client Review and user-side E2E sign-off; and
5. refresh the Phase 3A source lock and entry review against the accepted
   Phase 2C client boundary before Phase 3A product implementation resumes.

This is an inserted usability gate, not P2A-W4 and not a new state authority.

## Product outcome

An ordinary user can open Loom and start with the work they want done. A
repository, folder, Team, Runtime, Provider, DAG, or governance object appears
only when the work actually needs it.

The primary journey is:

```text
open Loom
-> describe a task
-> optionally Open Folder / choose Recent / continue without a folder
-> progressively choose or confirm Team, Runtime and Provider when required
-> review permission, privacy, budget and execution preflight
-> explicitly Start
-> follow output, milestones and decisions in one conversation
-> inspect Evidence, recover, accept, continue or archive
```

The Mission Board remains a secondary operational view. It is not the default
first-run or empty-state surface.

## Preferred client architecture for contract review

The contract review should challenge and then accept or replace this preferred
baseline:

- one React/TypeScript presentation core for a loopback-only local Web UI and a
  Tauri desktop shell on macOS, Windows, and Linux;
- the existing Go local product application service, Event Journal,
  Projection, policy, grants, and strict command boundary remain authoritative;
- any browser transport is private loopback, origin checked, session bound,
  bounded, and incapable of becoming another state authority;
- the current Swift app remains an installed migration oracle until the new
  desktop surface passes parity, rollback, and security gates; and
- Bubble Tea TUI uses the same journey semantics and authoritative service,
  without copying browser or desktop state.

Do not delete or silently deprecate the Swift client before replacement parity
is independently reviewed. Do not add a second Journal, queue, Projection,
Scheduler, settings database, or credential store.

## Single vertical WorkItem

Freeze at most one WorkItem:

`P2C-W1 Unified Task-first Client Experience`

It must close the user journey vertically. Do not split theme tokens, folder
picker, shell, telemetry, adapters, or E2E harnesses into wrapper-only
WorkItems.

### Required interaction behavior

1. Fresh launch opens a blank task/chat composer plus bounded recent work.
2. `Open Folder…` is visible beside the composer and in the File menu. Desktop
   uses the native folder picker; Web uses the applicable browser directory
   capability or a truthful unsupported state; TUI uses a bounded chooser.
3. Typing a filesystem path is never the only normal route. Git is optional;
   an ordinary folder and a no-folder knowledge task are both valid.
4. Folder selection is read-only until an explicit preflight grants scoped
   access. Recent folders store a safe display handle, not credentials or raw
   sensitive content.
5. A Team is not required before the user can express intent. Loom may propose
   an existing Team or a Team Draft after the task is understood; creation or
   execution still requires the existing confirmations.
6. Provider connection is contextual and recoverable. It must not replace the
   task composer with a setup wall.
7. The central pane is a continuous task/conversation timeline. The left pane
   holds tasks/recent work; the right pane progressively exposes Team, Plan,
   Changes, Evidence and technical details.
8. Board, topology, capacity and provider management remain reachable as
   secondary surfaces with truthful selected state and no inert tab-like copy.
9. Offline, reconnecting, stale-view and fatal states are distinct and include
   an actionable recovery path without inventing success.

### Visual system

Use semantic tokens only. Native framework colors may implement a token, but
product views must not independently mix `windowBackgroundColor`,
`underPageBackgroundColor`, `controlBackgroundColor`, `textBackgroundColor`,
`.bar`, raw status colors, and ad-hoc opacities.

Provisional Loom Graphite palette for contrast validation:

| Token | Light | Dark | Purpose |
|---|---:|---:|---|
| canvas | `#F7F8FA` | `#0F1115` | application background |
| rail | `#F1F3F5` | `#15181E` | navigation rail |
| surface | `#FFFFFF` | `#1B1F26` | cards and controls |
| raised | `#FFFFFF` | `#222733` | modal and selected surface |
| border | `#DDE1E7` | `#303744` | separators and outlines |
| text-primary | `#171A1F` | `#F3F5F7` | primary content |
| text-secondary | `#606975` | `#A7B0BE` | secondary content |
| accent | `#3B6FE8` | `#78A3FF` | one primary action family |

Before acceptance, automated checks must prove normal-text contrast of at least
4.5:1 and large-text/UI contrast of at least 3:1. Warning, danger, success,
attention and offline colors are separate semantic tokens and must not rely on
color alone.

All clients must also provide:

- one icon family and consistent selected/hover/focus/disabled states;
- 44-point/pixel minimum primary action targets where the platform requires it;
- full keyboard traversal, visible focus, predictable default focus and Escape;
- text scaling, compact-window behavior, screen-reader labels and reduced
  motion; and
- restrained 200–300 ms state transitions with no decorative blocking motion.

## Mandatory cross-client E2E and observability

Every accepted client capability must be exercised as a user journey, not only
through model/store tests:

- Desktop GUI: accessibility-driven Computer Use with screenshots and fresh
  element discovery;
- Web UI: Playwright against the real local application-service boundary;
- TUI: controlled PTY input and durable screen transcript; and
- all surfaces: the same fixture, commands, Projection version and expected
  Journal facts.

The minimum scenario set covers fresh launch, offline recovery, task entry,
Open Folder, no-folder work, Team Draft confirmation, Provider-needed state,
preflight, explicit Start, tentative output, decision, cancellation, recovery,
Evidence review, restart and reconnect.

Client observability is non-authoritative. Emit structured, redacted events
with at least:

`client_session_id`, `surface`, `journey_step`, `action`, `result`,
`latency_ms`, `connection_state`, `view_version`, and an optional authoritative
correlation ID.

Never log prompt content, raw output, raw filesystem paths, credentials, Grants,
hidden reasoning, or per-token deltas. Tests must correlate visible behavior,
client events and authoritative Journal/Projection facts and must flag missing,
duplicate, reordered, stale-generation, or unexplained transitions.

## Exit conditions

Phase 2C cannot pass until:

1. a clean install starts task-first rather than Board-first;
2. a user completes one no-folder task and one native Open Folder coding task
   without manually typing a path;
3. the task can be described before Team or Provider selection;
4. light and dark screenshot matrices show only the reviewed semantic layers;
5. keyboard and accessibility journeys identify every action distinctly;
6. GUI, Web UI and TUI run the same bounded end-to-end fixture through the real
   Go service boundary;
7. logs explain every visible transition without leaking protected data;
8. restart/reconnect preserves authoritative state and rejects stale views;
9. the old Swift client can be rolled back until replacement parity is signed;
10. independent implementation and whole-Phase Reviews return PASS; and
11. the user signs off the installed product journey.

## Explicit exclusions

- changing Loom's Event Journal, one-writer or Projection authority model;
- provider credential capture or OAuth token parsing in a client;
- public network binding, remote multi-user hosting or cloud sync;
- WebSocket/notification state authority;
- automatic Team creation or execution from ordinary conversation;
- hiding policy, permission, budget, Evidence or recovery gates for visual
  simplicity; and
- claiming cross-platform delivery before each packaged client passes its own
  installed E2E.
