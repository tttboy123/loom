# Phase 4 Build 208 installed RoundTable acceptance

Status: `ACCEPTED`

Date: 2026-08-27

## Installed identity

- App: `/Users/lune/Applications/Loom.app`
- Version/build: `0.5.3 (208)`
- App executable SHA-256:
  `56763db032f35e549117297e7894c81b51b2b8f2f83b061bc4418cfcb2a719ca`
- Bundled daemon SHA-256:
  `e6a18a841efe87ff6422c8cc48c28fc91652798bba2a81cf7d645f4ae7c35091`
- Managed launch: App PID `83067`, daemon PID `83081`; daemon canonical argv
  retained state, isolation, socket and managed parent identity.

## Build 208 regression

The production `productPiRuntimeAdapter` previously hid the delegate's
`AgentInputConsumer` capability. Loom Native therefore started with the default
one-turn/one-step budget and every RoundTable Steer was rejected as a conflict.
Build 208 forwards the exact capability, freezes it in the active Attempt and
returns a non-retryable `capability_gap` for unsupported Runtimes. The UI does
not offer live Steer for OpenCode.

## Installed live matrix

Session `rt-20260827-0040` used two independently frozen Loom Native/MiniMax
seats from the existing Mission-linked Team.

- Both Attempt Loop streams froze `max_turns=8` and
  `max_steps_per_turn=16`.
- Main Attempt `attempt-4ab0227e-5464-4399-8f12-d7d8a537f5cf` accepted
  intervention `intervention-live-1787762385062739000`.
- The Agent Inbox committed `AgentInputAdmitted` for target Step 2 and then
  `AgentInputConsumed` for the same input.
- The Attempt Loop committed Step 1 as `continue`, started and admitted Step 2,
  then recorded the Provider HTTP failure on that seat only.
- Peer Attempt `attempt-bcedf350-7833-4c56-9762-cd3b6fc71884` succeeded and
  retained its encrypted terminal output.
- No guidance or Provider body entered the RoundTable or Attempt Loop Journal.
- After a full App/managed-daemon restart (`83067/83081` to `99130/99169`), the
  Session restored the exact digest
  `5e35267e3b83eaafb1856be52c274f80353dacf8253f2fd36dce864e2301be46`,
  both terminal Attempts and the accepted Steer intervention.

Session `rt-20260827-0042` used OpenCode/MiniMax. An explicit Retry started a
new Attempt, and live Steer returned `capability_gap` in 3.55 seconds. No Steer
fact was appended and the user recovery path remained Retry after completion.

Earlier installed Phase 4 sessions retain the accepted Pause, independent
Retry, candidate acceptance, AlignmentSummary publication, restart restoration
and local Export/Import gates. The retained private export is
`rt-20260826-1958-build198.loom-roundtable`.

## Verification

- `go vet ./...`
- complete macOS suite: 394 XCTest cases, two conditional skips, zero failures
- Swift Testing: 20 contract tests, zero failures
- focused Runtime-wrapper, concurrent ingress, capability-gap and IPC deadline
  regressions
- strict deep bundle signature verification and transactional installation

The complete Go repository rerun is recorded in `docs/CURRENT.md` after the
five-observation IPC deadline fixture was corrected from its stale capacity of
four.
