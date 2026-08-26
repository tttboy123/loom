# P2D Provider / Runtime Recovery and Mission Index — v0.5.3 (65)

Status: VERIFIED SLICE / Phase 2D remains PARTIAL
Date: 2026-08-21

## Root cause

- The daemon still held 25 Providers, 4 Runtimes and 4 Conversation Profiles.
- `setup_snapshot` rebuilt credential projection once per Provider catalog entry.
- The response took about 10 seconds while Swift used a 5 second deadline, so UI recovery
  rendered an empty directory.
- OpenCode `native_runtime` status incorrectly reused Codex native-auth observation.

## Implemented

- One credential status directory snapshot supplies Provider and Provider Account state.
- Swift setup uses an extended bounded deadline and retries unusable empty snapshots.
- OpenCode availability follows the discovered OpenCode runtime independently of Codex auth.
- Mission home is a title-first list; workflow, Agent Team, Attempts and Evidence remain inside
  the selected Mission.
- New Missions can retain a local navigation relationship to their source Conversation.

The authoritative backend still keys Mission execution as `mission/<team_instance_id>`;
multiple independent Mission streams for the same Team remain a later Phase 2D migration.
This slice does not treat local presentation metadata as execution authority.

## Installed acceptance

- Bundle: `/Users/lune/Applications/Loom.app`, version `0.5.3`, build `65`.
- App PID and canonical managed loomd child were both present after launch.
- Swift UDS contract: 25 Providers, 4 Runtimes, 4 Conversation Profiles.
- Warm setup: about 1.5 seconds. Cold setup including runtime discovery: about 6.3 seconds.
- OpenCode conversation profile returned exactly `E2E-OK` through real network egress.
- UI screenshot verified Runtime & Providers contains Codex, Loom Native, OpenCode and Pi.
- UI screenshot verified Missions renders a title list with conversation link, Agent Team,
  progress and terminal status instead of five horizontal lanes.

## Regression

- Swift XCTest: 287 passed, 1 skipped, 0 failed.
- Swift Testing: 15 passed.
- Changed Go setup/OpenCode/composition tests passed.
- Full `go test ./... -p 1 -count=1` exposed pre-existing 10-minute infrastructure timeouts in
  operational-diagnostic fsync pressure and nested production Swift probe compilation. The
  implicated product composition test passed alone; this remains test-performance debt.
