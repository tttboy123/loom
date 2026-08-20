# Chat -> New Mission prefill · live acceptance (LoomBuild52)

Date: 2026-08-20
Repo: /Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-pi-rebuild
Branch: codex/loom-platform-slice2
Installed: /Users/lune/Applications/Loom.app (LoomBuild52, CFBundleVersion 65)

## What was fixed (RED-first)
Composer header "New Mission" button (~LoomWorkspaceShell.swift:1375) and the
navigation-rail "New Mission" button (~:484) previously opened the New Mission
sheet with an EMPTY objective, disconnected from the current conversation
(the user's repeated complaint: "New Mission 我没看懂" / high onboarding cost).

Added a pure, testable helper `composerNewMissionObjective(thread:)` that
pre-fills the objective from the latest conversation:
- newest proposal or user ask wins (reverse scan)
- a Loom/assistant reply is NEVER used as the objective; falls back to the
  latest user ask instead
- over-long text truncated to the 4096 objective limit
- empty/missing thread, or thread with only Loom replies -> "" (blank, same as today)

Both buttons now set `pendingMissionObjective = composerNewMissionObjective(...)`
before opening the mission board + New Mission sheet.

## RED-first evidence
`testComposerNewMissionObjectivePrefillsFromLatestConversation` was added first.
Run before implementation -> compile error "cannot find 'composerNewMissionObjective'".
A second live-found case was added and confirmed red:
  - thread ending in a Loom reply -> returned "2+2 is 4." (WRONG), expected the
    user ask "Original ask"
Then the helper was made role-aware; test went green.

## Gate results
- focused `swift test --filter testComposerNewMissionObjectivePrefillsFromLatestConversation` -> pass
- full `swift test` -> 279 tests, 1 skipped (visual, expected), 0 failures
- Swift Testing -> 15 tests passed
- `go test ./... -p 1` -> all ok
- `go vet ./...` -> clean
- `git diff --check` -> clean

## Live installed-app check (LoomBuild52)
Process: quit old app, moved old bundle aside, `scripts/build-loom-local-app.sh --output
/Users/lune/Applications/Loom.app`, codesign verify OK, relaunch.
Selected conversation "What is 2+2? Please answer in one sentence." whose
thread is:
  You: What is 2+2? Please answer in one sentence.
  Loom: 2+2 is 4.
(so the last message is a Loom reply -> must prefill the USER ask)

Ax-clicked "New Mission". AX dump of the opened sheet showed:
  AXStaticText value=[Mission objective]
  AXTextField value=[What is 2+2? Please answer in one sentence.]
=> the sheet pre-filled with the user's ask, NOT "2+2 is 4.".

Closed the sheet via "Close" (no real Mission started; no autonomous execution).
AX dump confirmed the objective field no longer present ("Mission objective"
gone), back to the workspace.

## Files changed
- apps/macos/Sources/LoomLocalAppUI/LoomWorkspaceShell.swift
- apps/macos/Tests/LoomLocalAppTests/LoomWorkspaceShellStateTests.swift
