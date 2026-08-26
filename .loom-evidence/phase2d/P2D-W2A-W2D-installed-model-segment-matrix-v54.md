# P2D-W2A/W2D Installed Model, Segment, and Conversation-First UI Evidence

Status: `VERIFIED SLICE / PHASE 2D PARTIAL`

Date: 2026-08-21

Scope: installed conversation catalog, immutable route metadata, conversation
continuity, service startup, Provider/Runtime projection, and conversation-first
empty-state UI. This evidence contains no credential, prompt, response body,
Authorization header, Provider body, endpoint fingerprint, or private path
content beyond the installed App location.

## Installed matrix

- Bundle: `/Users/lune/Applications/Loom.app`
- Full route matrix bundle: Loom `0.5.3 (73)`
- UI and startup recheck bundle: Loom `0.5.3 (74)`
- Context Meter vertical-slice bundle: Loom `0.5.3 (75)`
- Model/reasoning cells: `28 passed / 28 required`
- Native DeepSeek, native MiniMax, OpenCode DeepSeek/MiniMax, and Codex/OpenAI
  advertised cells all returned the exact bounded acceptance value.
- Every executed cell created exactly one immutable Conversation Segment and
  froze the expected profile ID, model ID, and reasoning effort metadata.
- One visible Conversation switched from Codex/OpenAI to Loom Native/DeepSeek
  through a new Segment; the prior Segment was not rewritten.
- Two independent sessions retained distinct thread/Segment continuity.

## Build 74 installed recheck

- App and bundled daemon launched together; no separate user daemon start was
  required.
- The bundled daemon ran with canonical state, isolation, socket, Codex,
  OpenCode, and managed-parent arguments.
- Setup snapshot completed in 1.234 seconds.
- Providers projected: `25`.
- Runtimes projected and online: `4` (`Loom Native` x2, `OpenCode`, `Pi`).
- Conversation Profiles projected: `4` (`DeepSeek`, `MiniMax`, `OpenCode`,
  `Codex/OpenAI`).
- Vault-backed DeepSeek and MiniMax accounts remained verified.
- One installed real DeepSeek conversation received a non-empty network reply.
- Main-window visual inspection showed `Local service ready`, `New conversation`,
  governance counters, the exact DeepSeek Provider Account, and `DeepSeek Chat`.
  The empty state no longer requires Mission or Team setup before conversation.

## Build 75 Context Meter recheck

- Capsule token budget and admitted token count are projected into each new
  Conversation Segment and Attempt and included in the schema-5 binding digest.
- Legacy records without token projection remain readable as 0/0; partial,
  inverted, over-budget, or digest-drifted projections fail closed.
- The installed probe requires a positive bounded Attempt projection before it
  accepts a real reply.
- Installed DeepSeek returned `96 / 8192` estimated tokens with a successful
  reply while the setup snapshot still projected 25 Providers, 4 online
  Runtimes, and 4 Conversation Profiles.
- UI copy says `Estimated shared context`; the value is the Loom Capsule budget,
  not a claim about full Provider model context capacity.

## Source regression

- Affected Go Provider/API/daemon contract suites: pass.
- Swift XCTest suite: `291 passed`, `1 intentional skip`, `0 failed`.
- Swift Testing strict wire-contract suite: `15 passed`, `0 failed`.
- Bundle signature verification: pass.
- Source diff whitespace validation: pass before installation.

## Remaining Phase 2D gates

This slice does not complete Phase 2D. Still open are the four-Harness,
four-Provider installed Team; full account-local auth/rate-limit/timeout/
corruption/revision failure matrix; explicit fallback approval and accounting
UI; installed MCP mid-flight revocation; and the complete Context Capsule
disclosure, omission, trust-domain, restart, and encrypted-storage matrix.
