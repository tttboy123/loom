# Phase 5 Build 247 recoverable errors and result summaries

Status: `INSTALLED CHECKPOINT / PHASE 5 IN PROGRESS`

Date: 2026-08-28

## User outcome

RoundTable remains usable beside Conversation when an operation fails or an
Agent returns a long result. Both RoundTable surfaces now present the same safe
operation stage, retryability, recovery action and Incident ID. Long Agent
results keep the conclusion beginning and latest action visible without
exposing raw Markdown or partial words; users can expand and collapse the exact
full result in place.

Diagnostic-preview and workspace-folder selection failures are no longer
silent. The App retains a visible alert with a retry or explicit setup recovery
action.

## Source verification

- `swift test --package-path apps/macos`: 426 XCTest cases passed, two
  conditional skips, plus 20 Swift Testing contract cases passed.
- Focused RoundTable preview tests cover full-result expansion, link removal,
  summary-plus-tail capacity, Markdown syntax across the cut and complete-word
  boundaries.
- Structured RoundTable store tests preserve remote stages and Incident IDs and
  use the request Correlation ID for local UDS transport failures.
- SwiftUI source tests cover both RoundTable error surfaces, diagnostic-preview
  recovery and folder-selection retry.
- `go test ./internal/localipc ./cmd/loomd` passed before the final UI-only
  preview refinement (`internal/localipc` 300.947 s; `cmd/loomd` 257.542 s).
- `go vet` for the changed daemon boundary and `git diff --check` passed.

## Package identity

- Version: Loom `0.5.3` Build 247.
- Installed path: `/Users/lune/Applications/Loom.app`.
- App executable SHA-256:
  `ff7afa968e9452ad6e9cbdd5899503168fd91e78c8456b5e73ea7dc3fc421b14`.
- Bundled daemon SHA-256:
  `87bfc6e7f96a36b37bda4237880bcd08772b83d5526d62c1844d179d9fae947a`.
- Staged and installed hashes match. Strict deep signature verification passes.
- The installed daemon runs with the App PID as its managed parent and the
  canonical private state, isolation and UDS arguments.

## Installed product checks

The concrete signed Swift setup probe reached the installed daemon over its
private UDS and decoded 24 Providers, seven Runtimes and seven Conversation
Profiles. Provider rows include OpenAI, Anthropic, DeepSeek, Kimi, MiniMax and
the compatible endpoint catalog. Runtime rows include Codex, Claude Code,
OpenCode, Pi and Loom Native variants. OpenCode combinations remain Routes,
not duplicate Provider Accounts.

The installed accessibility and visual loop verified:

1. The saved Mission-linked RoundTable restores without navigation or Retry.
2. Both MiniMax Agent headers, frozen routes, statuses and bounded summaries
   are visible together beside the still-mounted Conversation.
3. Neither `**` Markdown markers nor cut words are exposed at either summary
   boundary.
4. `Show full result from <Agent>` expands the complete result and changes to
   `Show less from <Agent>`; collapsing restores the bounded summary.
5. After a complete App and daemon restart, the same RoundTable, both collapsed
   summaries, Route/Model controls and focused Conversation composer return.

## Remaining Phase 5 gates

- Complete end-to-end keyboard traversal requires a macOS test session with
  Full Keyboard Access enabled.
- Structured error presentation is source-verified; installed fault-injection
  acceptance across every Conversation, Mission and RoundTable operation is
  still open.
- Repeated startup distribution and the remaining Agent Runtime authority and
  history restoration cost remain optimization work.

Build 247 is an installed Phase 5 checkpoint. It does not close Phase 5, Phase
2D or Phase 4 again, and it does not claim the explicitly deferred four-real-
Provider Team, real-account revoke/rate-limit or custom-endpoint live matrix.
