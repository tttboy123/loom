# P2D-W2D Lower-Friction Quick Start (V47)

Status: `SOURCE + INSTALLED-LIVE VERIFIED`

Date: 2026-08-16

Parent Goal: `Phase 2D - Multi-Provider, Multi-Model Agent Team Orchestration`

## Problem

Even after the Team/sidebar and New Mission clarity fixes, a new user still had
to discover the path by trial: open a folder, connect a Provider, create an
Agent Team, then start a Mission. The empty conversation gave no status (which
Provider is ready? is a folder open?) and a proposal message told the user to
"press u" instead of offering the action.

## Changes (client, `LoomWorkspaceShell.swift`)

1. **Quick-start guide on the empty conversation**: shows the current state in
   plain language —
   - "Chat is ready with <Provider>" (or "Connect a Provider …" when none),
   - "1. Open Folder so work has a home" (or ✓ Folder: <name>),
   - "2. Create an Agent Team when the work needs governed execution" (or ✓
     Agent Team ready).
   Each line uses a checkmark once its prerequisite is met, so a first-run user
   sees exactly the next step instead of a blank page.

2. **Proposal messages get an action**: a Loom proposal ("This task looks like
   it needs an Agent Team …") now has a **Create Agent Team** button right on the
   message that opens Teams and starts the blank builder — no need to know the
   `u` shortcut or hunt the governance inspector.

## Verification

- `swift build` + macOS package `236` tests, `0` failures (`1` visual-export
  skip).
- Live installed daemon snapshot: DeepSeek is the ready conversation Provider
  and no Team exists yet, so the quick start shows "Chat is ready with DeepSeek"
  and "Create an Agent Team when the work needs governed execution".
- App rebuilt + reinstalled; `git diff --check` clean.
