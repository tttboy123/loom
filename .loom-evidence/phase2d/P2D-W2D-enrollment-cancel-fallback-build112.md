# Phase 2D build 112 - Enrollment cancellation and explicit fallback

Status: `CURRENT / VERIFIED SOURCE + PARTIAL INSTALLED LIVE`

Date: 2026-08-22

## Installed boundary

- App: `/Users/lune/Applications/Loom.app`
- Version: `0.5.3 (112)`
- The bundle and nested daemon pass strict ad-hoc signature verification.
- The App cold-started the canonical daemon child with the existing Vault,
  Provider Accounts, Runtime directory and private MCP stdio helper intact.

## Fixed behavior

An MCP Enrollment revoked during an active OpenCode ToolCall previously wrote
`ToolExecutionFailed` but left the Harness process and Attempt Loop running.
The approved fallback could therefore never dispatch.

Build 112 preserves typed `remote_tool_binding_revoked` and
`remote_tool_binding_policy_drift` outcomes through the execution adapter. The
bridge cancels only the exact Attempt carrying that frozen Enrollment. The
Attempt Loop uses its uncancelled authority context to commit `StepEnded` and
`TurnEnded`, allowing Team recovery to schedule the next approved binding.

Focused source verification covers the exact cancellation cause, the failed
ToolExecution fact, failed Step/Turn terminal facts, normal remote-result
commit behavior and Enrollment revoke/policy-drift fail-closed behavior.

## Installed live result

The installed App executed a real OpenCode MCP stdio call, revoked the exact
Enrollment while the call was blocked, and produced this content-free chain:

1. source Attempt 1: `runtime_process_failed` after
   `remote_tool_binding_revoked`;
2. `TeamNodeRecoveryRecorded`: action `fallback`, consumed `true`, approval
   version `1`;
3. approved Attempt 2 scheduled with the frozen fallback binding;
4. no MCP result body was committed after revocation.

This closes the prior process-hang defect and proves installed fallback
dispatch/audit. The available Loom Native DeepSeek and MiniMax fallback routes
both subsequently encountered `provider_http`; therefore successful installed
fallback acceptance and its accounting row remain open. The same-Runtime
OpenCode native fallback candidate was rejected by preflight with
`invalid_request`, and no executable Codex/Claude subagent fallback Profile is
present in the current installed role directory. Those constraints were not
bypassed or inferred as success.

## Verification

- `go test ./... -count=1`: pass.
- `go vet ./...`: pass.
- macOS XCTest: 309 pass, one intentional visual-export skip; 16 strict Swift
  wire contracts pass.
- strict candidate signature, install dry run, transactional install and cold
  process identity for build 112: pass.
- installed MCP revoke to approved Attempt 2 scheduling: pass.
- installed fallback terminal success/accounting: open (`provider_http`).
- post-install real OpenCode conversation: exact `E2E-OK`; Vault `unlocked`,
  25 Providers, 7 Runtimes and 4 Conversation Profiles.

## Account-free Vault composition gate

`TestLocalProductChatVaultRestartPreservesEncryptedThreadAndRetrievableCapsule`
now composes one real `VaultStore` as both the encrypted chat document store
and Context Capsule store. It sends a two-Segment structured conversation,
persists a budget-omitted retrievable artifact, closes/reopens the Vault and
reconstructs the encrypted chat API. Thread/Capsule readback and exact
authority retrieval pass; forged role and Agent retrieval fail closed. The
test scans the SQLite database and sidecar files and finds neither transcript
nor Capsule sentinel in plaintext. It uses no Provider, network or credential.

## Installed encrypted Conversation restart gate

The installed build 112 created one fixed-ID Conversation with four immutable
DeepSeek/OpenCode route Segments covering `start_clean`,
`continue_with_context`, `summary_only` and a second `start_clean` transition.
Every Segment had a distinct disclosure receipt, a non-empty Context Capsule
digest and an explicit disclosed-context count; the budgeted transitions also
recorded omissions.

The App and its managed daemon then exited together. New App and daemon
processes cold-started from the installed bundle, and a verify-only UDS read of
the same Conversation recovered all four Segment/Profile/context-mode pairs,
Capsule digests, disclosure receipts and omission counts in 0.23 seconds. The
verify-only phase issued no Provider request. This closes installed restart
recovery for encrypted Conversation/Capsule metadata; the in-App scoped UI for
retrieving omitted content remains open.

No credential, Prompt, Provider response, MCP arguments/result body, nonce or
ciphertext is present in this evidence.
