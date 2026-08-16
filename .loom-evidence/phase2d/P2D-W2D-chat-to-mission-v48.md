# P2D-W2D Chat → Mission Merge (V48)

Status: `SOURCE + INSTALLED-LIVE VERIFIED`

Date: 2026-08-16

Parent Goal: `Phase 2D - Multi-Provider, Multi-Model Agent Team Orchestration`

## Problem

Mission was a separate surface reached by clicking "New Mission": users had to
leave the Chat, pick a Team, and re-type the objective they had already
described in the conversation. The merge direction was confirmed by the user.

## Changes

1. **Store exposes `executableTeams`** (`LocalProductStore`): the confirmed,
   executable, non-read-only Teams a Mission can run on; `MissionWorkbench` now
   reuses it instead of its private copy.

2. **Chat proposal → "Run as Mission"**: when a Loom proposal message appears
   and at least one executable Team exists, the message now shows a
   **Run as Mission** button (alongside Create Agent Team). Clicking it:
   - Builds the Mission objective from the proposal content (falling back to the
     latest user message in the thread),
   - Opens the New Mission sheet already pre-filled with that objective,
   - So the user only reviews the Team/preflight and starts — no re-typing, no
     hunting for New Mission.

3. **`MissionWorkbench` accepts `initialMissionObjective`** and pre-fills the
   objective field when the New Mission sheet appears.

## Verification

- macOS package `237` tests, `0` failures (`1` visual-export skip); new test
  `testExecutableTeamsExposesOnlyConfirmedRunnableTeams`.
- App rebuilt + reinstalled; `git diff --check` clean.
