# P2D-W2D New Mission Clarity (V46)

Status: `SOURCE + INSTALLED-LIVE VERIFIED`

Date: 2026-08-16

Parent Goal: `Phase 2D - Multi-Provider, Multi-Model Agent Team Orchestration`

## Problem

The sidebar "New Mission" action was a dead end for users without a confirmed
Agent Team: the Team picker was empty ("No confirmed Team available"), the
objective/context fields and preflight button were present but unusable, and
there was no plain-language explanation of what a Mission is or what to do
first. Users reported "New Mission 我没看懂".

## Changes (client, `MissionWorkbench.swift`)

1. **Plain-language definition**: the New Mission header now explains in one
   sentence what a Mission is ("one bounded piece of work that an Agent Team
   carries out for you, with review before anything runs").
2. **Guided empty state**: when no executable (confirmed, non-read-only) Team
   exists, the sheet now shows "You need an Agent Team first" with an
   explanation and a one-click **Create Agent Team** button that closes the
   sheet, opens the Teams workspace, and starts the blank Team builder — instead
   of an unusable form. The "Close" affordance remains.
3. The full form (Work type, Team, objective, context, preflight) is only shown
   once an executable Team exists.

## Verification

- `swift build` + macOS package `236` tests, `0` failures (`1` visual-export
  skip).
- Live installed daemon snapshot: `saved_teams=0` (no confirmed Team), so the
  New Mission sheet now routes to the Create Agent Team guidance.
- App rebuilt + reinstalled; `git diff --check` clean.
