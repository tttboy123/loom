# ADR-0022: Harness Gateway and Segment Session

**Date**: 2026-08-24
**Status**: accepted
**Deciders**: Product Owner, Loom Architecture Controller

## Context

Loom already freezes Provider Account, credential revision, model, Context
Capsule, policy and capabilities per Conversation Segment or Agent Attempt.
Runtime execution is still split across independent Conversation responders,
Agent adapters and process helpers. Codex Conversation starts a new ephemeral
CLI process for every turn, ordinary Conversation sends are globally
serialized, and there is no Response-scoped cancellation command.

## Decision

1. Loom adds one in-process, built-in Harness Gateway below existing governance
   admission. The Gateway consumes frozen authority; it never selects a
   Provider Account, credential revision, model, policy or disclosure.
2. Every executable Harness is represented by a versioned `ConfiguredHarness`.
   Phase 2D registers exactly Codex, Claude Code, OpenCode, Pi and Loom Native
   through a closed `BackendRegistry`. Arbitrary dynamic plugins remain out of
   scope.
3. A `SegmentSession` is keyed by the exact Conversation, immutable Segment,
   Configured Harness version, Workspace binding and frozen execution-binding
   digest. A changed Provider, account, credential revision, model, policy,
   Context Capsule route or Workspace creates a different Session boundary.
4. A Session may execute one Response at a time. Different Conversations may
   execute concurrently. The Gateway rejects duplicate Response IDs and stale
   binding substitution.
5. Response cancellation cancels only the exact active Response context. The
   owning Session and Workspace remain available for a later turn unless the
   Backend declares the Session unhealthy. Segment, Conversation, peer Session
   and daemon lifecycle are not cancelled by a Response command.
6. Session lifecycle is `Opening -> Ready -> Responding -> Ready -> Draining ->
   Closed`, with terminal `Failed`. Response lifecycle is `Started -> Completed
   | Failed | Cancelled`. Session opening and cleanup use independent
   Gateway-owned bounds; a Backend result returned after the opening deadline
   is cleaned up and never becomes Ready. All emitted events are versioned,
   ordered and content-free.
7. Events may contain opaque Conversation/Segment/Session/Response/Incident
   IDs, Harness/backend versions, stages, result codes, retryability and
   elapsed time. They never contain Prompt, transcript, Context Capsule body,
   Provider body, credential, Authorization header, native session secret or
   Workspace path.
8. Backend-native session handles remain bound to exact Session authority and
   are encrypted when persisted. They cannot cross Conversation, Segment,
   Provider Account, model or credential revision.
9. Codex is the first complete backend: one App Server process and one Codex
   thread are reused for multiple turns in the same Segment and Workspace.
   Switching to DeepSeek keeps the visible Loom Conversation and creates a new
   Segment/Session. Different Conversations can run in parallel.
10. Claude Code, OpenCode, Pi and Loom Native must enter through the same
    Configured Harness and Registry contracts. A compatibility Backend may
    initially wrap an existing responder, but direct production dispatch that
    bypasses the Gateway is not an accepted final state.
11. Gateway/Session references are bounded capabilities under ADR-0021. Journal,
    Vault root, policy/grant authority, state writers and terminal Attempt
    authority remain protected core capabilities and cannot be registered as a
    Backend.

## Consequences

- Session reuse and cancellation become explicit Loom lifecycle semantics
  rather than CLI-specific side effects.
- The Gateway must own per-Session synchronization and Response cancellation;
  `LocalProductChatAPI` can no longer use one global send lock.
- Workspace identity becomes part of frozen Segment execution authority while
  the raw path remains outside diagnostics and public projections.
- Build 127 remains valid historical evidence for Route/Context governance but
  does not complete this amended Phase 2D objective.

## Acceptance

- Contract, race and restart tests cover Registry immutability, Session-key
  substitution, event ordering, same-Segment reuse, cross-Conversation
  parallelism, exact Response cancellation and privacy-negative output.
- Installed Loom proves two Codex turns reuse one Session and Workspace;
  Codex-to-DeepSeek creates a second Segment in the same visible Conversation;
  two Conversations overlap; cancellation terminates only the selected
  Response; all frozen binding fields remain unchanged.
