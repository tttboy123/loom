# P2D-W2C Attempt Payload Store v1

**Status**: `SOURCE VERIFIED / PARTIAL`  
**Date**: 2026-08-13  
**Goal**: unified Phase 2D only  
**Source boundary**: post-build-64; installed Loom remains v0.5.2 build 39

## Implemented boundary

The Loom-owned Credential Vault now has an encrypted Attempt Payload Store for
bounded tool-result and continuation payloads. Each row is bound by canonical
AAD to Conversation, WorkItem, Run, claim generation, Runtime, frozen execution
binding digest, Context Capsule digest, payload/call ID, sequence, content type,
content digest, and authenticated `pending` or `delivered` status.

Payload content is encrypted with the existing per-Conversation DEK. Every
write and status transition uses a fresh AES-GCM nonce reserved in the shared
Conversation nonce registry. `pending -> delivered` decrypts and authenticates
the old row, then re-encrypts under a fresh nonce; a database-only status edit
therefore fails authentication. Content is returned as mutable bytes and the
caller can zeroize it with `Close`.

The Store supports idempotent pending writes, exact reads, deterministic
pending-list recovery for one frozen Attempt scope, authenticated delivery
transition, exact deletion, Vault restart, VMK rewrap rotation, and
Conversation-local crypto-erasure. Reusing a Run/generation/sequence, changing
the idempotency content, or drifting generation, Runtime, binding, Capsule,
digest, AAD, tag, or status fails closed. A stale scope for an existing Run is
reported as binding drift rather than silently appearing empty; a delivered
queue for the exact scope is correctly empty.

## Non-disclosure and isolation

- SQLite scans do not contain the test payload plaintext.
- The Event Journal, Board, Evidence, diagnostics, argv, environment, Prompt,
  and Provider body are unchanged by this source slice.
- Deleting one Conversation removes its Attempt payload rows through DEK
  crypto-erasure and leaves peer Conversations readable.
- Vault wrapping-key rotation preserves payload readability without
  re-encrypting each payload body.

## Verification

- focused Attempt Payload tests: 10 consecutive runs passed;
- focused Attempt Payload race tests: 20 consecutive runs passed;
- complete Credential Vault race suite: 10 consecutive runs passed;
- full repository `go test ./...` passed;
- repository `go vet ./...`, `go mod verify`, and `git diff --check` passed.

## Open boundary

This is storage infrastructure, not a claim that crash resume is complete.
`contextcapsule.Retriever` and the Pi, Codex, Claude Code, and Loom Native
transports do not yet persist a result before delivery or acknowledge delivery
through one common interface. Journal facts such as `ToolResultAccepted`,
`ToolResultDelivered`, and `AttemptResumed` are not added by this increment.

The next slice must connect the Store to a daemon-owned delivery coordinator so
payload commit precedes Harness delivery, acknowledgement precedes the
authenticated delivered transition, and restart resumes the same generation
without re-running a tool. General multi-tool loops, sibling/Aggregation
Attempts, encrypted export, installed CV6, and live mixed-Team acceptance also
remain open. No bundle, installed App, real credential, or Provider was touched.

MCP HTTP, Pi UDS, and Provider HTTP writes do not provide one common
application-level consumption acknowledgement. The first coordinator slice must
therefore state at-least-once delivery explicitly and keep side-effecting tool
re-execution forbidden; exactly-once may be claimed only after Journal facts
and a transport-specific acknowledgement/continuation fence close that gap.
