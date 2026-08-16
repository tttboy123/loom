# P2D-W2D Team Creation + Sidebar Friction Reduction (V45)

Status: `SOURCE + INSTALLED-LIVE VERIFIED`

Date: 2026-08-16

Parent Goal: `Phase 2D - Multi-Provider, Multi-Model Agent Team Orchestration`

## Problem

Creating an Agent Team forced a 4-step sequential Q&A gauntlet
(team_name → purpose → main_role → subagent_role) before the user could even
see the editable draft form. Teams were also only reachable through a nav item,
with no visible list or quick-create in the sidebar rail. Both made the cost of
using Teams too high.

## Changes

1. **Form-first blank Team builder (server)**:
   `BuilderSourceBlank` now presents the full editable draft immediately (like
   template/saved-team) instead of starting with the `team_name` question. The
   default Main Agent + SubAgent roles are pre-selected; only the two required
   fields (name, purpose) gate confirmation. Explicit confirmation semantics are
   unchanged (`ConfirmBuilder(false)` still returns
   `ErrBuilderConfirmationRequired`; `can_confirm` still requires name + purpose
   + compatible, gap-free preview).

2. **Form-first blank Team builder (client)**:
   `teamBuilderPanel` renders editable "Team name" and "Bounded purpose"
   TextFields that commit via `editBuilder(field: "team_name"/"purpose")`, so the
   user fills the two required fields inline and confirms — no Q&A step-through.

3. **Sidebar rail "AGENT TEAMS" section**:
   The navigation rail now lists confirmed Agent Teams (click to open in the
   Teams panel), an empty state when none exist, and a "New Agent Team" button
   that opens the Teams panel and starts a blank draft in one action.

## Verification

- Live (installed daemon): `builder_start` with source `blank` returns
  `question=""`, 2 default roles pre-selected, `can_confirm=false`; two inline
  `builder_edit` calls (team_name, purpose) flip `can_confirm=true` with no
  question pending.
- Go: `internal/app` and `cmd/loomd` full suites green (blank-flow tests updated
  to the form-first contract).
- macOS package: `236` tests, `0` failures (`1` visual-export skip).
- App rebuilt + reinstalled; `git diff --check` clean.

## Multica research note

The earlier "no network access" answer was wrong for this environment: web
search, GitHub API, and docs fetch all worked. Live-verified Multica state
(2026-08-16): `multica-ai/multica`, ~46k stars / ~5.9k forks, created
2026-01-13, Apache 2.0 + additional conditions, 20 agent CLIs, Next.js 16 +
Go (Chi + WS) + PostgreSQL 17 (pgvector), 4 explicit trigger methods (assign
issue, @mention, direct chat, autopilot), and agent creation requiring only
Name + Runtime with everything else defaulted and adjustable after creation —
which this slice's form-first blank builder now mirrors.
