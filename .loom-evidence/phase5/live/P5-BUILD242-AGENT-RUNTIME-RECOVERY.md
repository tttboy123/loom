# Phase 5 Build 242 Agent Runtime recovery

Status: `INSTALLED CHECKPOINT / PHASE 5 IN PROGRESS`

## Boundary

This checkpoint shortens governed Agent Runtime authority and history recovery
after Conversation and local IPC are already usable. It preserves fail-closed
frozen-binding validation, payload integrity, RoundTable restart continuity and
the Build 241 transient-preparation experience.

## Installed identity

- App version: `0.5.3`
- App build: `242`
- App bundle: `/Users/lune/Applications/Loom.app`
- Source branch: `codex/phase4-roundtable-governed-deliberation`
- App executable digest: `e2bc570b005c5fc13b833567598393c95b95014c7d6ae0fe5cfbf6f23de5ddfa`
- Bundled daemon digest: `87bfc6e7f96a36b37bda4237880bcd08772b83d5526d62c1844d179d9fae947a`
- Strict bundle signature: passed
- Packaged and installed executable digests: exact match

## Change

- Restart recovery validates each Attempt Loop's frozen claim, Runtime, Agent,
  execution-binding and capability identities before deciding whether it is a
  running recovery candidate.
- Terminal historical Attempts no longer trigger a second exact Run replay.
- Delivered payload reconciliation caches exact Run validation by the complete
  comparable frozen payload authority for one reconciliation pass. Distinct
  Incident or binding authority cannot share the result.

## Installed cold-start distribution

Run 1:

- Launch request: `2026-08-27T08:40:25Z`
- External Runtime catalog: 6,199 ms
- Agent Runtime aggregate: 18,128 ms
- Approximate authority/history work after catalog: 11.9 seconds

Run 2:

- Launch request: `2026-08-27T08:41:31Z`
- External Runtime catalog: 3,868 ms
- Agent Runtime aggregate: 12,732 ms
- Approximate authority/history work after catalog: 8.8 seconds

Preserved Build 241 baseline samples are 39,931 ms and 56,274 ms for the Agent
Runtime aggregate. Build 242 therefore materially shortens the observed restart
path, but does not claim that all governed history restoration is instantaneous.

## Installed UX acceptance

- Performed two complete App and daemon exits followed by cold launches.
- Conversation remained mounted with a visible, usable composer.
- The selected Mission-linked RoundTable restored automatically to `Concluded`.
- Both Agent results and their frozen Loom Native/MiniMax routes remained visible.
- No Retry, navigation, raw Swift error or manual daemon start was required.
- Normal execution remained on Loom Vault; diagnostics reported zero credential
  helper spawn attempts.

## Automated gates

- Complete macOS package: 417 XCTest cases passed, two conditional skips.
- Swift Testing contracts: 20 passed.
- Complete `internal/work` and `cmd/loomd` packages passed.
- Focused race coverage for the modified restart/payload paths passed.
- Complete Go repository passed with serial package execution (`-p=1`).
- `go vet ./...` passed.
- `git diff --check` passed.
- A parallel complete Go run exposed five process-timeout failures under resource
  contention; every failed case passed immediately in isolated serial reruns,
  and the complete serial repository run passed. This failed run is retained as
  environmental evidence rather than represented as a pass.

## Privacy

This record contains only non-secret identities, timings, digests and outcome
metadata. It excludes API keys, Authorization headers, prompts, transcript,
Agent output, Provider bodies and hidden reasoning.

Build 242 is an installed Phase 5 checkpoint, not Phase 5 completion.
