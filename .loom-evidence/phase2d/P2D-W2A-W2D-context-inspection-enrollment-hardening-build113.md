# Phase 2D Build 113 Context Inspection and Enrollment Hardening

Status: `VERIFIED SOURCE + PARTIAL INSTALLED LIVE`

Date: 2026-08-23

## Candidate and process identity

- Installed bundle: `Loom v0.5.3 (113)`.
- App SHA-256: `d36615b118ad357cb40ad7bd60777ee316c51bb723f45bbcf9ef2042ff7feca4`.
- daemon SHA-256: `8e61a4dab0d580fdae1ecf40c8218dac17ed79bd929e50f7d7b533473d6ffe17`.
- Reproducible build fixture, strict nested signing, installer rollback fixture,
  install dry-run and transactional install passed.
- Cold restart replaced daemon PID `56670` with PID `57224`; the new daemon's
  parent is installed App PID `57191` and its canonical argv contains the
  expected state, isolation, Socket, Runtime and managed-parent boundaries.

## Context disclosure result

- The new `chat_context_disclosure` route resolves an exact persisted
  Conversation Segment and exact encrypted Capsule/receipt authority.
- The route returns only allowlisted metadata: category, trust, scope,
  estimated token count, omission reason and scoped-retrieval eligibility. It
  never returns Context content, Prompt, Provider body, source refs or secret.
- Swift decoding rejects unknown fields at the response, item, thread message,
  Segment and Attempt boundaries. Canonical server time fields remain accepted.
- The installed App created fixed thread
  `thread-build113-restart-20260823t005000z` with four DeepSeek/OpenCode route
  Segments. Every Segment's disclosure route passed. After cold restart, a
  verify-only read recovered the same Segment, Capsule digest, receipt and
  omission metadata without another Provider call.
- A real installed OpenCode conversation returned the exact bounded acceptance
  marker; setup remained 25 Providers, 7 Runtimes and 4 Conversation Profiles.
- Native visual inspection was not run because macOS was locked. Source tests
  verify the inspector states, accurate retrieval copy and VoiceOver labels;
  installed click/visual review remains open.

## Enrollment result commit hardening

- Enrollment-backed Web/MCP capability is no longer globally published; it is
  available only for the exact frozen Enrollment identity and digest.
- Result ordering is revalidate, encrypted put, revalidate, then accept. If the
  second check observes revoke or policy drift, the exact encrypted payload is
  deleted and no result-accepted/completed fact is committed.
- Deterministic tests prove one encrypted put, one exact rollback and zero
  remaining payload. Cleanup failure returns a content-free rollback code.
- The installed MCP gate reached the real private stdio helper, revoked the
  exact Enrollment in flight, terminalized Attempt 1 and consumed explicit
  fallback approval version 1. MiniMax Attempt 2 was dispatched and ended at
  the previously observed account-local `provider_http` stage. This proves the
  hardening and fallback dispatch, not terminal fallback success.
- A second explicit DeepSeek fallback run did not reach the helper because the
  OpenCode model omitted the required ToolCall. The live test now fails fast on
  a terminal fallback failure and supports an explicit fallback Provider; it
  never silently changes the route.

## Verification

- `go test -p 4 ./... -count=1`: pass.
- focused Go race tests for revoke/commit/disclosure: pass.
- `go vet ./...`: pass.
- macOS XCTest: 313 executed, 1 intentional skip, 0 failures.
- strict Swift wire contracts: 16 pass.
- `scripts/test-build-loom-local-app.sh`: pass.
- `scripts/test-install-loom-local-app.sh`: pass.
- `scripts/test-phase2d-live-acceptance.sh`: pass.
- `git diff --check`: pass.

No credential, Prompt, conversation body, Provider response, MCP arguments or
tool result body is included in this evidence.
