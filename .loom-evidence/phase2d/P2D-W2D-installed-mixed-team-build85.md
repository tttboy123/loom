# P2D-W2D Installed Mixed-Team Completion - Build 85

Status: `CURRENT / VERIFIED SLICE`

Date: 2026-08-21

Parent Goal: `Phase 2D - Multi-Provider, Multi-Model Agent Team Orchestration`

## Installed boundary

- App: `/Users/lune/Applications/Loom.app`, version `0.5.3`, build `85`
- App SHA-256: `9160faa90aac1e6eaa37bd7c599309986b4c33050f32180d8617a9703fa4fe27`
- Bundled daemon SHA-256: `6fc98e39e036f60513b259c1b05cbee2e27d59331c52fa8f6fd4f1621c6ef1c1`
- App PID was the daemon parent; canonical daemon argv included state,
  isolation, Socket, Codex, OpenCode and managed-parent identities.
- Product Socket and operational diagnostics were owner-only `0600`.
- Credential Vault was unlocked. No credential body was read or recorded.

## Live Team result

- Team instance: `team-instance-6bfab51cf9068b0f084f236e59e6fd7f`
- Incident/correlation: `022b2505-4b19-4b53-95b7-d13f04a7b300`
- Preflight: `4/4 ready`
- Terminal result: `4/4 succeeded` in approximately 31 seconds
- DeepSeek account: 2 Attempts, 0 failed, usage/accounting/cost 2/2/2,
  17,533 total tokens, policy revision 1
- MiniMax account: 2 Attempts, 0 failed, usage/accounting/cost 2/2/2,
  5,234 total tokens, policy revision 1
- Every node projected an independent Execution Binding, Context Capsule,
  Route Segment, disclosure receipt and Provider Account policy.

## Denied Context closure

The previous installed attempt produced a valid Agent answer after a denied
Context read, but the encrypted Vault returned its storage-level missing-record
error. The Attempt loop had committed dispatch without accepted/delivered
result facts and therefore failed finalization.

Build 85 retains the storage cause while mapping it to the bounded Context
domain result. In the live run, each denied Context path produced the complete
authority sequence:

1. `ToolCallAdmitted`
2. `ToolDispatchCommitted`
3. `ToolResultAccepted`
4. `ToolResultDelivered` with Provider-continuation proof

The result body remained encrypted and content-free outside Provider
continuation. ACL denial was not relaxed.

## Catalog and OpenCode gate

The installed setup snapshot reported:

- 25 Providers
- 4 Runtimes
- 4 Conversation Profiles
- OpenCode Runtime present
- `conversation-opencode-default-v1` present

The installed OpenCode conversation returned exact `E2E-OK` through a Vault
credential lease.

## Privacy and verification

- All four source Evidence captures exist and contain no `<think>` blocks.
- Journal and operational diagnostics contain non-secret identifiers, stages,
  digests and accounting only; no credential, Authorization header, Prompt or
  Provider body was added by this evidence.
- Relevant Go suites passed for Context Capsule, Credential Vault, native
  Runtime, Provider, Projection, API, App and daemon packages; the focused Go
  vet boundary also passed.
- The macOS suite passed 291 XCTest cases with 1 intentional skip and 0
  failures, plus 15 strict wire-contract tests.
- Installed live gates passed:
  `TestLiveMixedProviderTeamE2E` and `TestLiveOpenCodeConversationE2E`.

## Remaining Phase 2D gates

This evidence does not complete Phase 2D. The four-Provider/four-Harness Team,
remaining account-local failure cells, explicit fallback, complete accounting
and governance UI, full Capsule disclosure/encryption matrix, and unlocked
installed visual walk-through remain open.
