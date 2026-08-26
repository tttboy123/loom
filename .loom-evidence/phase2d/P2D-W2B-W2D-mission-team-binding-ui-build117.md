# P2D-W2B / W2D - Mission and Team binding UI Build 117

**Date**: 2026-08-23
**Status**: SOURCE + INSTALLED VERIFIED
**Goal**: Phase 2D only

## Root cause and correction

- The daemon projected a valid current Mission board through a recoverable
  first-page `cursor_conflict` gap.
- Swift rejected each valid native OpenCode node because Mission decoding used
  the ordinary identifier grammar for `model_id`; that grammar rejects the
  slash in `opencode/big-pickle`.
- Mission decoding now reuses the bounded model-ID grammar already used by
  setup and execution. All account, credential revision, timeout, capability,
  Context, diagnostic and accounting checks remain strict.
- A recoverable first/final non-paginated gap publishes only the validated
  current board. It keeps global service state online and blocks side-task,
  Agent-input and cancellation mutations that require exact timeline
  continuity.

## Product result

- Mission Team Pulse displays each Agent's status, Attempt, Harness, Provider
  Account or native auth, Model, limits, Context policy and Incident.
- The fallback `Binding unavailable` label appears only when no validated
  binding exists.
- Teams renders one current configuration per visible Team and compact
  per-Agent route rows. Each Team opens its related single-workflow Mission;
  historical execution remains in Missions.
- Native visual and accessibility inspection found no overlapping controls or
  nested-card treatment. Conversation remains the primary App surface.

## Verification

- RED: a timeline node with `anthropic/claude-sonnet` failed strict Swift
  decoding before the correction.
- Targeted model and Store tests: pass.
- Real installed-daemon recoverable-gap Mission test: pass before and after
  cold restart.
- macOS XCTest: 315 executed, two intentional opt-in skips, zero failures.
- Strict Swift wire contracts: 16 pass.
- Reproducible native build fixture: pass.
- Installer dry-run, rollback/failure/signal fixtures, strict nested signing,
  arm64 identity and transaction install: pass.
- Installed Loom `v0.5.3 (117)`; App SHA-256
  `4f6f80f6588d235c0a074072f3651415cd2409be022c94a727c6fc62a323a64e`;
  daemon SHA-256
  `db0c1228e9df2485196f980a34c07773160985794559058abc4e3317e7519200`.
- The App cold-started and cold-restarted its own daemon with canonical argv.
- Before and after restart: Vault `unlocked`, 25 Providers, 7 Runtimes,
  4 Conversation Profiles, and exact real OpenCode reply `E2E-OK`.
- No Loom credential or Keychain helper process was present in the runtime
  hot path.

## Remaining Phase 2D gates

Phase 2D remains `ACTIVE / PARTIAL`. Successful approved Team fallback with
accounting, the installed four-distinct-Provider Team, and the remaining
controlled account-local failure cells are not claimed by this slice.
