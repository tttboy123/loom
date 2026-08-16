# P2D-W2C/W2D Attempt Payload Terminal Reconciliation v1

**Date**: 2026-08-13  
**Status**: `CURRENT / SOURCE VERIFIED / PARTIAL`  
**Goal**: unified Phase 2D only  
**Source boundary**: post-build-64; installed Loom remains v0.5.2 build 39

## Result

The execution runtime now repairs the crash window where a strong consumption
proof committed `ToolResultDelivered` to the Event Journal but the encrypted
Attempt Payload row remained `pending`. Startup reconstructs the authority from
the dedicated fact stream, strictly replays accepted/delivered causation, and
revalidates the current exact Run even when its phase is terminal.

The repair freezes Conversation, WorkItem, Run, claim ID/generation, Runtime,
Agent, execution-binding digest, Capsule digest, semantic call, sequence,
content type/digest, Incident ID and delivery proof. It never infers delivery
from HTTP/UDS write success and never reruns the Retriever or a tool.

## Failure isolation and diagnostics

- A `pending` row with delivered authority is authenticated and re-encrypted as
  `delivered` under a fresh Conversation nonce.
- An already-delivered row is an idempotent no-op.
- A missing row is accepted only as Conversation crypto-erasure.
- One unreadable or uncommittable Vault row returns an Attempt-local `blocked`
  outcome while other Agent payloads continue to reconcile.
- Repaired and blocked outcomes emit stage `context_delivery_reconcile` with
  Incident ID and non-secret Provider Account, Model, WorkItem, Run, generation,
  Runtime, Agent, binding and Capsule identity.
- Diagnostics never include payload content, Prompt, credential, nonce,
  ciphertext, Authorization Header or Provider response.

## Verification

- Work Authority tests prove terminal Run repair, idempotent replay, stable nil
  Context errors, and isolation where one corrupt row does not prevent an
  independent Agent payload from reaching `delivered`.
- A real LocalKeyFile Vault + SQLite Journal integration test prepares the
  encrypted payload, commits the strong delivered fact without the Vault state
  transition, terminalizes the Run, closes and reopens the Vault, then repairs
  and authenticates the same row.
- Vault and Journal file scans do not contain the Context result plaintext.
- Operational diagnostic tests prove the new stage carries exact non-secret
  Attempt identity and rejects unsafe record shapes.
- Complete affected normal and race package matrices passed for Vault, Work,
  Context Capsule, Loom Native, Harness adapters, Pi, Supervisor and daemon.

## Honest boundary

This closes terminal reconciliation for the bounded, side-effect-free Context
result slice. It does not expire accepted-but-unproved payloads after terminal
failure, implement general side-effecting tool recovery, claim exactly-once
execution, provide a multi-call state machine, or close TTL/compaction,
Aggregation, encrypted export, installed CV6 or live mixed-Team acceptance.

No bundle was built, launched or installed. No real credential, Prompt or
Provider was accessed. Phase 2D remains the sole `ACTIVE / PARTIAL` Goal.
