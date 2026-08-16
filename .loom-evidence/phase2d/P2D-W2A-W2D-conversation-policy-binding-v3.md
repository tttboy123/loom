# P2D-W2A/W2D Conversation Policy Binding v3

**Date**: 2026-08-12  
**Status**: `CURRENT / SOURCE VERIFIED`  
**Goal**: unified Phase 2D only  
**WorkItems**: existing P2D-W2A and P2D-W2D

## User-visible result

A Conversation remains one visible Loom Conversation when its Provider route or
Provider Account disclosure policy changes. Loom creates a new immutable
Segment after explicit context review instead of rewriting the previous Segment
or silently opening an unrelated Conversation.

Each new Segment and its Attempt freeze the daemon-resolved Provider ID, exact
Provider Account, policy schema/version, policy revision, policy digest, trust
domain, retention mode, and data region. The native disclosure inspector shows
that frozen policy beside the Segment rather than substituting the account's
current settings.

## Authority and race closure

The Swift review flow sends the exact non-secret execution binding that the user
confirmed as `expected_execution_binding`. The daemon independently resolves the
current authoritative binding and compares every field before it appends the
user message, creates an Attempt, acquires a credential lease, or calls a
Provider. A changed policy is a fail-closed Conversation route conflict and must
be reviewed again.

This closes the confirmation-to-dispatch race without putting a credential
reference, credential revision, secret, Prompt, message body, Authorization
header, or Provider response into the execution binding. Existing schema-2
Conversation records retain their original digest. Their next turn creates a
schema-3 Segment without rewriting history.

After a daemon route conflict, the App reloads the authoritative thread while
preserving the same Conversation ID, the user's selected target Profile, the
draft, Incident ID, and explicit recovery action. Retry returns to Segment and
context review. It does not silently mutate the old route or claim that a new
Conversation was opened.

## Verification

- Focused Go API and daemon tests pass for exact freezing, legacy upgrade,
  current-policy dispatch, stale-confirmation rejection before mutation, and
  Router revalidation.
- The focused Go race matrix passes five runs.
- Complete `go test ./...` and `go vet ./...` pass.
- Complete Swift passes 202 XCTest cases with one intentional visual-export
  skip and zero failures, plus nine Swift Testing contracts.
- Swift Store tests cover restart policy drift, conflict recovery, stale thread
  hydration, draft preservation, same Conversation identity, and exact expected
  binding transport.
- Swift IPC and model tests cover the exact wire key, strict binding schema,
  safe Attempt failure decoding, and substitution rejection.
- Native route-review UI tests cover Send and Retry preflight plus light/dark and
  accessibility-size rendering.
- `git diff --check` passes.

The existing Swift 6 forward-compatibility warning for
`QueueCommand<Input>: Sendable` remains unrelated, non-blocking debt.

## Open gates

The production increment is frozen as unlaunched, uninstalled v0.5.2 build 64.
Two independent builds and one ZIP extraction are byte- and mode-identical; the
exact artifact hashes are recorded in
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build64-candidate-2026-08-12/BUILD-MANIFEST.md`.

This result does not satisfy CV6. Installed Loom remains v0.5.2 build 39. Real
installed Codex to DeepSeek to Anthropic Segment transitions, Provider replies,
restart continuity, mixed-Team dispatch, real accounting, approved fallback,
and counter-backed no-Keychain observation remain open under the sole
`ACTIVE / PARTIAL` Phase 2D Goal.
