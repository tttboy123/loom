# Phase 5 Build 241 Conversation-ready startup

Status: `INSTALLED CHECKPOINT / PHASE 5 IN PROGRESS`

## Boundary

This checkpoint verifies that ordinary Conversation becomes available before
external Harness discovery and governed Agent Runtime restoration complete. It
also verifies that a persisted Mission-linked RoundTable opened during that
preparation restores automatically without losing its selected context.

## Installed identity

- App version: `0.5.3`
- App build: `241`
- App bundle: `/Users/lune/Applications/Loom.app`
- Source branch: `codex/phase4-roundtable-governed-deliberation`
- App executable digest: `ddf808669354807c8988a45c1207c4ea4df8595709a499577a3b90b2dfe928ee`
- Bundled daemon digest: `0eef5e2753aa3c1be6b1f91f6a7d9b793826dbce4d240083912553a4ae79feb9`
- Strict bundle signature: passed
- Packaged and installed executable digests: exact match

## Cold-start timeline

- Composition start: `2026-08-27T07:49:38.701Z`
- Core start: 1,649 ms
- Local IPC scope ready: `2026-08-27T07:49:40.738Z`, about 2.04 seconds after
  composition start
- External Harness catalog refresh: 4,105 ms, completed at
  `2026-08-27T07:49:44.848Z`
- Agent Runtime aggregate: 39,931 ms, completed at
  `2026-08-27T07:50:20.639Z`

The Agent Runtime aggregate includes its admission wait and catalog refresh.
About 35.8 seconds of authority/history restoration remained after catalog
refresh. This is an open Phase 5 performance problem, not a completed claim.

## Installed UX acceptance

- Cold-launched the installed App and observed a focused Conversation composer.
- Opened the persisted Mission RoundTable before Agent Runtime restoration
  completed.
- Observed `Preparing the Agent Team`; no raw protocol or Swift error was shown.
- Did not click Retry, leave the inspector or restart the App.
- The same inspector automatically transitioned to the concluded RoundTable.
- Both Agent results and their frozen Loom Native/MiniMax routes were restored.
- Conversation remained mounted and usable throughout the governance restore.

## Rejected predecessor

Build 240 moved Harness discovery behind IPC readiness, but its installed cold
loop exposed `LocalProductClientError error 4` and required a manual Retry.
Authoritative state was healthy and the same discussion restored immediately
after that Retry. Build 240 is therefore evidence of a failed UX acceptance,
not the current installed candidate.

Build 241 classifies local unavailability and timeout during bounded RoundTable
restore as transient preparation. It retries for up to 90 seconds, surfaces an
actionable Retry state after that bound, and still fails immediately for
deterministic request, identity or response errors.

## Automated gates

- Complete Swift package: 417 XCTest cases passed, two conditional skips.
- Swift Testing contracts: 20 passed.
- Complete `cmd/loomd` package passed after the daemon startup change.
- Focused startup race tests passed.
- Complete Go repository and `go vet ./...` passed before the final Swift-only
  recovery-classification change; no Go source changed afterward.
- `git diff --check` passed.

## Privacy

This record contains only non-secret identities, timings, digests and outcome
metadata. It excludes API keys, Authorization headers, prompts, transcript,
Agent output, Provider bodies and hidden reasoning.

Build 241 is an installed Phase 5 checkpoint, not Phase 5 completion.
