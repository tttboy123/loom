# P2D-W2B/W2D OpenCode, Mission and RoundTable - build 99

Status: `CURRENT / VERIFIED SLICE`

Date: 2026-08-22

Parent Goal: `Phase 2D - Multi-Provider, Multi-Model Agent Team Orchestration`

## User-facing result

Installed Loom `v0.5.3 (99)` starts its own bundled daemon and retains the
complete discovered catalog:

- 25 Providers;
- 6 Runtimes: Claude Code, Codex, two Loom Native routes, OpenCode and Pi;
- 4 Conversation Profiles.

Mission history is a title list. A row carries its optional Conversation link,
Agent Team, step progress and outcome; opening the row shows one governed
workflow with Outcome, Plan, Activity and Evidence and a separate Inspector for
Team, Plan, Changes and Evidence.

RoundTable shows configured Agents with Harness, Provider Account and Model.
Rows are draggable into a visible drop zone; Add is retained as the
keyboard/accessibility equivalent. An active seat has a remove command until
the round opens.

## OpenCode live evidence

The build 97 daemon and product source retained by the build 99 UI candidate
passed both installed Team routes:

- native OpenCode two-node Team: PASS in 93.02 seconds;
- OpenCode with the Vault-backed DeepSeek account: PASS in 22.56 seconds.

The native run used Incident
`7f212140-e275-4ace-a4e8-e4c1b3428b89`. Each Agent Attempt completed instead
of inheriting the prior 15-second durable-commit cancellation. The verifier
retains only the minimum compatible workspace capability and maps the exact
terminal output reason to governed acceptance; arbitrary successful text no
longer auto-accepts a candidate.

## RoundTable live defect and repair

Installed visual review added one Agent, producing `1/2 required`, and exposed
the remove control. The first remove committed the authoritative retire fact.
The returned view correctly retained the historical seat with
`available=false`, but Swift selected candidates by dictionary presence and
continued rendering the retired seat. A repeated remove therefore returned a
conflict.

Build 99 projects only seats whose authoritative `available` field is true.
After restart, reopening the same installed Session showed `0/2 required`, an
empty drop zone and no stale seat. The append-only history remains intact.

## Installed visual review

The build 99 installed App visibly confirms:

- conversation remains the primary workspace and reports `Local service ready`;
- Mission history uses titles, readable outcomes and `steps` progress;
- one Mission opens one workflow rather than a global kanban;
- the Inspector label and Team/Plan/Changes/Evidence tabs fit without wrapping;
- RoundTable exposes the drop zone, Agent route summary and remove control;
- a retired seat disappears from the active view after restart/reopen.

An unrelated UsageHub Keychain prompt was denied during visual review. No
credential was read or changed, and it is not part of the Loom acceptance path.

## Verification

- `swift test --package-path apps/macos`: 297 passed, 1 intentional skip, 0 failed;
- strict Swift Testing contracts: 16 passed;
- `go test ./...`: PASS after the Harness startup synchronization regression;
- `go vet ./...`: PASS;
- Harness cancellation/reap test: 10 consecutive passes;
- final candidate build: PASS;
- installed strict signature verification: PASS;
- installed `setup_snapshot`: 25 Providers / 6 Runtimes / 4 Profiles;
- build 98 two-build packaging fixture: PASS before the final UI-only active-seat correction.

No API key, Authorization header, Prompt, Provider response, credential body or
private diagnostic payload is included in this evidence.

## Remaining Phase 2D gates

Phase 2D remains `ACTIVE / PARTIAL`. The four-Harness/four-Provider installed
Team, expanded account-local failure matrix, explicit approved fallback,
complete accounting/governance UI and remaining Capsule disclosure/encryption
matrix are still open.
