# P2D-W2C Journal-backed Attempt Payload Delivery v1

**Date**: 2026-08-13  
**Status**: `CURRENT / SOURCE VERIFIED / PARTIAL`  
**Goal**: unified Phase 2D only  
**Source boundary**: post-build-64; installed Loom remains v0.5.2 build 39

## Result

Loom now persists one bounded Context retrieval result before exposing it to a
Provider or Harness and records content-free delivery authority in the Event
Journal. `internal/attemptpayload` defines the shared frozen Scope, Binding,
encrypted payload Store, Journal fact authority, and two accepted consumption
proofs:

- `provider_continuation` for a valid second Provider response after the exact
  tool result was supplied;
- `harness_final_output` for a validated Harness final result after the exact
  tool result was returned through MCP or Pi's extension.

The daemon composes one `DeliveryCoordinator` from the frozen Role Context
Capsule Authority, WorkItem, Run, claim ID/generation, Runtime, Agent,
execution-binding digest, Incident ID, Vault Store, scoped Retriever, and Work
Authority. Missing or half-configured stores fail closed.

## Ordering and recovery

Prepare follows this order:

1. derive a Loom-owned semantic call ID independent of Provider-native call ID;
2. read the Journal fact and exact pending Vault scope;
3. reuse an existing pending encrypted result without calling the Retriever;
4. otherwise retrieve and encode once, write encrypted `pending`, then append
   `ToolResultAccepted`;
5. return only the mutable payload copy to the selected transport.

Acknowledge appends `ToolResultDelivered` before re-encrypting the Vault row as
`delivered`. A restart between those writes repairs the Vault transition from
the Journal fact. A restart after `pending` but before acceptance appends only
the missing accepted fact. Binding, generation, Capsule, call, sequence,
content digest, or proof drift fails closed.

The Journal stream contains only IDs, frozen non-secret identity, content
digest, status, proof, causation, and Incident ID. Payload content remains in
the Conversation-DEK encrypted Vault and is zeroized after use.

## Transport proofs

- Loom Native DeepSeek/Kimi/MiniMax: the first Provider tool call prepares the
  result; only a valid non-tool second response acknowledges
  `provider_continuation`.
- Codex and Claude Code: MCP `tools/call` only returns the pending result. HTTP
  write success does not acknowledge it. A validated Harness final output
  acknowledges `harness_final_output`.
- Pi: UDS response and `tool_execution_end` remain pending. Only the complete
  second-turn final assistant, `agent_end`, `agent_settled`, clean process exit,
  and accounting validation acknowledge `harness_final_output`.

Provider-native `tool_call_id` is intentionally not the persistence identity.
A retry may receive a different native ID while returning the same persisted
Loom result to that native call.

## Verification

- A real LocalKeyFile Vault + SQLite Journal + Loom Native integration test
  closes the Vault after a second-round timeout, reopens it, receives a changed
  Provider tool-call ID, reuses the encrypted result without another retrieval,
  and reaches exactly one accepted plus one delivered Journal fact.
- Vault and Journal file scans do not contain the retrieved plaintext.
- MCP and Pi tests prove HTTP/UDS result writes leave the payload pending and
  only validated final output acknowledges delivery.
- Final authority review proves a nil Context returns a stable authority error
  instead of panicking, and a second semantic call/sequence in this bounded
  single-read slice conflicts before retrieval or another Vault payload write.
- Complete affected package tests passed for Attempt Payload, Context Capsule,
  Vault, Work Authority, Supervisor, Loom Native, Harness adapters, Pi, and
  daemon composition.
- The affected race command passed; `go vet ./...`, `go mod verify`, and
  `git diff --check` passed.
- The exact local Codex 0.144.1 and Claude Code 2.1.196 binaries completed their
  real CLI MCP contracts against loopback fake Providers after this change.
- The exact locked Pi 0.82.1 component completed its real local two-round
  contract and asserted one prepare, one `harness_final_output` acknowledgement,
  and a delivered payload.
- A broad parallel `go test -json ./internal/...` retained one unrelated
  process-cleanup failure in
  `TestSystemHarnessCommandRunnerKillsTimedOutProcessGroup`: the child PID file
  was empty under load. The Harness package passed in isolation. Long aggregate
  runners are also terminated by existing process-group cleanup tests, so this
  evidence does not claim a clean one-command full-repository test run.

## Honest boundary

This source slice provides at-least-once result delivery with no side-effecting
tool re-execution. It does not claim general exactly-once execution. A crash
after transport write but before a strong consumption proof may redeliver the
same encrypted result. General Web/MCP/local tool execution, multiple calls,
approval pause/resume, terminal-run reconciliation, sibling/Aggregation
Attempts, encrypted export, installed CV6, and the live mixed-Team matrix remain
open.

No bundle was built, launched, or installed. No real credential, Prompt, or
Provider was accessed. Phase 2D remains the sole `ACTIVE / PARTIAL` Goal.
