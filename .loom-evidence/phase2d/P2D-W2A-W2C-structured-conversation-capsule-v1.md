# P2D-W2A/W2C Structured Conversation Capsule v1

**Date**: 2026-08-12  
**Status**: `CURRENT / SOURCE VERIFIED / POST-BUILD-64`  
**Goal**: unified Phase 2D only  
**WorkItems**: existing P2D-W2A and P2D-W2C

## Result

Ordinary Conversation dispatch now uses the same structured, content-addressed
Role Context Capsule value object and encrypted Conversation-DEK Store as Team
Agent dispatch. A Provider route receives one canonical Capsule prompt rather
than a hand-built summary string or unfiltered message history.

Each current or prior user turn is authoritative conversation-shared context.
Prior visible model output is always `untrusted_model_output`. In
`summary_only`, the output is represented by a content-free
`policy_filtered` omission; in `continue_with_context`, it remains explicitly
untrusted. `start_clean` includes only the current user turn. Hidden reasoning,
credentials, Provider bodies, and local failure messages do not enter the
Capsule.

The daemon resolves the exact Capsule target from the selected immutable
Conversation Profile: Harness ContextAdapter, Provider, exact Provider Account,
Model, auth mode, and Provider Account disclosure policy digest/version. A
cross-account or stale Profile cannot resolve a target.

## Encrypted transaction boundary

The production Vault path persists the canonical Capsule body and dispatch
payload before thread metadata. If Capsule encryption fails, no user message,
Segment, Attempt, or Provider call is committed. If encrypted thread document
commit fails after Capsule persistence, Loom rolls back the in-memory thread
and deletes only the ciphertext that matches the complete Capsule Authority.
Authority substitution fails closed, deletion is idempotent, and nonce history
is retained to prevent reuse.

Restart tests close and reopen the real LocalKeyFile Vault and recover the same
Capsule Authority, trust classes, policy-filtered omission, and dispatch bytes.
Raw database scanning finds none of the user context or prior model output.

## Compatibility and bounds

- Installed Vault composition enables the structured Conversation Capsule;
  explicit legacy constructors retain their existing test/compatibility path
  and do not gain plaintext fallback.
- Codex and local Pi default responders remain supported. Pi Capsule payloads
  are bounded below the Pi adapter's 8 KiB prompt envelope.
- Existing Conversation binding schema v3 and Swift IPC models do not change.
- Build 64 does not contain this source. No Candidate or installed App was
  launched or modified.

## Verification

- Focused transaction, exact-account target, restart, non-disclosure, Pi, Codex,
  and daemon journey tests pass.
- Affected four-package race tests pass for three runs.
- Complete `go test ./...` and `go vet ./...` pass.
- Complete Swift passes 202 XCTest cases with one intentional visual-export
  skip and zero failures, plus nine Swift Testing contracts.
- `git diff --check` passes.

The existing Swift 6 `QueueCommand<Input>: Sendable` warning remains unrelated,
non-blocking debt.

## Open gates

W2A/W2C remain `ACTIVE / PARTIAL`. Model-specific tokenizer accounting,
Provider-specific multi-message/tool-result mapping, scoped retrieval,
user-visible full receipt inspection, parallel sibling/Aggregation Attempts,
encrypted context export/restore, and installed real-Provider restart/mixed-Team
acceptance remain open. Installed Loom remains v0.5.2 build 39.

## Postscript: scoped retrieval source

The daemon-side encrypted omission store and exact Attempt-bound retrieval
broker are now source-verified in
`P2D-W2C-scoped-context-retrieval-broker-v1.md`. The remaining scoped-retrieval
gates are Pi, Codex, and Claude Code result transports plus persisted delivery;
Loom Native DeepSeek/Kimi/MiniMax now have one bounded two-round Adapter proof.
Their tool is published only when `context_retrieval` is frozen in the exact
Attempt binding; runtime discovery and generated Profiles now carry that
capability without mutating historical Team bindings.
All other open gates above remain unchanged.
