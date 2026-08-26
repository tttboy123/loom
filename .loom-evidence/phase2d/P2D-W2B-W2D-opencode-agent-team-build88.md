# P2D-W2B/W2D OpenCode Agent Team - Build 88

Status: `CURRENT / VERIFIED SLICE`

Date: 2026-08-22

Parent Goal: `Phase 2D - Multi-Provider, Multi-Model Agent Team Orchestration`

## Installed boundary

- App: `/Users/lune/Applications/Loom.app`, version `0.5.3`, build `88`
- App executable SHA-256:
  `c9410b15be12a5ebf3c8a025b047a8797236d8565f7703009de5e4b720406f85`
- Bundled daemon SHA-256:
  `289fbee4195949819f47a5e55fd331621a860f2370e5caef177a5b1228dcc981`
- The App was the managed parent of the bundled daemon. Canonical daemon
  arguments included state, isolation, Socket, Codex, OpenCode and managed
  parent identities.
- Product Socket, daemon diagnostics and App diagnostics were owner-only
  `0600`. No credential body was read or recorded.

## Architecture correction

The prior Team catalog treated OpenCode both as a Harness and as a Provider,
then selected an OpenCode-hosted default model. The hosted route exited before
useful Agent output and surfaced a bounded `harness_unavailable` diagnostic.

Build 88 preserves OpenCode as the Harness while resolving the execution
binding from a verified Provider Account. The installed live slice froze:

- Harness: `opencode`
- Provider: `deepseek`
- Provider Account: `deepseek.primary`
- Model: `deepseek/deepseek-chat`
- Auth: brokered Vault credential with an exact positive revision

OpenCode Team role IDs include both Harness and Provider identity, so they do
not collide with Loom Native roles using the same Provider Account. Runtime
availability is derived from the attested OpenCode workspace capability, not a
hosted model catalog entry.

## Installed live result

- `TestLiveOpenCodeAgentTeamE2E`: PASS against build 88.
- The Mission preflight admitted both independently bound roles.
- The Loom Native/DeepSeek node and OpenCode/DeepSeek node completed with
  separate Harness, model, frozen binding, Context Capsule, Route Segment,
  disclosure and accounting provenance.
- `TestLiveOpenCodeConversationE2E`: PASS against build 88 with the exact
  bounded acceptance reply.
- Installed catalog integrity remained 25 Providers, 4 Runtimes and 4
  Conversation Profiles.
- A transient Agent recovery state now projects as `awaiting_recovery` at the
  Mission level instead of flashing a terminal `failed` state while governed
  recovery is still active.

## Route-context regression gate

Build 86 passed the complete 28/28 installed model/reasoning matrix, the
same-visible-Conversation route switch, and independent multi-session
continuity after Context Capsule content was separated from user messages.
Build 88 retains that source and additionally passed the focused installed
OpenCode conversation and Agent Team gates.

## Source and package verification

- Full `go test ./... -p 1 -count=1`: PASS.
- `go vet ./...`: PASS.
- macOS suite: 291 tests, 1 intentional skip, 0 failures.
- Strict Swift wire-contract suite: 15/15 PASS.
- Native App builder fixture: PASS across two reproducible release builds.
- Builder contract explicitly resolves `node`, `codex` and `opencode` into the
  bundled service PATH.
- `git diff --check`: PASS after the implementation and evidence updates.

## Remaining Phase 2D gates

This evidence closes the OpenCode conversation and OpenCode Agent Team slice,
not Phase 2D. The four-Harness/four-Provider installed Team, remaining
account-local failure cells, approved fallback, complete accounting/governance
UI, full Capsule disclosure/encryption matrix and unlocked installed visual
walk-through remain open.
