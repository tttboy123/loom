# P2D-W2D Encrypted Tool Proposal Approval Inspection v1

Status: `SOURCE VERIFIED / PI ASK PATH INTEGRATED / GENERAL ATTEMPT TOOL GATEWAY OPEN`

Date: 2026-08-14

## User outcome

A pending local ToolCall approval can show the exact command or path without
placing those arguments in the Event Journal. Loom stores the detail under the
Conversation DEK in Credential Vault and retrieves it only with the exact
approval ID, approval digest, WorkItem, and continuation/call digest.

If the encrypted detail is missing, damaged, stale, or belongs to a different
Attempt, the approval remains visible as `details unavailable` and cannot be
allowed. Reject remains available. This prevents blind approval while keeping
failure local to one request.

## Encrypted boundary

`internal/toolproposal` defines a mutable-byte record and Store contract. Its
binding freezes:

- Conversation, WorkItem, Run, and claim generation;
- Runtime and Agent instance;
- Execution Binding and Context Capsule digests;
- call ID and canonical call digest;
- approval ID and approval digest;
- tool, operation, and Incident identity;
- plaintext content digest.

`internal/credentials/vault` stores only these non-secret binding fields plus
AEAD ciphertext, nonce, and AAD digest. The strict AAD covers every binding
field and cipher/schema version. Data encryption reuses the per-Conversation
DEK and global Conversation nonce registry. Deleting the Conversation key
cascades and crypto-erases the proposal detail. Vault key rotation rewraps the
Conversation DEK without exposing or rewriting plaintext.

## Product wiring

- Pi ToolCall binding now carries the exact post-`RunStarted` Conversation,
  Run, generation, Runtime, Agent, Execution Binding, Capsule, Claim, and
  Incident identity.
- the existing Pi local execution Hook writes the canonical `ProposedCall`
  bytes to Vault only after an ask decision creates an exact approval.
- permission attention derives the call digest from the authoritative
  `ApprovalRequested.continuation_digest`, not a Job-level last-command guess.
- the App projection performs an exact Vault lookup, strictly decodes the call,
  recomputes its canonical digest, and then exposes bounded detail.
- the TUI and permission command boundary reject Allow when the configured
  Vault cannot produce authenticated detail. Swift renders an explicit locked
  unavailable state and sanitizes displayed command/path text.

## Privacy and isolation

- command/path bytes do not enter Journal, Evidence, diagnostics, argv, or
  global environment;
- copied `vault.db` bytes do not contain the proposal marker in plaintext;
- generation, Agent, Execution Binding, Capsule, approval, or call substitution
  fails closed;
- AAD/database tampering fails authentication before detail disclosure;
- one corrupt or missing proposal does not stop other approvals or Agents;
- returned mutable bytes are cleared by `Record.Close` after projection.

## Verification

Passed:

```text
go test -race ./internal/credentials/vault ./internal/app ./cmd/loomd -run 'TestToolProposalStore|TestP2DPermissionAttention|TestP2DWBridge|TestP2DApproval' -count=10
go test ./internal/credentials/vault ./internal/app ./internal/tui ./internal/runtime/piadapter ./cmd/loomd -count=1
swift test  # 202 XCTest, 1 intentional skip, 10 Swift Testing contracts
```

The Vault suite covers plaintext-negative disk checks, restart, exact lookup,
binding substitution, AAD tamper, idempotency, concurrent conflict, rotation,
and deletion. The bridge test also proves the same command marker is absent
from every Journal payload.

No App bundle was built, signed, launched, or installed. No real credential,
Provider, tool side effect, or user workspace was accessed. Source remains
post-build-64; installed Loom remains v0.5.2 build 39.

## Open gates

This slice hardens the existing Pi ask path; it does not claim that the legacy
Hook is inside the generalized Attempt Tool Gateway. Still required:

- admit/dispatch the local call through the strict Attempt/Turn/Step authority
  before side effects;
- bind approval consumption to that exact ToolCall/Attempt identity;
- add sandbox enforcement reports, tool Incident stages, and interrupted-side-
  effect recovery;
- provide actionable approve/reject controls in the native governance panel;
- extend the same encrypted inspection contract to Codex, Claude Code, Loom
  Native, Web, and MCP adapters;
- pass installed CV6 and mixed-Team live acceptance.
